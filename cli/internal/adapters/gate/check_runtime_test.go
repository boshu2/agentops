package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

func TestCheckRuntimeAppliesOnlyValidatedRangeScope(t *testing.T) {
	t.Setenv("AGENTOPS_GATE_RANGE", "sentinel")
	runtime := CheckRuntime{}
	if err := runtime.ApplyRangeScope("head"); err != nil {
		t.Fatalf("head: %v", err)
	}
	if got := os.Getenv("AGENTOPS_GATE_RANGE"); got != "sentinel" {
		t.Fatalf("non-range changed env to %q", got)
	}
	if err := runtime.ApplyRangeScope("range:main..HEAD"); err != nil {
		t.Fatalf("range: %v", err)
	}
	if got := os.Getenv("AGENTOPS_GATE_RANGE"); got != "main..HEAD" {
		t.Fatalf("range env = %q", got)
	}
	if err := runtime.ApplyRangeScope("range:HEAD"); err == nil {
		t.Fatal("expected malformed range error")
	}
}

func TestResolveRepoRootFindsLinkedDirectoryRootAndFallsBack(t *testing.T) {
	root := t.TempDir()
	if err := exec.Command("git", "-C", root, "init", "-q").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	subdir := filepath.Join(root, "cli", "cmd")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRepoRoot(subdir)
	canonicalRoot, canonicalErr := filepath.EvalSymlinks(root)
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if err != nil || got != canonicalRoot {
		t.Fatalf("got=%q err=%v want=%q", got, err, root)
	}
	nonRepo := t.TempDir()
	got, err = ResolveRepoRoot(nonRepo)
	if err != nil || got != nonRepo {
		t.Fatalf("fallback got=%q err=%v want=%q", got, err, nonRepo)
	}
}

func TestCheckRuntimeWorkflowCoverageDelegates(t *testing.T) {
	root := t.TempDir()
	workflow := filepath.Join(root, "validate.yml")
	if err := os.WriteFile(workflow, []byte("name: validate\njobs: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	coverage, err := (CheckRuntime{}).WorkflowCoverage(gates.NewRegistry(), root, workflow)
	if err != nil || coverage.WorkflowScriptCount != 0 {
		t.Fatalf("coverage=%+v err=%v", coverage, err)
	}
}
