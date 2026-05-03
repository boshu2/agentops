package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestContextVariantValuesAreSeparateFromBaselineModes(t *testing.T) {
	if ContextVariantOff != "context_off" {
		t.Fatalf("ContextVariantOff = %q, want context_off", ContextVariantOff)
	}
	if ContextVariantOn != "context_on" {
		t.Fatalf("ContextVariantOn = %q, want context_on", ContextVariantOn)
	}
	if got := AllBaselineModes(); len(got) != 3 {
		t.Fatalf("AllBaselineModes len = %d, want 3 (%v)", len(got), got)
	}
	if !IsValidContextMode("ab") || IsValidContextMode("both") {
		t.Fatalf("context modes should be independent from baseline modes")
	}
	for _, mode := range []string{"context_off", "context_on", "context-off", "context-on"} {
		if IsValidBaselineMode(mode) {
			t.Fatalf("context variant %q must not be accepted as ABBaselineMode", mode)
		}
	}
}

func TestContextDeltaScorecardSerializesEvidenceAndAttribution(t *testing.T) {
	tokenDelta := -128
	toolCallDelta := -2
	card := ContextDeltaScorecard{
		SchemaVersion: 1,
		SuiteID:       "suite.context",
		SuitePath:     "suite.json",
		ContextOff: ContextVariantRun{
			Variant:          ContextVariantOff,
			ContextRootLabel: "empty-agents",
			RunID:            "run-context-off",
			AggregateScore:   0.25,
		},
		ContextOn: ContextVariantRun{
			Variant:          ContextVariantOn,
			ContextRootLabel: "useful-agents",
			RunID:            "run-context-on",
			AggregateScore:   1,
		},
		AggregateDelta: 0.75,
		PerCase: []ContextCaseDelta{
			{
				CaseID: "helpful",
				ContextOff: ContextCaseVariantResult{
					Variant:          ContextVariantOff,
					ContextRootLabel: "empty-agents",
					RunID:            "run-context-off",
					Status:           StatusFail,
					Score:            0,
				},
				ContextOn: ContextCaseVariantResult{
					Variant:          ContextVariantOn,
					ContextRootLabel: "useful-agents",
					RunID:            "run-context-on",
					Status:           StatusPass,
					Score:            1,
				},
				ScoreDelta:    1,
				StatusDelta:   1,
				TokenDelta:    &tokenDelta,
				ToolCallDelta: &toolCallDelta,
				DecisionEvidence: []ContextEvidence{
					{Summary: "applied durable finding", Artifact: ".agents/findings/f-1.md"},
				},
				IgnoredContextEvidence: []ContextEvidence{
					{Summary: "ignored stale note", Artifact: ".agents/findings/stale.md"},
				},
				DegradedReason: "diagnostic-only context attribution",
				ArtifactAttribution: []ContextArtifactAttribution{
					{Artifact: ".agents/findings/f-1.md", Attribution: ContextAttributionHelpful, Evidence: "prevented known failure"},
					{Artifact: ".agents/findings/stale.md", Attribution: ContextAttributionIgnored, Evidence: "not relevant"},
				},
			},
		},
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	for _, key := range []string{"context_off", "context_on", "aggregate_delta", "per_case"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("scorecard missing key %q: %s", key, string(data))
		}
	}
	var decoded ContextDeltaScorecard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal scorecard: %v", err)
	}
	got := decoded.PerCase[0]
	if got.DecisionEvidence[0].Artifact != ".agents/findings/f-1.md" {
		t.Fatalf("decision evidence did not round-trip: %+v", got.DecisionEvidence)
	}
	if got.IgnoredContextEvidence[0].Summary != "ignored stale note" {
		t.Fatalf("ignored context evidence did not round-trip: %+v", got.IgnoredContextEvidence)
	}
	if got.DegradedReason == "" {
		t.Fatal("degraded reason should round-trip")
	}
	if got.TokenDelta == nil || *got.TokenDelta != -128 {
		t.Fatalf("token delta did not round-trip: %+v", got.TokenDelta)
	}
	if got.ToolCallDelta == nil || *got.ToolCallDelta != -2 {
		t.Fatalf("tool call delta did not round-trip: %+v", got.ToolCallDelta)
	}
	if got.ArtifactAttribution[0].Attribution != ContextAttributionHelpful {
		t.Fatalf("attribution vocabulary did not round-trip: %+v", got.ArtifactAttribution)
	}
}

