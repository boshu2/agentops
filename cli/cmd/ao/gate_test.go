// practices: [design-by-contract, continuous-delivery]
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"strings"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

// TestGatePendingReturnsBronzeCandidates verifies that gate pending
// returns only bronze-tier candidates awaiting human review.
func TestGatePendingReturnsBronzeCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	// Add candidates of different tiers
	bronzeCandidate := types.Candidate{
		ID:       "bronze-pending-1",
		Tier:     types.TierBronze,
		Content:  "Bronze content awaiting review",
		Maturity: types.MaturityProvisional,
	}
	silverCandidate := types.Candidate{
		ID:      "silver-1",
		Tier:    types.TierSilver,
		Content: "Silver content",
	}
	goldCandidate := types.Candidate{
		ID:      "gold-1",
		Tier:    types.TierGold,
		Content: "Gold content",
	}

	// Bronze requires human review
	bronzeScoring := types.Scoring{GateRequired: true}
	silverScoring := types.Scoring{GateRequired: false}
	goldScoring := types.Scoring{GateRequired: false}

	if err := p.Add(bronzeCandidate, bronzeScoring); err != nil {
		t.Fatalf("failed to add bronze candidate: %v", err)
	}
	if err := p.Add(silverCandidate, silverScoring); err != nil {
		t.Fatalf("failed to add silver candidate: %v", err)
	}
	if err := p.Add(goldCandidate, goldScoring); err != nil {
		t.Fatalf("failed to add gold candidate: %v", err)
	}

	// List pending reviews
	pending, err := p.ListPendingReview()
	if err != nil {
		t.Fatalf("ListPendingReview failed: %v", err)
	}

	// Should only return bronze candidates
	if len(pending) != 1 {
		t.Errorf("expected 1 pending review (bronze only), got %d", len(pending))
	}

	if len(pending) > 0 && pending[0].Candidate.ID != "bronze-pending-1" {
		t.Errorf("expected bronze-pending-1, got %s", pending[0].Candidate.ID)
	}
}

// TestGateApproveRecordsReview verifies that approving a candidate
// sets HumanReview fields correctly.
func TestGateApproveRecordsReview(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "approve-test-1",
		Tier:    types.TierBronze,
		Content: "Content to approve",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("failed to add candidate: %v", err)
	}

	// Approve the candidate
	reviewer := "test-reviewer"
	note := "Good specificity, approved"
	if err := p.Approve("approve-test-1", note, reviewer); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	// Verify review was recorded
	entry, err := p.Get("approve-test-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if entry.HumanReview == nil {
		t.Fatal("HumanReview not set after approval")
	}

	if !entry.HumanReview.Reviewed {
		t.Error("HumanReview.Reviewed should be true")
	}

	if !entry.HumanReview.Approved {
		t.Error("HumanReview.Approved should be true")
	}

	if entry.HumanReview.Reviewer != reviewer {
		t.Errorf("expected reviewer %q, got %q", reviewer, entry.HumanReview.Reviewer)
	}

	if entry.HumanReview.Notes != note {
		t.Errorf("expected notes %q, got %q", note, entry.HumanReview.Notes)
	}

	if entry.HumanReview.ReviewedAt.IsZero() {
		t.Error("ReviewedAt timestamp not set")
	}
}

// TestGateRejectRecordsReason verifies that rejecting a candidate
// sets the rejection reason and prevents future promotion.
func TestGateRejectRecordsReason(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-test-1",
		Tier:    types.TierBronze,
		Content: "Content to reject",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("failed to add candidate: %v", err)
	}

	// Reject the candidate
	reason := "Too vague, lacks specificity"
	reviewer := "test-reviewer"
	if err := p.Reject("reject-test-1", reason, reviewer); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// Verify rejection was recorded
	entry, err := p.Get("reject-test-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if entry.Status != types.PoolStatusRejected {
		t.Errorf("expected status rejected, got %s", entry.Status)
	}

	if entry.HumanReview == nil {
		t.Fatal("HumanReview not set after rejection")
	}

	if !entry.HumanReview.Reviewed {
		t.Error("HumanReview.Reviewed should be true")
	}

	if entry.HumanReview.Approved {
		t.Error("HumanReview.Approved should be false for rejection")
	}

	if entry.HumanReview.Notes != reason {
		t.Errorf("expected reason %q, got %q", reason, entry.HumanReview.Notes)
	}
}

