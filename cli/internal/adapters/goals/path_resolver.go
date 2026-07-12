package goals

import (
	"os"
	"path/filepath"
)

// PathResolver owns the goals family's ambient current-directory lookup.
// Root is a deterministic test seam; production leaves it empty to use cwd.
type PathResolver struct {
	Root string
}

func (resolver PathResolver) Resolve(explicit string) string {
	if explicit != "" {
		return explicit
	}
	root := resolver.ProjectRoot()
	if regularFile(filepath.Join(root, "GOALS.md")) {
		return "GOALS.md"
	}
	if regularFile(filepath.Join(root, "GOALS.yaml")) {
		return "GOALS.yaml"
	}
	return "GOALS.md"
}

func (resolver PathResolver) ProjectRoot() string {
	if resolver.Root != "" {
		return resolver.Root
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