func TestRunContextABSuffixesOutputsAndPreservesHooks(t *testing.T) {
	dir := t.TempDir()
	writeEvalFile(t, filepath.Join(dir, "fixture.txt"), "context packet canary\n")
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "context.ab",
  "name": "Context A/B",
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
      "title": "static context canary",
      "kind": "artifact_check",
      "objective": "Exercise context A/B runner plumbing.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "canary"}
      ]
    }
  ]
}`)

	outputPath := filepath.Join(dir, "run.json")
	scorecard, off, on, err := RunContextAB(RunOptions{
		SuitePath:  suitePath,
		RunID:      "context-run",
		OutputPath: outputPath,
		Now:        fixedEvalTime,
	}, ContextABOptions{})
	if err != nil {
		t.Fatalf("RunContextAB returned error: %v", err)
	}
	if off.RunID != "context-run-context-off" || on.RunID != "context-run-context-on" {
		t.Fatalf("run IDs = off:%s on:%s", off.RunID, on.RunID)
	}
	for _, path := range []string{
		filepath.Join(dir, "run-context-off.json"),
		filepath.Join(dir, "run-context-on.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected leg output %s: %v", path, err)
		}
	}
	if off.Environment.HooksDisabled || on.Environment.HooksDisabled {
		t.Fatalf("context A/B must preserve hooks, got off=%v on=%v", off.Environment.HooksDisabled, on.Environment.HooksDisabled)
	}
	if scorecard.ContextOff.Variant != ContextVariantOff || scorecard.ContextOn.Variant != ContextVariantOn {
		t.Fatalf("scorecard variants = off:%s on:%s", scorecard.ContextOff.Variant, scorecard.ContextOn.Variant)
	}
	if len(scorecard.PerCase) != 1 {
		t.Fatalf("per case len = %d, want 1", len(scorecard.PerCase))
	}
	if scorecard.PerCase[0].ContextOff.ContextRootLabel != string(ContextVariantOff) {
		t.Fatalf("context off root label = %q", scorecard.PerCase[0].ContextOff.ContextRootLabel)
	}
}

func TestRunContextABDerivesDistinctDefaultRunIDs(t *testing.T) {
	dir := t.TempDir()
	writeEvalFile(t, filepath.Join(dir, "fixture.txt"), "context packet canary\n")
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "context.default.ids",
  "name": "Context default IDs",
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
      "title": "static context canary",
      "kind": "artifact_check",
      "objective": "Exercise context A/B default run IDs.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "canary"}
      ]
    }
  ]
}`)

	_, off, on, err := RunContextAB(RunOptions{
		SuitePath: suitePath,
		Now:       fixedEvalTime,
	}, ContextABOptions{})
	if err != nil {
		t.Fatalf("RunContextAB returned error: %v", err)
	}
	if off.RunID == on.RunID {
		t.Fatalf("context leg run IDs should differ: %q", off.RunID)
	}
	if want := "eval-20260424T120000Z-context.default.ids-context-off"; off.RunID != want {
		t.Fatalf("off run ID = %q, want %q", off.RunID, want)
	}
	if want := "eval-20260424T120000Z-context.default.ids-context-on"; on.RunID != want {
		t.Fatalf("on run ID = %q, want %q", on.RunID, want)
	}
}