// TestGateApproveAlreadyReviewedReturnsError verifies that trying to
// approve an already-reviewed candidate returns an appropriate error.
func TestGateApproveAlreadyReviewedReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "already-reviewed-1",
		Tier:    types.TierBronze,
		Content: "Content already reviewed",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("failed to add candidate: %v", err)
	}

	// First approval should succeed
	if err := p.Approve("already-reviewed-1", "First approval", "first-reviewer"); err != nil {
		t.Fatalf("First Approve failed: %v", err)
	}

	// Second approval should fail with "already reviewed by X" error
	err := p.Approve("already-reviewed-1", "Second approval", "second-reviewer")
	if err == nil {
		t.Fatal("Expected error for already-reviewed candidate, got nil")
	}

	expectedMsg := "already reviewed by first-reviewer"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestGateStateMachine verifies the state machine transitions:
// pending -> approved -> [24h] -> promoted (silver)
// pending -> rejected (terminal)
func TestGateStateMachine(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	t.Run("pending to approved to promoted", func(t *testing.T) {
		candidate := types.Candidate{
			ID:       "state-promote-1",
			Tier:     types.TierBronze,
			Type:     types.KnowledgeTypeLearning,
			Content:  "Learning content for promotion",
			Maturity: types.MaturityCandidate,
		}

		if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
			t.Fatalf("failed to add candidate: %v", err)
		}

		// Verify initial state is pending
		entry, _ := p.Get("state-promote-1")
		if entry.Status != types.PoolStatusPending {
			t.Errorf("expected initial status pending, got %s", entry.Status)
		}

		// Approve
		if err := p.Approve("state-promote-1", "Approved for promotion", "reviewer"); err != nil {
			t.Fatalf("Approve failed: %v", err)
		}

		// Stage for promotion
		if err := p.Stage("state-promote-1", types.TierBronze); err != nil {
			t.Fatalf("Stage failed: %v", err)
		}

		// Verify staged
		entry, _ = p.Get("state-promote-1")
		if entry.Status != types.PoolStatusStaged {
			t.Errorf("expected status staged, got %s", entry.Status)
		}

		// Promote
		artifactPath, err := p.Promote("state-promote-1")
		if err != nil {
			t.Fatalf("Promote failed: %v", err)
		}

		if artifactPath == "" {
			t.Error("expected artifact path, got empty")
		}

		// Verify artifact was created
		if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
			t.Errorf("artifact not created: %s", artifactPath)
		}
	})

	t.Run("pending to rejected is terminal", func(t *testing.T) {
		candidate := types.Candidate{
			ID:      "state-reject-1",
			Tier:    types.TierBronze,
			Content: "Content to be rejected",
		}

		if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
			t.Fatalf("failed to add candidate: %v", err)
		}

		// Reject
		if err := p.Reject("state-reject-1", "Terminal rejection", "reviewer"); err != nil {
			t.Fatalf("Reject failed: %v", err)
		}

		// Verify rejected status
		entry, _ := p.Get("state-reject-1")
		if entry.Status != types.PoolStatusRejected {
			t.Errorf("expected status rejected, got %s", entry.Status)
		}

		// Verify rejected candidates cannot be staged
		err := p.Stage("state-reject-1", types.TierBronze)
		if err == nil {
			t.Error("Expected error when staging rejected candidate")
		}
	})
}

