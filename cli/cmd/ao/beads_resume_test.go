// Tests for ao beads resume (soc-vuu6.27 slice 3).
//
// Exercise the runBeadsResume flow via the three seams (show / claim /
// append-ledger) so tests never depend on a real bd binary or the live
// provenance ledger.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resumeFixture sets up the three seams + a temp ledger path. Returns a
// pointer to the captured ledger appends so the test can assert on them.
type capturedAppend struct {
	path  string
	event any
}

func resumeFixture(t *testing.T, prior, posterior staleBeadRecord, claimErr error) (*[]capturedAppend, func()) {
	t.Helper()
	prevShow := beadsResumeShowFunc
	prevClaim := beadsResumeClaimFunc
	prevAppend := beadsResumeAppendLedger
	prevAgent := beadsResumeAgentID
	prevLedger := beadsResumeLedgerPath
	prevJSON := beadsResumeJSON
	prevNow := beadsResumeNowOverride

	showCallCount := 0
	beadsResumeShowFunc = func(_ context.Context, _ string) (staleBeadRecord, error) {
		showCallCount++
		if showCallCount == 1 {
			return prior, nil
		}
		return posterior, nil
	}
	beadsResumeClaimFunc = func(_ context.Context, _, _ string) error { return claimErr }

	appends := []capturedAppend{}
	beadsResumeAppendLedger = func(p string, e any) error {
		appends = append(appends, capturedAppend{path: p, event: e})
		return nil
	}

	beadsResumeNowOverride = "2026-05-20T12:00:00Z"

	teardown := func() {
		beadsResumeShowFunc = prevShow
		beadsResumeClaimFunc = prevClaim
		beadsResumeAppendLedger = prevAppend
		beadsResumeAgentID = prevAgent
		beadsResumeLedgerPath = prevLedger
		beadsResumeJSON = prevJSON
		beadsResumeNowOverride = prevNow
	}
	return &appends, teardown
}

func TestRunBeadsResume_HappyPath_WritesClaimTransferredEvent(t *testing.T) {
	prior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "alice", UpdatedAt: "2026-05-20T03:00:00Z"}
	posterior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "bob", UpdatedAt: "2026-05-20T12:00:00Z"}
	appends, teardown := resumeFixture(t, prior, posterior, nil)
	defer teardown()

	beadsResumeAgentID = "bob"
	beadsResumeJSON = true

	buf := &bytes.Buffer{}
	beadsResumeCmd.SetOut(buf)
	defer beadsResumeCmd.SetOut(nil)

	if err := runBeadsResume(beadsResumeCmd, []string{"x"}); err != nil {
		t.Fatalf("runBeadsResume: %v", err)
	}
	if len(*appends) != 1 {
		t.Fatalf("expected 1 ledger append; got %d", len(*appends))
	}

	// Marshal the event back to JSON and inspect shape.
	raw, err := json.Marshal((*appends)[0].event)
	if err != nil {
		t.Fatalf("marshal captured event: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["event_type"] != "claim_transferred" {
		t.Errorf("event_type = %v; want claim_transferred", got["event_type"])
	}
	if got["bead_id"] != "x" {
		t.Errorf("bead_id = %v; want x", got["bead_id"])
	}
	orig := got["original_claimant"].(map[string]any)
	if orig["id"] != "alice" {
		t.Errorf("original_claimant.id = %v; want alice", orig["id"])
	}
	newC := got["new_claimant"].(map[string]any)
	if newC["id"] != "bob" {
		t.Errorf("new_claimant.id = %v; want bob", newC["id"])
	}
	tr := got["transfer"].(map[string]any)
	if tr["prior_revision"] != "alice@2026-05-20T03:00:00Z" {
		t.Errorf("transfer.prior_revision = %v; want alice@2026-05-20T03:00:00Z", tr["prior_revision"])
	}
	if tr["new_revision"] != "bob@2026-05-20T12:00:00Z" {
		t.Errorf("transfer.new_revision = %v; want bob@2026-05-20T12:00:00Z", tr["new_revision"])
	}
	// Stdout JSON also emitted.
	if !strings.Contains(buf.String(), "claim_transferred") {
		t.Errorf("stdout missing claim_transferred; got %q", buf.String())
	}
}

func TestRunBeadsResume_RefusesNonInProgress(t *testing.T) {
	prior := staleBeadRecord{ID: "x", Status: "open", Assignee: "alice", UpdatedAt: "2026-05-20T03:00:00Z"}
	appends, teardown := resumeFixture(t, prior, prior, nil)
	defer teardown()

	beadsResumeAgentID = "bob"
	err := runBeadsResume(beadsResumeCmd, []string{"x"})
	if err == nil {
		t.Fatalf("expected error on non-in-progress bead")
	}
	if !strings.Contains(err.Error(), "not in_progress") {
		t.Errorf("error = %v; want substring 'not in_progress'", err)
	}
	if len(*appends) != 0 {
		t.Errorf("expected no ledger appends; got %d", len(*appends))
	}
}

