package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectSearchRoots_UsesGitWorktreeList(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(t.TempDir(), "repo-rpi-test")

	runGit(t, repoRoot, "init", "-q")
	runGit(t, repoRoot, "config", "user.email", "test@example.com")
	runGit(t, repoRoot, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("seed\n"), 0644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-q", "-m", "seed")
	runGit(t, repoRoot, "worktree", "add", "-q", "-b", "rpi/test-search-roots", worktreeRoot, "HEAD")

	roots := collectSearchRoots(repoRoot)
	if !containsPath(roots, repoRoot) {
		t.Fatalf("expected roots to include repo root %q, got %v", repoRoot, roots)
	}
	if !containsPath(roots, worktreeRoot) {
		t.Fatalf("expected roots to include worktree root %q, got %v", worktreeRoot, roots)
	}
	if countPath(roots, repoRoot) != 1 {
		t.Fatalf("expected repo root once, got %d occurrences in %v", countPath(roots, repoRoot), roots)
	}
}

func TestCollectSearchRoots_FallbackSiblingGlob(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "project")
	sibling := filepath.Join(parent, "project-rpi-abc")

	if err := os.MkdirAll(cwd, 0755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	if err := os.MkdirAll(sibling, 0755); err != nil {
		t.Fatalf("mkdir sibling: %v", err)
	}

	roots := collectSearchRoots(cwd)
	if !containsPath(roots, cwd) {
		t.Fatalf("expected roots to include cwd %q, got %v", cwd, roots)
	}
	if !containsPath(roots, sibling) {
		t.Fatalf("expected roots to include sibling fallback %q, got %v", sibling, roots)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func containsPath(paths []string, target string) bool {
	return countPath(paths, target) > 0
}

func countPath(paths []string, target string) int {
	targetNorm := normalizeTestPath(target)
	count := 0
	for _, p := range paths {
		if normalizeTestPath(p) == targetNorm {
			count++
		}
	}
	return count
}

func normalizeTestPath(path string) string {
	clean := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	if abs, err := filepath.Abs(clean); err == nil {
		return filepath.Clean(abs)
	}
	return clean
}
