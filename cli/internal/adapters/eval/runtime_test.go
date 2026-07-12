package eval

import (
	"path/filepath"
	"testing"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

func TestRuntimeSatisfiesCorePortAndResolvesHostPaths(t *testing.T) {
	var port aoeval.CoreRuntime = Runtime{}
	var _ aoeval.CleanupRuntime = Runtime{}
	workDir, err := port.WorkDir()
	if err != nil {
		t.Fatalf("WorkDir: %v", err)
	}
	if !filepath.IsAbs(workDir) {
		t.Fatalf("WorkDir = %q, want absolute", workDir)
	}
	absolute, err := port.Abs("suite.json")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if !filepath.IsAbs(absolute) {
		t.Fatalf("Abs = %q, want absolute", absolute)
	}
}
