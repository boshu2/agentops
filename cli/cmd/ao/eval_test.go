// practices: [llm-eval-harness, dora-metrics]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	evalsub "github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/spf13/cobra"
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

func TestEvalRunContextModeABCommandJSON(t *testing.T) {
	if os.Getenv("GO_WANT_EVAL_CMD_CONTEXT_HELPER_PROCESS") == "1" {
		root := os.Getenv("AO_AGENTS_DIR")
		variant := os.Getenv("AO_CONTEXT_VARIANT")
		data, err := os.ReadFile(filepath.Join(root, "variant.txt"))
		if err != nil {
			os.Exit(1)
		}
		ok := string(data) == variant+"\n" && os.Getenv("AGENTOPS_HOOKS_DISABLED") != "1"
		if ok {
			fmt.Fprintf(os.Stdout, `{"ok":true,"variant":%q}`, variant)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stdout, `{"ok":false,"variant":%q}`, variant)
		os.Exit(1)
	}

	withEvalCommand(t)
	dir := t.TempDir()
	writeEvalCmdFile(t, filepath.Join(dir, "fixtures", "context-ab", "context-off", "agents", "variant.txt"), string(aoeval.ContextVariantOff)+"\n")
	writeEvalCmdFile(t, filepath.Join(dir, "fixtures", "context-ab", "context-on", "agents", "variant.txt"), string(aoeval.ContextVariantOn)+"\n")
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	suitePath := writeEvalCmdSuiteNamed(t, dir, "context-ab.json", fmt.Sprintf(`{
  "schema_version": 1,
  "id": "context.ab.cmd",
  "name": "Context AB command",
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
      "id": "agents-dir",
      "title": "agents dir is isolated per context leg",
      "kind": "command",
      "objective": "Verify context-mode runs with isolated AO_AGENTS_DIR roots.",
      "runtime": "shell",
      "inputs": {
        "argv": [%q, "-test.run=TestEvalRunContextModeABCommandJSON"],
        "env": {"GO_WANT_EVAL_CMD_CONTEXT_HELPER_PROCESS": "1"}
      },
      "expectations": [
        {"type": "exit_code", "value": 0},
        {"type": "json_path", "target": "stdout.ok", "value": true}
      ]
    }
  ]
}`, exe))
	outputPath := filepath.Join(dir, "run.json")

	out, err := executeCommand("eval", "run", suitePath, "--context-mode", "ab", "--run-id", "cmd-context", "--out", outputPath, "--json")
	if err != nil {
		t.Fatalf("ao eval run --context-mode=ab failed: %v\noutput: %s", err, out)
	}
	var scorecard aoeval.ContextDeltaScorecard
	if err := json.Unmarshal([]byte(out), &scorecard); err != nil {
		t.Fatalf("context scorecard JSON parse failed: %v\noutput: %s", err, out)
	}
	if scorecard.ContextOff.RunID != "cmd-context-context-off" || scorecard.ContextOn.RunID != "cmd-context-context-on" {
		t.Fatalf("run ids = off:%q on:%q", scorecard.ContextOff.RunID, scorecard.ContextOn.RunID)
	}
	for _, path := range []string{
		filepath.Join(dir, "run-context-off.json"),
		filepath.Join(dir, "run-context-on.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected context leg output %s: %v", path, err)
		}
	}
}

