// Package rpi implements the Research-Plan-Implement workflow orchestrator,
// including worktree management, toolchain execution, and branch lifecycle.
package rpi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
		return "", ErrDetachedHEAD
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
	if !errors.Is(err, ErrDetachedHEAD) {
		return "", false, err
	}

	preferred := resolveRecoveryBranch(branchPrefix)
	return attemptBranchHeal(repoRoot, timeout, preferred)
}

// resolveRecoveryBranch computes the recovery branch name from a prefix.
func resolveRecoveryBranch(branchPrefix string) string {
	prefix := strings.TrimSpace(branchPrefix)
	if prefix == "" {
		prefix = "codex/auto-rpi"
	}
	prefix = strings.TrimSuffix(prefix, "-")
	return prefix + detachedBranchSuffix
}

// attemptBranchHeal tries to create and switch to the recovery branch.
func attemptBranchHeal(repoRoot string, timeout time.Duration, preferred string) (string, bool, error) {
	branchCreateOut, branchErr := runGitCreateBranch(repoRoot, timeout, "branch", "-f", preferred, "HEAD")
	if branchErr == nil {
		return attemptBranchSwitch(repoRoot, timeout, preferred)
	}

	branchCreateOut = strings.TrimSpace(branchCreateOut)
	if isBranchBusyInWorktree(branchCreateOut) {
		return "", false, nil
	}
	if branchCreateOut != "" {
		return "", false, fmt.Errorf("%w: %s", ErrDetachedSelfHealFailed, branchCreateOut)
	}
	return "", false, ErrDetachedSelfHealFailed
}

// attemptBranchSwitch tries to switch to a branch after creation.
func attemptBranchSwitch(repoRoot string, timeout time.Duration, preferred string) (string, bool, error) {
	switchOut, switchErr := runGitCreateBranch(repoRoot, timeout, "switch", preferred)
	if switchErr == nil {
		return preferred, true, nil
	}
	switchOut = strings.TrimSpace(switchOut)
	if isBranchBusyInWorktree(switchOut) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("%w: %s", ErrDetachedSelfHealFailed, switchOut)
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
		return "", ErrNotGitRepo
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateWorktree creates a sibling git worktree for isolated RPI execution.
// Worktree checkouts are detached (no new branch created).
func CreateWorktree(cwd string, timeout time.Duration, verbosef func(string, ...any)) (worktreePath, runID string, err error) {
	repoRoot, err := GetRepoRoot(cwd, timeout)
	if err != nil {
		return "", "", err
	}

	if branch, err := GetCurrentBranch(repoRoot, timeout); err == nil {
		if verbosef != nil {
			verbosef("Creating detached worktree from current branch=%s\n", branch)
		}
	}

	currentCommit, err := resolveHeadCommit(repoRoot, timeout)
	if err != nil {
		return "", "", err
	}

	return tryCreateWorktree(repoRoot, currentCommit, timeout, verbosef)
}

// resolveHeadCommit returns the current HEAD commit SHA.
func resolveHeadCommit(repoRoot string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmdHead := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmdHead.Dir = repoRoot
	headOut, headErr := cmdHead.CombinedOutput()
	if headErr != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w (output: %s)", headErr, strings.TrimSpace(string(headOut)))
	}
	currentCommit := strings.TrimSpace(string(headOut))
	if currentCommit == "" {
		return "", ErrResolveHEAD
	}
	return currentCommit, nil
}

// tryCreateWorktree attempts to create a worktree up to 3 times, handling path collisions.
// initWorktreeAgentsDir creates the .agents/rpi/ directory in a new worktree,
// logging a warning on failure rather than failing the worktree creation.
func initWorktreeAgentsDir(worktreePath string, verbosef func(string, ...any)) {
	if mkErr := os.MkdirAll(filepath.Join(worktreePath, ".agents", "rpi"), 0750); mkErr != nil && verbosef != nil {
		verbosef("Warning: could not create .agents/rpi/ in worktree: %v\n", mkErr)
	}
}

// classifyWorktreeError inspects git worktree add output and returns a
// retryable flag (true = path collision, retry) or a terminal error.
func classifyWorktreeError(output []byte, ctxErr error, cmdErr error, timeout time.Duration) (retryable bool, err error) {
	if strings.Contains(string(output), "already exists") {
		return true, nil
	}
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return false, fmt.Errorf("git worktree add timed out after %s", timeout)
	}
	return false, fmt.Errorf("git worktree add failed: %w (output: %s)", cmdErr, string(output))
}

