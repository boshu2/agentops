// Tests for ao beads stale-claims (soc-vuu6.27 slice 2).
//
// Strategy: table tests for the pure computeStaleEvents function (no exec,
// no clock — all inputs explicit), plus a single L2 integration test that
// drives the cobra command via beadsStaleFetchCmd seam with canned bd-list
// JSON.
//
// Per .claude/rules/go.md: L2 first, L1 always. Exact value assertions,
// not just "not the wrong one". captureStdout for output-producing tests.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestComputeStaleEvents_TableDriven(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		beads     []staleBeadRecord
		threshold float64
		wantIDs   []string // expected bead ids in event output, oldest first
	}{
		{
			name:      "empty input yields no events",
			beads:     nil,
			threshold: 4.0,
			wantIDs:   nil,
		},
		{
			name: "only in_progress beads counted",
			beads: []staleBeadRecord{
				{ID: "a", Status: "open", UpdatedAt: "2026-05-20T00:00:00Z"},
				{ID: "b", Status: "closed", UpdatedAt: "2026-05-20T00:00:00Z"},
				{ID: "c", Status: "in_progress", UpdatedAt: "2026-05-20T00:00:00Z"},
			},
			threshold: 4.0,
			wantIDs:   []string{"c"},
		},
		{
			name: "fresh in_progress (within threshold) not stale",
			beads: []staleBeadRecord{
				{ID: "fresh", Status: "in_progress", UpdatedAt: "2026-05-20T11:00:00Z"},
				{ID: "stale", Status: "in_progress", UpdatedAt: "2026-05-20T03:00:00Z"},
			},
			threshold: 4.0,
			wantIDs:   []string{"stale"},
		},
		{
			name: "oldest first ordering",
			beads: []staleBeadRecord{
				{ID: "five-h", Status: "in_progress", UpdatedAt: "2026-05-20T07:00:00Z"},
				{ID: "ten-h", Status: "in_progress", UpdatedAt: "2026-05-20T02:00:00Z"},
				{ID: "twenty-h", Status: "in_progress", UpdatedAt: "2026-05-19T16:00:00Z"},
			},
			threshold: 4.0,
			wantIDs:   []string{"twenty-h", "ten-h", "five-h"},
		},
		{
			name: "missing updated_at silently skipped",
			beads: []staleBeadRecord{
				{ID: "skip", Status: "in_progress", UpdatedAt: ""},
				{ID: "keep", Status: "in_progress", UpdatedAt: "2026-05-20T02:00:00Z"},
			},
			threshold: 4.0,
			wantIDs:   []string{"keep"},
		},
		{
			name: "malformed updated_at silently skipped",
			beads: []staleBeadRecord{
				{ID: "skip", Status: "in_progress", UpdatedAt: "yesterday"},
				{ID: "keep", Status: "in_progress", UpdatedAt: "2026-05-20T02:00:00Z"},
			},
			threshold: 4.0,
			wantIDs:   []string{"keep"},
		},
		{
			name: "threshold change shifts the cutoff",
			beads: []staleBeadRecord{
				{ID: "two-h", Status: "in_progress", UpdatedAt: "2026-05-20T10:00:00Z"},
				{ID: "twelve-h", Status: "in_progress", UpdatedAt: "2026-05-20T00:00:00Z"},
			},
			// Threshold of 1h catches both; threshold of 8h catches only the older.
			threshold: 1.0,
			wantIDs:   []string{"twelve-h", "two-h"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeStaleEvents(tc.beads, now, tc.threshold)
			gotIDs := make([]string, len(got))
			for i, e := range got {
				gotIDs[i] = e.BeadID
			}
			if !equalStringSlice(gotIDs, tc.wantIDs) {
				t.Fatalf("computeStaleEvents IDs mismatch:\n  got:  %v\n  want: %v", gotIDs, tc.wantIDs)
			}
			// Spot-check non-ID fields on the first event when there is one.
			if len(got) > 0 {
				e := got[0]
				if e.SchemaVersion != 1 {
					t.Errorf("SchemaVersion = %d; want 1", e.SchemaVersion)
				}
				if e.EventType != "stale_detected" {
					t.Errorf("EventType = %q; want stale_detected", e.EventType)
				}
				if e.Evidence.ThresholdHours != tc.threshold {
					t.Errorf("ThresholdHours = %v; want %v", e.Evidence.ThresholdHours, tc.threshold)
				}
				if e.Evidence.LastTouchTS == "" {
					t.Errorf("LastTouchTS empty; want set from bead.UpdatedAt")
				}
				if e.Evidence.ClaimAgeHours <= 0 {
					t.Errorf("ClaimAgeHours = %v; want > 0", e.Evidence.ClaimAgeHours)
				}
			}
		})
	}
}

