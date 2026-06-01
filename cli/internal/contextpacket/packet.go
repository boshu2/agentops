package contextpacket

import (
	"os"
	"strings"
)

// DetectRepoName returns the repository name by walking up from cwd to find a .git directory.
func DetectRepoName(cwd string) string {
	dir := cwd
	for {
		if info, err := os.Stat(dir + "/.git"); err == nil && info.IsDir() {
			return FileBase(dir)
		}
		if info, err := os.Stat(dir + "/.git"); err == nil && !info.IsDir() {
			// worktree: .git is a file
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
