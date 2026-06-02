package background

import (
	"strings"
	"testing"
)

func TestDecideRequiresEligibilitySignal(t *testing.T) {
	got := Decide(Candidate{ID: "ag-1", Labels: []string{"docs"}})
	if got.Eligible {
		t.Fatalf("candidate without eligibility signal should be ineligible: %+v", got)
	}
	if !reasonsContain(got.Reasons, "missing background eligibility") {
		t.Fatalf("Reasons = %v, want missing eligibility reason", got.Reasons)
	}
}

func TestDecideAllowsEligibleLabel(t *testing.T) {
	got := Decide(Candidate{ID: "ag-1", Labels: []string{"background-agent-safe"}})
	if !got.Eligible {
		t.Fatalf("eligible label should pass: %+v", got)
	}
}

func TestDecideAllowsMetadataEligibility(t *testing.T) {
	got := Decide(Candidate{ID: "ag-1", Metadata: map[string]any{"background_eligible": "true"}})
	if !got.Eligible {
		t.Fatalf("metadata eligibility should pass: %+v", got)
	}
}

func TestDecideExcludesHoldoutEvaluatorPIIAndHuman(t *testing.T) {
	cases := []Candidate{
		{ID: "ag-h", Labels: []string{"background-agent-safe", "holdout"}},
		{ID: "ag-e", Labels: []string{"background-agent-safe", "evaluator"}},
		{ID: "ag-p", Labels: []string{"background-agent-safe", "contains-pii"}},
		{ID: "ag-human", Labels: []string{"background-agent-safe", "human"}},
		{ID: "ag-meta", Labels: []string{"background-agent-safe"}, Metadata: map[string]any{"operator_gated": true}},
	}
	for _, tc := range cases {
		got := Decide(tc)
		if got.Eligible {
			t.Fatalf("%s should be excluded: %+v", tc.ID, got)
		}
	}
}

func TestFilterEligiblePreservesDecisions(t *testing.T) {
	got := FilterEligible([]Candidate{
		{ID: "ag-1", Labels: []string{"background-agent-safe"}},
		{ID: "ag-2", Labels: []string{"docs"}},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if !got[0].Eligible || got[1].Eligible {
		t.Fatalf("eligibility = %+v, want first eligible second excluded", got)
	}
}

func reasonsContain(reasons []string, needle string) bool {
	for _, r := range reasons {
		if strings.Contains(r, needle) {
			return true
		}
	}
	return false
}
