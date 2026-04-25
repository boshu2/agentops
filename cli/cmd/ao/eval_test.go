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

func TestEvalCompareCommandJSONIncludesZeroAggregateDelta(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	writeEvalRunRecord(t, baselinePath, "baseline-run", 1.0)
	writeEvalRunRecord(t, candidatePath, "candidate-run", 1.0)

	out, err := executeCommand("eval", "compare", candidatePath, baselinePath, "--json")
	if err != nil {
		t.Fatalf("ao eval compare failed: %v\noutput: %s", err, out)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("eval compare JSON parse failed: %v\noutput: %s", err, out)
	}
	comparison, ok := raw["baseline_comparison"].(map[string]any)
	if !ok {
		t.Fatalf("baseline_comparison missing or invalid: %#v", raw["baseline_comparison"])
	}
	if delta, ok := comparison["aggregate_delta"].(float64); !ok || delta != 0 {
		t.Fatalf("aggregate_delta = %#v, want explicit 0", comparison["aggregate_delta"])
	}
}

func TestEvalScorecardCommandJSON(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	writeEvalScorecardRunRecord(t, candidatePath, "candidate-run", map[string]float64{
		"artifact-completeness": 1,
		"phase-order":           1,
		"objective-spine":       1,
		"validation-separation": 1,
		"scenario-satisfaction": 1,
		"runtime-safety":        1,
	})
	writeEvalScorecardRunRecord(t, baselinePath, "baseline-run", map[string]float64{
		"artifact-completeness": 0.8,
		"phase-order":           1,
		"objective-spine":       1,
		"validation-separation": 1,
		"scenario-satisfaction": 1,
		"runtime-safety":        1,
	})

	out, err := executeCommand("eval", "scorecard", candidatePath, baselinePath, "--kind", "rpi", "--json")
	if err != nil {
		t.Fatalf("ao eval scorecard failed: %v\noutput: %s", err, out)
	}
	var scorecard aoeval.Scorecard
	if err := json.Unmarshal([]byte(out), &scorecard); err != nil {
		t.Fatalf("eval scorecard JSON parse failed: %v\noutput: %s", err, out)
	}
	if scorecard.Kind != aoeval.ScorecardKindRPI {
		t.Fatalf("kind = %s, want rpi", scorecard.Kind)
	}
	if scorecard.CandidateRunID != "candidate-run" || scorecard.BaselineRunID != "baseline-run" {
		t.Fatalf("run ids = %q/%q, want candidate-run/baseline-run", scorecard.CandidateRunID, scorecard.BaselineRunID)
	}
	if scorecard.Verdict != aoeval.VerdictImprovement {
		t.Fatalf("verdict = %s, want improvement", scorecard.Verdict)
	}
	if len(scorecard.Categories) != 6 {
		t.Fatalf("category count = %d, want 6", len(scorecard.Categories))
	}
	category := evalScorecardCategory(t, scorecard, "artifact completeness")
	if category.CandidateScore != 1 {
		t.Fatalf("artifact completeness candidate_score = %v, want 1", category.CandidateScore)
	}
	if category.BaselineScore == nil || *category.BaselineScore != 0.8 {
		t.Fatalf("artifact completeness baseline_score = %v, want 0.8", category.BaselineScore)
	}
	if category.Delta == nil || *category.Delta != 0.2 {
		t.Fatalf("artifact completeness delta = %v, want 0.2", category.Delta)
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

func TestEvalCoverageCommandJSON(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	writeEvalCmdFile(t, filepath.Join(dir, "fixture.txt"), "ok\n")
	writeEvalCmdFile(t, filepath.Join(dir, "suite.json"), `{
  "schema_version": 1,
  "id": "coverage.cli",
  "name": "Coverage CLI",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "allowed_runtimes": ["static"],
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "fixture",
      "title": "fixture",
      "kind": "artifact_check",
      "objective": "fixture",
      "expectations": [
        {"type": "file_exists", "target": "fixture.txt"}
      ],
      "critical": true
    }
  ]
}`)

	out, err := executeCommand("eval", "coverage", "--root", dir, "--json")
	if err != nil {
		t.Fatalf("ao eval coverage failed: %v\noutput: %s", err, out)
	}
	var report aoeval.CoverageReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("eval coverage JSON parse failed: %v\noutput: %s", err, out)
	}
	if report.SuiteCount != 1 || report.CaseCount != 1 || report.CriticalCaseCount != 1 {
		t.Fatalf("coverage counts = suites:%d cases:%d critical:%d, want 1/1/1", report.SuiteCount, report.CaseCount, report.CriticalCaseCount)
	}
	if report.Domains["cli"].SuiteCount != 1 {
		t.Fatalf("cli domain coverage = %+v, want one suite", report.Domains["cli"])
	}
	if len(report.MissingRequiredDomains) == 0 {
		t.Fatalf("missing required domains empty; want gaps for non-cli domains")
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

func writeEvalScorecardRunRecord(t *testing.T, path, runID string, scores map[string]float64) {
	t.Helper()
	slugs := []string{
		"artifact-completeness",
		"phase-order",
		"objective-spine",
		"validation-separation",
		"scenario-satisfaction",
		"runtime-safety",
	}
	sum := 0.0
	results := make([]aoeval.CaseResult, 0, len(slugs))
	for _, slug := range slugs {
		score := scores[slug]
		sum += score
		results = append(results, aoeval.CaseResult{
			ID:     "scorecard." + slug + ".surface",
			Status: aoeval.StatusPass,
			Score:  score,
			DimensionScores: map[aoeval.Dimension]float64{
				aoeval.DimensionCorrectness: score,
			},
		})
	}
	aggregate := sum / float64(len(slugs))
	run := aoeval.RunRecord{
		SchemaVersion: 1,
		RunID:         runID,
		Suite: aoeval.SuiteRef{
			ID:         "agentops-core.rpi-scorecard",
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
		CaseResults:    results,
		AggregateScore: aggregate,
		DimensionScores: map[aoeval.Dimension]float64{
			aoeval.DimensionCorrectness:          aggregate,
			aoeval.DimensionProcessAdherence:     aggregate,
			aoeval.DimensionArtifactQuality:      aggregate,
			aoeval.DimensionRuntimeCompatibility: aggregate,
			aoeval.DimensionSafety:               aggregate,
			aoeval.DimensionLearningClosure:      aggregate,
		},
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		t.Fatalf("marshal scorecard run: %v", err)
	}
	writeEvalCmdFile(t, path, string(data))
}

func evalScorecardCategory(t *testing.T, scorecard aoeval.Scorecard, name string) aoeval.ScorecardCategory {
	t.Helper()
	for _, category := range scorecard.Categories {
		if category.Category == name {
			return category
		}
	}
	t.Fatalf("category %q not found in %+v", name, scorecard.Categories)
	return aoeval.ScorecardCategory{}
}

func fixedEvalCmdTime() time.Time {
	return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
}

func withEvalCommand(t *testing.T) {
	t.Helper()
	registerEvalCommand()
}
