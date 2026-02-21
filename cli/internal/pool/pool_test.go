package pool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestNewPool(t *testing.T) {
	p := NewPool("/tmp/test")
	if p.BaseDir != "/tmp/test" {
		t.Errorf("expected BaseDir /tmp/test, got %s", p.BaseDir)
	}
	if p.PoolPath != "/tmp/test/.agents/pool" {
		t.Errorf("expected PoolPath /tmp/test/.agents/pool, got %s", p.PoolPath)
	}
}

func TestPoolInit(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check directories were created
	dirs := []string{
		filepath.Join(p.PoolPath, PendingDir),
		filepath.Join(p.PoolPath, StagedDir),
		filepath.Join(p.PoolPath, RejectedDir),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("directory not created: %s", dir)
		}
	}
}

func TestPoolAddAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:         "test-candidate-1",
		Type:       types.KnowledgeTypeLearning,
		Tier:       types.TierSilver,
		Content:    "Test learning content",
		Utility:    0.75,
		Confidence: 0.8,
		Maturity:   types.MaturityCandidate,
		Source: types.Source{
			SessionID:      "session-123",
			TranscriptPath: "/path/to/transcript.jsonl",
		},
	}

	scoring := types.Scoring{
		RawScore: 0.72,
		Rubric: types.RubricScores{
			Specificity:   0.8,
			Actionability: 0.7,
			Novelty:       0.6,
			Context:       0.75,
			Confidence:    0.8,
		},
	}

	// Add candidate
	if err := p.Add(candidate, scoring); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Get candidate
	entry, err := p.Get("test-candidate-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if entry.Candidate.ID != "test-candidate-1" {
		t.Errorf("expected ID test-candidate-1, got %s", entry.Candidate.ID)
	}
	if entry.Candidate.Tier != types.TierSilver {
		t.Errorf("expected tier silver, got %s", entry.Candidate.Tier)
	}
}

