package eval

import (
	"testing"
	"time"
)

func TestIsValidBaselineMode(t *testing.T) {
	for _, mode := range AllBaselineModes() {
		if !IsValidBaselineMode(mode) {
			t.Fatalf("mode %q reported invalid", mode)
		}
	}
	for _, bad := range []string{"", "skill", "off", "ON", "Both"} {
		if IsValidBaselineMode(bad) {
			t.Fatalf("mode %q should be rejected", bad)
		}
	}
}

func TestDeltaSignTriad(t *testing.T) {
	tests := []struct {
		name string
		on   Status
		off  Status
		want int
	}{
		{"both pass", StatusPass, StatusPass, 0},
		{"both fail", StatusFail, StatusFail, 0},
		{"on passes off fails", StatusPass, StatusFail, +1},
		{"off passes on fails", StatusFail, StatusPass, -1},
		{"on passes off inconclusive", StatusPass, StatusInconclusive, +1},
		{"on fail off pass via error", StatusError, StatusPass, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := deltaSign(tc.on, tc.off); got != tc.want {
				t.Fatalf("deltaSign(%s,%s) = %d, want %d", tc.on, tc.off, got, tc.want)
			}
		})
	}
}

func TestComputeDeltaProducesPerCaseAndAggregateDelta(t *testing.T) {
	now := time.Now()
	on := &RunRecord{
		RunID:          "run-on",
		Suite:          SuiteRef{ID: "demo.suite"},
		StartedAt:      now,
		AggregateScore: 0.9,
		CaseResults: []CaseResult{
			{ID: "c1", Status: StatusPass, Score: 1.0},
			{ID: "c2", Status: StatusPass, Score: 1.0},
			{ID: "c3", Status: StatusFail, Score: 0.0},
		},
	}
	off := &RunRecord{
		RunID:          "run-off",
		Suite:          SuiteRef{ID: "demo.suite"},
		StartedAt:      now,
		AggregateScore: 0.4,
		CaseResults: []CaseResult{
			{ID: "c1", Status: StatusFail, Score: 0.0}, // skill saved this
			{ID: "c2", Status: StatusPass, Score: 1.0}, // tied
			{ID: "c3", Status: StatusFail, Score: 0.0}, // both fail
		},
	}
	score := computeDelta("path/to/suite.json", on, off)

	if score.SuiteID != "demo.suite" {
		t.Fatalf("SuiteID: got %q, want demo.suite", score.SuiteID)
	}
	if score.SuitePath != "path/to/suite.json" {
		t.Fatalf("SuitePath: got %q", score.SuitePath)
	}
	if score.SkillOnRunID != "run-on" || score.SkillOffRunID != "run-off" {
		t.Fatalf("RunID wiring: got on=%s off=%s", score.SkillOnRunID, score.SkillOffRunID)
	}
	if got := score.AggregateDelta; got <= 0.49 || got >= 0.51 {
		t.Fatalf("AggregateDelta: got %f, want ~0.5", got)
	}
	if len(score.PerCase) != 3 {
		t.Fatalf("PerCase len: got %d, want 3", len(score.PerCase))
	}
	wantDeltas := map[string]int{"c1": 1, "c2": 0, "c3": 0}
	for _, ad := range score.PerCase {
		want := wantDeltas[ad.CaseID]
		if ad.Delta != want {
			t.Fatalf("case %s delta: got %d, want %d", ad.CaseID, ad.Delta, want)
		}
	}
}

func TestComputeDeltaHandlesMissingOffSideCase(t *testing.T) {
	on := &RunRecord{
		RunID: "on", Suite: SuiteRef{ID: "demo"},
		AggregateScore: 1.0,
		CaseResults:    []CaseResult{{ID: "c1", Status: StatusPass, Score: 1.0}},
	}
	off := &RunRecord{
		RunID: "off", Suite: SuiteRef{ID: "demo"},
		AggregateScore: 0.0,
		CaseResults:    []CaseResult{},
	}
	score := computeDelta("p", on, off)
	if len(score.PerCase) != 1 {
		t.Fatalf("expected 1 case, got %d", len(score.PerCase))
	}
	if score.PerCase[0].SkillOffStatus != StatusInconclusive {
		t.Fatalf("missing off side should fall back to inconclusive, got %s", score.PerCase[0].SkillOffStatus)
	}
	if score.PerCase[0].Delta != 0 {
		t.Fatalf("missing off side should produce delta 0, got %d", score.PerCase[0].Delta)
	}
}

func TestAppendBaselineSuffix(t *testing.T) {
	tests := []struct {
		in     string
		suffix string
		want   string
	}{
		{"foo.json", "skill-on", "foo-skill-on.json"},
		{"a/b/c.json", "skill-off", "a/b/c-skill-off.json"},
		{"noext", "skill-on", "noext-skill-on"},
		{"a/b.dir/c", "skill-off", "a/b.dir/c-skill-off"},
	}
	for _, tc := range tests {
		if got := appendBaselineSuffix(tc.in, tc.suffix); got != tc.want {
			t.Fatalf("appendBaselineSuffix(%q,%q) = %q, want %q", tc.in, tc.suffix, got, tc.want)
		}
	}
}
