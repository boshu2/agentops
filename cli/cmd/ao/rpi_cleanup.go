// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/adapters/workspace_git"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

var (
	cleanupRunID          string
	cleanupAll            bool
	cleanupPruneWorktrees bool
	cleanupPruneBranches  bool
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
	cleanupCmd.Flags().BoolVar(&cleanupPruneBranches, "prune-branches", false, "Delete legacy RPI branches (rpi/*, codex/auto-rpi-*)")
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

	return executeRPICleanup(cwd, cleanupRunID, cleanupAll, cleanupPruneWorktrees, cleanupPruneBranches, cleanupDryRun, cleanupStaleAfter)
}

func executeRPICleanup(cwd, runID string, all, prune, pruneBranches bool, dryRun bool, staleAfter time.Duration) error {
	if !all && runID == "" {
		return fmt.Errorf("specify --all or --run-id <id>")
	}

	staleRuns := collectStaleRuns(cwd, runID, staleAfter)

	if len(staleRuns) == 0 {
		fmt.Println("No stale runs found.")
	} else {
		processStaleRuns(cwd, staleRuns, dryRun)
	}

	runCleanupPostActions(cwd, runID, all, prune, pruneBranches, dryRun)
	return nil
}

// collectStaleRuns gathers deduplicated stale runs across all search roots,
// optionally filtered to a specific runID.
func collectStaleRuns(cwd, runID string, staleAfter time.Duration) []staleRunEntry {
	roots := collectSearchRoots(cwd)
	var staleRuns []staleRunEntry
	seen := make(map[string]struct{})
	now := time.Now()

	for _, root := range roots {
		entries := findStaleRunsWithMinAge(root, staleAfter, now)
		for _, e := range entries {
			if _, ok := seen[e.RunID]; ok {
				continue
			}
			seen[e.RunID] = struct{}{}
			if runID != "" && e.RunID != runID {
				continue
			}
			staleRuns = append(staleRuns, e)
		}
	}
	return staleRuns
}

// processStaleRuns iterates over stale runs, marking or cleaning each one.
func processStaleRuns(cwd string, staleRuns []staleRunEntry, dryRun bool) {
	for _, sr := range staleRuns {
		if dryRun {
			reportDryRunCleanup(sr)
			continue
		}
		cleanStaleRun(cwd, sr)
	}
}

// reportDryRunCleanup prints what would happen for a stale run without making changes.
func reportDryRunCleanup(sr staleRunEntry) {
	if sr.Terminal == "" {
		fmt.Printf("[dry-run] Would mark run %s as stale (reason: %s)\n", sr.RunID, sr.Reason)
	} else {
		fmt.Printf("[dry-run] Would clean terminal run %s (%s)\n", sr.RunID, sr.Reason)
	}
	if sr.WorktreePath != "" {
		if _, err := os.Stat(sr.WorktreePath); err == nil {
			fmt.Printf("[dry-run] Would remove worktree: %s\n", sr.WorktreePath)
		}
	}
}

// cleanStaleRun marks a non-terminal run as stale and removes orphaned worktrees.
func cleanStaleRun(cwd string, sr staleRunEntry) {
	if sr.Terminal == "" {
		if err := markRunStale(sr); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to mark run %s as stale: %v\n", sr.RunID, err)
			return
		}
		fmt.Printf("Marked run %s as stale (reason: %s)\n", sr.RunID, sr.Reason)
	} else {
		fmt.Printf("Cleaning terminal run %s (%s)\n", sr.RunID, sr.Reason)
	}

	removeStaleWorktreeIfExists(cwd, sr)
}