func TestPoolList(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add test candidates
	candidates := []types.Candidate{
		{ID: "gold-1", Tier: types.TierGold, Content: "Gold content"},
		{ID: "silver-1", Tier: types.TierSilver, Content: "Silver content"},
		{ID: "bronze-1", Tier: types.TierBronze, Content: "Bronze content"},
	}

	for _, c := range candidates {
		if err := p.Add(c, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// List all
	entries, err := p.List(ListOptions{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// List by tier
	goldEntries, err := p.List(ListOptions{Tier: types.TierGold})
	if err != nil {
		t.Fatalf("List gold failed: %v", err)
	}
	if len(goldEntries) != 1 {
		t.Errorf("expected 1 gold entry, got %d", len(goldEntries))
	}
}

func TestPoolStageAndPromote(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:       "promote-test",
		Tier:     types.TierSilver,
		Type:     types.KnowledgeTypeLearning,
		Content:  "Promotable learning",
		Maturity: types.MaturityCandidate,
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Stage
	if err := p.Stage("promote-test", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Verify staged
	entry, err := p.Get("promote-test")
	if err != nil {
		t.Fatalf("Get after stage failed: %v", err)
	}
	if entry.Status != types.PoolStatusStaged {
		t.Errorf("expected status staged, got %s", entry.Status)
	}

	// Promote
	artifactPath, err := p.Promote("promote-test")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if artifactPath == "" {
		t.Error("expected artifact path, got empty")
	}

	// Verify artifact exists
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Errorf("artifact not created: %s", artifactPath)
	}
}

func TestPoolReject(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-test",
		Tier:    types.TierBronze,
		Content: "Rejectable content",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Reject
	if err := p.Reject("reject-test", "Too vague", "tester"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// Verify rejected
	entry, err := p.Get("reject-test")
	if err != nil {
		t.Fatalf("Get after reject failed: %v", err)
	}
	if entry.Status != types.PoolStatusRejected {
		t.Errorf("expected status rejected, got %s", entry.Status)
	}
	if entry.HumanReview == nil || entry.HumanReview.Notes != "Too vague" {
		t.Error("rejection reason not recorded")
	}
}

func TestPoolRejectPreventsPromotion(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-promote-test",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Rejectable content",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Reject the candidate
	if err := p.Reject("reject-promote-test", "Too vague", "tester"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// Attempt to promote rejected candidate should fail
	_, err := p.Promote("reject-promote-test")
	if err == nil {
		t.Error("expected error when promoting rejected candidate")
	}
	if err.Error() != "cannot promote rejected candidate" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Attempt to stage rejected candidate should also fail
	err = p.Stage("reject-promote-test", types.TierBronze)
	if err == nil {
		t.Error("expected error when staging rejected candidate")
	}
	if err.Error() != "cannot stage rejected candidate" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPoolPromoteRequiresStagedStatus(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "pending-promote-test",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Should require staging first",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	_, err := p.Promote("pending-promote-test")
	if err == nil {
		t.Fatal("expected promote from pending to fail")
	}
	if !strings.Contains(err.Error(), "must be staged before promotion") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolBulkApprove(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add old silver candidates
	for i := 0; i < 3; i++ {
		candidate := types.Candidate{
			ID:      string(rune('a'+i)) + "-silver",
			Tier:    types.TierSilver,
			Content: "Silver content",
		}
		if err := p.Add(candidate, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Bulk approve with minimum valid threshold (1 hour)
	// Candidates were just added, so they won't match the threshold,
	// but this tests the function doesn't error.
	approved, err := p.BulkApprove(time.Hour, "bulk-tester", false)
	if err != nil {
		t.Fatalf("BulkApprove failed: %v", err)
	}
	// Candidates were just added, so none should be older than 1 hour
	if len(approved) != 0 {
		t.Errorf("expected 0 approved (none old enough), got %d", len(approved))
	}
}

func TestPoolBulkApproveThresholdTooLow(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Threshold below minimum should return error
	_, err := p.BulkApprove(0, "bulk-tester", false)
	if err != ErrThresholdTooLow {
		t.Errorf("expected ErrThresholdTooLow, got %v", err)
	}

	// Just under 1 hour should also fail
	_, err = p.BulkApprove(59*time.Minute, "bulk-tester", false)
	if err != ErrThresholdTooLow {
		t.Errorf("expected ErrThresholdTooLow for 59m, got %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{30 * time.Minute, "30m"},
		{2 * time.Hour, "2h"},
		{48 * time.Hour, "2d"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.d)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.d, result, tt.expected)
		}
	}
}

func TestIsAboveThreshold(t *testing.T) {
	tests := []struct {
		tier     types.Tier
		minTier  types.Tier
		expected bool
	}{
		{types.TierGold, types.TierBronze, true},
		{types.TierSilver, types.TierSilver, true},
		{types.TierBronze, types.TierSilver, false},
		{types.TierGold, types.TierGold, true},
	}

	for _, tt := range tests {
		result := isAboveThreshold(tt.tier, tt.minTier)
		if result != tt.expected {
			t.Errorf("isAboveThreshold(%s, %s) = %v, expected %v",
				tt.tier, tt.minTier, result, tt.expected)
		}
	}
}

func TestPoolApprove(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "approve-test",
		Tier:    types.TierBronze,
		Content: "Content to approve",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// First approval should succeed
	if err := p.Approve("approve-test", "First approval", "first-reviewer"); err != nil {
		t.Fatalf("First Approve failed: %v", err)
	}

	// Verify review was recorded
	entry, err := p.Get("approve-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.HumanReview == nil || !entry.HumanReview.Reviewed {
		t.Error("HumanReview not recorded after approval")
	}
	if entry.HumanReview.Reviewer != "first-reviewer" {
		t.Errorf("expected reviewer first-reviewer, got %s", entry.HumanReview.Reviewer)
	}
}

func TestPoolApproveAlreadyReviewed(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "already-reviewed",
		Tier:    types.TierBronze,
		Content: "Already reviewed content",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// First approval
	if err := p.Approve("already-reviewed", "First note", "first-reviewer"); err != nil {
		t.Fatalf("First Approve failed: %v", err)
	}

	// Second approval should fail with "already reviewed by X"
	err := p.Approve("already-reviewed", "Second note", "second-reviewer")
	if err == nil {
		t.Fatal("Expected error for already-reviewed candidate")
	}

	expectedMsg := "already reviewed by first-reviewer"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestPoolListPendingReview(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add bronze candidates (only bronze should appear in pending review)
	bronzeCandidate := types.Candidate{
		ID:      "bronze-pending",
		Tier:    types.TierBronze,
		Content: "Bronze content",
	}
	silverCandidate := types.Candidate{
		ID:      "silver-no-review",
		Tier:    types.TierSilver,
		Content: "Silver content",
	}

	if err := p.Add(bronzeCandidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add bronze failed: %v", err)
	}
	if err := p.Add(silverCandidate, types.Scoring{GateRequired: false}); err != nil {
		t.Fatalf("Add silver failed: %v", err)
	}

	pending, err := p.ListPendingReview()
	if err != nil {
		t.Fatalf("ListPendingReview failed: %v", err)
	}

	// Should only return bronze candidates awaiting review
	if len(pending) != 1 {
		t.Errorf("expected 1 pending review (bronze only), got %d", len(pending))
	}

	if len(pending) > 0 && pending[0].Candidate.ID != "bronze-pending" {
		t.Errorf("expected bronze-pending, got %s", pending[0].Candidate.ID)
	}
}

func TestPoolRejectReasonTooLong(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reason-length-test",
		Tier:    types.TierBronze,
		Content: "Content to reject with long reason",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Create a reason that exceeds MaxReasonLength (1000 chars)
	longReason := make([]byte, MaxReasonLength+1)
	for i := range longReason {
		longReason[i] = 'x'
	}

	err := p.Reject("reason-length-test", string(longReason), "reviewer")
	if err != ErrReasonTooLong {
		t.Errorf("expected ErrReasonTooLong, got %v", err)
	}

	// Exactly at max should succeed
	exactReason := make([]byte, MaxReasonLength)
	for i := range exactReason {
		exactReason[i] = 'x'
	}
	err = p.Reject("reason-length-test", string(exactReason), "reviewer")
	if err != nil {
		t.Errorf("expected nil error for reason at max length, got %v", err)
	}
}

func TestPoolApproveNoteTooLong(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "note-length-test",
		Tier:    types.TierBronze,
		Content: "Content to approve with long note",
	}

	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Create a note that exceeds MaxReasonLength (1000 chars)
	longNote := make([]byte, MaxReasonLength+1)
	for i := range longNote {
		longNote[i] = 'x'
	}

	err := p.Approve("note-length-test", string(longNote), "reviewer")
	if err != ErrReasonTooLong {
		t.Errorf("expected ErrReasonTooLong, got %v", err)
	}

	// Exactly at max should succeed
	exactNote := make([]byte, MaxReasonLength)
	for i := range exactNote {
		exactNote[i] = 'x'
	}
	err = p.Approve("note-length-test", string(exactNote), "reviewer")
	if err != nil {
		t.Errorf("expected nil error for note at max length, got %v", err)
	}
}

func TestTruncateAtWordBoundary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected string
	}{
		{
			name:     "short string no truncation",
			input:    "hello world",
			limit:    77,
			expected: "hello world",
		},
		{
			name:     "truncate at word boundary",
			input:    "This is a very long string that needs to be truncated at word boundary properly",
			limit:    40,
			expected: "This is a very long string that needs",
		},
		{
			name:     "no spaces in truncation zone",
			input:    "superlongwordwithoutspaces and more",
			limit:    25,
			expected: "superlongwordwithoutspace",
		},
		{
			name:     "truncate respects last space",
			input:    "word1 word2 word3 word4 word5",
			limit:    15,
			expected: "word1 word2",
		},
		{
			name:     "exact limit equals length",
			input:    "hello",
			limit:    5,
			expected: "hello",
		},
		{
			name:     "single word longer than limit",
			input:    "supercalifragilistic",
			limit:    10,
			expected: "supercalif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateAtWordBoundary(tt.input, tt.limit)
			if result != tt.expected {
				t.Errorf("truncateAtWordBoundary(%q, %d) = %q, expected %q",
					tt.input, tt.limit, result, tt.expected)
			}
		})
	}
}

func TestValidateCandidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr string
	}{
		{"valid simple", "abc-123", ""},
		{"valid underscore", "my_candidate_1", ""},
		{"empty", "", "cannot be empty"},
		{"too long", strings.Repeat("a", 129), "too long"},
		{"invalid chars slash", "../../etc/passwd", "invalid characters"},
		{"invalid chars space", "has space", "invalid characters"},
		{"invalid chars dot", "has.dot", "invalid characters"},
		{"exactly 128 chars", strings.Repeat("x", 128), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCandidateID(tt.id)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestGetChain(t *testing.T) {
	t.Run("no chain file returns empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := NewPool(tmpDir)
		if err := p.Init(); err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		events, err := p.GetChain()
		if err != nil {
			t.Fatalf("GetChain failed: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})

	t.Run("chain records add and stage events", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := NewPool(tmpDir)

		candidate := types.Candidate{
			ID:      "chain-test",
			Tier:    types.TierSilver,
			Content: "Chain test content",
		}
		if err := p.Add(candidate, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := p.Stage("chain-test", types.TierBronze); err != nil {
			t.Fatalf("Stage failed: %v", err)
		}

		events, err := p.GetChain()
		if err != nil {
			t.Fatalf("GetChain failed: %v", err)
		}
		if len(events) < 2 {
			t.Fatalf("expected at least 2 events, got %d", len(events))
		}
		if events[0].Operation != "add" {
			t.Errorf("expected first event operation 'add', got %q", events[0].Operation)
		}
		if events[1].Operation != "stage" {
			t.Errorf("expected second event operation 'stage', got %q", events[1].Operation)
		}
	})

	t.Run("chain records reject event", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := NewPool(tmpDir)

		candidate := types.Candidate{
			ID:      "chain-reject",
			Tier:    types.TierBronze,
			Content: "Chain reject content",
		}
		if err := p.Add(candidate, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := p.Reject("chain-reject", "bad", "reviewer"); err != nil {
			t.Fatalf("Reject failed: %v", err)
		}

		events, err := p.GetChain()
		if err != nil {
			t.Fatalf("GetChain failed: %v", err)
		}

		found := false
		for _, e := range events {
			if e.Operation == "reject" && e.CandidateID == "chain-reject" {
				found = true
				if e.Reason != "bad" {
					t.Errorf("expected reason 'bad', got %q", e.Reason)
				}
				if e.Reviewer != "reviewer" {
					t.Errorf("expected reviewer 'reviewer', got %q", e.Reviewer)
				}
			}
		}
		if !found {
			t.Error("reject event not found in chain")
		}
	})

	t.Run("chain handles malformed lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		p := NewPool(tmpDir)
		if err := p.Init(); err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// Write a chain file with one good and one bad line
		chainPath := filepath.Join(p.PoolPath, ChainFile)
		good := ChainEvent{Operation: "add", CandidateID: "test-1"}
		goodJSON, _ := json.Marshal(good)
		content := string(goodJSON) + "\n{bad json\n"
		if err := os.WriteFile(chainPath, []byte(content), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		events, err := p.GetChain()
		if err != nil {
			t.Fatalf("GetChain failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 valid event (skipping malformed), got %d", len(events))
		}
	})
}

func TestPoolAddInvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty ID", ""},
		{"path traversal", "../evil"},
		{"spaces", "has space"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			p := NewPool(tmpDir)

			err := p.Add(types.Candidate{ID: tt.id, Content: "test"}, types.Scoring{})
			if err == nil {
				t.Error("expected error for invalid candidate ID")
			}
		})
	}
}

func TestPoolGetInvalidID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty ID", ""},
		{"path traversal", "../../etc/passwd"},
		{"too long", strings.Repeat("a", 129)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			p := NewPool(tmpDir)
			if err := p.Init(); err != nil {
				t.Fatalf("Init failed: %v", err)
			}

			_, err := p.Get(tt.id)
			if err == nil {
				t.Error("expected error for invalid candidate ID")
			}
			if !strings.Contains(err.Error(), "invalid candidate ID") {
				t.Errorf("expected 'invalid candidate ID' error, got %q", err.Error())
			}
		})
	}
}

func TestPoolGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err := p.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent candidate")
	}
	if !strings.Contains(err.Error(), "candidate not found") {
		t.Errorf("expected 'candidate not found' error, got %q", err.Error())
	}
}

func TestPoolStageTierBelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "low-tier",
		Tier:    types.TierBronze,
		Content: "Bronze content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Require silver but candidate is bronze
	err := p.Stage("low-tier", types.TierSilver)
	if err == nil {
		t.Error("expected error when tier below threshold")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Errorf("expected 'below minimum' error, got %q", err.Error())
	}
}

func TestPoolAddAt(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	pastTime := time.Now().Add(-48 * time.Hour)
	candidate := types.Candidate{
		ID:      "historical",
		Tier:    types.TierSilver,
		Content: "Historical content",
	}

	if err := p.AddAt(candidate, types.Scoring{}, pastTime); err != nil {
		t.Fatalf("AddAt failed: %v", err)
	}

	entry, err := p.Get("historical")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// AddedAt should reflect the supplied time, not now
	if entry.AddedAt.Sub(pastTime) > time.Second {
		t.Errorf("expected AddedAt near %v, got %v", pastTime, entry.AddedAt)
	}
}

func TestPoolAddWithGateRequired(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "gated",
		Tier:    types.TierBronze,
		Content: "Gated content",
	}
	scoring := types.Scoring{GateRequired: true}

	if err := p.Add(candidate, scoring); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entry, err := p.Get("gated")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if entry.HumanReview == nil {
		t.Fatal("expected HumanReview to be set for gated candidate")
	}
	if entry.HumanReview.Reviewed {
		t.Error("expected HumanReview.Reviewed to be false for new gated candidate")
	}
}