func TestRunContextABRejectsHookSuppression(t *testing.T) {
	dir := t.TempDir()
	writeEvalFile(t, filepath.Join(dir, "fixture.txt"), "context packet canary\n")
	suitePath := writeEvalSuite(t, dir, `{
  "schema_version": 1,
  "id": "context.hooks",
  "name": "Context hooks",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "environment": {"disable_hooks": true},
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
      "title": "static context canary",
      "kind": "artifact_check",
      "objective": "Exercise context A/B hook guard.",
      "expectations": [
        {"type": "artifact_contains", "target": "fixture.txt", "value": "canary"}
      ]
    }
  ]
}`)

	if _, _, _, err := RunContextAB(RunOptions{SuitePath: suitePath, Now: fixedEvalTime}, ContextABOptions{}); err == nil {
		t.Fatal("RunContextAB accepted suite-level hook suppression")
	}
	if _, _, _, err := RunContextAB(RunOptions{
		SuitePath:            suitePath,
		Now:                  fixedEvalTime,
		OverrideDisableHooks: true,
	}, ContextABOptions{}); err == nil {
		t.Fatal("RunContextAB accepted OverrideDisableHooks")
	}
}

func TestRunContextABSetsAgentsDirPerLeg(t *testing.T) {
	if os.Getenv("GO_WANT_EVAL_CONTEXT_HELPER_PROCESS") == "1" {
		root := os.Getenv("AO_AGENTS_DIR")
		variant := os.Getenv("AO_CONTEXT_VARIANT")
		data, err := os.ReadFile(filepath.Join(root, "variant.txt"))
		if err != nil {
			fmt.Printf(`{"ok":false,"variant":%q,"error":%q}`, variant, err.Error())
			os.Exit(1)
		}
		marker := string(data)
		ok := marker == variant+"\n" && os.Getenv("AGENTOPS_HOOKS_DISABLED") != "1"
		fmt.Printf(`{"ok":%t,"variant":%q,"marker":%q}`, ok, variant, marker)
		if !ok {
			os.Exit(1)
		}
		os.Exit(0)
	}

	dir := t.TempDir()
	offRoot := filepath.Join(dir, "context-off")
	onRoot := filepath.Join(dir, "context-on")
	writeEvalFile(t, filepath.Join(offRoot, "variant.txt"), string(ContextVariantOff)+"\n")
	writeEvalFile(t, filepath.Join(onRoot, "variant.txt"), string(ContextVariantOn)+"\n")
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	suitePath := writeEvalSuite(t, dir, fmt.Sprintf(`{
  "schema_version": 1,
  "id": "context.env",
  "name": "Context env",
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
      "objective": "Verify AO_AGENTS_DIR and AO_CONTEXT_VARIANT are injected per leg.",
      "runtime": "shell",
      "inputs": {
        "argv": [%q, "-test.run=TestRunContextABSetsAgentsDirPerLeg"],
        "env": {"GO_WANT_EVAL_CONTEXT_HELPER_PROCESS": "1", "AO_AGENTS_DIR": "wrong"}
      },
      "expectations": [
        {"type": "exit_code", "value": 0},
        {"type": "json_path", "target": "stdout.ok", "value": true}
      ]
    }
  ]
}`, exe))

	scorecard, off, on, err := RunContextAB(RunOptions{
		SuitePath: suitePath,
		RunID:     "context-env",
		Now:       fixedEvalTime,
	}, ContextABOptions{
		ContextOffAgentsDir: offRoot,
		ContextOnAgentsDir:  onRoot,
		ContextOffLabel:     "empty-fixture",
		ContextOnLabel:      "curated-fixture",
	})
	if err != nil {
		t.Fatalf("RunContextAB returned error: %v", err)
	}
	if off.Status != StatusPass || on.Status != StatusPass {
		t.Fatalf("statuses = off:%s on:%s", off.Status, on.Status)
	}
	if scorecard.ContextOff.ContextRootLabel != "empty-fixture" || scorecard.ContextOn.ContextRootLabel != "curated-fixture" {
		t.Fatalf("labels = off:%q on:%q", scorecard.ContextOff.ContextRootLabel, scorecard.ContextOn.ContextRootLabel)
	}
}
