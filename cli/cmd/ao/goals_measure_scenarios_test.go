// practices: [tdd, bdd-gherkin]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goalsMDWithScenarioGate is a GOALS.md fixture with one gate (whose Check
// touches a sentinel file so a test can prove whether the gate ran) and two
// directives that link executable-spec scenarios.
const goalsMDWithScenarioGate = "# Goals\n\n" +
	"Mission.\n\n" +
	"## Gates\n\n" +
	"| ID | Check | Weight | Description |\n" +
	"|----|-------|--------|-------------|\n" +
	"| sentinel-gate | `touch GATE_RAN.txt` | 5 | Touches a sentinel |\n\n" +
	"## Directives\n\n" +
	"### 1. Ship the parser\n\n" +
	"**Directive ID:** d-ship-parser\n" +
	"**Steer:** maintain\n" +
	"**Scenarios:** s-2026-05-01-001, s-2026-05-01-002\n\n" +
	"### 2. Harden the loader\n\n" +
	"**Directive ID:** d-harden-loader\n" +
	"**Steer:** maintain\n" +
	"**Scenarios:** s-2026-05-01-003\n" +
	"**Scenario threshold:** 0.5\n"

// scenarioResultsArtifact is a scenario-results.v1 artifact: d-ship-parser's
// two scenarios both pass (satisfaction 1.0), d-harden-loader's one scenario
// fails its own threshold (satisfaction 0.0).
const scenarioResultsArtifact = `{
  "schema_version": "scenario-results.v1",
  "run_id": "run-test",
  "iteration": 1,
  "generated_at": "2026-05-17T00:00:00Z",
  "results": [
    {"scenario_id": "s-2026-05-01-001", "directive_id": "d-ship-parser", "score": 0.95, "threshold": 0.8, "verdict": "pass", "judged_at": "2026-05-17T00:00:00Z", "evidence": []},
    {"scenario_id": "s-2026-05-01-002", "directive_id": "d-ship-parser", "score": 0.90, "threshold": 0.8, "verdict": "pass", "judged_at": "2026-05-17T00:00:00Z", "evidence": []},
    {"scenario_id": "s-2026-05-01-003", "directive_id": "d-harden-loader", "score": 0.20, "threshold": 0.8, "verdict": "fail", "judged_at": "2026-05-17T00:00:00Z", "evidence": []}
  ]
}`

// setupMeasureScenarioProject writes GOALS.md + the scenario-results artifact
// into a fresh temp dir, chdir's into it (so both the goals file and the
// artifact resolve against the same project root), and wires goalsFile.
func setupMeasureScenarioProject(t *testing.T, goalsMD string, withArtifact bool) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "GOALS.md"), []byte(goalsMD), 0o644); err != nil {
		t.Fatalf("write GOALS.md: %v", err)
	}
	if withArtifact {
		rpiDir := filepath.Join(dir, ".agents", "rpi")
		if err := os.MkdirAll(rpiDir, 0o755); err != nil {
			t.Fatalf("mkdir .agents/rpi: %v", err)
		}
		if err := os.WriteFile(filepath.Join(rpiDir, "scenario-results.json"), []byte(scenarioResultsArtifact), 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	// soc-hwgm/soc-xyt1: save+restore EVERY goalsMeasure package-level
	// global the command body reads. Previously only goalsFile and
	// goalsMeasureScenariosOnly were saved, leaving the other 4 measure
	// globals + the `output` global open to leak across tests under
	// -shuffle on. Symptom: TestGoalsMeasure_MissingArtifactYieldsUnknown
	// got empty stdout (unmarshal error) because prior test left
	// goalsMeasureDirectives=true, which made goalsMeasureCmd.RunE skip
	// the scenario-only branch and call into a different code path.
	oldGoalsFile := goalsFile
	oldScenariosOnly := goalsMeasureScenariosOnly
	oldGoalID := goalsMeasureGoalID
	oldDirectives := goalsMeasureDirectives
	oldExcludeTag := goalsMeasureExcludeTag
	oldTotalTimeout := goalsMeasureTotalTimeout
	oldOutput := output
	oldDryRun := dryRun
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		goalsFile = oldGoalsFile
		goalsMeasureScenariosOnly = oldScenariosOnly
		goalsMeasureGoalID = oldGoalID
		goalsMeasureDirectives = oldDirectives
		goalsMeasureExcludeTag = oldExcludeTag
		goalsMeasureTotalTimeout = oldTotalTimeout
		output = oldOutput
		dryRun = oldDryRun
	})
	// Zero out all measure globals on entry so a leaked value from a
	// prior test cannot taint this test's read.
	goalsMeasureScenariosOnly = false
	goalsMeasureGoalID = ""
	goalsMeasureDirectives = false
	goalsMeasureExcludeTag = ""
	goalsMeasureTotalTimeout = 0
	dryRun = false
	goalsFile = "GOALS.md"
	return dir
}

