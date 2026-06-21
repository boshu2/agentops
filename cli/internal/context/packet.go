package context

import (
	"os"
	"strings"
)

// DetectRepoName returns the repository name by walking up from cwd to find a .git directory.
func DetectRepoName(cwd string) string {
	dir := cwd
	for {
		// A repo root has a .git — a DIRECTORY in the canonical checkout, a FILE
		// (gitdir pointer) in a linked worktree. Either marks the root, so a single
		// existence check suffices (the prior code stat'd twice for the same result).
		if _, err := os.Stat(dir + "/.git"); err == nil {
			return FileBase(dir)
		}
		parent := dir[:max(strings.LastIndex(dir, "/"), 0)]
		if parent == "" || parent == dir {
			break
		}
		dir = parent
	}
	return FileBase(cwd)
}

// FileBase returns the last path component.
func FileBase(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
