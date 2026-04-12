package overnight

import "testing"

// TestIterationStatus_Validate exercises the exhaustive enum check plus
// the two error paths (empty and unknown legacy literal). The legacy
// "rolled-back" string is explicitly listed as invalid — it was the C2
// status-lie that Micro-epic 3 split into two distinct constants.
func TestIterationStatus_Validate(t *testing.T) {
	cases := []struct {
		name    string
		s       IterationStatus
		wantErr bool
	}{
		{"done valid", StatusDone, false},
		{"degraded valid", StatusDegraded, false},
		{"pre-commit rollback valid", StatusRolledBackPreCommit, false},
		{"post-commit halt valid", StatusHaltedOnRegressionPostCommit, false},
		{"failed valid", StatusFailed, false},
		{"empty invalid", "", true},
		{"legacy rolled-back invalid", "rolled-back", true},
		{"garbage invalid", "foo", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.s.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate(%q) err = %v, wantErr = %v", c.s, err, c.wantErr)
			}
		})
	}
}

// TestIterationStatus_IsCorpusCompounded locks the truth table: done and
// halted-on-regression-post-commit have corpus on disk; degraded,
// pre-commit rollback, and failed do not. This is the rehydration predicate;
// if it drifts, resume semantics break.
func TestIterationStatus_IsCorpusCompounded(t *testing.T) {
	cases := []struct {
		s    IterationStatus
		want bool
	}{
		{StatusDone, true},
		{StatusDegraded, false},
		{StatusHaltedOnRegressionPostCommit, true},
		{StatusRolledBackPreCommit, false},
		{StatusFailed, false},
	}
	for _, c := range cases {
		if got := c.s.IsCorpusCompounded(); got != c.want {
			t.Errorf("%s.IsCorpusCompounded() = %v, want %v", c.s, got, c.want)
		}
	}
}

// TestIterationStatus_UnknownIsNotCompounded guards that an unknown
// status string (e.g., a legacy on-disk "rolled-back" or a corrupted
// value) falls through to IsCorpusCompounded == false. This matches
// the conservative fallback documented in the plan: old files with
// the legacy string are treated as non-baselines during rehydration.
func TestIterationStatus_UnknownIsNotCompounded(t *testing.T) {
	legacy := IterationStatus("rolled-back")
	if legacy.IsCorpusCompounded() {
		t.Errorf("legacy %q should not be treated as compounded", legacy)
	}
	garbage := IterationStatus("totally-made-up")
	if garbage.IsCorpusCompounded() {
		t.Errorf("garbage %q should not be treated as compounded", garbage)
	}
}

// TestIterationStatus_StringValues locks the on-disk wire format. If
// any constant's string value changes, persisted iter-<N>.json files
// would silently fail to rehydrate and this test catches the drift.
func TestIterationStatus_StringValues(t *testing.T) {
	cases := map[IterationStatus]string{
		StatusDone:                         "done",
		StatusDegraded:                     "degraded",
		StatusRolledBackPreCommit:          "rolled-back-pre-commit",
		StatusHaltedOnRegressionPostCommit: "halted-on-regression-post-commit",
		StatusFailed:                       "failed",
	}
	for s, want := range cases {
		if string(s) != want {
			t.Errorf("string(%v) = %q, want %q", s, string(s), want)
		}
	}
}
