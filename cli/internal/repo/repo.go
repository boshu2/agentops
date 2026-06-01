// Package repo provides small, shared helpers for discovering repository
// structure: the git root and the presence of .git/.agents marker directories.
//
// These helpers consolidate logic that was previously duplicated as the
// unexported findGitRoot walker plus a scattering of ad-hoc
// os.Stat(filepath.Join(dir, ".git" | ".agents")) existence checks across
// cmd/ao. Behavior is intentionally identical to the original implementations.
package repo

import (
	"os"
	"path/filepath"
)

// FindRoot walks up the directory tree starting at start, returning the first
// directory that contains a ".git" entry. It returns "" if no such directory
// is found before reaching the filesystem root. The check uses os.Stat, so a
// ".git" file (as written by git worktrees) counts the same as a ".git"
// directory.
func FindRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// HasGit reports whether dir directly contains a ".git" entry (file or
// directory). It does not walk up the tree; use FindRoot for that.
func HasGit(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// HasAgents reports whether dir directly contains a ".agents" entry.
func HasAgents(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".agents"))
	return err == nil
}
