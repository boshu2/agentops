// practices: [microservices, team-topologies]
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairSharedCoreWorktreeConfig_MigratesLinkedWorktrees(t *testing.T) {
	repo := initTestRepo(t)
	worktreePath := filepath.Join(t.TempDir(), "linked")
	repoRealPath := realPathForTest(t, repo)

	runGitCommand(t, repo, "branch", "feature/worktree-config")
	runGitCommand(t, repo, "worktree", "add", worktreePath, "feature/worktree-config")
	defer runGitIgnoreErrorCommand(t, repo, "worktree", "remove", "--force", worktreePath)
	worktreeRealPath := realPathForTest(t, worktreePath)
	nestedDir := filepath.Join(worktreePath, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runGitCommand(t, repo, "config", "extensions.worktreeConfig", "true")
	runGitCommand(t, repo, "config", "core.worktree", repo)

	if got := realPathForTest(t, strings.TrimSpace(runGitOutputCommand(t, worktreePath, "rev-parse", "--show-toplevel"))); got != repoRealPath {
		t.Fatalf("broken linked worktree reproduction failed: got toplevel %q, want %q", got, repoRealPath)
	}

	if err := repairSharedCoreWorktreeConfig(nestedDir); err != nil {
		t.Fatalf("repairSharedCoreWorktreeConfig: %v", err)
	}
	if err := sanitizeGitProcessEnv(); err != nil {
		t.Fatalf("sanitizeGitProcessEnv: %v", err)
	}

	if got := realPathForTest(t, strings.TrimSpace(runGitOutputCommand(t, worktreePath, "rev-parse", "--show-toplevel"))); got != worktreeRealPath {
		t.Fatalf("linked worktree toplevel after repair = %q, want %q", got, worktreeRealPath)
	}

	if got := realPathForTest(t, strings.TrimSpace(runGitOutputCommand(t, repo, "rev-parse", "--show-toplevel"))); got != repoRealPath {
		t.Fatalf("main worktree toplevel after repair = %q, want %q", got, repoRealPath)
	}

	sharedCoreWorktree := strings.TrimSpace(runGitOutputAllowFailure(t, worktreePath, "config", "--local", "--get", "core.worktree"))
	if sharedCoreWorktree != "" {
		t.Fatalf("expected shared core.worktree to be unset, got %q", sharedCoreWorktree)
	}

	perWorktree := realPathForTest(t, strings.TrimSpace(runGitOutputCommand(t, worktreePath, "config", "--worktree", "--get", "core.worktree")))
	if perWorktree != worktreeRealPath {
		t.Fatalf("linked worktree core.worktree = %q, want %q", perWorktree, worktreeRealPath)
	}
}

func runGitCommand(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func runGitOutputCommand(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func runGitOutputAllowFailure(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func runGitIgnoreErrorCommand(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	_ = cmd.Run()
}

func realPathForTest(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", path, err)
	}
	return abs
}

func TestRepairSharedCoreWorktreeConfig_NoOpOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repairSharedCoreWorktreeConfig(dir); err != nil {
		t.Fatalf("expected no-op outside repo, got %v", err)
	}
}