func TestPoolPromoteDecisionType(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "decision-promote",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeDecision,
		Content: "Use PostgreSQL over MySQL for JSONB support",
		Context: "Evaluated during database selection phase",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("decision-promote", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	artifactPath, err := p.Promote("decision-promote")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Decision type should go to patterns directory
	if !strings.Contains(artifactPath, "patterns") {
		t.Errorf("expected artifact in patterns dir, got %s", artifactPath)
	}

	// Verify artifact content
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Decision:") {
		t.Error("expected '# Decision:' header in artifact")
	}
	if !strings.Contains(content, "## Context") {
		t.Error("expected '## Context' section in artifact with context")
	}
}

func TestPoolPromoteSolutionType(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "solution-promote",
		Tier:    types.TierGold,
		Type:    types.KnowledgeTypeSolution,
		Content: "Fix deadlock by acquiring locks in consistent order",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("solution-promote", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	artifactPath, err := p.Promote("solution-promote")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Solution type should go to learnings directory
	if !strings.Contains(artifactPath, "learnings") {
		t.Errorf("expected artifact in learnings dir, got %s", artifactPath)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "# Solution:") {
		t.Error("expected '# Solution:' header in artifact")
	}
}

func TestPoolPromoteDefaultType(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "default-type",
		Tier:    types.TierSilver,
		Type:    "",
		Content: "Some knowledge without explicit type",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("default-type", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	artifactPath, err := p.Promote("default-type")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Default type should go to learnings directory
	if !strings.Contains(artifactPath, "learnings") {
		t.Errorf("expected artifact in learnings dir, got %s", artifactPath)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "# Knowledge:") {
		t.Error("expected '# Knowledge:' header in artifact for default type")
	}
}

func TestPoolPromoteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err := p.Promote("nonexistent")
	if err == nil {
		t.Error("expected error when promoting nonexistent candidate")
	}
}

func TestPoolListByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add candidates
	for _, id := range []string{"a1", "a2", "a3"} {
		if err := p.Add(types.Candidate{ID: id, Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}
	// Stage one
	if err := p.Stage("a1", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// List only pending
	pending, err := p.List(ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("List pending failed: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	// List only staged
	staged, err := p.List(ListOptions{Status: types.PoolStatusStaged})
	if err != nil {
		t.Fatalf("List staged failed: %v", err)
	}
	if len(staged) != 1 {
		t.Errorf("expected 1 staged, got %d", len(staged))
	}
}

func TestPoolListWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	for i := 0; i < 5; i++ {
		id := strings.Repeat(string(rune('a'+i)), 3)
		if err := p.Add(types.Candidate{ID: id, Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	entries, err := p.List(ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries with limit, got %d", len(entries))
	}
}

func TestPoolBulkApproveDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add silver candidates with old timestamps
	for _, id := range []string{"old-silver-1", "old-silver-2"} {
		candidate := types.Candidate{
			ID:      id,
			Tier:    types.TierSilver,
			Content: "Old silver content",
		}
		pastTime := time.Now().Add(-25 * time.Hour)
		if err := p.AddAt(candidate, types.Scoring{}, pastTime); err != nil {
			t.Fatalf("AddAt failed: %v", err)
		}
	}

	// Dry run should return IDs without modifying entries
	approved, err := p.BulkApprove(2*time.Hour, "bulk-tester", true)
	if err != nil {
		t.Fatalf("BulkApprove dry-run failed: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("expected 2 dry-run approved, got %d", len(approved))
	}

	// Verify entries are still unreviewed
	for _, id := range approved {
		entry, err := p.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if entry.HumanReview != nil && entry.HumanReview.Reviewed {
			t.Errorf("dry-run should not modify entries, but %s was reviewed", id)
		}
	}
}

func TestPoolBulkApproveActual(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "bulk-actual",
		Tier:    types.TierSilver,
		Content: "Old silver content",
	}
	pastTime := time.Now().Add(-3 * time.Hour)
	if err := p.AddAt(candidate, types.Scoring{}, pastTime); err != nil {
		t.Fatalf("AddAt failed: %v", err)
	}

	approved, err := p.BulkApprove(2*time.Hour, "bulk-reviewer", false)
	if err != nil {
		t.Fatalf("BulkApprove failed: %v", err)
	}
	if len(approved) != 1 {
		t.Errorf("expected 1 approved, got %d", len(approved))
	}

	// Verify entry was actually approved
	entry, err := p.Get("bulk-actual")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.HumanReview == nil || !entry.HumanReview.Reviewed {
		t.Error("expected entry to be reviewed after bulk approve")
	}
}

func TestPoolScanDirectorySkipsNonJSON(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Add a valid entry
	candidate := types.Candidate{ID: "valid-entry", Tier: types.TierSilver, Content: "valid"}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Write a non-JSON file and a subdirectory to the pending dir
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.WriteFile(filepath.Join(pendingDir, "readme.txt"), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pendingDir, "subdir"), 0700); err != nil {
		t.Fatal(err)
	}

	entries, err := p.List(ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (skipping non-JSON and dirs), got %d", len(entries))
	}
}

func TestPoolScanDirectorySkipsMalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Write a malformed JSON file
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.WriteFile(filepath.Join(pendingDir, "bad.json"), []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}

	// Add a valid entry
	candidate := types.Candidate{ID: "good-entry", Tier: types.TierSilver, Content: "good"}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	entries, err := p.List(ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 valid entry (skipping malformed JSON), got %d", len(entries))
	}
}

func TestPoolWriteArtifactLongTitle(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Content longer than 80 chars on first line triggers truncation
	longContent := "This is a very long first line that exceeds eighty characters to test the word boundary truncation logic in artifact writing"
	candidate := types.Candidate{
		ID:       "long-title",
		Tier:     types.TierSilver,
		Type:     types.KnowledgeTypeLearning,
		Content:  longContent,
		Maturity: types.MaturityCandidate,
		Source: types.Source{
			SessionID:      "sess-1",
			TranscriptPath: "/path/to/transcript.jsonl",
			MessageIndex:   5,
		},
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("long-title", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	artifactPath, err := p.Promote("long-title")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)
	// The first line after "# Learning: " should end with "..."
	lines := strings.Split(content, "\n")
	if !strings.HasSuffix(lines[0], "...") {
		t.Errorf("expected truncated title ending with '...', got %q", lines[0])
	}
}

func TestPoolWriteArtifactMultilineContent(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "multiline",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "First line title\nSecond line detail\nThird line",
		Source: types.Source{
			SessionID:      "sess-1",
			TranscriptPath: "/path/to/transcript.jsonl",
		},
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("multiline", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	artifactPath, err := p.Promote("multiline")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)
	// Title should only be first line
	lines := strings.Split(content, "\n")
	if !strings.Contains(lines[0], "First line title") {
		t.Errorf("expected title to contain 'First line title', got %q", lines[0])
	}
	if strings.Contains(lines[0], "Second line") {
		t.Error("title should not contain second line content")
	}
}

func TestIsAboveThresholdDiscard(t *testing.T) {
	tests := []struct {
		name     string
		tier     types.Tier
		minTier  types.Tier
		expected bool
	}{
		{"discard below bronze", types.TierDiscard, types.TierBronze, false},
		{"discard meets discard", types.TierDiscard, types.TierDiscard, true},
		{"gold above discard", types.TierGold, types.TierDiscard, true},
		{"unknown tier", types.Tier("unknown"), types.TierBronze, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAboveThreshold(tt.tier, tt.minTier)
			if result != tt.expected {
				t.Errorf("isAboveThreshold(%s, %s) = %v, expected %v",
					tt.tier, tt.minTier, result, tt.expected)
			}
		})
	}
}

func TestFormatDurationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{"zero", 0, "0m"},
		{"exactly 1h", time.Hour, "1h"},
		{"exactly 24h", 24 * time.Hour, "1d"},
		{"59 minutes", 59 * time.Minute, "59m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.d)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, expected %s", tt.d, result, tt.expected)
			}
		})
	}
}