// TestGatePendingOldestFirst verifies that pending reviews are sorted
// by age with oldest items first.
func TestGatePendingOldestFirst(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	// Add candidates - note: we can't easily control AddedAt time,
	// but we verify the sorting logic by checking returned order
	candidates := []types.Candidate{
		{ID: "bronze-a", Tier: types.TierBronze, Content: "First"},
		{ID: "bronze-b", Tier: types.TierBronze, Content: "Second"},
		{ID: "bronze-c", Tier: types.TierBronze, Content: "Third"},
	}

	for _, c := range candidates {
		if err := p.Add(c, types.Scoring{GateRequired: true}); err != nil {
			t.Fatalf("failed to add candidate %s: %v", c.ID, err)
		}
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	pending, err := p.ListPendingReview()
	if err != nil {
		t.Fatalf("ListPendingReview failed: %v", err)
	}

	if len(pending) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(pending))
	}

	// Verify oldest first ordering - first added should be first in list
	if pending[0].Candidate.ID != "bronze-a" {
		t.Errorf("expected bronze-a first (oldest), got %s", pending[0].Candidate.ID)
	}
}

// TestGateChainEventRecording verifies that gate operations
// are recorded in the chain file for audit purposes.
func TestGateChainEventRecording(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "chain-test-1",
		Tier:    types.TierBronze,
		Content: "Content for chain test",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("failed to add candidate: %v", err)
	}

	// Approve
	if err := p.Approve("chain-test-1", "Chain test approval", "chain-reviewer"); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	// Check chain file exists and contains events
	chainPath := filepath.Join(tmpDir, ".agents/pool/chain.jsonl")
	if _, err := os.Stat(chainPath); os.IsNotExist(err) {
		t.Fatal("chain.jsonl not created")
	}

	// Read chain events
	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}

	if len(events) < 2 {
		t.Errorf("expected at least 2 events (add + approve), got %d", len(events))
	}

	// Find approve event
	foundApprove := false
	for _, e := range events {
		if e.Operation == "approve" && e.CandidateID == "chain-test-1" {
			foundApprove = true
			if e.Reviewer != "chain-reviewer" {
				t.Errorf("expected reviewer chain-reviewer, got %s", e.Reviewer)
			}
		}
	}

	if !foundApprove {
		t.Error("approve event not found in chain")
	}
}

// TestGateApproveNotExistReturnsError verifies error handling for
// non-existent candidates.
func TestGateApproveNotExistReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	// Initialize pool directories
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err := p.Approve("nonexistent-id", "note", "reviewer")
	if err == nil {
		t.Fatal("Expected error for non-existent candidate, got nil")
	}
}

// TestGateRejectNotExistReturnsError verifies error handling for
// non-existent candidates.
func TestGateRejectNotExistReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	// Initialize pool directories
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err := p.Reject("nonexistent-id", "reason", "reviewer")
	if err == nil {
		t.Fatal("Expected error for non-existent candidate, got nil")
	}
}

// TestGateRejectReasonTooLong verifies that rejection reason is
// limited to MaxReasonLength (1000 chars).
func TestGateRejectReasonTooLong(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-long-reason",
		Tier:    types.TierBronze,
		Content: "Content to reject with long reason",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("failed to add candidate: %v", err)
	}

	// Create a reason that exceeds 1000 chars
	longReason := make([]byte, 1001)
	for i := range longReason {
		longReason[i] = 'x'
	}

	err := p.Reject("reject-long-reason", string(longReason), "reviewer")
	if err == nil {
		t.Fatal("Expected error for reason exceeding 1000 chars, got nil")
	}

	expectedMsg := "reason/note exceeds maximum length of 1000 characters"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestGateApproveNoteTooLong verifies that approval note is
// limited to MaxReasonLength (1000 chars).
func TestGateApproveNoteTooLong(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "approve-long-note",
		Tier:    types.TierBronze,
		Content: "Content to approve with long note",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("failed to add candidate: %v", err)
	}

	// Create a note that exceeds 1000 chars
	longNote := make([]byte, 1001)
	for i := range longNote {
		longNote[i] = 'x'
	}

	err := p.Approve("approve-long-note", string(longNote), "reviewer")
	if err == nil {
		t.Fatal("Expected error for note exceeding 1000 chars, got nil")
	}

	expectedMsg := "reason/note exceeds maximum length of 1000 characters"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// ---------------------------------------------------------------------------
// gate.go — entryUrgency
// ---------------------------------------------------------------------------

