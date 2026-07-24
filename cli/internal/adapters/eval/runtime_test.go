package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"gopkg.in/yaml.v3"
)

// TestSaveBurnLedger_FileMode0600 (age-6j9ee.3): the holdout burn ledger pins 0o600 —
// holdout secrecy is load-bearing, so a world/group-readable ledger (which leaks which
// holdout scenarios have been spent) is a regression. POSIX-mode assert is skipped on
// windows via a GOOS branch (not t.Skip) so the write itself is still exercised there.
func TestSaveBurnLedger_FileMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "burn.json")
	if err := (Runtime{}).SaveBurnLedger(path, evalsubstrate.HoldoutBurnLedger{Budget: 1}); err != nil {
		t.Fatalf("SaveBurnLedger: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("burn ledger mode = %o, want 0600 (holdout secrecy)", got)
	}
}

func TestRuntimeSatisfiesCorePortAndResolvesHostPaths(t *testing.T) {
	var port aoeval.CoreRuntime = Runtime{}
	var _ aoeval.CleanupRuntime = Runtime{}
	var _ aoeval.TaskRuntime = Runtime{}
	var _ aoeval.SuiteRuntime = Runtime{}
	var _ aoeval.OutcomesRuntime = Runtime{}
	var _ aoeval.ScenarioRuntime = Runtime{}
	var _ aoeval.ScenarioABRuntime = Runtime{}
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
	if _, err := (aoeval.TaskService{Runtime: Runtime{}}).Run(context.Background(), aoeval.TaskRunRequest{
		TaskID: task.ID, SuiteRef: suite.ID, Seeds: "1,2,3", HarnessRef: "harness-1",
		GroundTruthRef: "gt-1", ModelSpecID: "../escape", DryRun: true,
	}); err == nil {
		t.Fatal("dry run accepted hostile --model-spec id")
	}
}

func TestRuntimeTaskServiceRejectsSymlinkParentEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	t.Setenv("AGENTOPS_EVALS_ROOT", root)
	if err := os.Symlink(outside, filepath.Join(root, "tasks")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	source := filepath.Join(t.TempDir(), "task.yaml")
	data, err := yaml.Marshal(evalsubstrate.Task{SchemaVersion: 1, ID: "task-escape", Stats: evalsubstrate.TaskStat{MinNSamples: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (aoeval.TaskService{Runtime: Runtime{}}).Add(context.Background(), aoeval.TaskAddRequest{SourcePath: source}); err == nil {
		t.Fatal("TaskService.Add through escaping symlink unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "task-escape", "task.yaml")); !os.IsNotExist(err) {
		t.Fatalf("outside task was created: %v", err)
	}
}

func TestRuntimeDeleteRunRejectsHostileIDAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "runs")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	runtime := Runtime{}
	if err := runtime.DeleteRun(root, "../escape"); err == nil {
		t.Fatal("DeleteRun accepted traversal")
	}
	if err := runtime.DeleteRun(root, "run-escape"); err == nil {
		t.Fatal("DeleteRun through escaping symlink unexpectedly succeeded")
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep" {
		t.Fatalf("outside sentinel changed: content=%q err=%v", got, err)
	}
}

func TestRuntimeLegacyColonRunCompatibilityStaysOnBoundedLegacyPath(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "runs", "Run:legacy")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := evalsubstrate.Manifest{
		SchemaVersion:   1,
		ID:              "Run:legacy",
		Status:          evalsubstrate.StatusPending,
		StartedAtUnixMs: 1,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	runtime := Runtime{}
	if err := runtime.Transition(root, "Run:legacy", evalsubstrate.StatusAborted, "test", time.Unix(10, 0)); err != nil {
		t.Fatalf("Transition legacy run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "runs", "Run%3Alegacy")); !os.IsNotExist(err) {
		t.Fatalf("transition created a competing canonical directory: %v", err)
	}
	loaded, err := evalsubstrate.LoadManifest(root, "Run:legacy")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != evalsubstrate.StatusAborted {
		t.Fatalf("status = %s, want aborted", loaded.Status)
	}
	if err := runtime.DeleteRun(root, "Run:legacy"); err != nil {
		t.Fatalf("DeleteRun legacy run: %v", err)
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("legacy run directory was not removed: %v", err)
	}
}

func TestSaveBurnLedger_StaleTempFile0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "burn-ledger.json")

	// Seed the exact stale state the validator probe pre-creates: a leftover
	// path+".tmp" at 0644, and a pre-existing final ledger at 0644.
	if err := os.WriteFile(path+".tmp", []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed stale temp: %v", err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed stale ledger: %v", err)
	}

	ledger := evalsubstrate.HoldoutBurnLedger{Budget: 5}
	if err := (Runtime{}).SaveBurnLedger(path, ledger); err != nil {
		t.Fatalf("SaveBurnLedger: %v", err)
	}
	assertLedgerMode0600(t, path)

	// The persisted content must be the new ledger, not the stale seed.
	got, err := (Runtime{}).LoadBurnLedger(path)
	if err != nil {
		t.Fatalf("LoadBurnLedger: %v", err)
	}
	if got.Budget != 5 {
		t.Fatalf("ledger budget = %d, want 5", got.Budget)
	}
}

func TestSaveBurnLedger_ConcurrentWritersDoNotCollide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "burn-ledger.json")
	const writers = 8
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = (Runtime{}).SaveBurnLedger(path, evalsubstrate.HoldoutBurnLedger{Budget: i})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("SaveBurnLedger[%d]: %v", i, err)
		}
	}
	assertLedgerMode0600(t, path)
	if _, err := (Runtime{}).LoadBurnLedger(path); err != nil {
		t.Fatalf("final ledger unreadable: %v", err)
	}
}

func assertLedgerMode0600(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat ledger %s: %v", path, err)
	}
	if runtime.GOOS == "windows" {
		if info.IsDir() {
			t.Fatalf("ledger %s is a directory, want a file", path)
		}
		return
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("ledger mode = %#o, want 0o600", got)
	}
}