func TestEvalRunContextModeRejectsSkillOffBaseline(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	suitePath := writeEvalCmdSuite(t, dir, `{
  "schema_version": 1,
  "id": "context.reject",
  "name": "Context reject",
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
      "id": "static",
      "title": "static",
      "kind": "artifact_check",
      "objective": "static",
      "expectations": [
        {"type": "file_exists", "target": "suite.json"}
      ]
    }
  ]
}`)

	out, err := executeCommand("eval", "run", suitePath, "--context-mode", "ab", "--baseline-mode", "skill-off")
	if err == nil {
		t.Fatalf("ao eval run accepted incompatible context/baseline modes; output: %s", out)
	}
	if !strings.Contains(out, "--context-mode=ab cannot be combined") {
		t.Fatalf("unexpected error output: %s", out)
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
	if report.EvidenceKinds[string(aoeval.EvidenceKindContractCanary)].CaseCount != 1 {
		t.Fatalf("contract_canary evidence coverage = %+v, want one case", report.EvidenceKinds[string(aoeval.EvidenceKindContractCanary)])
	}
	if len(report.MissingRequiredDomains) == 0 {
		t.Fatalf("missing required domains empty; want gaps for non-cli domains")
	}
}

func TestEvalBaselineAuditCommandJSON(t *testing.T) {
	withEvalCommand(t)
	dir := t.TempDir()
	writeEvalCmdFile(t, filepath.Join(dir, "fixture.txt"), "ok\n")
	writeEvalCmdFile(t, filepath.Join(dir, "suite.json"), `{
  "schema_version": 1,
  "id": "baseline.audit",
  "name": "Baseline Audit",
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
  "baseline_policy": {"mode": "compare"},
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

	out, err := executeCommand("eval", "baseline-audit", "--root", dir, "--baseline-dir", filepath.Join(dir, "baselines"), "--json")
	if err != nil {
		t.Fatalf("ao eval baseline-audit failed: %v\noutput: %s", err, out)
	}
	var report aoeval.BaselineAuditReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("baseline audit JSON parse failed: %v\noutput: %s", err, out)
	}
	if report.PolicyMismatchCount != 1 || len(report.MissingCompareBaselines) != 1 {
		t.Fatalf("baseline audit report = %+v, want one missing compare baseline", report)
	}
}

func TestEvalCleanupTransitionsDeleteTmpAndReport(t *testing.T) {
	root := t.TempDir()
	runsRoot := filepath.Join(root, "runs")
	now := time.Now().UTC()
	writeEvalSubManifest(t, root, "old-pending", evalsub.StatusPending, now.Add(-2*time.Minute))
	writeEvalSubManifest(t, root, "old-running", evalsub.StatusRunning, now.Add(-10*time.Minute))
	writeEvalSubManifest(t, root, "fresh-pending", evalsub.StatusPending, now.Add(-10*time.Second))
	writeEvalSubManifest(t, root, "failed-delete", evalsub.StatusFailed, now.Add(-20*time.Minute))
	writeEvalSubManifest(t, root, "retracted-keep", evalsub.StatusRetracted, now.Add(-20*time.Minute))

	report := CleanupReport{Touched: []string{}}
	if err := cleanupStaleTransitions(root, runsRoot, &report); err != nil {
		t.Fatalf("cleanup stale transitions failed: %v", err)
	}
	if report.TransitionsAborted != 1 || report.TransitionsFailed != 1 {
		t.Fatalf("transition counts = aborted:%d failed:%d, want 1/1", report.TransitionsAborted, report.TransitionsFailed)
	}
	assertEvalSubManifestStatus(t, root, "old-pending", evalsub.StatusAborted, "never_started")
	assertEvalSubManifestStatus(t, root, "old-running", evalsub.StatusFailed, "orphaned_process")
	assertEvalSubManifestStatus(t, root, "fresh-pending", evalsub.StatusPending, "")

	oldDryRun := evalCleanupDryRun
	oldAge := evalCleanupAge
	evalCleanupDryRun = false
	evalCleanupAge = 0
	t.Cleanup(func() {
		evalCleanupDryRun = oldDryRun
		evalCleanupAge = oldAge
	})
	if err := cleanupDeleteRuns(root, runsRoot, &report); err != nil {
		t.Fatalf("cleanup delete runs failed: %v", err)
	}
	for _, runID := range []string{"old-pending", "old-running", "failed-delete"} {
		if _, err := os.Stat(evalsub.RunDir(root, runID)); !os.IsNotExist(err) {
			t.Fatalf("%s dir still exists after delete cleanup: %v", runID, err)
		}
	}
	if _, err := os.Stat(evalsub.RunDir(root, "retracted-keep")); err != nil {
		t.Fatalf("retracted run should be kept: %v", err)
	}

	tmpPath := filepath.Join(root, "runs", "fresh-pending", "manifest.json.tmp")
	writeEvalCmdFile(t, tmpPath, "{}\n")
	if err := cleanupSweepTmpFiles(root, &report); err != nil {
		t.Fatalf("cleanup tmp files failed: %v", err)
	}
	if report.TmpFilesSwept != 1 {
		t.Fatalf("tmp files swept = %d, want 1", report.TmpFilesSwept)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("tmp file still exists after sweep: %v", err)
	}

	oldOutput := output
	oldVerbose := verbose
	output = "json"
	verbose = false
	t.Cleanup(func() {
		output = oldOutput
		verbose = oldVerbose
	})
	cmd := &cobra.Command{}
	var jsonBuf bytes.Buffer
	cmd.SetOut(&jsonBuf)
	if err := writeCleanupReport(cmd, &report); err != nil {
		t.Fatalf("write cleanup JSON report failed: %v", err)
	}
	var decoded CleanupReport
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("cleanup JSON did not parse: %v\n%s", err, jsonBuf.String())
	}
	if decoded.RunsDeleted != 3 || decoded.TmpFilesSwept != 1 {
		t.Fatalf("decoded cleanup report = %+v, want runs_deleted=3 tmp_files_swept=1", decoded)
	}

	output = "table"
	verbose = true
	var textBuf bytes.Buffer
	cmd.SetOut(&textBuf)
	if err := writeCleanupReport(cmd, &report); err != nil {
		t.Fatalf("write cleanup text report failed: %v", err)
	}
	text := textBuf.String()
	if !strings.Contains(text, "Eval cleanup:") || !strings.Contains(text, "Touched:") {
		t.Fatalf("cleanup text report missing summary or verbose touches:\n%s", text)
	}
}

func TestEvalTaskHelpersBuildManifestAndPrepareInputs(t *testing.T) {
	root := t.TempDir()
	restoreEvalTaskRunGlobals(t)
	t.Setenv("AGENTOPS_EVALS_ROOT", root)

	task := evalsub.Task{
		SchemaVersion: 1,
		ID:            "task-a",
		Domain:        "cli",
		Description:   "task fixture",
		HarnessRef:    "harness-a",
		Stats: evalsub.TaskStat{
			Metric:      "accuracy",
			Paired:      true,
			MinNSamples: 3,
			DecisionRule: evalsub.DecisionRule{
				Kind:       "bootstrap_ci",
				Confidence: 0.95,
				MinDelta:   0.05,
			},
		},
	}
	suite := evalsub.Suite{
		SchemaVersion: 1,
		ID:            "suite-a",
		Kind:          "comparison",
		VariedAxis: evalsub.VariedAxis{
			Kind:   "model",
			Values: []string{"baseline", "candidate-a", "candidate-b"},
		},
		HeldConstant: evalsub.HeldConstant{
			Task:               "task-a",
			Harness:            "harness-a",
			GroundTruthVersion: "gt-v1",
			Decoding:           map[string]interface{}{"temperature": 0},
		},
		SampleSplit: "holdout",
		NSamples:    7,
		Stats: evalsub.SuiteStat{
			DecisionRule: evalsub.DecisionRule{
				Kind:       "bootstrap_ci",
				Confidence: 0.95,
			},
			MultiComparisonMethod: "holm",
			ComparisonFamily:      "all_pairs",
			ReferenceArm:          "baseline",
			Paired:                true,
		},
	}
	writeEvalSubTask(t, root, task)
	writeEvalSubSuite(t, root, suite)
	writeEvalSubGroundTruth(t, root, []evalsub.GroundTruthRow{
		{ID: "gt-a", Value: "expected", Source: "test", Confidence: "strong", Split: "holdout"},
		{ID: "gt-a-v2", Value: "expected newer", Source: "test", Confidence: "strong", Split: "holdout", Supersedes: "gt-a"},
	})
	_, modelHash, err := evalsub.CaptureModelSpec(root, &evalsub.ModelSpec{
		ID:              "model-a",
		Provider:        "local",
		ModelName:       "fixture-model",
		ToolCallSupport: true,
		SamplingDefaults: map[string]interface{}{
			"temperature": 0,
		},
	})
	if err != nil {
		t.Fatalf("capture model spec failed: %v", err)
	}

	evalTaskRunSuiteRef = "suite-a"
	evalTaskRunSeeds = "1, 2, 3"
	evalTaskRunHarnessRef = "harness-a"
	evalTaskRunModelSpecID = "model-a"
	evalTaskRunGTRef = "gt-a"
	evalTaskRunSampleSplit = "dev"
	evalTaskRunNSamples = 5
	evalTaskRunInspectCommand = "inspect eval fixture"
	evalTaskRunInspectVersion = "0.3.216"
	evalTaskRunQuickSession = true
	evalTaskRunAllowWeak = false

	seeds, err := parseSeeds(evalTaskRunSeeds)
	if err != nil {
		t.Fatalf("parse seeds failed: %v", err)
	}
	gateInputs := evalsub.GateInputs{
		Suite:       &suite,
		Task:        &task,
		Harness:     &evalsub.Harness{ID: "harness-a", ContentHash: "sha256:harness"},
		GroundTruth: []evalsub.GroundTruthRow{{ID: "gt-a", Value: "expected", Source: "test", Confidence: "strong", Split: "holdout"}},
		GTRequested: "gt-a",
	}
	manifest := buildTaskRunManifest(&task, &suite, seeds, "rig-a", gateInputs)
	if manifest.SampleSplit != "dev" || manifest.NSamples != 5 || !manifest.QuickSession {
		t.Fatalf("manifest overrides = split:%q n:%d quick:%v, want dev/5/true", manifest.SampleSplit, manifest.NSamples, manifest.QuickSession)
	}
	if manifest.HarnessContentHash != "sha256:harness" || manifest.ModelSpecHash != modelHash {
		t.Fatalf("manifest hashes = harness:%q model:%q, want harness/model %q/%q", manifest.HarnessContentHash, manifest.ModelSpecHash, "sha256:harness", modelHash)
	}
	if manifest.GroundTruthHash == "" {
		t.Fatalf("manifest ground_truth_hash is empty")
	}
	if manifest.MultiComparisonMethod != "holm" || manifest.FamilySizeK != 3 {
		t.Fatalf("manifest multi-comparison = method:%q family_k:%d, want holm/3", manifest.MultiComparisonMethod, manifest.FamilySizeK)
	}

	preparedInputs, preparedSuite, preparedTask, preparedSeeds, err := prepareTaskRunInputs("task-a")
	if err != nil {
		t.Fatalf("prepare task run inputs failed: %v", err)
	}
	if preparedTask.ID != "task-a" || preparedSuite.ID != "suite-a" {
		t.Fatalf("prepared refs = task:%q suite:%q, want task-a/suite-a", preparedTask.ID, preparedSuite.ID)
	}
	if len(preparedSeeds) != 3 || preparedSeeds[2] != 3 {
		t.Fatalf("prepared seeds = %v, want [1 2 3]", preparedSeeds)
	}
	if len(preparedInputs.GroundTruth) != 2 {
		t.Fatalf("prepared ground truth rows = %d, want 2", len(preparedInputs.GroundTruth))
	}

	if _, err := parseSeeds("1,bad"); err == nil {
		t.Fatalf("parseSeeds accepted an invalid seed")
	}
	if split := pickSampleSplit(nil, ""); split != "dev" {
		t.Fatalf("pickSampleSplit(nil, empty) = %q, want dev", split)
	}
	if n := pickNSamples(nil, 0); n != 0 {
		t.Fatalf("pickNSamples(nil, 0) = %d, want 0", n)
	}
	if got := hashGroundTruthRef(nil, "gt-a"); got != "" {
		t.Fatalf("hashGroundTruthRef(nil) = %q, want empty", got)
	}
}

func TestEvalTaskRunDryRunCommand(t *testing.T) {
	withEvalCommand(t)
	root := t.TempDir()
	restoreEvalTaskRunGlobals(t)
	t.Setenv("AGENTOPS_EVALS_ROOT", root)
	writeEvalSubTask(t, root, evalsub.Task{
		SchemaVersion: 1,
		ID:            "task-dry-run",
		Domain:        "cli",
		HarnessRef:    "harness-dry-run",
		Stats: evalsub.TaskStat{
			Metric:      "accuracy",
			MinNSamples: 3,
			DecisionRule: evalsub.DecisionRule{
				Kind:       "bootstrap_ci",
				Confidence: 0.95,
			},
		},
	})
	writeEvalSubSuite(t, root, evalsub.Suite{
		SchemaVersion: 1,
		ID:            "suite-dry-run",
		Kind:          "comparison",
		VariedAxis:    evalsub.VariedAxis{Kind: "model", Values: []string{"a", "b"}},
		HeldConstant:  evalsub.HeldConstant{Task: "task-dry-run", Harness: "harness-dry-run", GroundTruthVersion: "gt-v1"},
		SampleSplit:   "dev",
		NSamples:      3,
		Stats:         evalsub.SuiteStat{DecisionRule: evalsub.DecisionRule{Kind: "bootstrap_ci", Confidence: 0.95}},
	})
	writeEvalSubGroundTruth(t, root, []evalsub.GroundTruthRow{
		{ID: "gt-dry-run", Value: "expected", Source: "test", Confidence: "strong", Split: "dev"},
	})

	out, err := executeCommand(
		"eval", "task", "run", "task-dry-run",
		"--suite", "suite-dry-run",
		"--seeds", "11,12,13",
		"--harness", "harness-dry-run",
		"--ground-truth", "gt-dry-run",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("ao eval task run --dry-run failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Dry run: gates passed") {
		t.Fatalf("dry-run output missing success message:\n%s", out)
	}
}

func writeEvalCmdSuite(t *testing.T, dir, body string) string {
	t.Helper()
	return writeEvalCmdSuiteNamed(t, dir, "suite.json", body)
}

func writeEvalCmdSuiteNamed(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
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
	resetEvalRunGlobals()
	registerEvalCommand()
}

func resetEvalRunGlobals() {
	evalRunOutput = ""
	evalRunID = ""
	evalRunRuntime = ""
	evalRunBaseline = ""
	evalRunBaselineMode = string(aoeval.BaselineModeSkillOn)
	evalRunContextMode = string(aoeval.ContextModeNone)
	evalRunContextOffDir = ""
	evalRunContextOnDir = ""
	evalRunDeltaOut = ""
}

func writeEvalSubManifest(t *testing.T, root, runID string, status evalsub.RunStatus, started time.Time) {
	t.Helper()
	manifest := evalsub.Manifest{
		SchemaVersion:       evalsub.SchemaVersion,
		ID:                  runID,
		Kind:                "task",
		Status:              status,
		StartedAt:           started.Format(time.RFC3339),
		StartedAtUnixMs:     started.UnixNano() / int64(time.Millisecond),
		TaskRef:             "task-a",
		SuiteRef:            "suite-a",
		HarnessRef:          "harness-a",
		HarnessContentHash:  "sha256:harness",
		ModelSpecRef:        "model-a",
		ModelSpecHash:       "sha256:model",
		GroundTruthRef:      "gt-a",
		GroundTruthHash:     "sha256:gt",
		SampleSplit:         "dev",
		NSamples:            3,
		Seeds:               []int{1, 2, 3},
		InspectCommand:      "inspect eval",
		InspectVersion:      "0.3.216",
		ValidityGatesPassed: []string{"held_constant_declared"},
		RigID:               "rig-a",
		CapturedBy:          "test",
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal eval substrate manifest: %v", err)
	}
	data = append(data, '\n')
	if err := evalsub.WriteAtomic(evalsub.ManifestPath(root, runID), data); err != nil {
		t.Fatalf("write eval substrate manifest: %v", err)
	}
}

func assertEvalSubManifestStatus(t *testing.T, root, runID string, status evalsub.RunStatus, reason string) {
	t.Helper()
	manifest, err := evalsub.LoadManifest(evalsub.ManifestPath(root, runID))
	if err != nil {
		t.Fatalf("load %s manifest: %v", runID, err)
	}
	if manifest.Status != status || manifest.RetractionReason != reason {
		t.Fatalf("%s status/reason = %s/%q, want %s/%q", runID, manifest.Status, manifest.RetractionReason, status, reason)
	}
}

func writeEvalSubTask(t *testing.T, root string, task evalsub.Task) {
	t.Helper()
	body, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal eval substrate task: %v", err)
	}
	if err := evalsub.WriteAtomic(filepath.Join(root, "tasks", task.ID, "task.yaml"), body); err != nil {
		t.Fatalf("write eval substrate task: %v", err)
	}
}

func writeEvalSubSuite(t *testing.T, root string, suite evalsub.Suite) {
	t.Helper()
	body, err := json.Marshal(suite)
	if err != nil {
		t.Fatalf("marshal eval substrate suite: %v", err)
	}
	if err := evalsub.WriteAtomic(filepath.Join(root, "suites", suite.ID, "suite.yaml"), body); err != nil {
		t.Fatalf("write eval substrate suite: %v", err)
	}
}

func writeEvalSubGroundTruth(t *testing.T, root string, rows []evalsub.GroundTruthRow) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			t.Fatalf("encode eval substrate ground truth: %v", err)
		}
	}
	if err := evalsub.WriteAtomic(filepath.Join(root, "ground-truth", "ground-truth.jsonl"), buf.Bytes()); err != nil {
		t.Fatalf("write eval substrate ground truth: %v", err)
	}
}

func restoreEvalTaskRunGlobals(t *testing.T) {
	t.Helper()
	oldSuiteRef := evalTaskRunSuiteRef
	oldRigID := evalTaskRunRigID
	oldSeeds := evalTaskRunSeeds
	oldHarnessRef := evalTaskRunHarnessRef
	oldHarnessDir := evalTaskRunHarnessDir
	oldModelSpecID := evalTaskRunModelSpecID
	oldGTRef := evalTaskRunGTRef
	oldSampleSplit := evalTaskRunSampleSplit
	oldNSamples := evalTaskRunNSamples
	oldInspectVersion := evalTaskRunInspectVersion
	oldInspectCommand := evalTaskRunInspectCommand
	oldCrossSpec := evalTaskRunCrossSpec
	oldAllowWeak := evalTaskRunAllowWeak
	oldQuickSession := evalTaskRunQuickSession
	oldDryRun := evalTaskRunDryRun
	evalTaskRunSuiteRef = ""
	evalTaskRunRigID = ""
	evalTaskRunSeeds = ""
	evalTaskRunHarnessRef = ""
	evalTaskRunHarnessDir = ""
	evalTaskRunModelSpecID = ""
	evalTaskRunGTRef = ""
	evalTaskRunSampleSplit = ""
	evalTaskRunNSamples = 0
	evalTaskRunInspectVersion = "0.3.216"
	evalTaskRunInspectCommand = ""
	evalTaskRunCrossSpec = false
	evalTaskRunAllowWeak = false
	evalTaskRunQuickSession = false
	evalTaskRunDryRun = false
	t.Cleanup(func() {
		evalTaskRunSuiteRef = oldSuiteRef
		evalTaskRunRigID = oldRigID
		evalTaskRunSeeds = oldSeeds
		evalTaskRunHarnessRef = oldHarnessRef
		evalTaskRunHarnessDir = oldHarnessDir
		evalTaskRunModelSpecID = oldModelSpecID
		evalTaskRunGTRef = oldGTRef
		evalTaskRunSampleSplit = oldSampleSplit
		evalTaskRunNSamples = oldNSamples
		evalTaskRunInspectVersion = oldInspectVersion
		evalTaskRunInspectCommand = oldInspectCommand
		evalTaskRunCrossSpec = oldCrossSpec
		evalTaskRunAllowWeak = oldAllowWeak
		evalTaskRunQuickSession = oldQuickSession
		evalTaskRunDryRun = oldDryRun
	})
}

func TestParseEvalRuntime(t *testing.T) {
	tests := []struct {
		input   string
		want    aoeval.Runtime
		wantErr bool
	}{
		{"static", aoeval.RuntimeStatic, false},
		{"mock", aoeval.RuntimeMock, false},
		{"shell", aoeval.RuntimeShell, false},
		{"claude", aoeval.RuntimeClaude, false},
		{"codex", aoeval.RuntimeCodex, false},
		{"", "", false},
		{"unknown", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseEvalRuntime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseEvalRuntime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parseEvalRuntime(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
