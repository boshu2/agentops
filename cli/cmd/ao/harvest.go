package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/harvest"
	"github.com/boshu2/agentops/cli/internal/overnight"
	"github.com/spf13/cobra"
)

var (
	harvestRootsFlag     string
	harvestOutputDir     string
	harvestPromoteTo     string
	harvestMinConfidence float64
	harvestInclude       string
	harvestQuiet         bool
	harvestMaxFileSize   int64
)

var harvestCmd = &cobra.Command{
	Use:   "harvest",
	Short: "Sweep all rigs, extract and deduplicate cross-rig knowledge",
	Long: `Walks all .agents/ directories across the workspace, extracts learnings,
patterns, and research, deduplicates across rigs, and promotes high-value
items to the global knowledge hub (~/.agents/learnings/).`,
	RunE: runHarvest,
}

func init() {
	harvestCmd.GroupID = "knowledge"
	rootCmd.AddCommand(harvestCmd)

	harvestCmd.Flags().StringVar(&harvestRootsFlag, "roots", "",
		"Base directories to scan (comma-separated) (default ~/gt)")
	harvestCmd.Flags().StringVar(&harvestOutputDir, "output-dir", ".agents/harvest",
		"Directory for harvest catalog output")
	harvestCmd.Flags().StringVar(&harvestPromoteTo, "promote-to", "",
		"Promotion destination for high-value artifacts (default ~/.agents/learnings)")
	harvestCmd.Flags().Float64Var(&harvestMinConfidence, "min-confidence", 0.5,
		"Minimum confidence for promotion")
	harvestCmd.Flags().StringVar(&harvestInclude, "include", "learnings,patterns,research",
		"Artifact types to include (comma-separated)")
	harvestCmd.Flags().BoolVar(&harvestQuiet, "quiet", false, "Suppress progress output")
	harvestCmd.Flags().Int64Var(&harvestMaxFileSize, "max-file-size", 1048576,
		"Skip files larger than this (bytes)")
}

// failIfDreamHoldsLock refuses to proceed when a live Dream run holds
// the overnight lock. Dream and ao harvest both write to
// ~/.agents/learnings/ via harvest.Promote; concurrent writes there
// are outside Dream's checkpoint boundary and would silently corrupt
// the global hub.
//
// Strategy:
//   - Look for .agents/overnight/run.lock at the repo root.
//   - If missing or if overnight.LockIsStale returns true, proceed.
//   - Otherwise, read the PID from the lock file; if that PID is
//     still alive (via overnight.ProcessAlive), refuse with a clear
//     error pointing the operator at `ao overnight status`.
//
// Errors reading the lock file (other than ENOENT) are logged as a
// warning but do not block harvest — the worst case of racing Dream
// is strictly better than hard-failing harvest on a corrupt lock
// file.
//
// This is the pm-011 fix from the Dream nightly compounder
// pre-mortem.
func failIfDreamHoldsLock(cwd string) error {
	lockPath := filepath.Join(cwd, ".agents", "overnight", "run.lock")

	// Cheap freshness check first: if the lock is stale (old mtime
	// AND dead/zero PID), LockIsStale returns true and we can proceed
	// without any further work.
	stale, err := overnight.LockIsStale(lockPath, 12*time.Hour)
	if err != nil {
		// Stat failed for a reason other than ENOENT (ENOENT is
		// reported as stale=false, err=nil). Log and proceed —
		// harvest must not hard-fail on a lock-file read error.
		fmt.Fprintf(os.Stderr, "harvest: warning: could not stat overnight lock %s: %v\n", lockPath, err)
		return nil
	}
	if stale {
		return nil
	}

	// Not stale means one of:
	//   1. lock file does not exist         -> proceed
	//   2. lock mtime is within maxAge      -> check PID
	//   3. lock references a live PID       -> refuse
	// LockIsStale collapses (1) and (2) into the same "not stale"
	// return, so distinguish them with an explicit stat.
	if _, statErr := os.Stat(lockPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		fmt.Fprintf(os.Stderr, "harvest: warning: could not stat overnight lock %s: %v\n", lockPath, statErr)
		return nil
	}

	pid := overnight.ReadLockPID(lockPath)
	if pid <= 0 {
		// Malformed or empty lock file — treat as no lock, don't
		// block harvest. Dream's own startup path will clean this up
		// via LockIsStale at cleanup time.
		return nil
	}
	if !overnight.ProcessAlive(pid) {
		// Lock owner is dead; safe to proceed.
		return nil
	}

	return fmt.Errorf("ao harvest: refusing to run while Dream holds the overnight lock (pid %d). Wait for the Dream run to finish, or check `ao overnight status`", pid)
}

