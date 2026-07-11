package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryFreshnessRejectsSymlinkedRepoMarkers(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(external, []byte(agentopsModuleLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "cli", "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindAgentopsRepoRoot(root); ok {
		t.Fatal("symlinked go.mod identified an AgentOps checkout")
	}
}

func TestRepoDeclaredVersionRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli", "cmd", "ao")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, (1<<16)+100)
	copy(body[(1<<16)+1:], `var version = "forged"`)
	if err := os.WriteFile(filepath.Join(path, "main.go"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(path, "..", "..", ".."))
	if _, ok := RepoDeclaredVersion(root); ok {
		t.Fatal("oversized main.go yielded a version")
	}
}
