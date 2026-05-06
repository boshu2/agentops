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
	// Structural-evidence rule (soc-3mtv): when gateVerdict is PASS or DONE
	// (positive structural signal — tracker says cycle work is complete), the
	// transcript-derived reward heuristic MUST NOT downgrade the verdict.
	// Previous behavior: low/mid reward overrode a PASS gate to WARN/FAIL,
	// producing false-WARN/FAIL on legitimate cycles whose transcript shape
	// didn't match the regex set.
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
		// soc-3mtv: PASS gate is authoritative regardless of reward.
		{"pass gate + low reward -> PASS (structural override)", 2, "", "PASS", true, 0.1, "PASS"},
		{"pass gate + mid reward -> PASS (structural override)", 2, "", "PASS", true, 0.4, "PASS"},
		{"pass gate + good reward -> PASS", 2, "", "PASS", true, 0.9, "PASS"},
		{"done gate + low reward -> PASS (structural override)", 2, "", "DONE", true, 0.1, "PASS"},
		{"done gate + mid reward -> PASS (structural override)", 2, "", "done", true, 0.4, "PASS"},
		{"done gate + no transcript -> PASS", 2, "", "DONE", false, 0.0, "PASS"},
		{"warn gate -> WARN", 2, "", "warn", false, 1.0, "WARN"},
		{"partial gate -> WARN", 2, "", "partial", false, 1.0, "WARN"},
		{"skip gate -> WARN", 2, "", "skip", false, 1.0, "WARN"},
		{"phase 1 tasklist -> WARN", 1, "tasklist", "PASS", false, 1.0, "PASS"}, // Note: PASS gate now authoritative even for tasklist
		// Empty/unknown gate falls back to reward heuristic (no structural signal).
		{"empty gate + low reward -> FAIL", 2, "", "", true, 0.1, "FAIL"},
		{"empty gate + mid reward -> WARN", 2, "", "", true, 0.4, "WARN"},
		{"empty gate + good reward -> PASS", 2, "", "", true, 0.9, "PASS"},
		{"empty gate + no transcript -> PASS", 2, "", "", false, 0.0, "PASS"},
		{"phase 1 tasklist + empty gate -> WARN", 1, "tasklist", "", false, 1.0, "WARN"},
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

	// soc-3mtv: when gate is PASS or DONE (positive structural signal),
	// transcript-derived reward findings are suppressed — they were producing
	// false-positive "weak completion" warnings on legitimate cycles.
	f5 := DefaultEvaluatorFindings(2, "", "PASS", true, 0.1, "/tmp/transcript", "ref")
	if len(f5) != 0 {
		t.Errorf("PASS gate must suppress low-reward findings, got %+v", f5)
	}

	f6 := DefaultEvaluatorFindings(2, "", "PASS", true, 0.4, "/tmp/transcript", "ref")
	if len(f6) != 0 {
		t.Errorf("PASS gate must suppress mid-reward findings, got %+v", f6)
	}

	f6b := DefaultEvaluatorFindings(2, "", "DONE", true, 0.1, "/tmp/transcript", "ref")
	if len(f6b) != 0 {
		t.Errorf("DONE gate must suppress low-reward findings, got %+v", f6b)
	}

	// Good reward -> no findings (regardless of gate)
	f7 := DefaultEvaluatorFindings(2, "", "PASS", true, 0.9, "/tmp/transcript", "ref")
	if len(f7) != 0 {
		t.Errorf("expected no findings, got %+v", f7)
	}

	// Empty/unknown gate falls back to transcript heuristic — low/mid reward findings still emitted.
	f8 := DefaultEvaluatorFindings(2, "", "", true, 0.1, "/tmp/transcript", "ref")
	if len(f8) == 0 || !strings.Contains(f8[0].Description, "failing session") {
		t.Errorf("empty gate + low reward must surface failing-session finding: %+v", f8)
	}
	f9 := DefaultEvaluatorFindings(2, "", "", true, 0.4, "/tmp/transcript", "ref")
	if len(f9) == 0 || !strings.Contains(f9[0].Description, "weak completion") {
		t.Errorf("empty gate + mid reward must surface weak-completion finding: %+v", f9)
	}
}

func TestUniqueFindings(t *testing.T) {
	items := []Finding{
		{Description: "a", Fix: "x"},
		{Description: "a", Fix: "x"}, // duplicate
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