func TestGoalsMeasure_ScenariosOnlyDoesNotExecuteGateCommands(t *testing.T) {
	dir := setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true

	out, err := captureStdout(t, func() error {
		return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
	})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// The gate command is `touch GATE_RAN.txt`. Under --scenarios-only no gate
	// subprocess may spawn, so the sentinel must NOT exist.
	if _, statErr := os.Stat(filepath.Join(dir, "GATE_RAN.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("gate command executed under --scenarios-only: GATE_RAN.txt exists (statErr=%v)", statErr)
	}
	if !strings.Contains(out, "scenarios-only") {
		t.Errorf("scenarios-only human output missing mode marker; got: %q", out)
	}
}

func TestGoalsMeasure_FullModeExecutesGateCommands(t *testing.T) {
	dir := setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = false

	_, err := captureStdout(t, func() error {
		return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
	})
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// Contrast with --scenarios-only: a full run DOES execute the gate, so the
	// sentinel must exist. This proves the --scenarios-only skip is real.
	if _, statErr := os.Stat(filepath.Join(dir, "GATE_RAN.txt")); statErr != nil {
		t.Fatalf("full run did not execute gate command: GATE_RAN.txt missing (%v)", statErr)
	}
}

func TestGoalsMeasure_ScenariosOnlyJSONFields(t *testing.T) {
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE returned error: %v", err)
		}
	})

	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v\nraw: %s", err, raw)
	}

	if payload.Mode != "scenarios-only" {
		t.Errorf("Mode = %q, want %q", payload.Mode, "scenarios-only")
	}
	if payload.Snapshot != nil {
		t.Errorf("Snapshot should be nil under --scenarios-only, got %+v", payload.Snapshot)
	}
	if len(payload.Directives) != 2 {
		t.Fatalf("Directives len = %d, want 2", len(payload.Directives))
	}

	byID := map[string]directiveScenarioReport{}
	for _, d := range payload.Directives {
		byID[d.DirectiveID] = d
	}

	parser, ok := byID["d-ship-parser"]
	if !ok {
		t.Fatalf("missing d-ship-parser directive in %+v", byID)
	}
	if parser.DirectiveNumber != 1 {
		t.Errorf("d-ship-parser DirectiveNumber = %d, want 1", parser.DirectiveNumber)
	}
	if parser.ScenarioCount != 2 {
		t.Errorf("d-ship-parser ScenarioCount = %d, want 2", parser.ScenarioCount)
	}
	if parser.EvaluatedCount != 2 {
		t.Errorf("d-ship-parser EvaluatedCount = %d, want 2", parser.EvaluatedCount)
	}
	if parser.MissingCount != 0 {
		t.Errorf("d-ship-parser MissingCount = %d, want 0", parser.MissingCount)
	}
	if parser.ScenarioSatisfaction != 1.0 {
		t.Errorf("d-ship-parser ScenarioSatisfaction = %v, want 1.0", parser.ScenarioSatisfaction)
	}
	if parser.ScenarioThreshold != 0.8 {
		t.Errorf("d-ship-parser ScenarioThreshold = %v, want 0.8 (default)", parser.ScenarioThreshold)
	}
	if parser.ScenarioVerdict != "pass" {
		t.Errorf("d-ship-parser ScenarioVerdict = %q, want pass", parser.ScenarioVerdict)
	}
	wantContributing := []string{"s-2026-05-01-001", "s-2026-05-01-002"}
	if strings.Join(parser.Contributing, ",") != strings.Join(wantContributing, ",") {
		t.Errorf("d-ship-parser Contributing = %v, want %v", parser.Contributing, wantContributing)
	}

	loader, ok := byID["d-harden-loader"]
	if !ok {
		t.Fatalf("missing d-harden-loader directive in %+v", byID)
	}
	if loader.ScenarioThreshold != 0.5 {
		t.Errorf("d-harden-loader ScenarioThreshold = %v, want 0.5 (declared)", loader.ScenarioThreshold)
	}
	if loader.ScenarioSatisfaction != 0.0 {
		t.Errorf("d-harden-loader ScenarioSatisfaction = %v, want 0.0", loader.ScenarioSatisfaction)
	}
	if loader.ScenarioVerdict != "fail" {
		t.Errorf("d-harden-loader ScenarioVerdict = %q, want fail", loader.ScenarioVerdict)
	}
}

