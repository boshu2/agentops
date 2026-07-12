package gate

import (
	"context"
	"testing"
	"time"

	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestReviewPoolListsAndMapsPendingEntries(t *testing.T) {
	root := t.TempDir()
	p := pool.NewPool(root)
	candidate := types.Candidate{ID: "cand-1", Tier: types.TierBronze, Content: "review me", Utility: 0.61}
	if err := p.AddAt(candidate, types.Scoring{GateRequired: true}, time.Now().Add(-13*time.Hour)); err != nil {
		t.Fatalf("AddAt: %v", err)
	}
	adapter := NewReviewPool(root, func() string { return "reviewer" })
	entries, err := adapter.ListPending(context.Background())
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(entries) != 1 || entries[0].Candidate.ID != "cand-1" || entries[0].Candidate.Tier != types.TierBronze || entries[0].Candidate.Utility != 0.61 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestReviewPoolPreservesNilPendingSlice(t *testing.T) {
	entries, err := NewReviewPool(t.TempDir(), nil).ListPending(context.Background())
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if entries != nil {
		t.Fatalf("entries = %#v, want nil to preserve legacy JSON null", entries)
	}
}

func TestReviewPoolDelegatesApproveWithReviewer(t *testing.T) {
	root := t.TempDir()
	p := pool.NewPool(root)
	if err := p.Add(types.Candidate{ID: "cand-1", Tier: types.TierBronze, Content: "review me"}, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	adapter := NewReviewPool(root, func() string { return "system-reviewer" })
	if got := adapter.Reviewer(); got != "system-reviewer" {
		t.Fatalf("Reviewer = %q", got)
	}
	if err := adapter.Approve(context.Background(), gateapp.ApproveRequest{CandidateID: "cand-1", Note: "good", Reviewer: adapter.Reviewer()}); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	entry, err := p.Get("cand-1")
	if err != nil || entry.HumanReview == nil || entry.HumanReview.Reviewer != "system-reviewer" {
		t.Fatalf("entry=%+v err=%v", entry, err)
	}
}

func TestReviewPoolHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	adapter := NewReviewPool(t.TempDir(), nil)
	if _, err := adapter.ListPending(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
}
