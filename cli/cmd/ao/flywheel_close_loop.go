package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	flywheelCloseLoopPendingDir string
	flywheelCloseLoopThreshold  string
	flywheelCloseLoopQuiet      bool
)

type flywheelCloseLoopResult struct {
	Ingest      poolIngestResult             `json:"ingest"`
	AutoPromote poolAutoPromotePromoteResult `json:"auto_promote"`
	AntiPattern struct {
		Eligible int      `json:"eligible"`
		Promoted int      `json:"promoted"`
		Paths    []string `json:"paths,omitempty"`
	} `json:"anti_pattern"`
	Store struct {
		Categorize bool   `json:"categorize"`
		Indexed    int    `json:"indexed"`
		IndexPath  string `json:"index_path,omitempty"`
	} `json:"store"`
}

var flywheelCloseLoopCmd = &cobra.Command{
	Use:   "close-loop",
	Short: "Close the knowledge flywheel loop",
	Long: `Close the knowledge flywheel loop by chaining:

  pool ingest → pool auto-promote (promote) → promote-anti-patterns → store (categorize)

Designed to be safe for hooks with --quiet.

Examples:
  ao flywheel close-loop
  ao flywheel close-loop --threshold 24h --pending-dir .agents/knowledge/pending
  ao flywheel close-loop -o json
  ao flywheel close-loop --dry-run`,
	RunE: runFlywheelCloseLoop,
}

func init() {
	flywheelCmd.AddCommand(flywheelCloseLoopCmd)
	flywheelCloseLoopCmd.Flags().StringVar(&flywheelCloseLoopPendingDir, "pending-dir", filepath.Join(".agents", "knowledge", "pending"), "Pending directory to ingest from")
	flywheelCloseLoopCmd.Flags().StringVar(&flywheelCloseLoopThreshold, "threshold", defaultAutoPromoteThreshold, "Minimum age for auto-promotion (default: 24h)")
	flywheelCloseLoopCmd.Flags().BoolVar(&flywheelCloseLoopQuiet, "quiet", false, "Suppress non-essential output (hook-friendly)")
}

func runFlywheelCloseLoop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	threshold, _, err := resolveAutoPromoteThreshold(cmd, "threshold", flywheelCloseLoopThreshold)
	if err != nil {
		return err
	}

	result := flywheelCloseLoopResult{}

	// 1) pool ingest (pending markdown → pool candidates)
	ingestFiles, err := resolveIngestFiles(cwd, flywheelCloseLoopPendingDir, nil)
	if err != nil {
		return err
	}
	result.Ingest, err = ingestPendingFilesToPool(cwd, ingestFiles)
	if err != nil {
		return err
	}

	// 2) auto-promote + promote
	p := pool.NewPool(cwd)
	result.AutoPromote, err = autoPromoteAndPromoteToArtifacts(p, threshold, true)
	if err != nil {
		return err
	}

	// 3) promote anti-patterns
	antiEligible, antiPromoted, antiPaths, err := promoteAntiPatternsForCloseLoop(cwd)
	if err != nil {
		return err
	}
	result.AntiPattern.Eligible = antiEligible
	result.AntiPattern.Promoted = antiPromoted
	result.AntiPattern.Paths = antiPaths

	// 4) store index (categorize) for newly created/changed artifacts
	pathsToIndex := append([]string{}, result.AutoPromote.Artifacts...)
	pathsToIndex = append(pathsToIndex, antiPaths...)
	result.Store.Categorize = true
	indexed, indexPath, err := storeIndexUpsert(cwd, pathsToIndex, true)
	if err != nil {
		return err
	}
	result.Store.Indexed = indexed
	result.Store.IndexPath = indexPath

	return outputFlywheelCloseLoopResult(result)
}

func outputFlywheelCloseLoopResult(res flywheelCloseLoopResult) error {
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	default:
		if flywheelCloseLoopQuiet {
			return nil
		}
		fmt.Println()
		fmt.Println("Flywheel Close-Loop Summary")
		fmt.Println("===========================")
		fmt.Printf("Pool ingest: added=%d (files=%d, skipped_existing=%d, skipped_malformed=%d)\n",
			res.Ingest.Added, res.Ingest.FilesScanned, res.Ingest.SkippedExisting, res.Ingest.SkippedMalformed)
		fmt.Printf("Auto-promote: promoted=%d (threshold=%s)\n", res.AutoPromote.Promoted, res.AutoPromote.Threshold)
		fmt.Printf("Anti-patterns: promoted=%d (eligible=%d)\n", res.AntiPattern.Promoted, res.AntiPattern.Eligible)
		fmt.Printf("Store: indexed=%d (categorize=%v)\n", res.Store.Indexed, res.Store.Categorize)
		fmt.Println()
		return nil
	}
}

