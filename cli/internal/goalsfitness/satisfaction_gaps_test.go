// Package goalsfitness — F2.T1 gap-fill: EvaluateSatisfaction boundary cases not
// covered by satisfaction_test.go.
package goalsfitness

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

func TestEvaluateSatisfaction_ScoreJustBelowPerScenarioThreshold_NotSatisfied(t *testing.T) {
	// Score 0.799 with threshold 0.8 must NOT count as satisfied.
	// Boundary: equality passes (0.8 >= 0.8), but strictly-below must not.
	results := []scenarioresults.ScenarioResult{
		res("s-1", 0.799, 0.8, "fail"),
	}
	a := aggFromResults(results)
	got := a.EvaluateSatisfaction(
		DirectiveLink{DirectiveID: "d-boundary", ScenarioIDs: []string{"s-1"}},
		0.8,
	)
	if got.Verdict != VerdictFail {
		t.Fatalf("Verdict = %q, want %q for score 0.799 < threshold 0.8", got.Verdict, VerdictFail)
	}
	if got.Satisfied != 0 {
		t.Fatalf("Satisfied = %d, want 0 (score strictly below per-scenario threshold)", got.Satisfied)
	}
	if got.Satisfaction != 0 {
		t.Fatalf("Satisfaction = %v, want 0.0", got.Satisfaction)
	}
}

func TestEvaluateSatisfaction_MixedSkipAndPass_SkipsNotCounted(t *testing.T) {
	// One skip + one pass: only the pass contributes to the satisfaction fraction.
	// Satisfaction = 1 satisfied / 2 linked = 0.5.
	// With directive threshold 0.4, that should pass; 0.6, should fail.
	results := []scenarioresults.ScenarioResult{
		res("s-1", 0.0, 0.8, "skip"),
		res("s-2", 0.9, 0.8, "pass"),
	}
	a := aggFromResults(results)
	link := DirectiveLink{DirectiveID: "d-mix-skip", ScenarioIDs: []string{"s-1", "s-2"}}

	// threshold 0.4: 0.5 >= 0.4 → pass.
	got := a.EvaluateSatisfaction(link, 0.4)
	if got.Verdict != VerdictPass {
		t.Fatalf("threshold 0.4: Verdict = %q, want pass", got.Verdict)
	}
	if got.Satisfied != 1 {
		t.Fatalf("Satisfied = %d, want 1 (only the pass-verdict scenario)", got.Satisfied)
	}
	if got.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", got.Skipped)
	}
	if got.Satisfaction != 0.5 {
		t.Fatalf("Satisfaction = %v, want 0.5", got.Satisfaction)
	}

	// threshold 0.6: 0.5 < 0.6 → fail.
	got2 := a.EvaluateSatisfaction(link, 0.6)
	if got2.Verdict != VerdictFail {
		t.Fatalf("threshold 0.6: Verdict = %q, want fail", got2.Verdict)
	}
}

func TestEvaluateSatisfaction_AllMissingNoArtifactResults_IsUnknown(t *testing.T) {
	// An artifact exists but none of the directive's linked scenarios appear in
	// it. Every linked scenario is "missing" → unknown, not fail.
	results := []scenarioresults.ScenarioResult{
		res("s-unrelated", 0.95, 0.8, "pass"),
	}
	a := aggFromResults(results)
	got := a.EvaluateSatisfaction(
		DirectiveLink{DirectiveID: "d-absent", ScenarioIDs: []string{"s-mine-1", "s-mine-2"}},
		0.8,
	)
	if got.Verdict != VerdictUnknown {
		t.Fatalf("Verdict = %q, want unknown when all linked scenarios missing", got.Verdict)
	}
	if got.Missing != 2 {
		t.Fatalf("Missing = %d, want 2", got.Missing)
	}
	if got.Satisfied != 0 {
		t.Fatalf("Satisfied = %d, want 0", got.Satisfied)
	}
}

func TestEvaluateSatisfaction_DirectiveIDMismatchInArtifact_StillLookedUpByScenarioID(t *testing.T) {
	// The aggregator indexes by scenario_id, not directive_id. A result whose
	// directive_id in the artifact differs from the directive we're evaluating
	// must still be counted if the scenario_id matches the link.
	// (The directive_id field in the artifact is informational; the gate enforces
	// links declared in GOALS.md, not artifact labels.)
	results := []scenarioresults.ScenarioResult{
		// directive_id in artifact is "d-other", but we link from "d-mine".
		{
			ScenarioID:  "s-2026-05-17-007",
			DirectiveID: "d-other",
			Score:       0.9,
			Threshold:   0.8,
			Verdict:     "pass",
			JudgedAt:    "2026-05-17T10:00:00Z",
		},
	}
	a := aggFromResults(results)
	got := a.EvaluateSatisfaction(
		DirectiveLink{DirectiveID: "d-mine", ScenarioIDs: []string{"s-2026-05-17-007"}},
		0.8,
	)
	// The scenario is found by its ID and its score (0.9) meets its threshold
	// (0.8), so the directive should pass.
	if got.Verdict != VerdictPass {
		t.Fatalf("Verdict = %q, want pass (scenario found by ID despite artifact directive_id mismatch)", got.Verdict)
	}
	if got.Satisfied != 1 {
		t.Fatalf("Satisfied = %d, want 1", got.Satisfied)
	}
}

func TestEvaluateSatisfaction_DirectiveThresholdOneRequiresAllPass(t *testing.T) {
	// Directive threshold 1.0 means every linked scenario must be individually
	// satisfied. One miss → fail, even if satisfaction fraction is 0.999.
	results := []scenarioresults.ScenarioResult{
		res("s-1", 0.99, 0.8, "pass"),
		res("s-2", 0.99, 0.8, "pass"),
		res("s-3", 0.79, 0.8, "fail"), // score < its threshold → not satisfied
	}
	a := aggFromResults(results)
	got := a.EvaluateSatisfaction(
		DirectiveLink{DirectiveID: "d-strict", ScenarioIDs: []string{"s-1", "s-2", "s-3"}},
		1.0,
	)
	if got.Verdict != VerdictFail {
		t.Fatalf("Verdict = %q, want fail (threshold 1.0 and one scenario unsatisfied)", got.Verdict)
	}
	if got.Satisfied != 2 {
		t.Fatalf("Satisfied = %d, want 2", got.Satisfied)
	}
	wantFrac := 2.0 / 3.0
	if got.Satisfaction != wantFrac {
		t.Fatalf("Satisfaction = %v, want %v", got.Satisfaction, wantFrac)
	}
}
