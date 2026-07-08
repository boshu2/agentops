package epicstatus

import "testing"

// closed builds a resolved, closed member.
func closed(id string) Member { return Member{ID: id, Present: true, Status: "closed"} }

// TestEvaluate_Guards is the L2 table over the three group-terminality guards
// plus the happy path and robustness cases. Each row asserts the exact verdict,
// reason code, terminal flag, and blocker classification.
func TestEvaluate_Guards(t *testing.T) {
	tests := []struct {
		name        string
		group       string
		members     []Member
		wantVerdict Verdict
		wantCode    string
		wantTerm    bool
		wantTotal   int
		wantDone    int
		wantBlock   int
		// wantBlockerClass, when non-empty, must equal the Class of the single
		// blocker (the guard-specific cases each have exactly one blocker).
		wantBlockerClass string
		wantBlockerID    string
	}{
		{
			// Happy path: every descendant closed → terminal/done.
			name:        "all closed → terminal",
			group:       "age-x",
			members:     []Member{closed("age-x.1"), closed("age-x.2"), closed("age-x.3")},
			wantVerdict: Terminal,
			wantCode:    ReasonAllTerminal,
			wantTerm:    true,
			wantTotal:   3,
			wantDone:    3,
			wantBlock:   0,
		},
		{
			// Guard 1: an unresolved/missing member → unknown-status placeholder,
			// never counts as done.
			name:  "missing member → not done (unknown-status)",
			group: "age-x",
			members: []Member{
				closed("age-x.1"),
				{ID: "age-x.2", Present: false},
			},
			wantVerdict:      NotTerminal,
			wantCode:         ReasonUnknownMember,
			wantTerm:         false,
			wantTotal:        2,
			wantDone:         1,
			wantBlock:        1,
			wantBlockerClass: ReasonUnknownMember,
			wantBlockerID:    "age-x.2",
		},
		{
			// Guard 2: a deliberately-open checkpoint/human-gate descendant →
			// not complete.
			name:  "open checkpoint child → not done",
			group: "age-x",
			members: []Member{
				closed("age-x.1"),
				{ID: "age-x.2", Present: true, Status: "open", Labels: []string{"checkpoint"}},
			},
			wantVerdict:      NotTerminal,
			wantCode:         ReasonOpenCheckpoint,
			wantTerm:         false,
			wantTotal:        2,
			wantDone:         1,
			wantBlock:        1,
			wantBlockerClass: ReasonOpenCheckpoint,
			wantBlockerID:    "age-x.2",
		},
		{
			// Guard 3: a zero-descendant, still-materializing group → skipped,
			// NOT done.
			name:        "zero descendants → skipped",
			group:       "age-x",
			members:     nil,
			wantVerdict: Skipped,
			wantCode:    ReasonNoDescendants,
			wantTerm:    false,
			wantTotal:   0,
			wantDone:    0,
			wantBlock:   0,
		},
		{
			// A plain (non-checkpoint) open descendant blocks too, with its own
			// class so guard 2 stays distinguishable.
			name:  "plain open member → not done (open-member)",
			group: "age-x",
			members: []Member{
				closed("age-x.1"),
				{ID: "age-x.2", Present: true, Status: "in_progress"},
			},
			wantVerdict:      NotTerminal,
			wantCode:         ReasonOpenMember,
			wantTerm:         false,
			wantTotal:        2,
			wantDone:         1,
			wantBlock:        1,
			wantBlockerClass: ReasonOpenMember,
			wantBlockerID:    "age-x.2",
		},
		{
			// A deferred descendant is a deliberately-open (checkpoint-class)
			// signal — recognized via status, not just labels.
			name:  "deferred member → open-checkpoint",
			group: "age-x",
			members: []Member{
				closed("age-x.1"),
				{ID: "age-x.2", Present: true, Status: "deferred"},
			},
			wantVerdict:      NotTerminal,
			wantCode:         ReasonOpenCheckpoint,
			wantTerm:         false,
			wantTotal:        2,
			wantDone:         1,
			wantBlock:        1,
			wantBlockerClass: ReasonOpenCheckpoint,
			wantBlockerID:    "age-x.2",
		},
		{
			// Tombstoned/deleted descendants are excluded from the live set: a
			// closed member alongside a tombstone is still terminal, counting
			// only the live member.
			name:  "tombstone excluded → terminal on remaining",
			group: "age-x",
			members: []Member{
				closed("age-x.1"),
				{ID: "age-x.2", Present: true, Status: "tombstone"},
			},
			wantVerdict: Terminal,
			wantCode:    ReasonAllTerminal,
			wantTerm:    true,
			wantTotal:   1,
			wantDone:    1,
			wantBlock:   0,
		},
		{
			// If every descendant is deleted, there are zero LIVE members →
			// skipped (guard 3), not vacuously done.
			name:  "all tombstoned → skipped",
			group: "age-x",
			members: []Member{
				{ID: "age-x.1", Present: true, Status: "tombstone"},
			},
			wantVerdict: Skipped,
			wantCode:    ReasonNoDescendants,
			wantTerm:    false,
			wantTotal:   0,
			wantDone:    0,
			wantBlock:   0,
		},
		{
			// Precedence: an unknown member outranks an open checkpoint for the
			// top-line code (you can't even see the unknown member's state).
			name:  "unknown + checkpoint → unknown-member wins",
			group: "age-x",
			members: []Member{
				closed("age-x.1"),
				{ID: "age-x.2", Present: true, Status: "open", Labels: []string{"checkpoint"}},
				{ID: "age-x.3", Present: false},
			},
			wantVerdict: NotTerminal,
			wantCode:    ReasonUnknownMember,
			wantTerm:    false,
			wantTotal:   3,
			wantDone:    1,
			wantBlock:   2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(tc.group, tc.members)

			if got.Group != tc.group {
				t.Errorf("Group = %q, want %q", got.Group, tc.group)
			}
			if got.Verdict != tc.wantVerdict {
				t.Errorf("Verdict = %q, want %q", got.Verdict, tc.wantVerdict)
			}
			if got.Terminal != tc.wantTerm {
				t.Errorf("Terminal = %v, want %v", got.Terminal, tc.wantTerm)
			}
			if got.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Total != tc.wantTotal {
				t.Errorf("Total = %d, want %d", got.Total, tc.wantTotal)
			}
			if got.Done != tc.wantDone {
				t.Errorf("Done = %d, want %d", got.Done, tc.wantDone)
			}
			if got.Blocking != tc.wantBlock {
				t.Errorf("Blocking = %d, want %d", got.Blocking, tc.wantBlock)
			}
			if len(got.Blockers) != tc.wantBlock {
				t.Errorf("len(Blockers) = %d, want %d", len(got.Blockers), tc.wantBlock)
			}
			if got.Reason == "" {
				t.Errorf("Reason is empty; want a human-readable explanation")
			}
			if tc.wantBlockerClass != "" {
				if len(got.Blockers) != 1 {
					t.Fatalf("expected exactly 1 blocker, got %d", len(got.Blockers))
				}
				if got.Blockers[0].Class != tc.wantBlockerClass {
					t.Errorf("Blockers[0].Class = %q, want %q", got.Blockers[0].Class, tc.wantBlockerClass)
				}
				if got.Blockers[0].ID != tc.wantBlockerID {
					t.Errorf("Blockers[0].ID = %q, want %q", got.Blockers[0].ID, tc.wantBlockerID)
				}
				if got.Blockers[0].Done {
					t.Errorf("Blockers[0].Done = true, want false")
				}
			}
		})
	}
}

