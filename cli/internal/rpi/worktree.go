package rpi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GenerateRunID creates a 12-char crypto-random hex identifier.
func GenerateRunID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%012x", time.Now().UnixNano()&0xffffffffffff)
	}
	return hex.EncodeToString(b)
}

// GetCurrentBranch returns the current branch name, or an error for detached HEAD.
func GetCurrentBranch(repoRoot string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git rev-parse timed out after %s", timeout)
		}
		return "", fmt.Errorf("get current branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD: worktree requires a named branch")
	}
	return branch, nil
}

// GetRepoRoot returns the git repository root directory.
func GetRepoRoot(dir string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git rev-parse timed out after %s", timeout)
		}
		return "", fmt.Errorf("not a git repository (run ao rpi phased from inside a git repo)")
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateWorktree creates a sibling git worktree for isolated RPI execution.
func CreateWorktree(cwd string, timeout time.Duration, verbosef func(string, ...interface{})) (worktreePath, runID string, err error) {
	repoRoot, err := GetRepoRoot(cwd, timeout)
	if err != nil {
		return "", "", err
	}

	currentBranch, err := GetCurrentBranch(repoRoot, timeout)
	if err != nil {
		return "", "", err
	}

	for attempt := 0; attempt < 3; attempt++ {
		runID = GenerateRunID()
		repoBasename := filepath.Base(repoRoot)
		worktreePath = filepath.Join(filepath.Dir(repoRoot), repoBasename+"-rpi-"+runID)
		branchName := "rpi/" + runID

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreePath, currentBranch)
		cmd.Dir = repoRoot
		output, cmdErr := cmd.CombinedOutput()
		cancel()

		if cmdErr == nil {
			if mkErr := os.MkdirAll(filepath.Join(worktreePath, ".agents", "rpi"), 0755); mkErr != nil {
				if verbosef != nil {
					verbosef("Warning: could not create .agents/rpi/ in worktree: %v\n", mkErr)
				}
			}
			return worktreePath, runID, nil
		}

		if strings.Contains(string(output), "already exists") {
			if verbosef != nil {
				verbosef("Worktree branch collision on %s, retrying (%d/3)\n", branchName, attempt+1)
			}
			continue
		}

		if ctx.Err() == context.DeadlineExceeded {
			return "", "", fmt.Errorf("git worktree add timed out after %s", timeout)
		}
		return "", "", fmt.Errorf("git worktree add failed: %w (output: %s)", cmdErr, string(output))
	}
	return "", "", fmt.Errorf("failed to create unique worktree branch after 3 attempts")
}

// MergeWorktree merges the RPI worktree branch back into the original branch.
func MergeWorktree(repoRoot, runID string, timeout time.Duration, verbosef func(string, ...interface{})) error {
	var dirtyErr error
	for attempt := 0; attempt < 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		checkCmd := exec.CommandContext(ctx, "git", "diff-index", "--quiet", "HEAD")
		checkCmd.Dir = repoRoot
		dirtyErr = checkCmd.Run()
		cancel()

		if dirtyErr == nil {
			break
		}
		if attempt < 4 && verbosef != nil {
			verbosef("Repo dirty (another merge in progress?), retrying in 2s (%d/5)\n", attempt+1)
		}
		if attempt < 4 {
			time.Sleep(2 * time.Second)
		}
	}
	if dirtyErr != nil {
		return fmt.Errorf("original repo has uncommitted changes after 5 retries: commit or stash before merge")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	branchName := "rpi/" + runID
	mergeMsg := fmt.Sprintf("Merge %s (ao rpi phased worktree)", branchName)
	mergeCmd := exec.CommandContext(ctx, "git", "merge", "--no-ff", "-m", mergeMsg, branchName)
	mergeCmd.Dir = repoRoot
	if err := mergeCmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git merge timed out after %s", timeout)
		}
		conflictCmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
		conflictCmd.Dir = repoRoot
		conflictOut, _ := conflictCmd.Output()
		abortCmd := exec.Command("git", "merge", "--abort")
		abortCmd.Dir = repoRoot
		_ = abortCmd.Run() //nolint:errcheck
		files := strings.TrimSpace(string(conflictOut))
		if files != "" {
			return fmt.Errorf("merge conflict in %s.\nConflicting files:\n%s\nResolve manually: cd %s && git merge %s",
				branchName, files, repoRoot, branchName)
		}
		return fmt.Errorf("git merge failed: %w", err)
	}
	return nil
}

// RemoveWorktree removes a worktree directory and its branch.
func RemoveWorktree(repoRoot, worktreePath, runID string, timeout time.Duration) error {
	absPath, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		absPath, err = filepath.Abs(worktreePath)
		if err != nil {
			return fmt.Errorf("invalid worktree path: %w", err)
		}
	}
	resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		resolvedRoot = repoRoot
	}
	expectedBasename := filepath.Base(resolvedRoot) + "-rpi-" + runID
	expectedPath := filepath.Join(filepath.Dir(resolvedRoot), expectedBasename)
	if absPath != expectedPath {
		return fmt.Errorf("refusing to remove %s: expected %s (path validation failed)", absPath, expectedPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", absPath, "--force")
	cmd.Dir = repoRoot
	if _, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(absPath) //nolint:errcheck
	}

	branchName := "rpi/" + runID
	branchCmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	branchCmd.Dir = repoRoot
	_ = branchCmd.Run() //nolint:errcheck

	return nil
}
