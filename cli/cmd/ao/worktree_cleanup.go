package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/adapters/workspace_git"
	"github.com/boshu2/agentops/cli/internal/ports"
	rpilib "github.com/boshu2/agentops/cli/internal/rpi"
)

type rpiRunInfo = rpilib.RPIRunInfo

const phasedStateFile = "phased-state.json"

func pruneWorktrees(cwd string) error {
	fmt.Println("Running: git worktree prune")
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func preserveWorktreeCommits(repoRoot, worktreePath, runID string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", fmt.Errorf("preserveWorktreeCommits: runID required (preserve branch name must be deterministic)")
	}
	if _, err := os.Stat(worktreePath); err != nil {
		return "", nil
	}

	headOut, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("preserveWorktreeCommits: rev-parse HEAD: %w", err)
	}
	worktreeHEAD := strings.TrimSpace(string(headOut))
	if worktreeHEAD == "" {
		return "", nil
	}

	rlOut, rlErr := exec.Command("git", "-C", repoRoot, "rev-list", worktreeHEAD, "^main").Output()
	if rlErr != nil {
		VerbosePrintf("preserveWorktreeCommits: rev-list %s ^main failed (%v); preserving unconditionally\n", worktreeHEAD, rlErr)
	} else if len(strings.TrimSpace(string(rlOut))) == 0 {
		return "", nil
	}

	branchName := "codex/preserve-" + runID
	if out, berr := exec.Command("git", "-C", repoRoot, "branch", "--no-track", branchName, worktreeHEAD).CombinedOutput(); berr != nil {
		existing, _ := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", branchName).Output()
		if strings.TrimSpace(string(existing)) == worktreeHEAD {
			return worktreeHEAD, nil
		}
		return "", fmt.Errorf("preserveWorktreeCommits: branch %s: %s: %w", branchName, string(out), berr)
	}
	return worktreeHEAD, nil
}

func removeOrphanedWorktree(repoRoot, worktreePath, runID string) error {
	if err := rpilib.ValidateWorktreeSibling(repoRoot, worktreePath); err != nil {
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

	if strings.TrimSpace(runID) != "" {
		branchName := "rpi/" + runID
		branchCmd := exec.Command("git", "branch", "-D", branchName)
		branchCmd.Dir = repoRoot
		_ = branchCmd.Run()
	}
	return nil
}

// discoverRPIRuns scans the run registry and returns run info. Since the RPI
// engine has been removed, all discovered runs are marked inactive — no
// heartbeat or tmux liveness checks are performed.
func discoverRPIRuns(repoRoot string) []rpiRunInfo {
	roots := collectSearchRoots(repoRoot)

	seen := make(map[string]struct{})
	var runs []rpiRunInfo
	for _, root := range roots {
		for _, run := range scanRegistryRunsForGC(root) {
			if _, ok := seen[run.RunID]; ok {
				continue
			}
			seen[run.RunID] = struct{}{}
			runs = append(runs, run)
		}
	}
	return runs
}

// scanRegistryRunsForGC reads run directories and returns rpiRunInfo with
// IsActive=false (RPI engine removed; no runs can be active).
func scanRegistryRunsForGC(root string) []rpiRunInfo {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}

	runs := make([]rpiRunInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		statePath := filepath.Join(runsDir, runID, "phased-state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var state struct {
			RunID     string `json:"run_id"`
			StartedAt string `json:"started_at"`
			Worktree  string `json:"worktree_path"`
		}
		if json.Unmarshal(data, &state) != nil || state.RunID == "" {
			continue
		}
		runs = append(runs, rpiRunInfo{
			RunID:     state.RunID,
			StartedAt: state.StartedAt,
			Worktree:  state.Worktree,
			IsActive:  false,
		})
	}
	return runs
}

func collectSearchRoots(cwd string) []string {
	var roots []string
	seen := make(map[string]struct{})

	tryAddSearchRoot(cwd, seen, &roots)

	if discovered := discoverGitWorktreeRoots(cwd); len(discovered) > 0 {
		for _, root := range discovered {
			tryAddSearchRoot(root, seen, &roots)
		}
		return roots
	}

	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*")
	matches, _ := filepath.Glob(pattern)
	for _, m := range matches {
		tryAddSearchRoot(m, seen, &roots)
	}
	return roots
}

func tryAddSearchRoot(path string, seen map[string]struct{}, roots *[]string) {
	if path == "" {
		return
	}
	normalized := normalizeSearchRootPath(path)
	if _, ok := seen[normalized]; ok {
		return
	}
	info, err := os.Stat(normalized)
	if err != nil || !info.IsDir() {
		return
	}
	stored := filepath.Clean(path)
	if abs, err := filepath.Abs(stored); err == nil {
		stored = filepath.Clean(abs)
	}
	seen[normalized] = struct{}{}
	*roots = append(*roots, stored)
}

func normalizeSearchRootPath(path string) string {
	clean := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	if abs, err := filepath.Abs(clean); err == nil {
		return filepath.Clean(abs)
	}
	return clean
}

func discoverGitWorktreeRoots(cwd string) []string {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var roots []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if path != "" {
			roots = append(roots, path)
		}
	}
	return roots
}