func TestGoalsMeasure_FullModeJSONCarriesSnapshotAndScenarios(t *testing.T) {
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = false
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE returned error: %v", err)
		}
	})

	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v\nraw: %s", err, raw)
	}

	if payload.Mode != "full" {
		t.Errorf("Mode = %q, want %q", payload.Mode, "full")
	}
	if payload.Snapshot == nil {
		t.Fatal("full-mode JSON must carry the gate snapshot, got nil")
	}
	// The gate snapshot is preserved unchanged: the sentinel gate ran.
	if len(payload.Snapshot.Goals) != 1 {
		t.Errorf("Snapshot.Goals len = %d, want 1", len(payload.Snapshot.Goals))
	}
	if len(payload.Directives) != 2 {
		t.Errorf("Directives len = %d, want 2", len(payload.Directives))
	}
}

func TestGoalsMeasure_MissingArtifactYieldsUnknownNotError(t *testing.T) {
	// No artifact written: a missing scenario-results artifact is a clean skip,
	// never an invocation error and never a false pass.
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, false)
	goalsMeasureScenariosOnly = true
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE returned error for missing artifact: %v", err)
		}
	})

	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v\nraw: %s", err, raw)
	}
	for _, d := range payload.Directives {
		if d.ScenarioVerdict != "unknown" {
			t.Errorf("directive %s verdict = %q, want unknown with no artifact", d.DirectiveID, d.ScenarioVerdict)
		}
	}
}

const goalsMDBadThreshold = "# Goals\n\nMission.\n\n" +
	"## Directives\n\n" +
	"### 1. Bad threshold directive\n\n" +
	"**Directive ID:** d-bad\n" +
	"**Steer:** maintain\n" +
	"**Scenarios:** s-2026-05-01-009\n" +
	"**Scenario threshold:** 2.5\n"

func TestGoalsMeasure_StructurallyInvalidThresholdIsError(t *testing.T) {
	// A malformed scenario threshold is a structurally-invalid input, a
	// distinct exit class from a failing scenario verdict: it must return an
	// error so the invocation exits non-zero.
	setupMeasureScenarioProject(t, goalsMDBadThreshold, true)
	goalsMeasureScenariosOnly = true

	_, err := captureStdout(t, func() error {
		return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
	})
	if err == nil {
		t.Fatal("expected error for out-of-range scenario threshold, got nil")
	}
	if !strings.Contains(err.Error(), "scenario threshold") {
		t.Errorf("error = %q, want it to mention 'scenario threshold'", err.Error())
	}
}

func TestGoalsMeasure_FailingScenarioVerdictExitsZero(t *testing.T) {
	// A failing scenario verdict is a measurement outcome, not an invocation
	// error: consistent with `ao goals measure` exiting 0 on a red gate, a red
	// scenario verdict must NOT make RunE return an error. Callers gate on the
	// JSON scenario_verdict field instead.
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true

	_, err := captureStdout(t, func() error {
		return goalsMeasureCmd.RunE(goalsMeasureCmd, nil)
	})
	// d-harden-loader's scenario fails, but RunE must still return nil.
	if err != nil {
		t.Fatalf("RunE returned error for a failing scenario verdict, want nil: %v", err)
	}
}
