// practices: [microservices, team-topologies]
package worktreeconfig

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

	if err := RepairSharedCoreWorktreeConfig(nestedDir); err != nil {
		t.Fatalf("RepairSharedCoreWorktreeConfig: %v", err)
	}
	if err := SanitizeGitProcessEnv(); err != nil {
		t.Fatalf("SanitizeGitProcessEnv: %v", err)
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

func TestRepairSharedCoreWorktreeConfig_NoOpOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RepairSharedCoreWorktreeConfig(dir); err != nil {
		t.Fatalf("expected no-op outside repo, got %v", err)
	}
}

// initTestRepo creates a real git repo with one commit in a temp dir and
// returns its root. It skips the test when git is unavailable.
func initTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "seed")
	return root
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
