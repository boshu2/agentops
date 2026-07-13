package checks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".github", "workflows", "validate.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repo root with .github/workflows/validate.yml not found (out-of-tree build)")
		}
		dir = parent
	}
}

// TestRegistryBackingScriptsExist asserts every seeded Backing script still
// exists on disk. The legacy bash-gate coverage net was retired with
// scripts/pre-push-gate.sh; validate.yml-vs-registry coverage lives in
// workflow_coverage(_test).go and ao gate check --require-workflow-parity.
func TestRegistryBackingScriptsExist(t *testing.T) {
	root := repoRootFromTest(t)
	for _, c := range gates.Default.All() {
		if c.Backing == "" {
			continue
		}
		path := filepath.Join(root, c.ArtifactPath())
		if _, err := os.Stat(path); err != nil {
			t.Errorf("registry check %q backs missing script %s: %v", c.ID, path, err)
		}
	}
}
