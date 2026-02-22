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
	initialBranch, err := GetCurrentBranch(repo, 30*time.Second)
	if err != nil {
		t.Fatalf("GetCurrentBranch initial: %v", err)
	}

	sha := runGitOutput(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "checkout", strings.TrimSpace(sha))

	branch, healed, err := EnsureAttachedBranch(repo, 30*time.Second, "codex/auto-rpi")
	if err != nil {
		t.Fatalf("EnsureAttachedBranch: %v", err)
	}
	if !healed {
		t.Fatal("expected detached HEAD to be healed")
	}
	if branch != "codex/auto-rpi-recovery" {
		t.Fatalf("unexpected healed branch: %q", branch)
	}

	runGit(t, repo, "checkout", "--detach", strings.TrimSpace(sha))

	branch, healed, err = EnsureAttachedBranch(repo, 30*time.Second, "codex/auto-rpi")
	if err != nil {
		t.Fatalf("EnsureAttachedBranch: %v", err)
	}
	if !healed {
		t.Fatal("expected second detached heal to reuse stable branch")
	}
	if branch != "codex/auto-rpi-recovery" {
		t.Fatalf("unexpected healed branch on second run: %q", branch)
	}

	currentBranch, err := GetCurrentBranch(repo, 30*time.Second)
	if err != nil {
		t.Fatalf("GetCurrentBranch after heal: %v", err)
	}
	if currentBranch == "" {
		t.Fatal("expected current branch after named checkout")
	}
	if currentBranch != "codex/auto-rpi-recovery" {
		t.Fatalf("expected recovery branch, got %q", currentBranch)
	}
	baseBranch := initialBranch

	branches := listBranches(t, repo, "codex/auto-rpi-*")
	if len(branches) != 1 {
		t.Fatalf("expected one recovery branch, found %d (%v)", len(branches), branches)
	}
	if branches[0] != "codex/auto-rpi-recovery" {
		t.Fatalf("expected only codex/auto-rpi-recovery, got %q", branches[0])
	}

	runGit(t, repo, "checkout", baseBranch)
	currentBranch, err = GetCurrentBranch(repo, 30*time.Second)
	if err != nil {
		t.Fatalf("GetCurrentBranch after checkout %s: %v", baseBranch, err)
	}
	if currentBranch != baseBranch {
		t.Fatalf("expected %s after checkout, got %q", baseBranch, currentBranch)
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

func TestEnsureAttachedBranch_DetachedHEAD_WorktreeConflictFallsBackDetached(t *testing.T) {
	repo := initGitRepo(t)

	worktreeRoot := t.TempDir()
	conflictingBranch := "codex/auto-rpi-recovery"
	runGit(t, repo, "branch", "-f", conflictingBranch, "HEAD")

	conflictPath := filepath.Join(worktreeRoot, "conflict")
	if err := os.MkdirAll(conflictPath, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "worktree", "add", conflictPath, conflictingBranch)
	defer runGitIgnoreError(t, repo, "worktree", "remove", "--force", conflictPath)

	runGit(t, repo, "checkout", "--detach", "HEAD")

	branch, healed, err := EnsureAttachedBranch(repo, 30*time.Second, "codex/auto-rpi")
	if err != nil {
		t.Fatalf("EnsureAttachedBranch: %v", err)
	}
	if healed {
		t.Fatal("expected no recovery branch switch when branch is used by another worktree")
	}
	if branch != "" {
		t.Fatalf("expected detached path with no switch, got %q", branch)
	}

	if _, err := GetCurrentBranch(repo, 30*time.Second); err == nil {
		t.Fatal("expected repository to remain detached when recovery branch is unavailable")
	}

	branches := listBranches(t, repo, "codex/auto-rpi-*")
	if len(branches) != 1 {
		t.Fatalf("expected one recovery branch pattern entry, found %d (%v)", len(branches), branches)
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

func listBranches(t *testing.T, cwd string, pattern string) []string {
	t.Helper()
	cmd := exec.Command("git", "branch", "--list", pattern)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list %q failed: %v\n%s", pattern, err, string(out))
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "* ") {
			line = strings.TrimPrefix(line, "* ")
		}
		branches = append(branches, line)
	}
	return branches
}

func runGitIgnoreError(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	_ = cmd.Run()
}