func ingestPendingFilesToPool(cwd string, files []string) (poolIngestResult, error) {
	p := pool.NewPool(cwd)
	res := poolIngestResult{FilesScanned: len(files)}
	if len(files) == 0 {
		return res, nil
	}

	for _, f := range files {
		data, rerr := os.ReadFile(f)
		if rerr != nil {
			res.Errors++
			continue
		}
		fileDate, sessionHint := parsePendingFileHeader(string(data), f)
		blocks := parseLearningBlocks(string(data))
		res.CandidatesFound += len(blocks)
		for _, b := range blocks {
			cand, scoring, ok := buildCandidateFromLearningBlock(b, f, fileDate, sessionHint)
			if !ok {
				res.SkippedMalformed++
				continue
			}
			if _, gerr := p.Get(cand.ID); gerr == nil {
				res.SkippedExisting++
				continue
			}
			if GetDryRun() {
				res.Added++
				res.AddedIDs = append(res.AddedIDs, cand.ID)
				continue
			}
			if err := p.AddAt(cand, scoring, cand.ExtractedAt); err != nil {
				res.Errors++
				continue
			}
			res.Added++
			res.AddedIDs = append(res.AddedIDs, cand.ID)
		}
	}

	return res, nil
}

func autoPromoteAndPromoteToArtifacts(p *pool.Pool, threshold time.Duration, includeGold bool) (poolAutoPromotePromoteResult, error) {
	entries, err := p.List(pool.ListOptions{
		Status: types.PoolStatusPending,
	})
	if err != nil {
		return poolAutoPromotePromoteResult{}, fmt.Errorf("list pending: %w", err)
	}

	result := poolAutoPromotePromoteResult{
		Threshold: threshold.String(),
	}
	citationCounts, promotedContent := loadPromotionGateContext(p.BaseDir)

	for _, e := range entries {
		if e.Candidate.Tier != types.TierSilver && !(includeGold && e.Candidate.Tier == types.TierGold) {
			continue
		}
		if e.ScoringResult.GateRequired {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			continue
		}
		if e.Age < threshold {
			continue
		}
		if reason := checkPromotionCriteria(p.BaseDir, e, threshold, citationCounts, promotedContent); reason != "" {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			VerbosePrintf("Skipping %s: %s\n", e.Candidate.ID, reason)
			continue
		}

		result.Considered++

		if GetDryRun() {
			result.Promoted++
			continue
		}

		if err := p.Stage(e.Candidate.ID, types.TierSilver); err != nil {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			continue
		}
		artifactPath, err := p.Promote(e.Candidate.ID)
		if err != nil {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, e.Candidate.ID)
			continue
		}
		result.Promoted++
		result.Artifacts = append(result.Artifacts, artifactPath)
		promotedContent[normalizeContent(e.Candidate.Content)] = true
	}

	return result, nil
}

func promoteAntiPatternsForCloseLoop(cwd string) (eligible int, promoted int, changedPaths []string, err error) {
	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		return 0, 0, nil, nil
	}

	results, err := ratchet.ScanForMaturityTransitions(learningsDir)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("scan transitions: %w", err)
	}

	var antiPatternPromotions []*ratchet.MaturityTransitionResult
	for _, r := range results {
		if r.NewMaturity == types.MaturityAntiPattern {
			antiPatternPromotions = append(antiPatternPromotions, r)
		}
	}
	eligible = len(antiPatternPromotions)
	if eligible == 0 {
		return 0, 0, nil, nil
	}

	if GetDryRun() {
		return eligible, 0, nil, nil
	}

	for _, r := range antiPatternPromotions {
		learningPath, ferr := findLearningFile(filepath.Dir(learningsDir), r.LearningID)
		if ferr != nil {
			continue
		}
		applyResult, aerr := ratchet.ApplyMaturityTransition(learningPath)
		if aerr != nil {
			continue
		}
		if applyResult.Transitioned && applyResult.NewMaturity == types.MaturityAntiPattern {
			promoted++
			changedPaths = append(changedPaths, learningPath)
		}
	}

	return eligible, promoted, changedPaths, nil
}

// storeIndexUpsert updates the store index for the provided paths, de-duplicating by path.
// It returns how many paths were (re)indexed and the index path.
func storeIndexUpsert(baseDir string, paths []string, categorize bool) (int, string, error) {
	indexPath := filepath.Join(baseDir, IndexDir, IndexFileName)
	if len(paths) == 0 {
		return 0, indexPath, nil
	}

	// Load existing entries (best-effort).
	existing := make(map[string]IndexEntry)
	f, err := os.Open(indexPath)
	if err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var e IndexEntry
			if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
				continue
			}
			if e.Path != "" {
				existing[e.Path] = e
			}
		}
		_ = f.Close() //nolint:errcheck // best-effort
	}

	// Upsert requested paths.
	indexed := 0
	for _, p := range paths {
		if p == "" {
			continue
		}
		// Only index paths that exist.
		if _, err := os.Stat(p); err != nil {
			continue
		}
		entry, err := createIndexEntry(p, categorize)
		if err != nil {
			continue
		}
		existing[p] = *entry
		indexed++
	}

	if GetDryRun() {
		return indexed, indexPath, nil
	}

	// Rewrite index deterministically.
	indexDir := filepath.Dir(indexPath)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return indexed, indexPath, err
	}
	out, err := os.OpenFile(indexPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return indexed, indexPath, err
	}
	defer func() {
		_ = out.Close() //nolint:errcheck // write completed
	}()

	pathsSorted := make([]string, 0, len(existing))
	for p := range existing {
		pathsSorted = append(pathsSorted, p)
	}
	sort.Strings(pathsSorted)

	enc := json.NewEncoder(out)
	for _, p := range pathsSorted {
		e := existing[p]
		if err := enc.Encode(e); err != nil {
			return indexed, indexPath, err
		}
	}

	return indexed, indexPath, nil
}
