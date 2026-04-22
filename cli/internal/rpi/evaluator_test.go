package rpi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPhaseNameForNumber(t *testing.T) {
	cases := map[int]string{
		1:  "discovery",
		2:  "implementation",
		3:  "validation",
		4:  "phase-4",
		99: "phase-99",
	}
	for in, want := range cases {
		if got := PhaseNameForNumber(in); got != want {
			t.Errorf("PhaseNameForNumber(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPhaseEvaluatorVerdict(t *testing.T) {
	cases := []struct {
		name          string
		phaseNum      int
		trackerMode   string
		gateVerdict   string
		hasTranscript bool
		reward        float64
		want          string
	}{
		{"fail gate -> FAIL", 2, "", "FAIL", false, 1.0, "FAIL"},
		{"blocked gate -> FAIL", 2, "", "blocked", false, 1.0, "FAIL"},
		{"low reward -> FAIL", 2, "", "PASS", true, 0.1, "FAIL"},
		{"warn gate -> WARN", 2, "", "warn", false, 1.0, "WARN"},
		{"partial gate -> WARN", 2, "", "partial", false, 1.0, "WARN"},
		{"skip gate -> WARN", 2, "", "skip", false, 1.0, "WARN"},
		{"phase 1 tasklist -> WARN", 1, "tasklist", "PASS", false, 1.0, "WARN"},
		{"mid reward -> WARN", 2, "", "PASS", true, 0.4, "WARN"},
		{"good reward -> PASS", 2, "", "PASS", true, 0.9, "PASS"},
		{"no transcript -> PASS", 2, "", "PASS", false, 0.0, "PASS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PhaseEvaluatorVerdict(tc.phaseNum, tc.trackerMode, tc.gateVerdict, tc.hasTranscript, tc.reward)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPhaseEvaluatorSummary(t *testing.T) {
	got := PhaseEvaluatorSummary(2, "", "PASS", true, 0.9, 0)
	if !strings.Contains(got, "implementation evaluator marked the phase PASS") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "gate=PASS") {
		t.Errorf("should include gate: %q", got)
	}
	if !strings.Contains(got, "reward=0.90") {
		t.Errorf("should include reward: %q", got)
	}

	// With findings
	got2 := PhaseEvaluatorSummary(2, "", "PASS", false, 0, 3)
	if !strings.Contains(got2, "findings=3") {
		t.Errorf("should include findings: %q", got2)
	}

	// Phase 1 tasklist fallback note
	got3 := PhaseEvaluatorSummary(1, "tasklist", "PASS", false, 0, 0)
	if !strings.Contains(got3, "tracker degraded") {
		t.Errorf("should note tasklist: %q", got3)
	}
}

func TestDefaultEvaluatorFindings(t *testing.T) {
	// FAIL gate produces a finding
	f := DefaultEvaluatorFindings(2, "", "FAIL", false, 0, "", "ref-1")
	if len(f) == 0 {
		t.Fatal("expected FAIL finding")
	}
	if !strings.Contains(f[0].Description, "FAIL") {
		t.Errorf("finding desc = %q", f[0].Description)
	}

	// BLOCKED
	f2 := DefaultEvaluatorFindings(2, "", "BLOCKED", false, 0, "", "ref")
	if len(f2) == 0 || !strings.Contains(f2[0].Description, "blocked") {
		t.Errorf("blocked finding missing: %+v", f2)
	}

	// PARTIAL
	f3 := DefaultEvaluatorFindings(2, "", "PARTIAL", false, 0, "", "ref")
	if len(f3) == 0 || !strings.Contains(f3[0].Description, "partial") {
		t.Errorf("partial finding missing: %+v", f3)
	}

	// phase 1 tasklist
	f4 := DefaultEvaluatorFindings(1, "tasklist", "PASS", false, 0, "", "ref")
	found := false
	for _, item := range f4 {
		if strings.Contains(item.Description, "Tracker degraded") {
			found = true
		}
	}
	if !found {
		t.Errorf("tasklist finding missing: %+v", f4)
	}

	// Low reward transcript
	f5 := DefaultEvaluatorFindings(2, "", "PASS", true, 0.1, "/tmp/transcript", "ref")
	if len(f5) == 0 || !strings.Contains(f5[0].Description, "failing session") {
		t.Errorf("low reward finding missing: %+v", f5)
	}

	// Medium reward transcript
	f6 := DefaultEvaluatorFindings(2, "", "PASS", true, 0.4, "/tmp/transcript", "ref")
	if len(f6) == 0 || !strings.Contains(f6[0].Description, "weak completion") {
		t.Errorf("weak completion finding missing: %+v", f6)
	}

	// Good reward -> no findings
	f7 := DefaultEvaluatorFindings(2, "", "PASS", true, 0.9, "/tmp/transcript", "ref")
	if len(f7) != 0 {
		t.Errorf("expected no findings, got %+v", f7)
	}
}

func TestUniqueFindings(t *testing.T) {
	items := []Finding{
		{Description: "a", Fix: "x"},
		{Description: "a", Fix: "x"},    // duplicate
		{Description: "b"},
		{Description: "", Fix: "", Ref: ""}, // empty -> dropped
	}
	got := UniqueFindings(items)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(got), got)
	}
	if got[0].Description != "a" || got[1].Description != "b" {
		t.Errorf("ordering: %+v", got)
	}
}

func TestSessionIDFromEventDetails(t *testing.T) {
	// Valid
	data, _ := json.Marshal(map[string]any{"session_id": "s123"})
	if got := SessionIDFromEventDetails(data); got != "s123" {
		t.Errorf("got %q", got)
	}

	// Empty
	if got := SessionIDFromEventDetails(nil); got != "" {
		t.Errorf("got %q", got)
	}

	// Invalid JSON
	if got := SessionIDFromEventDetails(json.RawMessage("not json")); got != "" {
		t.Errorf("got %q", got)
	}

	// Missing field
	data2, _ := json.Marshal(map[string]any{"other": "value"})
	if got := SessionIDFromEventDetails(data2); got != "" {
		t.Errorf("got %q", got)
	}

	// Whitespace gets trimmed
	data3, _ := json.Marshal(map[string]any{"session_id": "  s456  "})
	if got := SessionIDFromEventDetails(data3); got != "s456" {
		t.Errorf("got %q", got)
	}
}
