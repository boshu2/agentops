// practices: [microservices, team-topologies]
package worktreeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTrustedGitBin proves git is resolved on a PATH that excludes everything the repo
// containing cwd could control — relative entries, ".", and any absolute dir inside the
// ENCLOSING REPO ROOT — even when ao runs from a subdirectory; and that it returns "" (fail
// closed) rather than bare "git" when no trusted git exists.
func TestTrustedGitBin(t *testing.T) {
	sep := string(os.PathListSeparator)

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Hostile git in a repo-internal bin dir (ABOVE the subdir we run from).
	repoBin := filepath.Join(repo, "bin")
	if err := os.MkdirAll(repoBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoBin, "git"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A real system bin dir (outside the repo) with git.
	sys := t.TempDir()
	sysGit := filepath.Join(sys, "git")
	if err := os.WriteFile(sysGit, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("subdir excludes enclosing-repo bin and prefers system git", func(t *testing.T) {
		t.Setenv("PATH", strings.Join([]string{".", repoBin, sys}, sep))
		if got := trustedGitBin(sub); got != sysGit {
			t.Fatalf("trustedGitBin(sub) = %q, want %q (must skip repo-internal %q + \".\")", got, sysGit, filepath.Join(repoBin, "git"))
		}
	})

	t.Run("no trusted git -> empty (fail closed, never bare git)", func(t *testing.T) {
		// Only the repo-internal + relative dirs on PATH -> nothing trusted.
		t.Setenv("PATH", strings.Join([]string{".", repoBin, "rel/bin"}, sep))
		if got := trustedGitBin(sub); got != "" {
			t.Fatalf("trustedGitBin with only untrusted dirs = %q, want \"\" (fail closed)", got)
		}
	})
}
