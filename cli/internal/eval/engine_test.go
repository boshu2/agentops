package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunSuiteArtifactCheckPasses(t *testing.T) {
	dir := t.TempDir()
	writeEvalFile(t, filepath.Join(dir, "fixture.txt"), "alpha\nneedle\nomega\n")
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "fixture.pass",
  "name": "Fixture pass",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "contains",
      "title": "fixture contains needle",
      "kind": "artifact_check",
      "objective": "Verify static fixtures are scored offline.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "needle"}
      ]
    }
  ]
}`)

	run, err := RunSuite(RunOptions{
		SuitePath: suitePath,
		RunID:     "run-pass",
		Now:       fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunSuite returned error: %v", err)
	}

	if run.Status != StatusPass || run.Verdict != VerdictPass {
		t.Fatalf("status/verdict = %s/%s, want pass/pass", run.Status, run.Verdict)
	}
	if run.AggregateScore != 1 {
		t.Fatalf("aggregate_score = %v, want 1", run.AggregateScore)
	}
	if got := run.DimensionScores["correctness"]; got != 1 {
		t.Fatalf("correctness score = %v, want 1", got)
	}
	if len(run.CaseResults) != 1 || run.CaseResults[0].Score != 1 {
		t.Fatalf("case results = %+v, want one passing case", run.CaseResults)
	}
	if len(run.Suite.SHA256) != 64 {
		t.Fatalf("suite sha length = %d, want 64", len(run.Suite.SHA256))
	}
}

func TestRunSuiteCommandCasePasses(t *testing.T) {
	if os.Getenv("GO_WANT_EVAL_HELPER_PROCESS") == "1" {
		fmt.Print(`{"ok":true,"message":"hello from helper"}`)
		os.Exit(0)
	}

	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	suitePath := writeEvalSuite(t, dir, fmt.Sprintf(`{
  "schema_version": 1,
  "id": "command.pass",
  "name": "Command pass",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "helper",
      "title": "helper command emits JSON",
      "kind": "command",
      "objective": "Run a local deterministic command.",
      "runtime": "shell",
      "inputs": {
        "argv": [%q, "-test.run=TestRunSuiteCommandCasePasses"],
        "env": {"GO_WANT_EVAL_HELPER_PROCESS": "1"}
      },
      "expectations": [
        {"type": "exit_code", "value": 0},
        {"type": "stdout_contains", "value": "hello"},
        {"type": "json_path", "target": "stdout.ok", "value": true}
      ]
    }
  ]
}`, exe))

	run, err := RunSuite(RunOptions{
		SuitePath: suitePath,
		RunID:     "run-command",
		Now:       fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("RunSuite returned error: %v", err)
	}
	if run.Status != StatusPass {
		t.Fatalf("status = %s, want pass; result=%+v", run.Status, run.CaseResults)
	}
	if run.Runtime.Name != RuntimeShell {
		t.Fatalf("runtime = %s, want shell", run.Runtime.Name)
	}
}

func TestRunSuiteExpectationFailureFailsRun(t *testing.T) {
	dir := t.TempDir()
	writeEvalFile(t, filepath.Join(dir, "fixture.txt"), "safe text\n")
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "fixture.fail",
  "name": "Fixture fail",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "missing",
      "title": "fixture lacks needle",
      "kind": "artifact_check",
      "objective": "Required expectations fail the case.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "needle"}
      ]
    }
  ]
}`)

	run, err := RunSuite(RunOptions{SuitePath: suitePath, RunID: "run-fail", Now: fixedEvalTime})
	if err != nil {
		t.Fatalf("RunSuite returned error: %v", err)
	}
	if run.Status != StatusFail || run.Verdict != VerdictFail {
		t.Fatalf("status/verdict = %s/%s, want fail/fail", run.Status, run.Verdict)
	}
	if run.CaseResults[0].FailureMessage == "" {
		t.Fatalf("expected failure message, got %+v", run.CaseResults[0])
	}
}