func runHarvest(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	if err := failIfDreamHoldsLock(cwd); err != nil {
		return err
	}

	// Resolve home-relative defaults at runtime so generated docs don't embed absolute paths.
	applyHarvestRuntimeDefaults()
	roots := harvestCSVList(harvestRootsFlag)
	includeDirs := harvestCSVList(harvestInclude)
	opts := newHarvestWalkOptions(roots, includeDirs)

	rigs, discoveryWarnings, err := harvest.DiscoverRigsWithWarnings(opts)
	if err != nil {
		return fmt.Errorf("discovering rigs: %w", err)
	}

	if !harvestQuiet {
		fmt.Printf("Discovered %d rigs\n", len(rigs))
	}

	allArtifacts, warnings, totalCandidateFiles := collectHarvestArtifacts(rigs, opts, discoveryWarnings)
	catalog := newHarvestCatalog(allArtifacts, rigs, warnings, roots, includeDirs, totalCandidateFiles)

	if !harvestQuiet {
		fmt.Printf("Extracted %d artifacts (%d unique, %d duplicate excess, %d promotion candidates, %d warnings)\n",
			catalog.Summary.ArtifactsExtracted,
			catalog.Summary.UniqueArtifacts,
			catalog.Summary.DuplicateExcess,
			catalog.Summary.PromotionCandidates,
			catalog.Summary.WarningCount,
		)
	}

	promoted, err := promoteHarvestCatalog(catalog)
	if err != nil {
		return err
	}
	catalog.PromotionCount = promoted

	return outputHarvestCatalog(catalog, harvestOutputDir, promoted)
}

func applyHarvestRuntimeDefaults() {
	if harvestRootsFlag == "" {
		home, _ := os.UserHomeDir()
		defaultRoots := []string{filepath.Join(home, "gt")}
		// Include Claude project directories if they exist
		claudeProjects := filepath.Join(home, ".claude", "projects")
		if info, err := os.Stat(claudeProjects); err == nil && info.IsDir() {
			defaultRoots = append(defaultRoots, claudeProjects)
		}
		harvestRootsFlag = strings.Join(defaultRoots, ",")
	}
	if harvestPromoteTo == "" {
		home, _ := os.UserHomeDir()
		harvestPromoteTo = filepath.Join(home, ".agents", "learnings")
	}
}

func harvestCSVList(value string) []string {
	items := strings.Split(value, ",")
	for i := range items {
		items[i] = strings.TrimSpace(items[i])
	}
	return items
}

func newHarvestWalkOptions(roots, includeDirs []string) harvest.WalkOptions {
	return harvest.WalkOptions{
		Roots:       roots,
		MaxFileSize: harvestMaxFileSize,
		SkipDirs:    harvest.DefaultWalkOptions().SkipDirs,
		IncludeDirs: includeDirs,
	}
}

func collectHarvestArtifacts(
	rigs []harvest.RigInfo,
	opts harvest.WalkOptions,
	discoveryWarnings []harvest.HarvestWarning,
) ([]harvest.Artifact, []harvest.HarvestWarning, int) {
	var allArtifacts []harvest.Artifact
	warnings := append([]harvest.HarvestWarning{}, discoveryWarnings...)
	for _, warning := range discoveryWarnings {
		printHarvestWarning(warning)
	}

	totalCandidateFiles := 0
	for _, rig := range rigs {
		result := harvest.ExtractArtifactsWithStats(rig, opts)
		totalCandidateFiles += result.CandidateFiles
		for _, warning := range result.Warnings {
			printHarvestWarning(warning)
		}
		warnings = append(warnings, result.Warnings...)
		allArtifacts = append(allArtifacts, result.Artifacts...)
	}
	return allArtifacts, warnings, totalCandidateFiles
}

