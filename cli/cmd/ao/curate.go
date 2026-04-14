package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

// Type aliases for backwards compatibility within cmd/ao tests.
type curateArtifact = lifecycle.CurateArtifact
type curateVerifyResult = lifecycle.CurateVerifyResult
type curateStatusResult = lifecycle.CurateStatusResult

var validArtifactTypes = lifecycle.ValidArtifactTypes

func curateParseFrontmatter(data string) (map[string]any, string) {
	return lifecycle.ParseFrontmatter(data)
}

func curateFrontmatterString(fm map[string]any, key string) string {
	return lifecycle.FrontmatterString(fm, key)
}

func resolveCurateGoalsFile() (string, error) {
	return lifecycle.ResolveCurateGoalsFile()
}

func generateArtifactID(artifactType, date, content string) string {
	return lifecycle.GenerateArtifactID(artifactType, date, content)
}

func curateArtifactDir(artifactType string) string {
	return lifecycle.ArtifactDir(artifactType)
}

func countArtifactsInDir(dir string) (map[string]int, time.Time) {
	return lifecycle.CountArtifactsInDir(dir)
}

func countArtifactsSince(learningsDir, patternsDir string, since time.Time) int {
	return lifecycle.CountArtifactsSince(learningsDir, patternsDir, since)
}

// curateOutWriter returns cmd.OutOrStdout() when cmd is non-nil, otherwise os.Stdout.
func curateOutWriter(cmd *cobra.Command) io.Writer {
	if cmd != nil {
		return cmd.OutOrStdout()
	}
	return os.Stdout
}

var curateVerifySince string

var curateCmd = &cobra.Command{
	Use:   "curate",
	Short: "Curation pipeline operations",
	Long: `Curate manages the knowledge curation pipeline: catalog artifacts,
verify gate health, and view status.

Commands:
  catalog <path>   Catalog a knowledge artifact
  verify           Verify gate health against baselines
  status           Show curation pipeline status`,
}

var curateCatalogCmd = &cobra.Command{
	Use:   "catalog <path>",
	Short: "Catalog a knowledge artifact",
	Args:  cobra.ExactArgs(1),
	RunE:  runCurateCatalog,
}

var curateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify gate health against baselines",
	RunE:  runCurateVerify,
}

var curateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show curation pipeline status",
	RunE:  runCurateStatus,
}

func init() {
	curateCmd.GroupID = "core"
	rootCmd.AddCommand(curateCmd)

	curateCmd.AddCommand(curateCatalogCmd)
	curateCmd.AddCommand(curateVerifyCmd)
	curateCmd.AddCommand(curateStatusCmd)

	curateVerifyCmd.Flags().StringVar(&curateVerifySince, "since", "", "Filter to changes within duration (e.g. 24h, 7d)")
}

func runCurateCatalog(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading artifact: %w", err)
	}

	fm, body := curateParseFrontmatter(string(data))

	// Validate required fields
	artifactType := curateFrontmatterString(fm, "type")
	if artifactType == "" {
		return fmt.Errorf("artifact missing required 'type' field in frontmatter")
	}

	if !validArtifactTypes[artifactType] {
		return fmt.Errorf("unknown artifact type %q: must be one of learning, decision, failure, pattern", artifactType)
	}

	content := body
	if content == "" {
		content = curateFrontmatterString(fm, "content")
	}
	if content == "" {
		return fmt.Errorf("artifact has no content (empty body and no 'content' frontmatter field)")
	}

	date := curateFrontmatterString(fm, "date")
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	// Assign ID if missing
	id := curateFrontmatterString(fm, "id")
	if id == "" {
		id = generateArtifactID(artifactType, date, content)
	}

	now := time.Now().UTC()

	artifact := curateArtifact{
		ID:            id,
		Type:          artifactType,
		Content:       content,
		Date:          date,
		SchemaVersion: 1,
		CuratedAt:     now.Format(time.RFC3339),
	}

	// Write to target directory
	targetDir := curateArtifactDir(artifactType)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return fmt.Errorf("creating artifact dir: %w", err)
	}

	filename := fmt.Sprintf("%s.json", id)
	targetPath := filepath.Join(targetDir, filename)
	artifact.Path = targetPath

	artifactData, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling artifact: %w", err)
	}

	if err := os.WriteFile(targetPath, artifactData, 0o600); err != nil {
		return fmt.Errorf("writing artifact: %w", err)
	}

	// Output
	if GetOutput() == "json" {
		enc := json.NewEncoder(curateOutWriter(cmd))
		enc.SetIndent("", "  ")
		return enc.Encode(artifact)
	}

	fmt.Printf("Cataloged %s artifact: %s\n", artifactType, id)
	fmt.Printf("  Path: %s\n", targetPath)
	fmt.Printf("  Date: %s\n", date)
	return nil
}