func tryCreateWorktree(repoRoot, currentCommit string, timeout time.Duration, verbosef func(string, ...any)) (string, string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		runID := GenerateRunID()
		repoBasename := filepath.Base(repoRoot)
		worktreePath := filepath.Join(filepath.Dir(repoRoot), repoBasename+"-rpi-"+runID)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, currentCommit)
		cmd.Dir = repoRoot
		output, cmdErr := cmd.CombinedOutput()
		cancel()

		if cmdErr == nil {
			initWorktreeAgentsDir(worktreePath, verbosef)
			return worktreePath, runID, nil
		}

		retryable, err := classifyWorktreeError(output, ctx.Err(), cmdErr, timeout)
		if !retryable {
			return "", "", err
		}
		if verbosef != nil {
			verbosef("Worktree path collision on %s, retrying (%d/3)\n", worktreePath, attempt+1)
		}
	}
	return "", "", ErrWorktreeCollision
}

// acquireMergeLock obtains an exclusive file lock for merge serialization.
func acquireMergeLock(repoRoot string) (*os.File, error) {
	lockPath := filepath.Join(repoRoot, ".git", "agentops", "merge.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open merge lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, errors.Join(fmt.Errorf("acquire merge lock: %w", err), fmt.Errorf("close merge lock: %w", closeErr))
		}
		return nil, fmt.Errorf("acquire merge lock: %w", err)
	}
	return f, nil
}

// releaseMergeLock releases the exclusive merge lock.
func releaseMergeLock(f *os.File) {
	if f == nil {
		return
	}
	_ = unlockFile(f)
	_ = f.Close()
}

// MergeWorktree merges the RPI worktree commit back into the original branch.
func MergeWorktree(repoRoot, worktreePath, runID string, timeout time.Duration, verbosef func(string, ...any)) error {
	lock, err := acquireMergeLock(repoRoot)
	if err != nil {
		return fmt.Errorf("merge lock: %w", err)
	}
	defer releaseMergeLock(lock)

	if err := waitForCleanRepo(repoRoot, timeout, verbosef); err != nil {
		return err
	}

	worktreePath = resolveWorktreePath(worktreePath, repoRoot, runID)
	if worktreePath == "" {
		return ErrMergeSourceUnavailable
	}

	mergeSource, err := resolveMergeSource(worktreePath, timeout)
	if err != nil {
		return err
	}

	return performMerge(repoRoot, runID, mergeSource, timeout)
}

const mergeCleanRetryDelayEnv = "AGENTOPS_RPI_MERGE_RETRY_DELAY"

func mergeCleanRetryDelay() time.Duration {
	raw := strings.TrimSpace(os.Getenv(mergeCleanRetryDelayEnv))
	if raw == "" {
		return 2 * time.Second
	}
	delay, err := time.ParseDuration(raw)
	if err != nil || delay < 0 {
		return 2 * time.Second
	}
	return delay
}

// waitForCleanRepo polls the repo until it has no uncommitted changes, retrying up to 5 times.
func waitForCleanRepo(repoRoot string, timeout time.Duration, verbosef func(string, ...any)) error {
	retryDelay := mergeCleanRetryDelay()
	for attempt := 0; attempt < 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		checkCmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "--untracked-files=all")
		checkCmd.Dir = repoRoot
		statusOut, statusErr := checkCmd.Output()
		cancel()

		if statusErr == nil && strings.TrimSpace(string(statusOut)) == "" {
			return nil
		}
		if statusErr != nil && ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git status --porcelain timed out after %s", timeout)
		}
		if attempt < 4 {
			if verbosef != nil {
				verbosef("Repo dirty (another merge in progress?), retrying in %s (%d/5)\n", retryDelay, attempt+1)
			}
			time.Sleep(retryDelay)
		}
	}
	return ErrRepoUnclean
}

// resolveWorktreePath returns the worktree path, inferring from runID if needed. Returns empty string if neither is available.
func resolveWorktreePath(worktreePath, repoRoot, runID string) string {
	if strings.TrimSpace(worktreePath) != "" {
		return worktreePath
	}
	if strings.TrimSpace(runID) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"-rpi-"+runID)
}

// resolveMergeSource resolves the HEAD commit of the worktree.
func resolveMergeSource(worktreePath string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	revCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	revCmd.Dir = worktreePath
	revOut, revErr := revCmd.CombinedOutput()
	cancel()
	if revErr != nil {
		return "", fmt.Errorf("resolve worktree merge source: %w (output: %s)", revErr, strings.TrimSpace(string(revOut)))
	}
	mergeSource := strings.TrimSpace(string(revOut))
	if mergeSource == "" {
		return "", ErrEmptyMergeSource
	}
	return mergeSource, nil
}