func TestGate_entryUrgency(t *testing.T) {
	tests := []struct {
		name  string
		entry pool.PoolEntry
		want  string
	}{
		{
			name: "approaching auto promote is HIGH",
			entry: pool.PoolEntry{
				ApproachingAutoPromote: true,
				Age:                    23 * time.Hour,
			},
			want: "HIGH (approaching 24h)",
		},
		{
			name: "over 12h is MEDIUM",
			entry: pool.PoolEntry{
				ApproachingAutoPromote: false,
				Age:                    13 * time.Hour,
			},
			want: "MEDIUM",
		},
		{
			name: "under 12h is LOW",
			entry: pool.PoolEntry{
				ApproachingAutoPromote: false,
				Age:                    6 * time.Hour,
			},
			want: "LOW",
		},
		{
			name: "zero age is LOW",
			entry: pool.PoolEntry{
				ApproachingAutoPromote: false,
				Age:                    0,
			},
			want: "LOW",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := entryUrgency(tc.entry)
			if got != tc.want {
				t.Errorf("entryUrgency() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// gate.go — outputGatePending (table mode)
// ---------------------------------------------------------------------------

func TestGate_outputGatePending_table(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	t.Run("empty list", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := outputGatePending(nil)

		_ = w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("outputGatePending: %v", err)
		}

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		_ = r.Close()
		out := string(buf[:n])

		if !strings.Contains(out, "No pending reviews") {
			t.Errorf("expected 'No pending reviews', got: %s", out)
		}
	})

	t.Run("with entries", func(t *testing.T) {
		entries := []pool.PoolEntry{
			{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "gate-entry-1",
						Tier:    types.TierBronze,
						Utility: 0.55,
					},
				},
				AgeString:              "18h",
				Age:                    18 * time.Hour,
				ApproachingAutoPromote: false,
			},
			{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "gate-entry-2",
						Tier:    types.TierBronze,
						Utility: 0.60,
					},
				},
				AgeString:              "23h",
				Age:                    23 * time.Hour,
				ApproachingAutoPromote: true,
			},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := outputGatePending(entries)

		_ = w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("outputGatePending: %v", err)
		}

		buf := make([]byte, 8192)
		n, _ := r.Read(buf)
		_ = r.Close()
		out := string(buf[:n])

		if !strings.Contains(out, "Pending Reviews (2)") {
			t.Errorf("expected 'Pending Reviews (2)', got: %s", out)
		}
		if !strings.Contains(out, "approaching 24h") {
			t.Errorf("expected auto-promote warning, got: %s", out)
		}
	})
}

// ---------------------------------------------------------------------------
// gate.go — outputGatePending (JSON mode)
// ---------------------------------------------------------------------------

func TestGate_outputGatePending_json(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	entries := []pool.PoolEntry{
		{
			PoolEntry: types.PoolEntry{
				Candidate: types.Candidate{
					ID:   "gate-json-1",
					Tier: types.TierBronze,
				},
				Status: types.PoolStatusPending,
			},
			AgeString: "5h",
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGatePending(entries)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputGatePending: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "gate-json-1") {
		t.Errorf("expected JSON with candidate ID, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// gate.go — printAutoPromoteWarning
// ---------------------------------------------------------------------------

func TestGate_printAutoPromoteWarning(t *testing.T) {
	t.Run("no approaching entries", func(t *testing.T) {
		entries := []pool.PoolEntry{
			{ApproachingAutoPromote: false},
			{ApproachingAutoPromote: false},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printAutoPromoteWarning(entries)

		_ = w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		_ = r.Close()
		out := string(buf[:n])

		if strings.Contains(out, "approaching 24h") {
			t.Errorf("should not show warning when no entries approaching, got: %s", out)
		}
	})

	t.Run("some approaching entries", func(t *testing.T) {
		entries := []pool.PoolEntry{
			{ApproachingAutoPromote: false},
			{ApproachingAutoPromote: true},
			{ApproachingAutoPromote: true},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printAutoPromoteWarning(entries)

		_ = w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		_ = r.Close()
		out := string(buf[:n])

		if !strings.Contains(out, "2 candidate(s) approaching 24h") {
			t.Errorf("expected warning about 2 approaching candidates, got: %s", out)
		}
	})
}
