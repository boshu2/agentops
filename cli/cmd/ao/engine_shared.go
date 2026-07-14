// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/adapters/workspace_git"
	"github.com/boshu2/agentops/cli/internal/ports"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/boshu2/agentops/cli/internal/worktree"
)

func uniqueStringsPreserveOrder(items []string) []string {
	return cliRPI.UniqueStringsPreserveOrder(items)
}

func compiledChecklistSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return cliRPI.CompiledChecklistSummaryFromContent(id, string(data))
}

func compiledSummariesForFindings(cwd, subdir string, findingIDs []string) []string {
	summaries := make([]string, 0, len(findingIDs))
	for _, id := range uniqueStringsPreserveOrder(findingIDs) {
		path := filepath.Join(cwd, ".agents", subdir, id+".md")
		if summary := compiledChecklistSummary(path); summary != "" {
			summaries = append(summaries, summary)
		}
	}
	return uniqueStringsPreserveOrder(summaries)
}

// compiledPremortemSummariesForFindings reads canonical checks first while
// preserving the permanent legacy-directory fallback. Equal same-ID content
// is emitted once; different same-ID content fails closed and names both paths.
func compiledPremortemSummariesForFindings(cwd string, findingIDs []string) ([]string, error) {
	paths, err := reconciledPremortemCheckPaths(cwd, findingIDs)
	if err != nil {
		return nil, err
	}
	summaries := make([]string, 0, len(findingIDs))
	for _, path := range paths {
		if summary := compiledChecklistSummary(path); summary != "" {
			summaries = append(summaries, summary)
		}
	}
	return uniqueStringsPreserveOrder(summaries), nil
}

func premortemCheckIDs(cwd string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, directory := range []string{"premortem-checks", "pre-mortem-checks"} {
		root := filepath.Join(cwd, ".agents", directory)
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read premortem check directory %s: %w", root, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
				continue
			}
			seen[strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

const canonicalPremortemReadDirectory = "premortem-checks"

// reconciledPremortemCheckPaths selects canonical content first and fills
// missing IDs from the legacy directory. Equal dual content is returned once;
// conflicting dual content fails closed and identifies both paths.
func reconciledPremortemCheckPaths(cwd string, findingIDs []string) ([]string, error) {
	paths := make([]string, 0, len(findingIDs))
	for _, id := range uniqueStringsPreserveOrder(findingIDs) {
		canonical := filepath.Join(cwd, ".agents", canonicalPremortemReadDirectory, id+".md")
		legacy := filepath.Join(cwd, ".agents", "pre-mortem-checks", id+".md")
		canonicalData, canonicalErr := os.ReadFile(canonical)
		legacyData, legacyErr := os.ReadFile(legacy)
		canonicalExists := canonicalErr == nil
		legacyExists := legacyErr == nil
		if canonicalErr != nil && !os.IsNotExist(canonicalErr) {
			return nil, fmt.Errorf("read canonical premortem check %s: %w", canonical, canonicalErr)
		}
		if legacyErr != nil && !os.IsNotExist(legacyErr) {
			return nil, fmt.Errorf("read legacy premortem check %s: %w", legacy, legacyErr)
		}
		if canonicalExists && legacyExists && !bytes.Equal(canonicalData, legacyData) {
			return nil, fmt.Errorf("conflicting premortem checks for %s: %s and %s", id, canonical, legacy)
		}
		selected := canonical
		switch {
		case canonicalExists:
		case legacyExists:
			selected = legacy
		default:
			continue
		}
		paths = append(paths, selected)
	}
	return paths, nil
}

// worktreeTimeout is the timeout for git worktree operations.
const worktreeTimeout = 30 * time.Second

// getCurrentBranch returns the current branch name, or error if detached HEAD.
func getCurrentBranch(repoRoot string) (string, error) {
	return worktree.GetCurrentBranch(repoRoot, worktreeTimeout)
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
	if err := worktree.ValidateWorktreeSibling(repoRoot, worktreePath); err != nil {
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
