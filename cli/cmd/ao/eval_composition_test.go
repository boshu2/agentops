package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalEvalSuite is a self-contained deterministic suite: one static
// artifact_contains case over a fixture file, no network, no subprocesses.
const minimalEvalSuite = `{
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
}`

func writeMinimalSuite(t *testing.T) (dir, suitePath string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.txt"), []byte("alpha\nneedle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	suitePath = filepath.Join(dir, "suite.json")
	if err := os.WriteFile(suitePath, []byte(minimalEvalSuite), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, suitePath
}

// TestEvalRun_DeterministicSuite exercises the full production wiring
// (composition -> module -> CoreService -> adapter Runtime -> engine) over a
// real on-disk suite and asserts the durable run record.
func TestEvalRun_DeterministicSuite(t *testing.T) {
	dir, suitePath := writeMinimalSuite(t)
	out := filepath.Join(dir, "run.json")

	output, err := executeCommand("eval", "run", suitePath, "--run-id", "run-1", "--out", out)
	if err != nil {
		t.Fatalf("ao eval run failed: %v\n%s", err, output)
	}

	payload, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("run record not written: %v", err)
	}
	var record struct {
		SchemaVersion  int     `json:"schema_version"`
		RunID          string  `json:"run_id"`
		Status         string  `json:"status"`
		Verdict        string  `json:"verdict"`
		AggregateScore float64 `json:"aggregate_score"`
	}
	if err := json.Unmarshal(payload, &record); err != nil {
		t.Fatalf("run record is not JSON: %v", err)
	}
	if record.RunID != "run-1" || record.Status != "pass" || record.Verdict != "pass" {
		t.Fatalf("run record = %+v, want run-1/pass/pass", record)
	}
	if record.AggregateScore != 1 {
		t.Fatalf("aggregate_score = %v, want 1", record.AggregateScore)
	}
}

// TestEvalCompare_IdenticalRunsPass compares a run record against itself:
// zero regression must yield a pass verdict.
func TestEvalCompare_IdenticalRunsPass(t *testing.T) {
	dir, suitePath := writeMinimalSuite(t)
	out := filepath.Join(dir, "run.json")
	if output, err := executeCommand("eval", "run", suitePath, "--run-id", "run-1", "--out", out); err != nil {
		t.Fatalf("seed run failed: %v\n%s", err, output)
	}

	output, err := executeCommand("eval", "compare", out, out)
	if err != nil {
		t.Fatalf("ao eval compare failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "pass") {
		t.Fatalf("compare output does not report pass:\n%s", output)
	}
}

// TestEvalHelp_PublishesMeasurementSurface pins the top-level subcommand set.
func TestEvalHelp_PublishesMeasurementSurface(t *testing.T) {
	output, err := executeCommand("eval", "--help")
	if err != nil {
		t.Fatalf("ao eval --help failed: %v", err)
	}
	for _, subcommand := range []string{"run", "compare", "baseline", "scorecard", "coverage", "task", "suite", "outcomes", "scenario"} {
		if !strings.Contains(output, subcommand) {
			t.Errorf("ao eval --help missing subcommand %q", subcommand)
		}
	}
	// The retired alias seats must not resurface: their production
	// implementations were removed with the legacy knowledge stack.
	for _, retired := range []string{"session-outcome", "chaos", "bench"} {
		if strings.Contains(output, retired) {
			t.Errorf("ao eval --help exposes retired alias %q", retired)
		}
	}
}