func runCurateVerify(cmd *cobra.Command, args []string) error {
	result := curateVerifyResult{
		Regressions: []string{},
	}

	// Load goals and measure
	goalsPath, resolveErr := resolveCurateGoalsFile()
	if resolveErr != nil {
		// If no goals file, report zero gates
		if GetOutput() == "json" {
			result.Verified = true
			enc := json.NewEncoder(curateOutWriter(cmd))
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		fmt.Println("No GOALS file found — nothing to verify")
		return nil
	}

	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		if GetOutput() == "json" {
			result.Verified = false
			enc := json.NewEncoder(curateOutWriter(cmd))
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		fmt.Printf("Could not load goals from %s — nothing to verify\n", goalsPath)
		return nil
	}

	timeout := 120 * time.Second
	snap := goals.Measure(gf, timeout)

	// Count pass/fail
	for _, m := range snap.Goals {
		switch m.Result {
		case "pass":
			result.GatesPassed++
		case "fail":
			result.GatesFailed++
		}
	}

	// Check for uncommitted changes
	gitCmd := exec.Command("git", "status", "--porcelain")
	gitOut, gitErr := gitCmd.Output()
	if gitErr == nil && len(strings.TrimSpace(string(gitOut))) > 0 {
		VerbosePrintf("Uncommitted changes detected\n")
	}

	// Load baseline and compare for regressions
	baselineDir := ".agents/ao/baselines"
	regressions, regErr := detectVerifyRegressions(baselineDir, snap)
	if regErr != nil {
		return regErr
	}
	result.Regressions = append(result.Regressions, regressions...)

	result.Verified = len(result.Regressions) == 0 && result.GatesFailed == 0

	// Save current snapshot as new baseline
	if _, saveErr := goals.SaveSnapshot(snap, baselineDir); saveErr != nil {
		VerbosePrintf("Warning: could not save baseline: %v\n", saveErr)
	}

	// Output
	if GetOutput() == "json" {
		enc := json.NewEncoder(curateOutWriter(cmd))
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Verified {
		fmt.Printf("VERIFIED: %d gates passed, 0 regressions\n", result.GatesPassed)
	} else {
		fmt.Printf("NOT VERIFIED: %d passed, %d failed\n", result.GatesPassed, result.GatesFailed)
		if len(result.Regressions) > 0 {
			fmt.Printf("  Regressions: %s\n", strings.Join(result.Regressions, ", "))
		}
	}
	return nil
}

// detectVerifyRegressions loads the latest baseline and compares against a snapshot,
// respecting the --since filter. Returns regressed goal IDs.
func detectVerifyRegressions(baselineDir string, snap *goals.Snapshot) ([]string, error) {
	baseline, baseErr := goals.LoadLatestSnapshot(baselineDir)
	if baseErr != nil {
		return nil, nil
	}

	applyFilter := true
	if curateVerifySince != "" {
		dur, parseErr := parseDuration(curateVerifySince)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid --since value %q: %w", curateVerifySince, parseErr)
		}
		if dur < 0 {
			return nil, fmt.Errorf("--since value must be positive: %q", curateVerifySince)
		}
		cutoff := time.Now().Add(-dur)
		if ts, tsErr := time.Parse(time.RFC3339, baseline.Timestamp); tsErr == nil {
			if ts.Before(cutoff) {
				applyFilter = false
			}
		} else if ts, tsErr := time.Parse("2006-01-02T15:04:05.000", baseline.Timestamp); tsErr == nil {
			if ts.Before(cutoff) {
				applyFilter = false
			}
		}
	}

	if !applyFilter {
		return nil, nil
	}

	var regressions []string
	drifts := goals.ComputeDrift(baseline, snap)
	for _, d := range drifts {
		if d.Delta == "regressed" {
			regressions = append(regressions, d.GoalID)
		}
	}
	return regressions, nil
}

func runCurateStatus(cmd *cobra.Command, args []string) error {
	return emitCurateStatus(cmd, curateStatusSnapshot())
}

func curateStatusSnapshot() curateStatusResult {
	learningsDir := ".agents/learnings"
	patternsDir := ".agents/patterns"

	result := curateStatusResult{}
	curateStatusCounts(&result, learningsDir, patternsDir)
	result.LastVerifyAt = curateStatusLastVerifyAt(".agents/ao/baselines")
	result.PendingVerify = curateStatusPendingVerify(result.Total, result.LastVerifyAt, result.LastCatalogAt, learningsDir, patternsDir)
	return result
}

func curateStatusCounts(result *curateStatusResult, learningsDir, patternsDir string) {
	var latestCatalog time.Time

	learningsCounts, learningsLatest := countArtifactsInDir(learningsDir)
	result.Learnings = learningsCounts["learning"]
	result.Decisions = learningsCounts["decision"]
	result.Failures = learningsCounts["failure"]
	if learningsLatest.After(latestCatalog) {
		latestCatalog = learningsLatest
	}

	patternsCounts, patternsLatest := countArtifactsInDir(patternsDir)
	result.Patterns = patternsCounts["pattern"]
	if patternsLatest.After(latestCatalog) {
		latestCatalog = patternsLatest
	}

	result.Total = result.Learnings + result.Decisions + result.Failures + result.Patterns
	if !latestCatalog.IsZero() {
		result.LastCatalogAt = latestCatalog.Format(time.RFC3339)
	}
}

func curateStatusLastVerifyAt(baselineDir string) string {
	entries, err := os.ReadDir(baselineDir)
	if err != nil {
		return ""
	}

	var latest time.Time
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			info, infoErr := e.Info()
			if infoErr == nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.Format(time.RFC3339)
}

func curateStatusPendingVerify(total int, lastVerifyAt, lastCatalogAt string, learningsDir, patternsDir string) int {
	if lastVerifyAt != "" && lastCatalogAt != "" {
		verifyTime, _ := time.Parse(time.RFC3339, lastVerifyAt)
		catalogTime, _ := time.Parse(time.RFC3339, lastCatalogAt)
		if catalogTime.After(verifyTime) {
			return countArtifactsSince(learningsDir, patternsDir, verifyTime)
		}
		return 0
	}

	if total > 0 && lastVerifyAt == "" {
		return total
	}

	return 0
}

func emitCurateStatus(cmd *cobra.Command, result curateStatusResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(curateOutWriter(cmd))
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println("Curation Pipeline Status")
	fmt.Println("========================")
	fmt.Printf("  Learnings:  %d\n", result.Learnings)
	fmt.Printf("  Decisions:  %d\n", result.Decisions)
	fmt.Printf("  Failures:   %d\n", result.Failures)
	fmt.Printf("  Patterns:   %d\n", result.Patterns)
	fmt.Printf("  Total:      %d\n", result.Total)
	fmt.Println()
	if result.LastCatalogAt != "" {
		fmt.Printf("  Last catalog: %s\n", result.LastCatalogAt)
	}
	if result.LastVerifyAt != "" {
		fmt.Printf("  Last verify:  %s\n", result.LastVerifyAt)
	}
	if result.PendingVerify > 0 {
		fmt.Printf("  Pending:      %d artifact(s) not yet verified\n", result.PendingVerify)
	}

	return nil
}

// parseDuration is defined in vibe_check.go — reuse it for --since flag parsing.