// removeStaleWorktreeIfExists removes the worktree directory associated with
// a stale run if it still exists on disk.
func removeStaleWorktreeIfExists(cwd string, sr staleRunEntry) {
	if sr.WorktreePath == "" {
		return
	}
	if _, statErr := os.Stat(sr.WorktreePath); statErr != nil {
		return
	}
	repoRoot := resolveCleanupRepoRoot(cwd, sr.WorktreePath)
	if rmErr := removeOrphanedWorktree(repoRoot, sr.WorktreePath, sr.RunID); rmErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove worktree %s: %v\n", sr.WorktreePath, rmErr)
	} else {
		fmt.Printf("Removed worktree: %s\n", sr.WorktreePath)
	}
}

// runCleanupPostActions runs worktree pruning and legacy branch cleanup after
// stale run processing.
func runCleanupPostActions(cwd, runID string, all, prune, pruneBranches, dryRun bool) {
	if prune && !dryRun {
		if err := pruneWorktrees(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: git worktree prune failed: %v\n", err)
		}
	}
	if pruneBranches {
		if err := cleanupLegacyRPIBranches(cwd, runID, all, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: legacy branch cleanup failed: %v\n", err)
		}
	}
}

// deleteLegacyBranches iterates over candidate branches, skipping active ones,
// and deletes (or dry-run reports) each.
func deleteLegacyBranches(cwd string, candidates []string, activeBranches map[string]bool, dryRun bool) {
	for _, name := range candidates {
		if activeBranches[name] {
			fmt.Printf("Skipping active branch: %s\n", name)
			continue
		}
		if dryRun {
			fmt.Printf("[dry-run] Would delete branch: %s\n", name)
			continue
		}
		cmd := exec.Command("git", "branch", "-D", name)
		cmd.Dir = cwd
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete branch %s: %v\n", name, err)
			continue
		}
		fmt.Printf("Deleted branch: %s\n", name)
	}
}

// cleanupLegacyRPIBranches removes legacy RPI branches for the selected scope.
func cleanupLegacyRPIBranches(cwd, runID string, all, dryRun bool) error {
	runID = strings.TrimSpace(runID)
	if runID == "" && !all {
		return fmt.Errorf("specify --all or --run-id to prune branches")
	}
	if strings.TrimSpace(cwd) == "" {
		return fmt.Errorf("cleanup branch command missing repository path")
	}

	candidates, err := collectLegacyRPIBranches(cwd, runID, all)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		fmt.Println("No legacy RPI branches found for cleanup.")
		return nil
	}

	activeBranches, err := checkedOutBranchSet(cwd)
	if err != nil {
		return err
	}

	deleteLegacyBranches(cwd, candidates, activeBranches, dryRun)
	return nil
}

func collectLegacyRPIBranches(cwd, runID string, all bool) ([]string, error) {
	branchPatterns := []string{}
	if all {
		branchPatterns = append(branchPatterns, "rpi/*", "codex/auto-rpi-*")
	} else {
		branchPatterns = append(branchPatterns, "rpi/"+runID)
	}

	seen := map[string]struct{}{}
	var branches []string

	for _, pattern := range branchPatterns {
		refPattern := "refs/heads/" + pattern
		cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", refPattern)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("list branches (%s): %w", pattern, err)
		}

		for _, raw := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				branches = append(branches, name)
			}
		}
	}
	return branches, nil
}

func checkedOutBranchSet(cwd string) (map[string]bool, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	active := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "branch "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		ref := strings.TrimPrefix(line, prefix)
		ref = strings.TrimSpace(ref)
		const refsHeads = "refs/heads/"
		if strings.HasPrefix(ref, refsHeads) {
			active[strings.TrimPrefix(ref, refsHeads)] = true
		}
	}

	return active, nil
}

// resolveCleanupRepoRoot picks a controller worktree root to execute
// `git worktree remove` against. It prefers a sibling worktree in the same
// parent directory as targetWorktree, avoiding attempts to remove a worktree
// from within itself.
func resolveCleanupRepoRoot(cwd, targetWorktree string) string {
	return rpi.ResolveCleanupRepoRoot(cwd, targetWorktree, collectSearchRoots(cwd))
}

