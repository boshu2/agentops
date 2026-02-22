package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	cleanupRunID          string
	cleanupAll            bool
	cleanupPruneWorktrees bool
	cleanupDryRun         bool
	cleanupStaleAfter     time.Duration
)

func init() {
	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up stale RPI runs",
		Long: `Detect and clean up stale RPI phased runs.

A run is considered stale if it has no active heartbeat, no live tmux session,
and is not at a terminal phase (completed). Stale runs are marked with terminal
metadata so they no longer appear as "running" or "unknown".

Examples:
  ao rpi cleanup --all --dry-run     # Preview cleanup actions
  ao rpi cleanup --all               # Clean up all stale runs
  ao rpi cleanup --run-id abc123     # Clean up a specific run
  ao rpi cleanup --all --prune-worktrees  # Also run git worktree prune`,
		RunE: runRPICleanup,
	}
	cleanupCmd.Flags().StringVar(&cleanupRunID, "run-id", "", "Clean up a specific run by ID")
	cleanupCmd.Flags().BoolVar(&cleanupAll, "all", false, "Clean up all stale runs")
	cleanupCmd.Flags().BoolVar(&cleanupPruneWorktrees, "prune-worktrees", false, "Run 'git worktree prune' after cleanup")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would be done without making changes")
	cleanupCmd.Flags().DurationVar(&cleanupStaleAfter, "stale-after", 0, "Only clean runs older than this age (0 disables age filtering)")
	rpiCmd.AddCommand(cleanupCmd)
}

func runRPICleanup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	return executeRPICleanup(cwd, cleanupRunID, cleanupAll, cleanupPruneWorktrees, cleanupDryRun, cleanupStaleAfter)
}

func executeRPICleanup(cwd, runID string, all, prune, dryRun bool, staleAfter time.Duration) error {
	if !all && runID == "" {
		return fmt.Errorf("specify --all or --run-id <id>")
	}

	roots := collectSearchRoots(cwd)
	var staleRuns []staleRunEntry
	seen := make(map[string]struct{})
	now := time.Now()

	for _, root := range roots {
		entries := findStaleRunsWithMinAge(root, staleAfter, now)
		for _, e := range entries {
			if _, ok := seen[e.runID]; ok {
				continue
			}
			seen[e.runID] = struct{}{}

			if runID != "" && e.runID != runID {
				continue
			}
			staleRuns = append(staleRuns, e)
		}
	}

	if len(staleRuns) == 0 {
		fmt.Println("No stale runs found.")
		if prune && !dryRun {
			return pruneWorktrees(cwd)
		}
		return nil
	}

	for _, sr := range staleRuns {
		if dryRun {
			if sr.terminal == "" {
				fmt.Printf("[dry-run] Would mark run %s as stale (reason: %s)\n", sr.runID, sr.reason)
			} else {
				fmt.Printf("[dry-run] Would clean terminal run %s (%s)\n", sr.runID, sr.reason)
			}
			if sr.worktreePath != "" {
				if _, err := os.Stat(sr.worktreePath); err == nil {
					fmt.Printf("[dry-run] Would remove worktree: %s\n", sr.worktreePath)
				}
			}
			continue
		}

		if sr.terminal == "" {
			if err := markRunStale(sr); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to mark run %s as stale: %v\n", sr.runID, err)
				continue
			}
			fmt.Printf("Marked run %s as stale (reason: %s)\n", sr.runID, sr.reason)
		} else {
			fmt.Printf("Cleaning terminal run %s (%s)\n", sr.runID, sr.reason)
		}

		// Remove orphaned worktree directory if it still exists.
		if sr.worktreePath != "" {
			if _, statErr := os.Stat(sr.worktreePath); statErr == nil {
				repoRoot := resolveCleanupRepoRoot(cwd, sr.worktreePath)
				if rmErr := removeOrphanedWorktree(repoRoot, sr.worktreePath, sr.runID); rmErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not remove worktree %s: %v\n", sr.worktreePath, rmErr)
				} else {
					fmt.Printf("Removed worktree: %s\n", sr.worktreePath)
				}
			}
		}
	}

	if prune && !dryRun {
		if err := pruneWorktrees(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: git worktree prune failed: %v\n", err)
		}
	}

	return nil
}

// resolveCleanupRepoRoot picks a controller worktree root to execute
// `git worktree remove` against. It prefers a sibling worktree in the same
// parent directory as targetWorktree, avoiding attempts to remove a worktree
// from within itself.
func resolveCleanupRepoRoot(cwd, targetWorktree string) string {
	target := filepath.Clean(targetWorktree)
	targetParent := filepath.Dir(target)

	roots := collectSearchRoots(cwd)
	for _, root := range roots {
		cleanRoot := filepath.Clean(root)
		if cleanRoot == target {
			continue
		}
		if filepath.Dir(cleanRoot) == targetParent {
			return cleanRoot
		}
	}
	return cwd
}

type staleRunEntry struct {
	runID        string
	root         string
	statePath    string
	reason       string
	worktreePath string
	terminal     string
}

// findStaleRuns scans the registry for runs that are not active and not completed.
func findStaleRuns(root string) []staleRunEntry {
	return findStaleRunsWithMinAge(root, 0, time.Now())
}