func TestComputeStaleEvents_AssigneeDefaultUnknown(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	beads := []staleBeadRecord{
		{ID: "with-claim", Status: "in_progress", Assignee: "alice", UpdatedAt: "2026-05-20T00:00:00Z"},
		{ID: "no-claim", Status: "in_progress", Assignee: "", UpdatedAt: "2026-05-20T00:00:00Z"},
	}
	events := computeStaleEvents(beads, now, 4.0)
	if len(events) != 2 {
		t.Fatalf("expected 2 events; got %d", len(events))
	}
	byID := map[string]string{}
	for _, e := range events {
		byID[e.BeadID] = e.OriginalClaimant.ID
	}
	if byID["with-claim"] != "alice" {
		t.Errorf("with-claim claimant = %q; want alice", byID["with-claim"])
	}
	if byID["no-claim"] != "unknown" {
		t.Errorf("no-claim claimant = %q; want unknown", byID["no-claim"])
	}
}

func TestRunBeadsStale_JSON_OutputShape(t *testing.T) {
	// Inject canned bd-list JSON via the seam.
	canned := `[
		{"id":"a","status":"in_progress","assignee":"alice","updated_at":"2026-05-20T03:00:00Z"},
		{"id":"b","status":"in_progress","assignee":"","updated_at":"2026-05-20T11:00:00Z"}
	]`
	prevFetch := beadsStaleFetchCmd
	defer func() { beadsStaleFetchCmd = prevFetch }()
	beadsStaleFetchCmd = func(_ context.Context) ([]byte, error) {
		return []byte(canned), nil
	}

	prevNow := beadsStaleNowOverride
	defer func() { beadsStaleNowOverride = prevNow }()
	beadsStaleNowOverride = "2026-05-20T12:00:00Z"

	prevJSON := beadsStaleJSON
	defer func() { beadsStaleJSON = prevJSON }()
	beadsStaleJSON = true

	buf := &bytes.Buffer{}
	beadsStaleCmd.SetOut(buf)
	defer beadsStaleCmd.SetOut(nil)

	if err := runBeadsStale(beadsStaleCmd, nil); err != nil {
		t.Fatalf("runBeadsStale: %v", err)
	}

	var got []staleEvent
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %s", err, buf.String())
	}
	// `b` was 1h old → below 4h threshold → only `a` should appear.
	if len(got) != 1 {
		t.Fatalf("expected 1 event; got %d", len(got))
	}
	if got[0].BeadID != "a" {
		t.Errorf("BeadID = %q; want a", got[0].BeadID)
	}
	if got[0].OriginalClaimant.ID != "alice" {
		t.Errorf("OriginalClaimant.ID = %q; want alice", got[0].OriginalClaimant.ID)
	}
	if got[0].DetectedAt != "2026-05-20T12:00:00Z" {
		t.Errorf("DetectedAt = %q; want 2026-05-20T12:00:00Z", got[0].DetectedAt)
	}
}

func TestRunBeadsStale_HumanOutput_HasZeroMessage(t *testing.T) {
	prevFetch := beadsStaleFetchCmd
	defer func() { beadsStaleFetchCmd = prevFetch }()
	beadsStaleFetchCmd = func(_ context.Context) ([]byte, error) {
		return []byte("[]"), nil
	}
	prevJSON := beadsStaleJSON
	defer func() { beadsStaleJSON = prevJSON }()
	beadsStaleJSON = false

	buf := &bytes.Buffer{}
	beadsStaleCmd.SetOut(buf)
	defer beadsStaleCmd.SetOut(nil)

	if err := runBeadsStale(beadsStaleCmd, nil); err != nil {
		t.Fatalf("runBeadsStale: %v", err)
	}
	if !strings.Contains(buf.String(), "none") {
		t.Errorf("expected zero-state message containing 'none'; got %q", buf.String())
	}
}

func TestRunBeadsStale_BdMalformed(t *testing.T) {
	prevFetch := beadsStaleFetchCmd
	defer func() { beadsStaleFetchCmd = prevFetch }()
	beadsStaleFetchCmd = func(_ context.Context) ([]byte, error) {
		return []byte("not json"), nil
	}
	err := runBeadsStale(beadsStaleCmd, nil)
	if err == nil {
		t.Fatalf("expected error on malformed bd-list JSON; got nil")
	}
	if !strings.Contains(err.Error(), "parse bd list") {
		t.Errorf("error = %v; want wrapping 'parse bd list'", err)
	}
}

func TestComputeStaleEvents_ClaimAgeRounding(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	// Exactly 5.55 hours ago.
	earlier := now.Add(-5*time.Hour - 33*time.Minute)
	beads := []staleBeadRecord{{
		ID:        "x",
		Status:    "in_progress",
		Assignee:  "a",
		UpdatedAt: earlier.Format(time.RFC3339),
	}}
	got := computeStaleEvents(beads, now, 4.0)
	if len(got) != 1 {
		t.Fatalf("expected 1 event; got %d", len(got))
	}
	// 5h33m = 5.55h, rounded to 1dp = 5.6.
	if got[0].Evidence.ClaimAgeHours != 5.6 {
		t.Errorf("ClaimAgeHours = %v; want 5.6", got[0].Evidence.ClaimAgeHours)
	}
}

// equalStringSlice — small helper local to this test file. Existing _test
// files in cli/cmd/ao define their own; using a unique name avoids clash.
func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
