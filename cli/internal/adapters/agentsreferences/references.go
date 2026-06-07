// practices: [wiki-knowledge-surface, design-by-contract]
package agentsreferences

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ScanProduction returns the set of top-level .agents/<name> subdirs
// referenced from production code. It mirrors the shell scanner in
// scripts/check-agents-write-surfaces.sh so read-side and write-side gates use
// the same accepted reference syntax.
func ScanProduction(repoRoot string) (map[string]bool, error) {
	literalRe := regexp.MustCompile(`\.agents/([a-z][a-zA-Z0-9_-]*)`)
	joinRe := regexp.MustCompile(`filepath\.Join\([^)]*"\.agents"[[:space:]]*,[[:space:]]*"([a-z][a-zA-Z0-9_-]*)"`)
	found := map[string]bool{}

	walkOne := func(rootDir string, isProductionFile func(path string, d fs.DirEntry) bool) error {
		root := filepath.Join(repoRoot, rootDir)
		if _, err := os.Stat(root); err != nil {
			return nil
		}
		rootFS := os.DirFS(root)
		return fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !isProductionFile(path, d) {
				return nil
			}
			data, readErr := fs.ReadFile(rootFS, path)
			if readErr != nil {
				return nil
			}
			for _, m := range literalRe.FindAllSubmatch(data, -1) {
				found[string(m[1])] = true
			}
			for _, m := range joinRe.FindAllSubmatch(data, -1) {
				found[string(m[1])] = true
			}
			return nil
		})
	}

	if err := walkOne("cli", func(path string, d fs.DirEntry) bool {
		name := d.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}); err != nil {
		return nil, err
	}

	for _, dir := range []string{"scripts", "hooks", "lib"} {
		if err := walkOne(dir, func(path string, d fs.DirEntry) bool {
			name := d.Name()
			return strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".bash")
		}); err != nil {
			return nil, err
		}
	}

	return found, nil
}
