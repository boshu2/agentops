package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func evalSymlinksT(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
}

// TestRepoRootOrCwd_ResolvesFromSubdirectory is the age-6sg.1 regression guard:
// a command reading a repo-rooted artifact (docs/contracts/claim-registry.yaml)
// must resolve the REPO ROOT even when invoked from a subdirectory like cli/.
// Before the fix, resolveProjectDir returned the raw cwd (the subdir) and the
// read failed with "cli/docs/contracts/claim-registry.yaml not found".
func TestRepoRootOrCwd_ResolvesFromSubdirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	subdir := filepath.Join(root, "cli")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Pin the effective project dir to the SUBDIR (as if `ao` ran from cli/).
	orig := testProjectDir
	testProjectDir = subdir
	defer func() { testProjectDir = orig }()

	got, err := repoRootOrCwd()
	if err != nil {
		t.Fatalf("repoRootOrCwd: %v", err)
	}

	// git rev-parse may canonicalize symlinks (e.g. macOS /var → /private/var),
	// so compare via EvalSymlinks rather than raw string equality.
	wantReal := evalSymlinksT(t, root)
	gotReal := evalSymlinksT(t, got)
	if gotReal != wantReal {
		t.Errorf("repoRootOrCwd from subdir = %q, want repo root %q", gotReal, wantReal)
	}
	if gotReal == evalSymlinksT(t, subdir) {
		t.Errorf("repoRootOrCwd returned the subdir %q, not the repo root — the age-6sg.1 bug", subdir)
	}
}

// TestRepoRootOrCwd_FallsBackOutsideRepo verifies that when the dir is NOT in a
// git repo, repoRootOrCwd falls back to the dir itself rather than erroring.
func TestRepoRootOrCwd_FallsBackOutsideRepo(t *testing.T) {
	root := t.TempDir() // a bare temp dir, not a git repo
	orig := testProjectDir
	testProjectDir = root
	defer func() { testProjectDir = orig }()

	got, err := repoRootOrCwd()
	if err != nil {
		t.Fatalf("repoRootOrCwd: %v", err)
	}
	if evalSymlinksT(t, got) != evalSymlinksT(t, root) {
		t.Errorf("repoRootOrCwd outside a repo = %q, want the cwd fallback %q", got, root)
	}
}