// TestEvaluate_UnknownMemberStatusPlaceholder pins that a missing member is
// rolled up with the UnknownStatus placeholder (guard 1's exact shape).
func TestEvaluate_UnknownMemberStatusPlaceholder(t *testing.T) {
	got := Evaluate("age-x", []Member{{ID: "age-x.1", Present: false}})
	if len(got.Members) != 1 {
		t.Fatalf("len(Members) = %d, want 1", len(got.Members))
	}
	m := got.Members[0]
	if m.Status != UnknownStatus {
		t.Errorf("member Status = %q, want %q", m.Status, UnknownStatus)
	}
	if m.Present {
		t.Errorf("member Present = true, want false")
	}
	if m.Done {
		t.Errorf("member Done = true, want false")
	}
}

// TestEvaluate_DeterministicMemberOrder pins that member roll-up order is
// deterministic (sorted by id) regardless of input order.
func TestEvaluate_DeterministicMemberOrder(t *testing.T) {
	a := Evaluate("age-x", []Member{closed("age-x.3"), closed("age-x.1"), closed("age-x.2")})
	b := Evaluate("age-x", []Member{closed("age-x.1"), closed("age-x.2"), closed("age-x.3")})
	if len(a.Members) != len(b.Members) {
		t.Fatalf("member counts differ: %d vs %d", len(a.Members), len(b.Members))
	}
	for i := range a.Members {
		if a.Members[i].ID != b.Members[i].ID {
			t.Errorf("member[%d] id differs: %q vs %q (order not deterministic)", i, a.Members[i].ID, b.Members[i].ID)
		}
	}
	want := []string{"age-x.1", "age-x.2", "age-x.3"}
	for i, id := range want {
		if a.Members[i].ID != id {
			t.Errorf("Members[%d].ID = %q, want %q", i, a.Members[i].ID, id)
		}
	}
}
