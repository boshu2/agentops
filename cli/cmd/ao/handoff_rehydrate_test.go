package main

import (
	"strings"
	"testing"
)

// TestParseInProgressBeadID parses the claimed/active bead from `br list
// --status in_progress --json`. (ag-8c00a: the old `bd current` is dead — bd
// is retired; the active bead now comes from br.)
func TestParseInProgressBeadID(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{
			name: "issues wrapper, one in_progress",
			json: `{"issues":[{"id":"ag-8c00a","status":"in_progress","assignee":"bo"}]}`,
			want: "ag-8c00a",
		},
		{
			// br in_progress is global (many stale claims); the freshest
			// updated_at is the lane's active bead — ag-bbb here, not ag-aaa.
			name: "most-recently-updated in_progress wins",
			json: `{"issues":[{"id":"ag-aaa","status":"in_progress","updated_at":"2026-06-06T00:00:00Z"},{"id":"ag-bbb","status":"in_progress","updated_at":"2026-06-15T12:00:00Z"}]}`,
			want: "ag-bbb",
		},
		{
			name: "bare array shape also handled",
			json: `[{"id":"ag-zzz","status":"in_progress"}]`,
			want: "ag-zzz",
		},
		{name: "empty", json: `{"issues":[]}`, want: ""},
		{name: "garbage is safe", json: `not json`, want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseInProgressBeadID(c.json); got != c.want {
				t.Errorf("parseInProgressBeadID() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestParseReadyCount counts ready beads from `br ready --json`.
func TestParseReadyCount(t *testing.T) {
	cases := []struct {
		json string
		want int
	}{
		{`{"issues":[{"id":"a"},{"id":"b"},{"id":"c"}]}`, 3},
		{`[{"id":"a"}]`, 1},
		{`{"issues":[]}`, 0},
		{`garbage`, 0},
	}
	for _, c := range cases {
		if got := parseReadyCount(c.json); got != c.want {
			t.Errorf("parseReadyCount(%q) = %d, want %d", c.json, got, c.want)
		}
	}
}

// TestParseReservationPaths captures held file reservations (so a rehydrating
// agent restores its lock landscape) from `am robot reservations`.
func TestParseReservationPaths(t *testing.T) {
	j := `{"all_active":[{"agent":"EmeraldJaguar","path":"cli/"},{"agent":"RedIsland","path":"yieldledger/"}]}`
	got := parseReservationPaths(j)
	if len(got) != 2 {
		t.Fatalf("got %d reservations, want 2: %v", len(got), got)
	}
	joined := strings.Join(got, " | ")
	for _, want := range []string{"cli/", "EmeraldJaguar", "yieldledger/", "RedIsland"} {
		if !strings.Contains(joined, want) {
			t.Errorf("reservations missing %q: %v", want, got)
		}
	}
	// Empty / absent is safe (am unavailable → no reservations, no panic).
	if r := parseReservationPaths(`{"all_active":[]}`); len(r) != 0 {
		t.Errorf("empty all_active should yield no reservations, got %v", r)
	}
	if r := parseReservationPaths(`garbage`); len(r) != 0 {
		t.Errorf("garbage should yield no reservations, got %v", r)
	}
}

// TestDeriveContinuation builds a non-empty continuation pointer from the
// captured state so a rehydrating agent knows the next action (scenario 3).
func TestDeriveContinuation(t *testing.T) {
	withBead := deriveContinuation(&handoffState{ActiveBead: "ag-8c00a", OpenBeadsCount: 4})
	if withBead == "" {
		t.Fatal("continuation must be non-empty when an active bead is present")
	}
	if !strings.Contains(withBead, "ag-8c00a") {
		t.Errorf("continuation should name the active bead, got %q", withBead)
	}

	// No active bead → empty (don't fabricate a next action).
	if c := deriveContinuation(&handoffState{}); c != "" {
		t.Errorf("no active bead should yield empty continuation, got %q", c)
	}
	if c := deriveContinuation(nil); c != "" {
		t.Errorf("nil state should yield empty continuation, got %q", c)
	}
}
