// practices: [wiki-knowledge-surface, refactoring]
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

var (
	defragPrune      bool
	defragDedup      bool
	defragStaleDays  int
	defragOutputDir  string
	defragQuiet      bool
	defragNoSnapshot bool
)

// Type aliases for cmd/ao test compatibility.
type DefragReport = lifecycle.DefragReport
type PruneResult = lifecycle.PruneResult
type DefragDedupResult = lifecycle.DefragDedupResult

var defragCmd = &cobra.Command{
	Use:   "defrag",
	Short: "Prune and deduplicate .agents/ learning artifacts",
	Long: `Defrag performs mechanical cleanup of the knowledge base:

  --prune  Find orphaned learnings (no references, >N days old)
  --dedup  Flag learnings with >80% content similarity

By default, defrag applies prune/dedup changes unless you pass the global
--dry-run flag. Use ao --dry-run defrag ... to inspect the report without
deleting orphaned or duplicate learnings.

Output: .agents/defrag/YYYY-MM-DD.json with full delta report.

Before applying a destructive prune (anything that deletes from .agents/), defrag
takes an automatic corpus snapshot — the same tar.gz that ao corpus snapshot
writes — so an over-aggressive cleanup is always recoverable via ao corpus
restore. Pass --no-snapshot to skip it. A snapshot failure aborts the prune.

Examples:
  ao --dry-run defrag --prune --dedup           # full report only (no snapshot)
  ao defrag --prune --stale-days 14             # snapshot, then apply prune/delete`,
	RunE: runDefrag,
}

func init() {
	defragCmd.GroupID = "knowledge"
	rootCmd.AddCommand(defragCmd)
	defragCmd.Flags().BoolVar(&defragPrune, "prune", false,
		"Find orphaned learnings not referenced in patterns or research")
	defragCmd.Flags().BoolVar(&defragDedup, "dedup", false,
		"Flag learnings with >80% content similarity")
	defragCmd.Flags().IntVar(&defragStaleDays, "stale-days", 30,
		"Days after which an unreferenced learning is considered stale")
	defragCmd.Flags().StringVar(&defragOutputDir, "output-dir", ".agents/defrag",
		"Directory for defrag report JSON")
	defragCmd.Flags().BoolVar(&defragQuiet, "quiet", false, "Suppress progress output")
	defragCmd.Flags().BoolVar(&defragNoSnapshot, "no-snapshot", false,
		"Skip the automatic corpus snapshot taken before a destructive prune")
}

func runDefrag(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	isDryRun := GetDryRun()
	defragDefaultModes()

	if err := autoSnapshotBeforeDefrag(cwd, isDryRun, cmd.OutOrStdout()); err != nil {
		return err
	}

	report := &DefragReport{
		Timestamp: time.Now().UTC(),
		DryRun:    isDryRun,
	}

	if err := runDefragPhases(cwd, isDryRun, report); err != nil {
		return err
	}

	return writeDefragReport(defragOutputDir, report, cmd.OutOrStdout())
}

// autoSnapshotBeforeDefrag takes a recoverable corpus snapshot before defrag
// applies a destructive prune. It is a no-op for dry-runs (nothing is deleted),
// when prune is disabled (dedup-only defrag only flags, never deletes), or when
// --no-snapshot is set. A snapshot failure is fatal: we refuse to delete corpus
// files we could not first back up (safe-by-default — ag-32xk moat durability).
func autoSnapshotBeforeDefrag(cwd string, isDryRun bool, w io.Writer) error {
	if isDryRun || !defragPrune || defragNoSnapshot {
		return nil
	}
	manifest, _, err := createCorpusSnapshot(cwd, snapshotOutputDir)
	if err != nil {
		return fmt.Errorf("defrag: pre-prune corpus snapshot failed (refusing to delete; pass --no-snapshot to override): %w", err)
	}
	if !defragQuiet && GetOutput() != "json" {
		fmt.Fprintf(w, "Corpus snapshot (pre-prune): %s (%d files)\n", manifest.SnapshotPath, manifest.FileCount)
	}
	return nil
}

func defragDefaultModes() {
	if !defragPrune && !defragDedup {
		defragPrune = true
		defragDedup = true
	}
}

func runDefragPhases(cwd string, isDryRun bool, report *DefragReport) error {
	if defragPrune {
		result, err := lifecycle.ExecutePrune(cwd, isDryRun, defragStaleDays)
		if err != nil {
			return err
		}
		report.Prune = result
	}

	if defragDedup {
		result, err := lifecycle.ExecuteDedup(cwd, isDryRun)
		if err != nil {
			return err
		}
		report.Dedup = result
	}

	return nil
}

// Thin wrappers preserved for tests.
func executePrune(cwd string, isDryRun bool, staleDays int) (*PruneResult, error) {
	return lifecycle.ExecutePrune(cwd, isDryRun, staleDays)
}
func executeDedup(cwd string, isDryRun bool) (*DefragDedupResult, error) {
	return lifecycle.ExecuteDedup(cwd, isDryRun)
}
func findOrphanLearnings(cwd string, staleDays int) (*PruneResult, error) {
	return lifecycle.FindOrphanLearnings(cwd, staleDays)
}
func collectReferenceContent(cwd string) (string, error) {
	return lifecycle.CollectReferenceContent(cwd)
}
func isHashNamed(name string) bool { return lifecycle.IsHashNamed(name) }
func findDuplicateLearnings(cwd string) (*DefragDedupResult, error) {
	return lifecycle.FindDuplicateLearnings(cwd)
}
func buildTrigrams(text string) map[string]bool   { return lifecycle.BuildTrigrams(text) }
func trigramOverlap(a, b map[string]bool) float64 { return lifecycle.TrigramOverlap(a, b) }

// writeDefragReport writes the report as dated JSON and latest.json.
func writeDefragReport(dir string, r *DefragReport, w io.Writer) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	data = append(data, '\n')

	dateStr := r.Timestamp.Format("2006-01-02")
	datedPath := filepath.Join(dir, dateStr+".json")
	latestPath := filepath.Join(dir, "latest.json")

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	_ = hash

	if err := os.WriteFile(datedPath, data, 0o644); err != nil {
		return fmt.Errorf("write dated report: %w", err)
	}
	if err := os.WriteFile(latestPath, data, 0o644); err != nil {
		return fmt.Errorf("write latest report: %w", err)
	}

	if GetOutput() == "json" {
		return json.NewEncoder(w).Encode(r)
	}
	if !defragQuiet {
		printDefragSummary(w, r)
	}

	return nil
}

func printDefragSummary(w io.Writer, r *DefragReport) {
	fmt.Fprintf(w, "Defrag report: %s\n", r.Timestamp.Format(time.RFC3339))
	if r.DryRun {
		fmt.Fprintln(w, "  Mode: dry-run (no changes applied)")
	}
	if r.Prune != nil {
		fmt.Fprintf(w, "  Prune: %d total learnings, %d stale, %d orphans\n",
			r.Prune.TotalLearnings, r.Prune.StaleCount, len(r.Prune.Orphans))
		if len(r.Prune.Deleted) > 0 {
			fmt.Fprintf(w, "  Deleted: %d files\n", len(r.Prune.Deleted))
		}
	}
	if r.Dedup != nil {
		fmt.Fprintf(w, "  Dedup: %d checked, %d duplicate pairs\n",
			r.Dedup.Checked, len(r.Dedup.DuplicatePairs))
	}
}