// staleRunEntry is a thin alias for rpi.StaleRunEntry.
type staleRunEntry = rpi.StaleRunEntry

// findStaleRuns scans the registry for runs that are not active and not completed.
func findStaleRuns(root string) []staleRunEntry {
	return findStaleRunsWithMinAge(root, 0, time.Now())
}

// classifyRunEntry reads and parses a run's state file, returning a staleRunEntry
// if the run qualifies as stale. Returns ok=false if the run is active or
// cannot be parsed.
func classifyRunEntry(runID, root, runsDir string, minAge time.Duration, now time.Time) (staleRunEntry, bool) {
	statePath := filepath.Join(runsDir, runID, phasedStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return staleRunEntry{}, false
	}
	state, err := parsePhasedState(data)
	if err != nil || state.RunID == "" {
		return staleRunEntry{}, false
	}

	if state.TerminalStatus != "" {
		return checkTerminalRunStale(runID, root, statePath, state, minAge, now)
	}
	return checkNonTerminalRunStale(runID, root, statePath, state, minAge, now)
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
		if sr, ok := classifyRunEntry(entry.Name(), root, runsDir, minAge, now); ok {
			stale = append(stale, sr)
		}
	}
	return stale
}

// checkTerminalRunStale delegates to rpi.CheckTerminalRunStale.
func checkTerminalRunStale(runID, root, statePath string, state *phasedState, minAge time.Duration, now time.Time) (staleRunEntry, bool) {
	return rpi.CheckTerminalRunStale(
		runID, root, statePath,
		state.TerminalStatus, state.TerminalReason, state.TerminatedAt, state.StartedAt, state.WorktreePath,
		minAge, now,
	)
}

// checkNonTerminalRunStale returns a staleRunEntry for an inactive, non-completed run if it qualifies.
func checkNonTerminalRunStale(runID, root, statePath string, state *phasedState, minAge time.Duration, now time.Time) (staleRunEntry, bool) {
	isActive, _ := determineRunLiveness(root, state)
	if isActive {
		return staleRunEntry{}, false
	}
	if state.Phase >= completedPhaseNumber(*state) {
		return staleRunEntry{}, false
	}
	if minAge > 0 {
		startedAt, parseErr := time.Parse(time.RFC3339, state.StartedAt)
		if parseErr != nil || now.Sub(startedAt) < minAge {
			return staleRunEntry{}, false
		}
	}
	reason := "no heartbeat, no tmux session"
	if state.WorktreePath != "" {
		if _, statErr := os.Stat(state.WorktreePath); statErr != nil {
			reason = "worktree missing"
		}
	}
	return staleRunEntry{
		RunID:        runID,
		Root:         root,
		StatePath:    statePath,
		Reason:       reason,
		WorktreePath: state.WorktreePath,
	}, true
}

// markRunStale delegates to rpi.MarkRunStaleInState.
func markRunStale(sr staleRunEntry) error {
	return rpi.MarkRunStaleInState(sr, sr.Root)
}

// updateFlatStateIfMatches delegates to rpi.UpdateFlatStateIfMatches.
func updateFlatStateIfMatches(flatPath, runID, reason, terminatedAt string) {
	rpi.UpdateFlatStateIfMatches(flatPath, runID, reason, terminatedAt)
}