// findStaleRunsWithMinAge scans the registry for runs that are not active and
// not completed, optionally filtering to runs older than minAge.
func findStaleRunsWithMinAge(root string, minAge time.Duration, now time.Time) []staleRunEntry {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}

	var stale []staleRunEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		statePath := filepath.Join(runsDir, runID, phasedStateFile)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		state, err := parsePhasedState(data)
		if err != nil || state.RunID == "" {
			continue
		}

		// Check liveness.
		isActive, _ := determineRunLiveness(root, state)
		if isActive {
			continue
		}

		// Terminal runs (except completed) are cleanup candidates only when their
		// worktree still exists.
		if state.TerminalStatus != "" {
			if state.TerminalStatus == "completed" {
				continue
			}
			if state.WorktreePath == "" {
				continue
			}
			if _, statErr := os.Stat(state.WorktreePath); statErr != nil {
				continue
			}
			if minAge > 0 {
				candidateAt := state.TerminatedAt
				if candidateAt == "" {
					candidateAt = state.StartedAt
				}
				parsedAt, parseErr := time.Parse(time.RFC3339, candidateAt)
				if parseErr != nil || now.Sub(parsedAt) < minAge {
					continue
				}
			}

			reason := state.TerminalReason
			if reason == "" {
				reason = "terminal status: " + state.TerminalStatus
			}
			stale = append(stale, staleRunEntry{
				runID:        runID,
				root:         root,
				statePath:    statePath,
				reason:       reason,
				worktreePath: state.WorktreePath,
				terminal:     state.TerminalStatus,
			})
			continue
		}

		// Non-terminal completed runs are not stale.
		if state.Phase >= completedPhaseNumber(*state) {
			continue
		}
		// Optional age filter to reduce risk of cleaning recently interrupted runs.
		if minAge > 0 {
			startedAt, parseErr := time.Parse(time.RFC3339, state.StartedAt)
			if parseErr != nil {
				// Skip runs with malformed or missing start timestamps when age filtering is enabled.
				continue
			}
			if now.Sub(startedAt) < minAge {
				continue
			}
		}
		// Determine reason.
		reason := "no heartbeat, no tmux session"
		if state.WorktreePath != "" {
			if _, statErr := os.Stat(state.WorktreePath); statErr != nil {
				reason = "worktree missing"
			}
		}

		stale = append(stale, staleRunEntry{
			runID:        runID,
			root:         root,
			statePath:    statePath,
			reason:       reason,
			worktreePath: state.WorktreePath,
		})
	}
	return stale
}

// markRunStale writes terminal metadata to the state file.
func markRunStale(sr staleRunEntry) error {
	data, err := os.ReadFile(sr.statePath)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}

	raw["terminal_status"] = "stale"
	raw["terminal_reason"] = sr.reason
	raw["terminated_at"] = time.Now().Format(time.RFC3339)

	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	updated = append(updated, '\n')

	if err := writePhasedStateAtomic(sr.statePath, updated); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	// Also update the flat state file if it exists and matches this run.
	flatPath := filepath.Join(sr.root, ".agents", "rpi", phasedStateFile)
	if flatData, fErr := os.ReadFile(flatPath); fErr == nil {
		var flatRaw map[string]interface{}
		if json.Unmarshal(flatData, &flatRaw) == nil {
			if flatRunID, ok := flatRaw["run_id"].(string); ok && flatRunID == sr.runID {
				flatRaw["terminal_status"] = "stale"
				flatRaw["terminal_reason"] = sr.reason
				flatRaw["terminated_at"] = raw["terminated_at"]
				if flatUpdated, mErr := json.MarshalIndent(flatRaw, "", "  "); mErr == nil {
					flatUpdated = append(flatUpdated, '\n')
					_ = writePhasedStateAtomic(flatPath, flatUpdated)
				}
			}
		}
	}

	return nil
}

// removeOrphanedWorktree removes a worktree directory and its branch.
func removeOrphanedWorktree(repoRoot, worktreePath, runID string) error {
	// Safety: validate that worktreePath is a sibling of the repo root (same parent dir).
	// Worktrees are created as ../repo-rpi-<id>/ — siblings of the repo, not children.
	// This prevents corrupted state files from directing os.RemoveAll at unrelated paths.
	repoParent := filepath.Dir(filepath.Clean(repoRoot))
	wtParent := filepath.Dir(filepath.Clean(worktreePath))
	if wtParent != repoParent {
		return fmt.Errorf("worktree path %q is not a sibling of repo %q; refusing removal", worktreePath, repoRoot)
	}
	// Additional safety: never remove the repo root itself.
	cleanWT := filepath.Clean(worktreePath)
	if cleanWT == filepath.Clean(repoRoot) {
		return fmt.Errorf("worktree path %q is the repo root; refusing removal", worktreePath)
	}

	// Force remove the worktree.
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		// If worktree remove fails (already pruned), just remove the directory.
		if rmErr := os.RemoveAll(worktreePath); rmErr != nil {
			return fmt.Errorf("git worktree remove: %s; manual rm: %w", string(out), rmErr)
		}
	}

	// Delete the branch.
	branchName := "rpi/" + runID
	branchCmd := exec.Command("git", "branch", "-D", branchName)
	branchCmd.Dir = repoRoot
	_ = branchCmd.Run() // Best-effort; branch may not exist.

	return nil
}

// pruneWorktrees runs `git worktree prune`.
func pruneWorktrees(cwd string) error {
	fmt.Println("Running: git worktree prune")
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
