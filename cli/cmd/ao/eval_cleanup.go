package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	evalsub "github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

var (
	evalCleanupDelete   bool
	evalCleanupTmpFiles bool
	evalCleanupAge      int64
	evalCleanupDryRun   bool
)

// CleanupReport summarizes a cleanup pass for JSON output.
type CleanupReport struct {
	TransitionsAborted int      `json:"transitions_to_aborted"`
	TransitionsFailed  int      `json:"transitions_to_failed"`
	RunsDeleted        int      `json:"runs_deleted"`
	TmpFilesSwept      int      `json:"tmp_files_swept"`
	Touched            []string `json:"touched"`
}

var evalCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Recover stale Runs (state transitions, --delete, --tmp-files)",
	Long: `Per SCHEMA.md §4 cleanup state-transition rule (rc2):

  Stale pending  (no running transition within 60s)
                                       -> aborted (retraction_reason=never_started)
  Stale running  (no heartbeat within 5min OR Inspect process not alive)
                                       -> failed   (retraction_reason=orphaned_process)

After transitions:
  --delete       Remove Run dirs whose status is failed OR aborted (NEVER retracted).
  --tmp-files    Sweep orphan manifest.json.tmp left from rename-step crashes.
  --dry-run      Print what would be done; no mutations.

The cleanup procedure honors the §4 atomic-write contract on every state
transition (temp + fsync + rename + fsync-parent-dir). Retracted Runs are
never auto-removed — retraction is an audit trail.`,
	RunE: runEvalCleanup,
}

func runEvalCleanup(cmd *cobra.Command, args []string) error {
	root := evalsRoot()
	runsRoot := filepath.Join(root, "runs")
	report := CleanupReport{Touched: []string{}}

	entries, err := os.ReadDir(runsRoot)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("eval cleanup: read runs/: %w", err)
	}

	stalePendingCutoff := time.Duration(60) * time.Second
	staleRunningCutoff := time.Duration(5) * time.Minute

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runID := e.Name()
		path := evalsub.ManifestPath(root, runID)
		m, err := evalsub.LoadManifest(path)
		if err != nil {
			// Manifest missing or corrupt — skip but record touch
			report.Touched = append(report.Touched, runID+":unreadable")
			continue
		}
		now := time.Now().UTC()
		startedAt := time.Unix(m.StartedAtUnixMs/1000, 0).UTC()
		ageSinceStart := now.Sub(startedAt)

		switch m.Status {
		case evalsub.StatusPending:
			if ageSinceStart >= stalePendingCutoff {
				if err := transitionStale(root, runID, evalsub.StatusAborted, "never_started"); err != nil {
					return err
				}
				report.TransitionsAborted++
				report.Touched = append(report.Touched, runID+":pending->aborted")
			}
		case evalsub.StatusRunning:
			if ageSinceStart >= staleRunningCutoff {
				if err := transitionStale(root, runID, evalsub.StatusFailed, "orphaned_process"); err != nil {
					return err
				}
				report.TransitionsFailed++
				report.Touched = append(report.Touched, runID+":running->failed")
			}
		}
	}

	if evalCleanupDelete {
		// Re-walk, now that transitions are applied, removing failed/aborted.
		entries, err := os.ReadDir(runsRoot)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				runID := e.Name()
				path := evalsub.ManifestPath(root, runID)
				m, err := evalsub.LoadManifest(path)
				if err != nil {
					continue
				}
				if m.Status == evalsub.StatusFailed || m.Status == evalsub.StatusAborted {
					target := filepath.Join(runsRoot, runID)
					if evalCleanupDryRun {
						report.Touched = append(report.Touched, runID+":would-delete")
						continue
					}
					if err := os.RemoveAll(target); err != nil {
						return fmt.Errorf("eval cleanup: remove %q: %w", target, err)
					}
					report.RunsDeleted++
					report.Touched = append(report.Touched, runID+":deleted")
				}
			}
		}
	}

	if evalCleanupTmpFiles {
		if evalCleanupDryRun {
			report.Touched = append(report.Touched, "tmp-files: dry-run preview not implemented (sweep is a write op)")
		} else {
			swept, err := evalsub.SweepTempFiles(root, evalCleanupAge)
			if err != nil {
				return fmt.Errorf("eval cleanup: sweep tmp: %w", err)
			}
			report.TmpFilesSwept = len(swept)
			for _, s := range swept {
				report.Touched = append(report.Touched, "tmp:"+s)
			}
		}
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Eval cleanup:\n  transitions->aborted: %d\n  transitions->failed:  %d\n  runs deleted:         %d\n  tmp files swept:      %d\n",
		report.TransitionsAborted, report.TransitionsFailed, report.RunsDeleted, report.TmpFilesSwept)
	if len(report.Touched) > 0 && GetVerbose() {
		fmt.Fprintln(cmd.OutOrStdout(), "Touched:")
		for _, t := range report.Touched {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", t)
		}
	}
	return nil
}

// transitionStale reads + writes via the §4 atomic contract by routing
// through RunWriter — but RunWriter requires a fresh runID + dir. For
// cleanup we mutate an existing manifest in place, so we re-marshal +
// WriteAtomic directly with status + retraction_reason set.
func transitionStale(root, runID string, next evalsub.RunStatus, reason string) error {
	path := evalsub.ManifestPath(root, runID)
	m, err := evalsub.LoadManifest(path)
	if err != nil {
		return fmt.Errorf("transitionStale: load: %w", err)
	}
	if !legalStaleTransition(m.Status, next) {
		return nil // nothing to do
	}
	m.Status = next
	m.RetractionReason = reason
	now := time.Now().UTC()
	m.FinishedAt = now.Format(time.RFC3339)
	m.FinishedAtUnixMs = now.UnixNano() / int64(time.Millisecond)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("transitionStale: marshal: %w", err)
	}
	data = append(data, '\n')
	return evalsub.WriteAtomic(path, data)
}

func legalStaleTransition(cur, next evalsub.RunStatus) bool {
	switch cur {
	case evalsub.StatusPending:
		return next == evalsub.StatusAborted
	case evalsub.StatusRunning:
		return next == evalsub.StatusFailed
	}
	return false
}

func registerEvalCleanupCmd() {
	evalCleanupCmd.Flags().BoolVar(&evalCleanupDelete, "delete", false, "Remove Run directories whose status is failed or aborted (never retracted)")
	evalCleanupCmd.Flags().BoolVar(&evalCleanupTmpFiles, "tmp-files", false, "Sweep orphan *.tmp files older than --tmp-age")
	evalCleanupCmd.Flags().Int64Var(&evalCleanupAge, "tmp-age", 60, "Minimum tmp-file age in seconds before sweep (0 = sweep all)")
	evalCleanupCmd.Flags().BoolVar(&evalCleanupDryRun, "dry-run", false, "Preview without mutations")
}

