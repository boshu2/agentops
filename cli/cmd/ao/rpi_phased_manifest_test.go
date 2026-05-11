// practices: [agile-manifesto, dora-metrics]
package main

import (
	"strings"
	"testing"
)

func TestBuildHandoffContext_WithManifest_Phase2(t *testing.T) {
	handoffs := []*phaseHandoff{
		{
			Phase: 1, PhaseName: "discovery", Status: "completed",
			Goal:              "add auth",
			EpicID:            "ag-123",
			Verdicts:          map[string]string{"pre_mortem": "PASS"},
			ArtifactsProduced: []string{"plan.md"},
			AppliedFindings:   []string{"f-2026-03-09-001"},
			PlanningRules:     []string{"f-2026-03-09-001 — Do not skip prevention context"},
			KnownRisks:        []string{"f-2026-03-09-001 — Validate before implementation"},
			DecisionsMade:     []string{"use JWT"},
			OpenRisks:         []string{"migration downtime"},
			Narrative:         "Discovery completed.",
		},
	}

	// Phase 2 manifest: has decisions + risks, no artifacts
	manifest := defaultPhaseManifests[2]
	ctx := buildHandoffContext(handoffs, manifest)

	if !strings.Contains(ctx, "use JWT") {
		t.Errorf("phase 2 manifest should include decisions\ngot:\n%s", ctx)
	}
	if !strings.Contains(ctx, "migration downtime") {
		t.Errorf("phase 2 manifest should include risks\ngot:\n%s", ctx)
	}
	if !strings.Contains(ctx, "f-2026-03-09-001") {
		t.Errorf("phase 2 manifest should include applied findings\ngot:\n%s", ctx)
	}
	// artifacts_produced is NOT in phase 2 manifest HandoffFields
	if strings.Contains(ctx, "plan.md") {
		t.Errorf("phase 2 manifest should NOT include artifacts\ngot:\n%s", ctx)
	}
}

func TestBuildHandoffContext_WithManifest_Phase3(t *testing.T) {
	handoffs := []*phaseHandoff{
		{
			Phase: 2, PhaseName: "implementation", Status: "completed",
			Goal:              "add auth",
			EpicID:            "ag-456",
			Verdicts:          map[string]string{"crank": "PASS"},
			ArtifactsProduced: []string{"auth.go", "auth_test.go"},
			AppliedFindings:   []string{"f-2026-03-09-001"},
			PlanningRules:     []string{"f-2026-03-09-001 — Do not skip prevention context"},
			KnownRisks:        []string{"f-2026-03-09-001 — Validate before implementation"},
			DecisionsMade:     []string{"use JWT"},
			OpenRisks:         []string{"migration downtime"},
			Narrative:         "Implementation completed.",
		},
	}

	// Phase 3 manifest: has artifacts, no decisions or risks
	manifest := defaultPhaseManifests[3]
	ctx := buildHandoffContext(handoffs, manifest)

	if !strings.Contains(ctx, "auth.go") {
		t.Errorf("phase 3 manifest should include artifacts\ngot:\n%s", ctx)
	}
	if !strings.Contains(ctx, "f-2026-03-09-001") {
		t.Errorf("phase 3 manifest should include applied findings\ngot:\n%s", ctx)
	}
	// decisions_made is NOT in phase 3 manifest HandoffFields
	if strings.Contains(ctx, "use JWT") {
		t.Errorf("phase 3 manifest should NOT include decisions\ngot:\n%s", ctx)
	}
	// open_risks is NOT in phase 3 manifest HandoffFields
	if strings.Contains(ctx, "migration downtime") {
		t.Errorf("phase 3 manifest should NOT include risks\ngot:\n%s", ctx)
	}
	// planning_rules / known_risks are NOT in phase 3 manifest HandoffFields
	if strings.Contains(ctx, "Do not skip prevention context") || strings.Contains(ctx, "Validate before implementation") {
		t.Errorf("phase 3 manifest should NOT include planning rules or known risks\ngot:\n%s", ctx)
	}
}

func TestBuildHandoffContext_NarrativeCap(t *testing.T) {
	longNarrative := strings.Repeat("a", 1000)
	handoffs := []*phaseHandoff{
		{
			Phase: 1, PhaseName: "discovery", Status: "completed",
			Narrative: longNarrative,
		},
	}

	// NarrativeCap=500 should truncate at 500, not 1000
	manifest := phaseManifest{NarrativeCap: 500}
	ctx := buildHandoffContext(handoffs, manifest)

	if !strings.Contains(ctx, "...") {
		t.Error("expected truncation marker for narrative exceeding cap")
	}

	// Extract the narrative line and verify length
	// The truncated narrative should be 500 chars of 'a' + "..."
	truncated := strings.Repeat("a", 500) + "..."
	if !strings.Contains(ctx, truncated) {
		t.Errorf("expected narrative truncated to 500 chars + '...'\ngot:\n%s", ctx)
	}

	// Full 1000-char narrative should NOT appear
	if strings.Contains(ctx, longNarrative) {
		t.Error("full narrative should be truncated at NarrativeCap=500")
	}
}
