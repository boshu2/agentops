package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

func TestEvalRunCommandJSON(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	writeEvalCmdFile(t, filepath.Join(dir, "fixture.txt"), "needle\n")
	suitePath := writeEvalCmdSuite(t, dir, `{
  "schema_version": 1,
  "id": "cmd.pass",
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
      "id": "contains",
      "title": "fixture contains needle",
      "kind": "artifact_check",
      "objective": "Score fixture deterministically.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "needle"}
      ]
    }
  ]
}`)

	out, err := executeCommand("eval", "run", suitePath, "--run-id", "cmd-pass", "--json")
	if err != nil {
		t.Fatalf("ao eval run failed: %v\noutput: %s", err, out)
	}
	var run aoeval.RunRecord
	if err := json.Unmarshal([]byte(out), &run); err != nil {
		t.Fatalf("eval run JSON parse failed: %v\noutput: %s", err, out)
	}
	if run.RunID != "cmd-pass" || run.Status != aoeval.StatusPass {
		t.Fatalf("run_id/status = %q/%s, want cmd-pass/pass", run.RunID, run.Status)
	}
}

func TestEvalRunCommandMissingSuiteFails(t *testing.T) {
	withEvalCommand(t)
	out, err := executeCommand("eval", "run", filepath.Join(t.TempDir(), "missing.json"), "--json")
	if err == nil {
		t.Fatalf("ao eval run missing suite succeeded; output: %s", out)
	}
}

func TestEvalCompareCommandJSON(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	writeEvalRunRecord(t, baselinePath, "baseline-run", 0.80)
	writeEvalRunRecord(t, candidatePath, "candidate-run", 0.95)

	out, err := executeCommand("eval", "compare", candidatePath, baselinePath, "--json")
	if err != nil {
		t.Fatalf("ao eval compare failed: %v\noutput: %s", err, out)
	}
	var compared aoeval.RunRecord
	if err := json.Unmarshal([]byte(out), &compared); err != nil {
		t.Fatalf("eval compare JSON parse failed: %v\noutput: %s", err, out)
	}
	if compared.Verdict != aoeval.VerdictImprovement {
		t.Fatalf("verdict = %s, want improvement", compared.Verdict)
	}
	if compared.BaselineComparison == nil || compared.BaselineComparison.BaselineRunID != "baseline-run" {
		t.Fatalf("baseline comparison = %+v", compared.BaselineComparison)
	}
}

func TestEvalBaselineCommandJSON(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	runPath := filepath.Join(dir, "run.json")
	outputPath := filepath.Join(dir, "baseline.json")
	writeEvalRunRecord(t, runPath, "candidate-run", 1.0)

	out, err := executeCommand(
		"eval", "baseline", runPath,
		"--out", outputPath,
		"--promoted-by", "tester",
		"--rationale", "stable canary",
		"--json",
	)
	if err != nil {
		t.Fatalf("ao eval baseline failed: %v\noutput: %s", err, out)
	}
	var promoted aoeval.RunRecord
	if err := json.Unmarshal([]byte(out), &promoted); err != nil {
		t.Fatalf("eval baseline JSON parse failed: %v\noutput: %s", err, out)
	}
	if promoted.Baseline == nil || promoted.Baseline.Mode != aoeval.BaselineModePromote {
		t.Fatalf("baseline metadata = %+v, want promote", promoted.Baseline)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("baseline output not written: %v", err)
	}
}

func writeEvalCmdSuite(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "suite.json")
	writeEvalCmdFile(t, path, body)
	return path
}

func writeEvalCmdFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeEvalRunRecord(t *testing.T, path, runID string, score float64) {
	t.Helper()
	run := aoeval.RunRecord{
		SchemaVersion: 1,
		RunID:         runID,
		Suite: aoeval.SuiteRef{
			ID:         "cmd.pass",
			Path:       "suite.json",
			Visibility: aoeval.VisibilityPublicCanary,
			Tier:       aoeval.TierDeterministic,
		},
		StartedAt: fixedEvalCmdTime(),
		Status:    aoeval.StatusPass,
		Verdict:   aoeval.VerdictPass,
		Git: aoeval.GitRecord{
			CandidateRef: "test",
			CandidateSHA: "0000000",
			Dirty:        false,
		},
		Runtime: aoeval.RuntimeRecord{
			Name: aoeval.RuntimeStatic,
			Live: false,
		},
		Environment: aoeval.EnvironmentRecord{
			ScrubbedEnvPrefixes: []string{"AGENTOPS_RPI_RUNTIME"},
			IsolatedHome:        false,
			IsolatedCodexHome:   false,
			NetworkAccess:       aoeval.NetworkDisabled,
		},
		CaseResults: []aoeval.CaseResult{
			{
				ID:     "case",
				Status: aoeval.StatusPass,
				Score:  score,
				DimensionScores: map[aoeval.Dimension]float64{
					aoeval.DimensionCorrectness: score,
				},
			},
		},
		AggregateScore: score,
		DimensionScores: map[aoeval.Dimension]float64{
			aoeval.DimensionCorrectness: score,
		},
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	writeEvalCmdFile(t, path, string(data))
}

func fixedEvalCmdTime() time.Time {
	return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
}

func withEvalCommand(t *testing.T) {
	t.Helper()
	registerEvalCommand()
}