func TestRunBeadsResume_ClaimErrorBubblesUp(t *testing.T) {
	prior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "alice", UpdatedAt: "2026-05-20T03:00:00Z"}
	appends, teardown := resumeFixture(t, prior, prior, errors.New("bd unavailable"))
	defer teardown()

	beadsResumeAgentID = "bob"
	err := runBeadsResume(beadsResumeCmd, []string{"x"})
	if err == nil {
		t.Fatalf("expected error when bd claim fails")
	}
	if !strings.Contains(err.Error(), "claim transfer") {
		t.Errorf("error = %v; want wrapping 'claim transfer'", err)
	}
	if len(*appends) != 0 {
		t.Errorf("expected no ledger append on claim failure; got %d", len(*appends))
	}
}

func TestRunBeadsResume_AgentResolution_FlagWinsOverEnv(t *testing.T) {
	prior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "alice", UpdatedAt: "2026-05-20T03:00:00Z"}
	posterior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "from-flag", UpdatedAt: "2026-05-20T12:00:00Z"}
	appends, teardown := resumeFixture(t, prior, posterior, nil)
	defer teardown()

	t.Setenv("BEADS_ACTOR", "from-env")
	beadsResumeAgentID = "from-flag"

	if err := runBeadsResume(beadsResumeCmd, []string{"x"}); err != nil {
		t.Fatalf("runBeadsResume: %v", err)
	}
	raw, _ := json.Marshal((*appends)[0].event)
	var got map[string]any
	json.Unmarshal(raw, &got)
	newC := got["new_claimant"].(map[string]any)
	if newC["id"] != "from-flag" {
		t.Errorf("new_claimant.id = %v; want from-flag", newC["id"])
	}
}

func TestRunBeadsResume_AgentResolution_EnvWhenNoFlag(t *testing.T) {
	prior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "alice", UpdatedAt: "2026-05-20T03:00:00Z"}
	posterior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "from-env", UpdatedAt: "2026-05-20T12:00:00Z"}
	appends, teardown := resumeFixture(t, prior, posterior, nil)
	defer teardown()

	t.Setenv("BEADS_ACTOR", "from-env")
	beadsResumeAgentID = ""

	if err := runBeadsResume(beadsResumeCmd, []string{"x"}); err != nil {
		t.Fatalf("runBeadsResume: %v", err)
	}
	raw, _ := json.Marshal((*appends)[0].event)
	var got map[string]any
	json.Unmarshal(raw, &got)
	newC := got["new_claimant"].(map[string]any)
	if newC["id"] != "from-env" {
		t.Errorf("new_claimant.id = %v; want from-env (BEADS_ACTOR fallback)", newC["id"])
	}
}

func TestRunBeadsResume_AgentResolution_DefaultWhenNothingSet(t *testing.T) {
	prior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "alice", UpdatedAt: "2026-05-20T03:00:00Z"}
	posterior := staleBeadRecord{ID: "x", Status: "in_progress", Assignee: "ao-beads-resume", UpdatedAt: "2026-05-20T12:00:00Z"}
	appends, teardown := resumeFixture(t, prior, posterior, nil)
	defer teardown()

	t.Setenv("BEADS_ACTOR", "")
	beadsResumeAgentID = ""

	if err := runBeadsResume(beadsResumeCmd, []string{"x"}); err != nil {
		t.Fatalf("runBeadsResume: %v", err)
	}
	raw, _ := json.Marshal((*appends)[0].event)
	var got map[string]any
	json.Unmarshal(raw, &got)
	newC := got["new_claimant"].(map[string]any)
	if newC["id"] != "ao-beads-resume" {
		t.Errorf("new_claimant.id = %v; want ao-beads-resume (default)", newC["id"])
	}
}

func TestFingerprint(t *testing.T) {
	tests := []struct {
		name string
		in   staleBeadRecord
		want string
	}{
		{"both set", staleBeadRecord{Assignee: "a", UpdatedAt: "2026-05-20T00:00:00Z"}, "a@2026-05-20T00:00:00Z"},
		{"no assignee", staleBeadRecord{Assignee: "", UpdatedAt: "2026-05-20T00:00:00Z"}, "_@2026-05-20T00:00:00Z"},
		{"no updated", staleBeadRecord{Assignee: "a", UpdatedAt: ""}, "a@_"},
		{"neither", staleBeadRecord{}, "unset"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fingerprint(tc.in)
			if got != tc.want {
				t.Errorf("fingerprint(%+v) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBeadsResumeAppendLedger_RealFileSink(t *testing.T) {
	// Restore the real append function so we exercise the actual file IO,
	// then point the ledger at a tempdir.
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.jsonl")

	// Two appends to the same file should produce two lines, each valid JSON.
	if err := beadsResumeAppendLedger(ledger, map[string]any{"a": 1}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := beadsResumeAppendLedger(ledger, map[string]any{"b": 2}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines; got %d (%q)", len(lines), string(data))
	}
	for i, l := range lines {
		if !json.Valid([]byte(l)) {
			t.Errorf("line %d not valid JSON: %q", i, l)
		}
	}
}