// performMerge executes the git merge and handles conflict reporting.
func performMerge(repoRoot, runID, mergeSource string, timeout time.Duration) error {
	shortMergeSource := mergeSource
	if len(shortMergeSource) > 12 {
		shortMergeSource = shortMergeSource[:12]
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mergeMsg := "Merge ao rpi worktree (detached checkout)"
	if strings.TrimSpace(runID) != "" {
		mergeMsg = fmt.Sprintf("Merge %s (ao rpi worktree)", runID)
	}
	mergeCmd := exec.CommandContext(ctx, "git", "merge", "--no-ff", "-m", mergeMsg, mergeSource)
	mergeCmd.Dir = repoRoot
	if err := mergeCmd.Run(); err != nil {
		return handleMergeFailure(repoRoot, mergeSource, shortMergeSource, ctx, err, timeout)
	}
	return nil
}

// handleMergeFailure processes a git merge failure: reports conflicts and aborts the merge.
func handleMergeFailure(repoRoot, mergeSource, shortMergeSource string, ctx context.Context, mergeErr error, timeout time.Duration) error {
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("git merge timed out after %s", timeout)
	}
	conflictCmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	conflictCmd.Dir = repoRoot
	conflictOut, conflictErr := conflictCmd.Output()
	if conflictErr != nil {
		// Non-fatal: best-effort conflict file listing
		_ = conflictErr
	}
	abortCmd := exec.Command("git", "merge", "--abort")
	abortCmd.Dir = repoRoot
	_ = abortCmd.Run() //nolint:errcheck
	files := strings.TrimSpace(string(conflictOut))
	if files != "" {
		return fmt.Errorf("merge conflict in %s.\nConflicting files:\n%s\nResolve manually: cd %s && git merge %s",
			shortMergeSource, files, repoRoot, mergeSource)
	}
	return fmt.Errorf("git merge failed: %w", mergeErr)
}

func rpiRunIDFromWorktree(repoRoot, worktreePath string) string {
	base := filepath.Base(worktreePath)
	prefix := filepath.Base(repoRoot) + "-rpi-"
	runID, ok := strings.CutPrefix(base, prefix)
	if !ok {
		return ""
	}
	return runID
}

// RemoveWorktree removes a worktree directory and optionally a legacy branch reference.
func RemoveWorktree(repoRoot, worktreePath, runID string, timeout time.Duration) error {
	absPath, resolvedRoot, runID, err := resolveRemovePaths(repoRoot, worktreePath, runID)
	if err != nil {
		return err
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
		branchCmd.Dir = resolvedRoot
		_ = branchCmd.Run() //nolint:errcheck
	}

	return nil
}

// resolveAbsPath resolves worktreePath to an absolute path, preferring EvalSymlinks.
func resolveAbsPath(worktreePath string) (string, error) {
	absPath, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		absPath, err = filepath.Abs(worktreePath)
		if err != nil {
			return "", fmt.Errorf("invalid worktree path: %w", err)
		}
	}
	return absPath, nil
}

// resolveRemovePaths validates and resolves the absolute worktree path, repo root,
// and run ID for a safe worktree removal.
func resolveRemovePaths(repoRoot, worktreePath, runID string) (absPath, resolvedRoot, resolvedRunID string, err error) {
	absPath, err = resolveAbsPath(worktreePath)
	if err != nil {
		return "", "", "", err
	}
	resolvedRoot, err = filepath.EvalSymlinks(repoRoot)
	if err != nil {
		resolvedRoot = repoRoot
	}
	resolvedRunID = runID
	if strings.TrimSpace(resolvedRunID) == "" {
		resolvedRunID = rpiRunIDFromWorktree(resolvedRoot, absPath)
		if strings.TrimSpace(resolvedRunID) == "" {
			return "", "", "", fmt.Errorf("invalid run id for removeWorktree path %s", absPath)
		}
	}
	expectedBasename := filepath.Base(resolvedRoot) + "-rpi-" + resolvedRunID
	expectedPath := filepath.Join(filepath.Dir(resolvedRoot), expectedBasename)
	if absPath != expectedPath {
		return "", "", "", fmt.Errorf("refusing to remove %s: expected %s (path validation failed)", absPath, expectedPath)
	}
	return absPath, resolvedRoot, resolvedRunID, nil
}