func TestAtomicMove(t *testing.T) {
	t.Run("successful move", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcPath := filepath.Join(tmpDir, "source.json")
		destPath := filepath.Join(tmpDir, "dest.json")

		content := []byte(`{"test": true}`)
		if err := os.WriteFile(srcPath, content, 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		if err := atomicMove(srcPath, destPath); err != nil {
			t.Fatalf("atomicMove failed: %v", err)
		}

		// Source should be gone
		if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
			t.Error("source file should be removed after move")
		}

		// Dest should have the content
		data, err := os.ReadFile(destPath)
		if err != nil {
			t.Fatalf("ReadFile dest failed: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("expected content %q, got %q", string(content), string(data))
		}
	})

	t.Run("source does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := atomicMove(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dest"))
		if err == nil {
			t.Error("expected error when source does not exist")
		}
	})
}

func TestPoolListPendingReviewFiltersReviewed(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add two bronze candidates
	for _, id := range []string{"review-pending", "review-done"} {
		candidate := types.Candidate{ID: id, Tier: types.TierBronze, Content: "content"}
		if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Approve one
	if err := p.Approve("review-done", "looks good", "reviewer"); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	pending, err := p.ListPendingReview()
	if err != nil {
		t.Fatalf("ListPendingReview failed: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending review, got %d", len(pending))
	}
	if len(pending) > 0 && pending[0].Candidate.ID != "review-pending" {
		t.Errorf("expected review-pending, got %s", pending[0].Candidate.ID)
	}
}