// preserveWorktreeCommits inspects the worktree's HEAD and, if it points at a
// commit that is NOT reachable from the repo's main branch, creates a
// `codex/preserve-<runID>` branch in repoRoot pointing at the worktree HEAD.
// This guards against the 2026-05-06 overnight failure mode where auto-cleanup
// of failed-cycle worktrees orphaned real commits (see soc-ewne).
//
// Returns the preserved commit SHA on success (empty string when no preserve
// was needed because the worktree HEAD was already on main). Errors are
// returned but the caller (removeOrphanedWorktree) treats them as warnings
// and continues with cleanup so a preserve failure does not block the cleanup
// path; the warning surfaces in supervisor output.
func preserveWorktreeCommits(repoRoot, worktreePath, runID string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", fmt.Errorf("preserveWorktreeCommits: runID required (preserve branch name must be deterministic)")
	}
	if _, err := os.Stat(worktreePath); err != nil {
		return "", nil // worktree already gone
	}

	headOut, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("preserveWorktreeCommits: rev-parse HEAD: %w", err)
	}
	worktreeHEAD := strings.TrimSpace(string(headOut))
	if worktreeHEAD == "" {
		return "", nil
	}

	// `git rev-list <head> ^main` returns the commits unique to the worktree.
	// Empty output means the worktree HEAD is already on main; no preserve needed.
	rlOut, rlErr := exec.Command("git", "-C", repoRoot, "rev-list", worktreeHEAD, "^main").Output()
	if rlErr != nil {
		// main missing or rev-list failed for any reason — fall back to
		// preserving unconditionally; better an extra branch than orphaned work.
		VerbosePrintf("preserveWorktreeCommits: rev-list %s ^main failed (%v); preserving unconditionally\n", worktreeHEAD, rlErr)
	} else if len(strings.TrimSpace(string(rlOut))) == 0 {
		return "", nil
	}

	branchName := "codex/preserve-" + runID
	if out, berr := exec.Command("git", "-C", repoRoot, "branch", "--no-track", branchName, worktreeHEAD).CombinedOutput(); berr != nil {
		// Idempotency: if the branch exists pointing at the same SHA, accept it.
		existing, _ := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", branchName).Output()
		if strings.TrimSpace(string(existing)) == worktreeHEAD {
			return worktreeHEAD, nil
		}
		return "", fmt.Errorf("preserveWorktreeCommits: branch %s: %s: %w", branchName, string(out), berr)
	}
	return worktreeHEAD, nil
}

// removeOrphanedWorktree removes a worktree directory and any legacy branch marker.
//
// Before destructive cleanup, preserveWorktreeCommits is invoked so any
// commits made inside the worktree are kept reachable on a
// `codex/preserve-<runID>` branch. Preservation is best-effort: a failure logs
// a warning and cleanup continues, since the existing failure mode (orphaned
// commits in git fsck) is already as bad as a lost preserve attempt.
func removeOrphanedWorktree(repoRoot, worktreePath, runID string) error {
	// Safety validation is delegated to internal/rpi.
	if err := rpi.ValidateWorktreeSibling(repoRoot, worktreePath); err != nil {
		return err
	}

	if preserved, err := preserveWorktreeCommits(repoRoot, worktreePath, runID); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: preserveWorktreeCommits(%s, %s): %v (continuing cleanup)\n", worktreePath, runID, err)
	} else if preserved != "" {
		shortLen := 12
		if len(preserved) < shortLen {
			shortLen = len(preserved)
		}
		fmt.Printf("Preserved worktree commit %s on branch codex/preserve-%s\n", preserved[:shortLen], runID)
	}

	// Force remove the worktree via the WorkspacePort adapter. The adapter
	// runs `git worktree remove --force` and, on failure (e.g. already pruned),
	// falls back to a direct directory removal — preserving the prior behavior.
	// WorkspaceID must be non-empty; fall back to the path when runID is blank.
	workspaceID := strings.TrimSpace(runID)
	if workspaceID == "" {
		workspaceID = worktreePath
	}
	if _, err := workspace_git.New(repoRoot).Cleanup(context.Background(), ports.WorkspaceRequest{
		WorkspaceID: workspaceID,
		Path:        worktreePath,
	}); err != nil {
		return err
	}

	// Delete legacy branch marker if present.
	if strings.TrimSpace(runID) != "" {
		branchName := "rpi/" + runID
		branchCmd := exec.Command("git", "branch", "-D", branchName)
		branchCmd.Dir = repoRoot
		_ = branchCmd.Run() // Best-effort; branch may not exist.
	}

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
