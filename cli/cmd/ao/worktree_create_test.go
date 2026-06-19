package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// worktreeFixtureRepo builds a throwaway git repo with one commit on `main`.
func worktreeFixtureRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestWorktreeCreate_CreatesIsolatedWorktree(t *testing.T) {
	repo := worktreeFixtureRepo(t)
	t.Chdir(repo)

	out, err := executeCommand("worktree", "create")
	if err != nil {
		t.Fatalf("worktree create failed: %v\noutput: %s", err, out)
	}
	path := strings.TrimSpace(out)
	if path == "" {
		t.Fatal("expected a worktree path on stdout, got empty")
	}
	t.Cleanup(func() {
		rm := exec.Command("git", "worktree", "remove", "--force", path)
		rm.Dir = repo
		_ = rm.Run()
	})

	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("expected created worktree dir at %q, stat err=%v", path, statErr)
	}
	// A linked worktree carries a .git FILE (gitdir pointer), not a directory —
	// the exact property `ao orchestrate preflight` checks for isolation.
	gitMarker := filepath.Join(path, ".git")
	fi, e := os.Stat(gitMarker)
	if e != nil || fi.IsDir() {
		t.Fatalf("expected %q to be a linked worktree (.git file, not dir), err=%v", gitMarker, e)
	}
}
