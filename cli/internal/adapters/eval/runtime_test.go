package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"gopkg.in/yaml.v3"
)

func TestRuntimeSatisfiesCorePortAndResolvesHostPaths(t *testing.T) {
	var port aoeval.CoreRuntime = Runtime{}
	var _ aoeval.CleanupRuntime = Runtime{}
	var _ aoeval.TaskRuntime = Runtime{}
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

func TestRuntimeTaskServiceDryRunUsesRealFilesystemAdapter(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOPS_EVALS_ROOT", root)
	task := evalsubstrate.Task{SchemaVersion: 1, ID: "task-1", HarnessRef: "harness-1", Stats: evalsubstrate.TaskStat{MinNSamples: 3, DecisionRule: evalsubstrate.DecisionRule{Kind: "bootstrap_ci", Confidence: .95}}}
	suite := evalsubstrate.Suite{SchemaVersion: 1, ID: "suite-1", Kind: "comparison", VariedAxis: evalsubstrate.VariedAxis{Kind: "model", Values: []string{"a", "b"}}, HeldConstant: evalsubstrate.HeldConstant{Task: "task-1", Harness: "harness-1", GroundTruthVersion: "gt-v1"}, SampleSplit: "dev", NSamples: 3, Stats: evalsubstrate.SuiteStat{DecisionRule: evalsubstrate.DecisionRule{Kind: "bootstrap_ci", Confidence: .95}}}
	writeYAML := func(path string, value any) {
		t.Helper()
		data, err := yaml.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeYAML(filepath.Join(root, "tasks", task.ID, "task.yaml"), task)
	writeYAML(filepath.Join(root, "suites", suite.ID, "suite.yaml"), suite)
	gtPath := filepath.Join(root, "ground-truth", "ground-truth.jsonl")
	if err := os.MkdirAll(filepath.Dir(gtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gtPath, []byte("{\"id\":\"gt-1\",\"value\":\"ok\",\"source\":\"test\",\"confidence\":\"strong\",\"split\":\"dev\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := (aoeval.TaskService{Runtime: Runtime{}}).Run(context.Background(), aoeval.TaskRunRequest{TaskID: task.ID, SuiteRef: suite.ID, Seeds: "1,2,3", HarnessRef: "harness-1", GroundTruthRef: "gt-1", DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.DryRun {
		t.Fatalf("result = %#v", result)
	}
}