func TestRunSuiteRejectsLiveTier(t *testing.T) {
	dir := t.TempDir()
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "live.out.of.scope",
  "name": "Live suite",
  "domain": "runtime",
  "visibility": "public_canary",
  "tier": "live",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "prompt",
      "title": "runtime prompt",
      "kind": "runtime_prompt",
      "objective": "Live adapters are not deterministic.",
      "runtime": "claude",
      "expectations": [
        {"type": "manual_review"}
      ]
    }
  ]
}`)

	if _, err := RunSuite(RunOptions{SuitePath: suitePath, RunID: "run-live", Now: fixedEvalTime}); err == nil {
		t.Fatal("RunSuite succeeded for live tier, want error")
	}
}

func TestCompareRunsMarksRegression(t *testing.T) {
	baseline := minimalRunRecord("baseline-run", 0.90, map[Dimension]float64{DimensionCorrectness: 0.90})
	candidate := minimalRunRecord("candidate-run", 0.70, map[Dimension]float64{DimensionCorrectness: 0.70})

	compared, err := CompareRuns(candidate, baseline, CompareOptions{})
	if err != nil {
		t.Fatalf("CompareRuns returned error: %v", err)
	}

	if compared.Verdict != VerdictRegression || compared.Status != StatusPass {
		t.Fatalf("status/verdict = %s/%s, want pass/regression", compared.Status, compared.Verdict)
	}
	if compared.BaselineComparison == nil {
		t.Fatal("expected baseline comparison")
	}
	if got := compared.BaselineComparison.AggregateDelta; got != -0.2 {
		t.Fatalf("aggregate delta = %v, want -0.2", got)
	}
	if len(compared.BaselineComparison.Regressions) != 1 {
		t.Fatalf("regressions = %+v, want one regression", compared.BaselineComparison.Regressions)
	}
}

func TestPromoteBaselineWritesRunRecord(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "baseline.json")
	run := minimalRunRecord("candidate-run", 1, map[Dimension]float64{DimensionCorrectness: 1})

	promoted, err := PromoteBaseline(run, BaselineOptions{
		OutputPath: outputPath,
		PromotedBy: "tester",
		Rationale:  "stable deterministic canary",
		Now:        fixedEvalTime,
	})
	if err != nil {
		t.Fatalf("PromoteBaseline returned error: %v", err)
	}

	if promoted.Baseline == nil || promoted.Baseline.Mode != BaselineModePromote {
		t.Fatalf("baseline metadata = %+v, want promote", promoted.Baseline)
	}
	if promoted.Baseline.PromotedBy != "tester" {
		t.Fatalf("promoted_by = %q, want tester", promoted.Baseline.PromotedBy)
	}
	loaded, err := LoadRun(outputPath)
	if err != nil {
		t.Fatalf("LoadRun(%s): %v", outputPath, err)
	}
	if loaded.RunID != promoted.RunID {
		t.Fatalf("loaded run_id = %q, want %q", loaded.RunID, promoted.RunID)
	}
}

func fixedEvalTime() time.Time {
	return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
}

func writeEvalSuite(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "suite.json")
	writeEvalFile(t, path, body)
	return path
}

func writeEvalFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func minimalRunRecord(runID string, aggregate float64, dimensions map[Dimension]float64) *RunRecord {
	return &RunRecord{
		SchemaVersion: 1,
		RunID:         runID,
		Suite: SuiteRef{
			ID:         "fixture.pass",
			Path:       "suite.json",
			Visibility: VisibilityPublicCanary,
			Tier:       TierDeterministic,
		},
		StartedAt: fixedEvalTime(),
		Status:    StatusPass,
		Verdict:   VerdictPass,
		Git: GitRecord{
			CandidateRef: "test",
			CandidateSHA: "0000000",
			Dirty:        false,
		},
		Runtime: RuntimeRecord{
			Name: RuntimeStatic,
			Live: false,
		},
		Environment: EnvironmentRecord{
			ScrubbedEnvPrefixes: []string{"AGENTOPS_RPI_RUNTIME"},
			IsolatedHome:        false,
			IsolatedCodexHome:   false,
			NetworkAccess:       NetworkDisabled,
		},
		CaseResults: []CaseResult{
			{
				ID:              "case",
				Status:          StatusPass,
				Score:           aggregate,
				DimensionScores: dimensions,
			},
		},
		AggregateScore:  aggregate,
		DimensionScores: dimensions,
	}
}

func TestRunRecordJSONRoundTrip(t *testing.T) {
	run := minimalRunRecord("roundtrip", 1, map[Dimension]float64{DimensionCorrectness: 1})
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded RunRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.RunID != run.RunID {
		t.Fatalf("decoded run_id = %q, want %q", decoded.RunID, run.RunID)
	}
}
