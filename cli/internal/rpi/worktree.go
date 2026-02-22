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

const detachedBranchSuffix = "-recovery"

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

// EnsureAttachedBranch repairs detached HEAD state when possible by switching to
// a stable recovery branch. If recovery cannot be performed safely (for example,
// the branch is already checked out in another worktree), it returns the current
// state and no error so callers can continue in detached mode.
func EnsureAttachedBranch(repoRoot string, timeout time.Duration, branchPrefix string) (branch string, healed bool, err error) {
	branch, err = GetCurrentBranch(repoRoot, timeout)
	if err == nil {
		return branch, false, nil
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		return "", false, err
	}

	prefix := strings.TrimSpace(branchPrefix)
	if prefix == "" {
		prefix = "codex/auto-rpi"
	}
	prefix = strings.TrimSuffix(prefix, "-")
	preferred := prefix + detachedBranchSuffix

	branchCreateOut, branchErr := runGitCreateBranch(repoRoot, timeout, "branch", "-f", preferred, "HEAD")
	if branchErr == nil {
		switchOut, switchErr := runGitCreateBranch(repoRoot, timeout, "switch", preferred)
		if switchErr == nil {
			return preferred, true, nil
		}
		switchOut = strings.TrimSpace(switchOut)
		if isBranchBusyInWorktree(switchOut) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("detached HEAD self-heal failed: %s", switchOut)
	}

	branchCreateOut = strings.TrimSpace(branchCreateOut)
	if isBranchBusyInWorktree(branchCreateOut) {
		return "", false, nil
	}
	if branchCreateOut != "" {
		return "", false, fmt.Errorf("detached HEAD self-heal failed: %s", branchCreateOut)
	}
	return "", false, fmt.Errorf("detached HEAD self-heal failed")

}

func isBranchBusyInWorktree(message string) bool {
	if message == "" {
		return false
	}
	message = strings.ToLower(message)
	return strings.Contains(message, "used by worktree") || strings.Contains(message, "already used by worktree")
}

func runGitCreateBranch(repoRoot string, timeout time.Duration, subcommand string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := append([]string{subcommand}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("git %s timed out after %s", subcommand, timeout)
	}
	return string(out), err
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
// Worktree checkouts are detached (no new branch created).
func CreateWorktree(cwd string, timeout time.Duration, verbosef func(string, ...interface{})) (worktreePath, runID string, err error) {
	repoRoot, err := GetRepoRoot(cwd, timeout)
	if err != nil {
		return "", "", err
	}

	if branch, err := GetCurrentBranch(repoRoot, timeout); err == nil {
		if verbosef != nil {
			verbosef("Creating detached worktree from current branch=%s\n", branch)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmdHead := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmdHead.Dir = repoRoot
	headOut, headErr := cmdHead.CombinedOutput()
	cancel()
	if headErr != nil {
		return "", "", fmt.Errorf("git rev-parse HEAD: %w (output: %s)", headErr, strings.TrimSpace(string(headOut)))
	}
	currentCommit := strings.TrimSpace(string(headOut))
	if currentCommit == "" {
		return "", "", fmt.Errorf("unable to resolve HEAD commit for detached worktree creation")
	}

	for attempt := 0; attempt < 3; attempt++ {
		runID = GenerateRunID()
		repoBasename := filepath.Base(repoRoot)
		worktreePath = filepath.Join(filepath.Dir(repoRoot), repoBasename+"-rpi-"+runID)

		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, currentCommit)
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
				verbosef("Worktree path collision on %s, retrying (%d/3)\n", worktreePath, attempt+1)
			}
			continue
		}

		if ctx.Err() == context.DeadlineExceeded {
			return "", "", fmt.Errorf("git worktree add timed out after %s", timeout)
		}
		return "", "", fmt.Errorf("git worktree add failed: %w (output: %s)", cmdErr, string(output))
	}
	return "", "", fmt.Errorf("failed to create unique worktree path after 3 attempts")
}

// MergeWorktree merges the RPI worktree commit back into the original branch.
func MergeWorktree(repoRoot, worktreePath, runID string, timeout time.Duration, verbosef func(string, ...interface{})) error {
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

	if strings.TrimSpace(worktreePath) == "" {
		if strings.TrimSpace(runID) == "" {
			return fmt.Errorf("merge source unavailable: missing worktree path and run ID")
		}
		worktreePath = filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"-rpi-"+runID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	revCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	revCmd.Dir = worktreePath
	revOut, revErr := revCmd.CombinedOutput()
	cancel()
	if revErr != nil {
		return fmt.Errorf("resolve worktree merge source: %w (output: %s)", revErr, strings.TrimSpace(string(revOut)))
	}
	mergeSource := strings.TrimSpace(string(revOut))
	if mergeSource == "" {
		return fmt.Errorf("worktree merge source commit is empty")
	}
	shortMergeSource := mergeSource
	if len(shortMergeSource) > 12 {
		shortMergeSource = shortMergeSource[:12]
	}

	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mergeMsg := "Merge ao rpi worktree (detached checkout)"
	if strings.TrimSpace(runID) != "" {
		mergeMsg = fmt.Sprintf("Merge %s (ao rpi worktree)", runID)
	}
	mergeCmd := exec.CommandContext(ctx, "git", "merge", "--no-ff", "-m", mergeMsg, mergeSource)
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
				shortMergeSource, files, repoRoot, mergeSource)
		}
		return fmt.Errorf("git merge failed: %w", err)
	}
	return nil
}

func rpiRunIDFromWorktree(repoRoot, worktreePath string) string {
	base := filepath.Base(worktreePath)
	prefix := filepath.Base(repoRoot) + "-rpi-"
	if !strings.HasPrefix(base, prefix) {
		return ""
	}
	return strings.TrimPrefix(base, prefix)
}

// RemoveWorktree removes a worktree directory and optionally a legacy branch reference.
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
	if strings.TrimSpace(runID) == "" {
		runID = rpiRunIDFromWorktree(resolvedRoot, absPath)
		if strings.TrimSpace(runID) == "" {
			return fmt.Errorf("invalid run id for removeWorktree path %s", absPath)
		}
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

	// Best-effort cleanup for legacy branch-based runs.
	if strings.TrimSpace(runID) != "" {
		branchName := "rpi/" + runID
		branchCmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
		branchCmd.Dir = repoRoot
		_ = branchCmd.Run() //nolint:errcheck
	}

	return nil
}
