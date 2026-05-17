// practices: [tdd, bdd-gherkin]
// F2.T1 gap-fill: goals_measure_scenarios boundary cases not covered by
// goals_measure_scenarios_test.go.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// goalsMDWithNoScenarios is a GOALS.md where both directives have no linked
// scenarios — every directive should yield unknown verdict.
const goalsMDWithNoScenarios = "# Goals\n\n" +
	"Mission.\n\n" +
	"## Directives\n\n" +
	"### 1. Alpha\n\n" +
	"**Directive ID:** d-alpha\n" +
	"**Steer:** maintain\n\n" +
	"### 2. Beta\n\n" +
	"**Directive ID:** d-beta\n" +
	"**Steer:** maintain\n"

func TestGoalsMeasure_JSONStdoutCarriesNoHumanWarnings(t *testing.T) {
	// JSON output must be clean — human-readable warnings (e.g. "no artifact")
	// belong to stderr or the JSON warning field, never as freeform text mixed
	// into the JSON stdout stream. This test verifies that the raw bytes written
	// to stdout parse as valid JSON with no preamble or trailing prose.
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, false) // no artifact → unknown
	goalsMeasureScenariosOnly = true
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	// The entire stdout must unmarshal as measureScenarioJSON — no prose, no
	// warning preamble, no trailing text.
	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("stdout is not clean JSON: %v\nraw stdout:\n%s", err, raw)
	}

	// Any artifact-missing warning must be inside the JSON warning fields, not
	// as raw text in stdout. Verify by checking the raw bytes contain no
	// bare "Warning:" prefix outside the JSON structure.
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("stdout does not start with '{': %q", trimmed[:min(80, len(trimmed))])
	}
	if !strings.HasSuffix(trimmed, "}") {
		t.Fatalf("stdout does not end with '}': %q", trimmed[max(0, len(trimmed)-80):])
	}
}

func TestGoalsMeasure_ZeroLinkedScenariosYieldsUnknownWithWarning(t *testing.T) {
	// A directive that links no scenarios must yield verdict "unknown" and carry
	// a non-empty warning field in the JSON report (zero-linked warning).
	setupMeasureScenarioProject(t, goalsMDWithNoScenarios, true)
	goalsMeasureScenariosOnly = true
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, raw)
	}
	if len(payload.Directives) != 2 {
		t.Fatalf("Directives len = %d, want 2", len(payload.Directives))
	}
	for _, d := range payload.Directives {
		if d.ScenarioVerdict != "unknown" {
			t.Errorf("directive %s verdict = %q, want unknown (zero linked scenarios)", d.DirectiveID, d.ScenarioVerdict)
		}
		if d.ScenarioCount != 0 {
			t.Errorf("directive %s ScenarioCount = %d, want 0", d.DirectiveID, d.ScenarioCount)
		}
		if d.Warning == "" {
			t.Errorf("directive %s Warning = empty, want zero-linked-scenarios warning", d.DirectiveID)
		}
	}
}

func TestGoalsMeasure_ScenariosOnlyJSONModeField(t *testing.T) {
	// The "mode" field must be exactly "scenarios-only" (not empty, not "full")
	// when --scenarios-only is active. This is a contract that downstream
	// consumers (CI parsers, dashboard collectors) depend on.
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = true
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Mode != measureModeScenariosOnly {
		t.Fatalf("Mode = %q, want %q", payload.Mode, measureModeScenariosOnly)
	}
}

func TestGoalsMeasure_FullModeJSONModeField(t *testing.T) {
	// The "mode" field must be exactly "full" in default (non --scenarios-only) mode.
	setupMeasureScenarioProject(t, goalsMDWithScenarioGate, true)
	goalsMeasureScenariosOnly = false
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var payload measureScenarioJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Mode != measureModeFull {
		t.Fatalf("Mode = %q, want %q", payload.Mode, measureModeFull)
	}
}

func TestGoalsMeasure_ContributingIsEmptySliceNotNil(t *testing.T) {
	// The "contributing" field in JSON must be an empty array (not null/nil)
	// when no scenarios contribute. JSON null for a list field breaks consumers
	// that iterate over it without a nil check.
	setupMeasureScenarioProject(t, goalsMDWithNoScenarios, true)
	goalsMeasureScenariosOnly = true
	oldOutput := output
	t.Cleanup(func() { output = oldOutput })
	output = "json"

	raw := captureJSONStdout(t, func() {
		if err := goalsMeasureCmd.RunE(goalsMeasureCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	// Check that "contributing":null does not appear anywhere in the JSON bytes.
	if strings.Contains(raw, `"contributing":null`) {
		t.Fatalf("JSON contains 'contributing':null; want empty array []\nraw: %s", raw)
	}
	// Also check without space variant.
	if strings.Contains(raw, `"contributing": null`) {
		t.Fatalf("JSON contains 'contributing': null; want empty array []\nraw: %s", raw)
	}
}

