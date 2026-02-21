package rpi

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureAttachedBranch_DetachedHEAD(t *testing.T) {
	repo := initGitRepo(t)

	sha := runGitOutput(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "checkout", strings.TrimSpace(sha))

	branch, healed, err := EnsureAttachedBranch(repo, 30*time.Second, "codex/auto-rpi")
	if err != nil {
		t.Fatalf("EnsureAttachedBranch: %v", err)
	}
	if !healed {
		t.Fatal("expected detached HEAD to be healed")
	}
	if !strings.HasPrefix(branch, "codex/auto-rpi-") {
		t.Fatalf("unexpected healed branch: %q", branch)
	}

	current, err := GetCurrentBranch(repo, 30*time.Second)
	if err != nil {
		t.Fatalf("GetCurrentBranch after heal: %v", err)
	}
	if current == "HEAD" {
		t.Fatal("expected named branch after heal")
	}
}

func TestEnsureAttachedBranch_NoopOnNamedBranch(t *testing.T) {
	repo := initGitRepo(t)

	current, err := GetCurrentBranch(repo, 30*time.Second)
	if err != nil {
		t.Fatalf("GetCurrentBranch: %v", err)
	}

	branch, healed, err := EnsureAttachedBranch(repo, 30*time.Second, "codex/auto-rpi")
	if err != nil {
		t.Fatalf("EnsureAttachedBranch: %v", err)
	}
	if healed {
		t.Fatal("expected no heal on named branch")
	}
	if branch != current {
		t.Fatalf("branch mismatch: got %q want %q", branch, current)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial")
	return dir
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func runGitOutput(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s output failed: %v", strings.Join(args, " "), err)
	}
	return string(out)
}
