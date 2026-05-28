// Package verdictledger — F5.T1 gap-fill regression tests.
//
// Fills genuine coverage gaps not addressed by verdictledger_test.go:
//   - ValidDirectiveID / ValidVerdict public API boundaries
//   - FailureStreak reset by skip and unknown verdicts
//   - InCooldown exactly at the window boundary (off-by-one guard)
//   - Writer rejects malformed IterationInput / CooldownInput
package verdictledger

import (
	"testing"
	"time"
)

// TestValidDirectiveID_Boundaries pins the stable-ID pattern exactly.
func TestValidDirectiveID_Boundaries(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"d-x", true},
		{"d-reduce-flaky-tests", true},
		{"d-a1b2", true},
		{"d-123", true},
		// Invalid: missing prefix, uppercase, spaces, leading digit after d-
		{"", false},
		{"dx", false},       // no hyphen
		{"D-x", false},      // uppercase
		{"d-X", false},      // uppercase body
		{"d- x", false},     // space
		{"reduce-flaky", false}, // no d- prefix
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			if got := ValidDirectiveID(tc.id); got != tc.want {
				t.Errorf("ValidDirectiveID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

// TestValidVerdict_AllValues exercises every defined verdict constant plus an
// unknown string — the full enum surface.
func TestValidVerdict_AllValues(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{VerdictPass, true},
		{VerdictFail, true},
		{VerdictSkip, true},
		{VerdictUnknown, true},
		{"", false},
		{"maybe", false},
		{"PASS", false},
	}
	for _, tc := range cases {
		t.Run(tc.v, func(t *testing.T) {
			if got := ValidVerdict(tc.v); got != tc.want {
				t.Errorf("ValidVerdict(%q) = %v, want %v", tc.v, got, tc.want)
			}
		})
	}
}

// TestFailureStreak_ResetBySkipAndUnknown verifies that a skip or unknown
// verdict resets the failure streak to 0 (ADR-0006 §FAILURE STREAK).
func TestFailureStreak_ResetBySkipAndUnknown(t *testing.T) {
	sat := 0.5
	cnt := 2
	for _, resetVerdict := range []string{VerdictSkip, VerdictUnknown} {
		t.Run("reset by "+resetVerdict, func(t *testing.T) {
			ledger := &Ledger{
				SchemaVersion: SchemaVersion,
				Records: []Record{
					{RecordType: RecordIteration, DirectiveID: "d-x", RunTime: "2026-05-17T10:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
					{RecordType: RecordIteration, DirectiveID: "d-x", RunTime: "2026-05-17T11:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
					{RecordType: RecordIteration, DirectiveID: "d-x", RunTime: "2026-05-17T12:00:00Z", ScenarioVerdict: resetVerdict, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
					{RecordType: RecordIteration, DirectiveID: "d-x", RunTime: "2026-05-17T13:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
				},
			}
			got := ledger.FailureStreak("d-x")
			if got != 1 {
				t.Errorf("FailureStreak after %s reset = %d, want 1 (only tail fail)", resetVerdict, got)
			}
		})
	}
}

// TestInCooldown_ExactWindowBoundary pins off-by-one behaviour: a cooldown
// record whose timestamp equals the window-start iteration's RunTime is IN
// cooldown; one timestamp strictly before the window-start is OUT.
func TestInCooldown_ExactWindowBoundary(t *testing.T) {
	sat := 0.6
	cnt := 3
	// 4 iterations for d-y, cooldown proposal between iteration 1 and 2.
	ledger := &Ledger{
		SchemaVersion: SchemaVersion,
		Records: []Record{
			{RecordType: RecordIteration, DirectiveID: "d-y", RunTime: "2026-05-17T10:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			{RecordType: RecordIteration, DirectiveID: "d-y", RunTime: "2026-05-17T11:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			{RecordType: RecordIteration, DirectiveID: "d-y", RunTime: "2026-05-17T12:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			{RecordType: RecordIteration, DirectiveID: "d-y", RunTime: "2026-05-17T13:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			// Cooldown timestamp equal to iteration[1] (11:00) → exactly at window boundary for window=3.
			{RecordType: RecordCooldown, DirectiveID: "d-y", RunTime: "2026-05-17T11:00:00Z", CooldownKind: CooldownProposed, MutationType: MutationPriorityBump},
		},
	}
	// Window of 3: window-start = iterations[4-3] = iteration[1] at 11:00.
	// Cooldown at 11:00 >= 11:00 → in cooldown.
	if !ledger.InCooldown("d-y", 3) {
		t.Error("InCooldown(window=3) = false, want true (cooldown at exact window start)")
	}
	// Window of 2: window-start = iteration[2] at 12:00.
	// Cooldown at 11:00 < 12:00 → out of cooldown.
	if ledger.InCooldown("d-y", 2) {
		t.Error("InCooldown(window=2) = true, want false (cooldown before window start)")
	}
}

// TestWriter_RejectsInvalidIterationInput verifies the writer surfaces a
// validation error for a malformed IterationInput (bad directive ID).
func TestWriter_RejectsInvalidIterationInput(t *testing.T) {
	w := Writer{}
	root := t.TempDir()
	_, err := w.AppendIteration(root, IterationInput{
		DirectiveID:          "INVALID",
		RunTime:              time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC),
		ScenarioVerdict:      VerdictFail,
		ScenarioSatisfaction: 0.5,
		ScenarioCount:        2,
		EvaluatedCount:       2,
	})
	if err == nil {
		t.Fatal("AppendIteration(bad directive_id) error = nil, want non-nil")
	}
}

// TestWriter_RejectsInvalidCooldownInput verifies the writer surfaces a
// validation error for a malformed CooldownInput (unknown kind).
func TestWriter_RejectsInvalidCooldownInput(t *testing.T) {
	w := Writer{}
	root := t.TempDir()
	_, err := w.AppendCooldown(root, CooldownInput{
		DirectiveID:  "d-x",
		RunTime:      time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC),
		CooldownKind: "bogus-kind",
		MutationType: MutationPriorityBump,
	})
	if err == nil {
		t.Fatal("AppendCooldown(bad cooldown_kind) error = nil, want non-nil")
	}
}

// TestFailureStreak_AtStreakLengthBoundary pins streak counting exactly at the
// N boundary (3 fails → streak=3, not 2 or 4).
func TestFailureStreak_AtStreakLengthBoundary(t *testing.T) {
	sat := 0.4
	cnt := 3
	ledger := &Ledger{
		SchemaVersion: SchemaVersion,
		Records: []Record{
			{RecordType: RecordIteration, DirectiveID: "d-z", RunTime: "2026-05-17T10:00:00Z", ScenarioVerdict: VerdictPass, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			{RecordType: RecordIteration, DirectiveID: "d-z", RunTime: "2026-05-17T11:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			{RecordType: RecordIteration, DirectiveID: "d-z", RunTime: "2026-05-17T12:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
			{RecordType: RecordIteration, DirectiveID: "d-z", RunTime: "2026-05-17T13:00:00Z", ScenarioVerdict: VerdictFail, ScenarioSatisfaction: &sat, ScenarioCount: &cnt, EvaluatedCount: &cnt},
		},
	}
	got := ledger.FailureStreak("d-z")
	if got != 3 {
		t.Errorf("FailureStreak = %d, want 3 (pass resets, then 3 consecutive fails)", got)
	}
}
