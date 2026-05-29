package main

import "testing"

// TestIngestOutcomesScore_ProducesVerdictRecord: an Outcomes score at/above the
// rubric threshold maps to a PASS verdict record in the one council verdict shape
// (skills/council/schemas/verdict.json) — closing the Outcomes → Knowledge
// Flywheel loop without forking the verdict format.
func TestIngestOutcomesScore_ProducesVerdictRecord(t *testing.T) {
	s := outcomesScore{
		SourceTaskID:     "task-capital-cities",
		JudgeContentHash: "sha256:abc123",
		Aggregate:        0.92,
		Threshold:        0.8,
		CriterionScores:  map[string]float64{"accuracy": 0.95, "concision": 0.85},
	}
	v := ingestOutcomesScore(s)

	if v.Verdict != "PASS" {
		t.Errorf("Verdict = %q, want PASS", v.Verdict)
	}
	if v.SatisfactionScore == nil || *v.SatisfactionScore != 0.92 {
		t.Errorf("SatisfactionScore = %v, want 0.92", v.SatisfactionScore)
	}
	if v.SatisfactionBreakdown["accuracy"] != 0.95 {
		t.Errorf("breakdown[accuracy] = %v, want 0.95", v.SatisfactionBreakdown["accuracy"])
	}
	if v.SchemaVersion != 4 {
		t.Errorf("SchemaVersion = %d, want 4", v.SchemaVersion)
	}
	if v.Findings == nil {
		t.Error("Findings must be a non-nil (possibly empty) slice for verdict.json validity")
	}
}

// TestIngestOutcomesScore_VerdictBands: aggregate below threshold downgrades to
// WARN, and far below (< 70% of threshold) to FAIL.
func TestIngestOutcomesScore_VerdictBands(t *testing.T) {
	cases := []struct {
		agg  float64
		want string
	}{
		{0.90, "PASS"}, // >= 0.8
		{0.70, "WARN"}, // < 0.8 but >= 0.56
		{0.40, "FAIL"}, // < 0.56
	}
	for _, c := range cases {
		got := ingestOutcomesScore(outcomesScore{Aggregate: c.agg, Threshold: 0.8}).Verdict
		if got != c.want {
			t.Errorf("aggregate %.2f: Verdict = %q, want %q", c.agg, got, c.want)
		}
	}
}