func newHarvestCatalog(
	allArtifacts []harvest.Artifact,
	rigs []harvest.RigInfo,
	warnings []harvest.HarvestWarning,
	roots []string,
	includeDirs []string,
	totalCandidateFiles int,
) *harvest.Catalog {
	catalog := harvest.BuildCatalog(allArtifacts, harvestMinConfidence)
	catalog.Roots = append([]string{}, roots...)
	catalog.IncludeDirs = append([]string{}, includeDirs...)
	catalog.PromoteTo = harvestPromoteTo
	catalog.MinConfidence = harvestMinConfidence
	catalog.DryRun = GetDryRun()
	catalog.Rigs = append([]harvest.RigInfo{}, rigs...)
	catalog.Warnings = append([]harvest.HarvestWarning{}, warnings...)
	catalog.RigsScanned = len(rigs)
	catalog.TotalFiles = totalCandidateFiles
	catalog.Timestamp = time.Now().UTC()
	catalog.Summary.WarningCount = len(catalog.Warnings)
	return catalog
}

func promoteHarvestCatalog(catalog *harvest.Catalog) (int, error) {
	if GetDryRun() {
		return 0, nil
	}
	promoted, err := harvest.Promote(catalog, harvestPromoteTo, false)
	if err != nil {
		return 0, fmt.Errorf("promoting artifacts: %w", err)
	}
	return promoted, nil
}

func outputHarvestCatalog(catalog *harvest.Catalog, outputDir string, promoted int) error {
	if err := harvest.WriteCatalog(outputDir, catalog); err != nil {
		return fmt.Errorf("writing catalog: %w", err)
	}
	if GetOutput() == "json" {
		data, marshalErr := json.MarshalIndent(catalog, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("marshaling catalog: %w", marshalErr)
		}
		fmt.Println(string(data))
		return nil
	}
	if !harvestQuiet {
		fmt.Printf("Catalog written to %s\n", outputDir)
		if GetDryRun() {
			fmt.Println("Dry run: no artifacts promoted")
		} else {
			fmt.Printf("Promoted %d artifacts to %s\n", promoted, harvestPromoteTo)
		}
		printExclusionReport(catalog)
	}
	VerbosePrintf("Rigs scanned: %d, Total files: %d\n", catalog.RigsScanned, catalog.TotalFiles)
	return nil
}

// printExclusionReport surfaces the count of artifacts that cleared dedup
// but did NOT make the confidence threshold, plus the top-5 near-misses.
// This replaces silent drops with actionable visibility so the user can
// decide whether to lower --min-confidence.
func printExclusionReport(catalog *harvest.Catalog) {
	excluded := len(catalog.ExcludedCandidates)
	if excluded == 0 {
		return
	}
	fmt.Printf("Excluded %d candidate(s) under confidence threshold %.2f\n", excluded, catalog.MinConfidence)
	near := catalog.TopExcludedNearMiss(5)
	if len(near) == 0 {
		return
	}
	fmt.Println("  Top near-miss candidates (closest to threshold):")
	for _, a := range near {
		title := a.Title
		if title == "" {
			title = a.ID
		}
		fmt.Printf("    %.2f  %s  (%s)\n", a.Confidence, title, a.Type)
	}
}

func printHarvestWarning(warning harvest.HarvestWarning) {
	fmt.Fprintf(os.Stderr, "harvest: warning: %s: %s\n", warning.Stage, warning.Message)
}

// duplicateArtifactCount returns the total number of duplicate artifacts
// (excluding the kept winner from each group).
func duplicateArtifactCount(cat *harvest.Catalog) int {
	count := 0
	for _, dg := range cat.Duplicates {
		count += dg.Count - 1
	}
	return count
}
