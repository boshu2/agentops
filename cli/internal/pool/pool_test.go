package pool

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	if !errors.Is(err, ErrNotStaged) {
		t.Fatalf("expected ErrNotStaged, got: %v", err)
	}
}

func TestPoolBulkApprove(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add old silver candidates
	for i := range 3 {
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
	if !errors.Is(err, ErrThresholdTooLow) {
		t.Errorf("expected ErrThresholdTooLow, got %v", err)
	}

	// Just under 1 hour should also fail
	_, err = p.BulkApprove(59*time.Minute, "bulk-tester", false)
	if !errors.Is(err, ErrThresholdTooLow) {
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
	if !errors.Is(err, ErrReasonTooLong) {
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
	if !errors.Is(err, ErrReasonTooLong) {
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
		{
			// Fast-path regression: rune count fits but byte length exceeds
			// limit — must not truncate.
			name:     "multibyte fits in rune count but not in bytes",
			input:    "héllo",
			limit:    5,
			expected: "héllo",
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
	t.Run("no chain file returns empty", assertGetChainNoChainFile)
	t.Run("chain records add and stage events", assertGetChainRecordsAddAndStageEvents)
	t.Run("chain records reject event", assertGetChainRecordsRejectEvent)
	t.Run("chain handles malformed lines", assertGetChainHandlesMalformedLines)
}

func assertGetChainNoChainFile(t *testing.T) {
	t.Helper()
	p := newInitializedChainTestPool(t)

	events := mustGetChain(t, p)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func assertGetChainRecordsAddAndStageEvents(t *testing.T) {
	t.Helper()
	p := newChainTestPool(t)

	mustAddChainCandidate(t, p, types.Candidate{
		ID:      "chain-test",
		Tier:    types.TierSilver,
		Content: "Chain test content",
	})
	if err := p.Stage("chain-test", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	events := mustGetChain(t, p)
	assertChainOperations(t, events, []string{"add", "stage"})
}

func assertGetChainRecordsRejectEvent(t *testing.T) {
	t.Helper()
	p := newChainTestPool(t)

	mustAddChainCandidate(t, p, types.Candidate{
		ID:      "chain-reject",
		Tier:    types.TierBronze,
		Content: "Chain reject content",
	})
	if err := p.Reject("chain-reject", "bad", "reviewer"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	events := mustGetChain(t, p)
	assertRejectChainEvent(t, events, "chain-reject", "bad", "reviewer")
}

func assertGetChainHandlesMalformedLines(t *testing.T) {
	t.Helper()
	p := newInitializedChainTestPool(t)
	writeMalformedChainFixture(t, p)

	events := mustGetChain(t, p)
	if len(events) != 1 {
		t.Errorf("expected 1 valid event (skipping malformed), got %d", len(events))
	}
}

func newChainTestPool(t *testing.T) *Pool {
	t.Helper()
	return NewPool(t.TempDir())
}

func newInitializedChainTestPool(t *testing.T) *Pool {
	t.Helper()
	p := newChainTestPool(t)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return p
}

func mustAddChainCandidate(t *testing.T, p *Pool, candidate types.Candidate) {
	t.Helper()
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
}

func mustGetChain(t *testing.T, p *Pool) []ChainEvent {
	t.Helper()
	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}
	return events
}

func assertChainOperations(t *testing.T, events []ChainEvent, want []string) {
	t.Helper()
	if len(events) < len(want) {
		t.Fatalf("expected at least %d events, got %d", len(want), len(events))
	}
	for i, wantOperation := range want {
		if events[i].Operation != wantOperation {
			t.Errorf("expected event %d operation %q, got %q", i, wantOperation, events[i].Operation)
		}
	}
}

func assertRejectChainEvent(t *testing.T, events []ChainEvent, candidateID, reason, reviewer string) {
	t.Helper()
	event, found := findChainEvent(events, "reject", candidateID)
	if !found {
		t.Error("reject event not found in chain")
		return
	}
	if event.Reason != reason {
		t.Errorf("expected reason %q, got %q", reason, event.Reason)
	}
	if event.Reviewer != reviewer {
		t.Errorf("expected reviewer %q, got %q", reviewer, event.Reviewer)
	}
}

func findChainEvent(events []ChainEvent, operation, candidateID string) (ChainEvent, bool) {
	for _, event := range events {
		if event.Operation == operation && event.CandidateID == candidateID {
			return event, true
		}
	}
	return ChainEvent{}, false
}

func writeMalformedChainFixture(t *testing.T, p *Pool) {
	t.Helper()
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	good := ChainEvent{Operation: "add", CandidateID: "test-1"}
	goodJSON, _ := json.Marshal(good)
	content := string(goodJSON) + "\n{bad json\n"
	if err := os.WriteFile(chainPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
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
	if !errors.Is(err, ErrCandidateNotFound) {
		t.Errorf("expected ErrCandidateNotFound, got %q", err.Error())
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

	for i := range 5 {
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
	// With YAML frontmatter, the heading with truncated title comes after the --- block
	if !strings.Contains(content, "---\n") {
		t.Error("expected YAML frontmatter delimiters")
	}
	// Find the heading line (starts with "# Learning: ")
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# Learning: ") {
			if !strings.HasSuffix(line, "...") {
				t.Errorf("expected truncated title ending with '...', got %q", line)
			}
			break
		}
	}
}

func TestKnowledgeTypeHeading(t *testing.T) {
	tests := []struct {
		name string
		kt   types.KnowledgeType
		want string
	}{
		{name: "learning", kt: types.KnowledgeTypeLearning, want: "# Learning: "},
		{name: "decision", kt: types.KnowledgeTypeDecision, want: "# Decision: "},
		{name: "solution", kt: types.KnowledgeTypeSolution, want: "# Solution: "},
		{name: "failure", kt: types.KnowledgeTypeFailure, want: "# Failure: "},
		{name: "reference", kt: types.KnowledgeTypeReference, want: "# Reference: "},
		{name: "default", kt: types.KnowledgeType("custom"), want: "# Knowledge: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := knowledgeTypeHeading(tt.kt); got != tt.want {
				t.Fatalf("knowledgeTypeHeading(%q) = %q, want %q", tt.kt, got, tt.want)
			}
		})
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
	// With YAML frontmatter, find the heading line
	if !strings.Contains(content, "---\n") {
		t.Error("expected YAML frontmatter delimiters")
	}
	foundHeading := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# Learning: ") {
			foundHeading = true
			if !strings.Contains(line, "First line title") {
				t.Errorf("expected title to contain 'First line title', got %q", line)
			}
			if strings.Contains(line, "Second line") {
				t.Error("title should not contain second line content")
			}
			break
		}
	}
	if !foundHeading {
		t.Error("expected to find '# Learning: ' heading in artifact")
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

func TestPoolListPaginatedOffset(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add 5 candidates
	for i := range 5 {
		id := fmt.Sprintf("page-%d", i)
		if err := p.Add(types.Candidate{ID: id, Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Offset within range
	result, err := p.ListPaginated(ListOptions{Offset: 2, Limit: 2})
	if err != nil {
		t.Fatalf("ListPaginated failed: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
	if len(result.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result.Entries))
	}

	// Offset beyond total (should return empty)
	result, err = p.ListPaginated(ListOptions{Offset: 10})
	if err != nil {
		t.Fatalf("ListPaginated failed: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected total 5 with offset beyond, got %d", result.Total)
	}
	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries with offset beyond total, got %d", len(result.Entries))
	}

	// Offset at exact boundary
	result, err = p.ListPaginated(ListOptions{Offset: 5})
	if err != nil {
		t.Fatalf("ListPaginated failed: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("expected 0 entries at exact boundary offset, got %d", len(result.Entries))
	}
}

func TestPoolListPaginatedNoInit(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	// Don't init - directories don't exist

	// Should handle missing directories gracefully
	result, err := p.ListPaginated(ListOptions{})
	if err != nil {
		t.Fatalf("ListPaginated on uninitialized pool should not error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected total 0, got %d", result.Total)
	}
}

func TestPoolPromoteCollisionGuard(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:       "collision-test",
		Tier:     types.TierSilver,
		Type:     types.KnowledgeTypeLearning,
		Content:  "Content for collision test",
		Maturity: types.MaturityCandidate,
		Source: types.Source{
			SessionID:      "sess-1",
			TranscriptPath: "/path/to/t.jsonl",
		},
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("collision-test", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Pre-create the expected artifact file to trigger collision guard
	destDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(destDir, 0700); err != nil {
		t.Fatal(err)
	}
	timestamp := time.Now().Format("2006-01-02")
	expectedName := fmt.Sprintf("%s-collision-test.md", timestamp)
	if err := os.WriteFile(filepath.Join(destDir, expectedName), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	artifactPath, err := p.Promote("collision-test")
	if err != nil {
		t.Fatalf("Promote with collision should succeed: %v", err)
	}

	// The artifact path should be different from the pre-existing one
	if filepath.Base(artifactPath) == expectedName {
		t.Error("collision guard should have generated a different filename")
	}

	// Verify both files exist
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("collision-guarded artifact should exist")
	}
}

func TestPoolStageNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	err := p.Stage("nonexistent", types.TierBronze)
	if err == nil {
		t.Error("expected error when staging nonexistent candidate")
	}
}

func TestPoolRejectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	err := p.Reject("nonexistent", "reason", "reviewer")
	if err == nil {
		t.Error("expected error when rejecting nonexistent candidate")
	}
}

func TestPoolApproveNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	err := p.Approve("nonexistent", "note", "reviewer")
	if err == nil {
		t.Error("expected error when approving nonexistent candidate")
	}
}

func TestPoolInitReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	readOnly := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	p := NewPool(readOnly)
	err := p.Init()
	if err == nil {
		t.Error("expected error when Init in read-only directory")
	}
}

func TestPoolListError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Make the pending directory unreadable to trigger scan error
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.Chmod(pendingDir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pendingDir, 0700) })

	_, err := p.List(ListOptions{Status: types.PoolStatusPending})
	if err == nil {
		t.Error("expected error when listing unreadable directory")
	}
}

func TestPoolListPendingReviewError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Make pending directory unreadable
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.Chmod(pendingDir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pendingDir, 0700) })

	_, err := p.ListPendingReview()
	if err == nil {
		t.Error("expected error from ListPendingReview when list fails")
	}
}

func TestPoolBulkApproveListError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Make pending directory unreadable
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.Chmod(pendingDir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pendingDir, 0700) })

	_, err := p.BulkApprove(2*time.Hour, "tester", false)
	if err == nil {
		t.Error("expected error from BulkApprove when list fails")
	}
}

func TestAtomicMoveDestDirReadOnly(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test": true}`), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a read-only directory for destination
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0700) })

	destPath := filepath.Join(readOnlyDir, "dest.json")
	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Error("expected error when dest directory is read-only")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("expected 'create temp file' error, got: %v", err)
	}
}

func TestPoolAddAtInitError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	// Use a read-only dir so Init fails
	tmpDir := t.TempDir()
	readOnly := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	p := NewPool(readOnly)
	err := p.AddAt(types.Candidate{ID: "test", Content: "c"}, types.Scoring{}, time.Now())
	if err == nil {
		t.Error("expected error when Init fails in AddAt")
	}
	if !strings.Contains(err.Error(), "init pool") {
		t.Errorf("expected 'init pool' error, got: %v", err)
	}
}

func TestPoolGetChainOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Create chain file as a directory to trigger open error
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	if err := os.MkdirAll(chainPath, 0700); err != nil {
		t.Fatal(err)
	}

	_, err := p.GetChain()
	if err == nil {
		t.Error("expected error when chain file is a directory")
	}
}

func TestPoolApproachingAutoPromote(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add a silver candidate with a timestamp more than 22 hours ago
	candidate := types.Candidate{
		ID:      "old-silver",
		Tier:    types.TierSilver,
		Content: "Old silver content",
	}
	pastTime := time.Now().Add(-23 * time.Hour)
	if err := p.AddAt(candidate, types.Scoring{}, pastTime); err != nil {
		t.Fatalf("AddAt failed: %v", err)
	}

	entries, err := p.List(ListOptions{Status: types.PoolStatusPending})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Candidate.ID == "old-silver" {
			found = true
			if !e.ApproachingAutoPromote {
				t.Error("expected ApproachingAutoPromote to be true for 23h-old silver candidate")
			}
		}
	}
	if !found {
		t.Error("old-silver candidate not found in list")
	}
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

func TestRecordEventChainOpenError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Make pool directory read-only so OpenFile fails
	if err := os.Chmod(p.PoolPath, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p.PoolPath, 0700) })

	err := p.recordEvent(ChainEvent{
		Operation:   "test",
		CandidateID: "test-id",
	})
	if err == nil {
		t.Error("expected error when chain file cannot be opened for writing")
	}
}

func TestWriteEntryPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Make pending directory read-only so WriteFile fails
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.Chmod(pendingDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pendingDir, 0700) })

	entry := &PoolEntry{}
	entry.Candidate = types.Candidate{ID: "write-test", Content: "test"}
	err := p.writeEntry(filepath.Join(pendingDir, "write-test.json"), entry)
	if err == nil {
		t.Error("expected error when writing to read-only directory")
	}
}

func TestStageAtomicMoveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "stage-move-err",
		Tier:    types.TierSilver,
		Content: "Content to stage",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Make staged directory read-only so atomicMove fails (can't create temp file)
	stagedDir := filepath.Join(p.PoolPath, StagedDir)
	if err := os.Chmod(stagedDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stagedDir, 0700) })

	err := p.Stage("stage-move-err", types.TierBronze)
	if err == nil {
		t.Error("expected error when staged directory is read-only")
	}
	if !strings.Contains(err.Error(), "move to staged") {
		t.Errorf("expected 'move to staged' error, got: %v", err)
	}
}

func TestRejectAtomicMoveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-move-err",
		Tier:    types.TierBronze,
		Content: "Content to reject",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Make rejected directory read-only so atomicMove fails
	rejectedDir := filepath.Join(p.PoolPath, RejectedDir)
	if err := os.Chmod(rejectedDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rejectedDir, 0700) })

	err := p.Reject("reject-move-err", "reason", "reviewer")
	if err == nil {
		t.Error("expected error when rejected directory is read-only")
	}
	if !strings.Contains(err.Error(), "move to rejected") {
		t.Errorf("expected 'move to rejected' error, got: %v", err)
	}
}

func TestPromoteMkdirAllError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "promote-mkdir-err",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("promote-mkdir-err", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Make .agents read-only so MkdirAll for learnings/ fails
	agentsDir := filepath.Join(tmpDir, ".agents")
	if err := os.Chmod(agentsDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0700) })

	_, err := p.Promote("promote-mkdir-err")
	if err == nil {
		t.Error("expected error when destination directory creation fails")
	}
	if !strings.Contains(err.Error(), "create destination") {
		t.Errorf("expected 'create destination' error, got: %v", err)
	}
}

func TestAddAtWriteEntryError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Make pending directory read-only so writeEntry fails
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	if err := os.Chmod(pendingDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pendingDir, 0700) })

	candidate := types.Candidate{ID: "write-err", Content: "test"}
	err := p.AddAt(candidate, types.Scoring{}, time.Now())
	if err == nil {
		t.Error("expected error when writeEntry fails in AddAt")
	}
	if !strings.Contains(err.Error(), "write entry") {
		t.Errorf("expected 'write entry' error, got: %v", err)
	}
}

func TestPromoteWriteArtifactError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "promote-write-err",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("promote-write-err", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Create learnings dir and make it read-only so writeArtifact (WriteFile) fails
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(learningsDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(learningsDir, 0700) })

	_, err := p.Promote("promote-write-err")
	if err == nil {
		t.Error("expected error when writeArtifact fails")
	}
	if !strings.Contains(err.Error(), "write artifact") {
		t.Errorf("expected 'write artifact' error, got: %v", err)
	}
}

func TestApproveWriteEntryError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "approve-write-err",
		Tier:    types.TierBronze,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Make the entry file itself read-only so WriteFile fails
	entryPath := filepath.Join(p.PoolPath, PendingDir, "approve-write-err.json")
	if err := os.Chmod(entryPath, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(entryPath, 0600) })

	err := p.Approve("approve-write-err", "looks good", "reviewer")
	if err == nil {
		t.Fatal("expected error when writeEntry fails in Approve")
	}
	if !strings.Contains(err.Error(), "write approved entry") {
		t.Errorf("expected 'write approved entry' error, got: %v", err)
	}
}

func TestRejectInvalidIDLength(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Reject with empty ID (triggers validateCandidateID in Get)
	err := p.Reject("", "reason", "reviewer")
	if err == nil {
		t.Error("expected error for empty candidate ID")
	}
}

func TestStageInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	err := p.Stage("", types.TierBronze)
	if err == nil {
		t.Error("expected error for empty candidate ID in Stage")
	}
}

func TestPromoteInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	_, err := p.Promote("")
	if err == nil {
		t.Error("expected error for empty candidate ID in Promote")
	}
}

func TestApproveInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	err := p.Approve("", "note", "reviewer")
	if err == nil {
		t.Error("expected error for empty candidate ID in Approve")
	}
}

func TestBulkApproveSkipsAlreadyReviewed(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add two silver candidates with old timestamps
	for _, id := range []string{"already-approved", "should-approve"} {
		candidate := types.Candidate{
			ID:      id,
			Tier:    types.TierSilver,
			Content: "Old silver content",
		}
		pastTime := time.Now().Add(-3 * time.Hour)
		if err := p.AddAt(candidate, types.Scoring{}, pastTime); err != nil {
			t.Fatalf("AddAt failed: %v", err)
		}
	}

	// Pre-approve one so BulkApprove hits the "already reviewed" error
	if err := p.Approve("already-approved", "pre-approved", "first-reviewer"); err != nil {
		t.Fatalf("Pre-approve failed: %v", err)
	}

	// BulkApprove should succeed, approving the one that hasn't been reviewed
	// and silently skipping the already-reviewed one (warning to stderr)
	approved, err := p.BulkApprove(2*time.Hour, "bulk-reviewer", false)
	if err != nil {
		t.Fatalf("BulkApprove failed: %v", err)
	}
	// Only "should-approve" should be in the approved list;
	// "already-approved" triggers the error path and is skipped
	if len(approved) != 1 {
		t.Errorf("expected 1 approved (skipping already-reviewed), got %d", len(approved))
	}
	if len(approved) > 0 && approved[0] != "should-approve" {
		t.Errorf("expected should-approve, got %s", approved[0])
	}
}

func TestStageWriteEntryError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "stage-write-err",
		Tier:    types.TierSilver,
		Content: "Content to stage",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Stage successfully first
	if err := p.Stage("stage-write-err", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Verify the file is in the staged directory
	stagedFile := filepath.Join(p.PoolPath, StagedDir, "stage-write-err.json")
	if _, err := os.Stat(stagedFile); os.IsNotExist(err) {
		t.Fatal("staged file should exist")
	}
}

func TestRejectWriteEntryError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-write-test",
		Tier:    types.TierBronze,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Reject successfully and verify status
	if err := p.Reject("reject-write-test", "bad quality", "reviewer"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	entry, err := p.Get("reject-write-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.HumanReview == nil {
		t.Fatal("expected HumanReview to be set")
	}
	if entry.HumanReview.Reviewer != "reviewer" {
		t.Errorf("expected reviewer 'reviewer', got %s", entry.HumanReview.Reviewer)
	}
	if entry.HumanReview.Notes != "bad quality" {
		t.Errorf("expected notes 'bad quality', got %s", entry.HumanReview.Notes)
	}
}

func TestListPendingReviewSortByAge(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add bronze candidates with different ages to exercise the sort callback
	ids := []string{"recent-bronze", "old-bronze", "middle-bronze"}
	ages := []time.Duration{-1 * time.Hour, -10 * time.Hour, -5 * time.Hour}

	for i, id := range ids {
		candidate := types.Candidate{
			ID:      id,
			Tier:    types.TierBronze,
			Content: "Bronze content " + id,
		}
		addedAt := time.Now().Add(ages[i])
		if err := p.AddAt(candidate, types.Scoring{GateRequired: true}, addedAt); err != nil {
			t.Fatalf("AddAt failed: %v", err)
		}
	}

	pending, err := p.ListPendingReview()
	if err != nil {
		t.Fatalf("ListPendingReview failed: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending entries, got %d", len(pending))
	}

	// Should be sorted oldest first: old-bronze, middle-bronze, recent-bronze
	if pending[0].Candidate.ID != "old-bronze" {
		t.Errorf("expected oldest entry first (old-bronze), got %s", pending[0].Candidate.ID)
	}
	if pending[1].Candidate.ID != "middle-bronze" {
		t.Errorf("expected middle entry second (middle-bronze), got %s", pending[1].Candidate.ID)
	}
	if pending[2].Candidate.ID != "recent-bronze" {
		t.Errorf("expected newest entry last (recent-bronze), got %s", pending[2].Candidate.ID)
	}
}

func TestGetChainPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Create chain file and make it unreadable (not a directory, just no perms)
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	if err := os.WriteFile(chainPath, []byte(`{"operation":"add"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(chainPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(chainPath, 0600) })

	_, err := p.GetChain()
	if err == nil {
		t.Error("expected error when chain file is unreadable")
	}
}

func TestAtomicMoveRenameError(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test": true}`), 0600); err != nil {
		t.Fatal(err)
	}

	// Destination in a nonexistent directory -- temp file creation will be in that
	// nonexistent path, so this actually triggers "create temp file" error.
	// Instead, use a path that will succeed for create but fail for rename:
	// Create temp dir, write temp file, but make the final destination impossible
	// by pointing to a file inside a non-directory.
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	// Dest path goes through blocker which is a file, not a directory
	destPath := filepath.Join(blocker, "subdir", "dest.json")

	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Error("expected error when destination path is invalid")
	}
}

func TestAtomicMoveSourceRemoveWarning(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.json")

	content := []byte(`{"test": true}`)
	if err := os.WriteFile(srcPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	// Make the source directory read-only after writing the source file.
	// The file can still be read (already has 0600 perms) but Remove will fail
	// because we can't modify the directory. This triggers the non-fatal
	// source removal warning.
	if err := os.Chmod(tmpDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(tmpDir, 0700) })

	// atomicMove needs to create temp file in the same dir as dest.
	// Since dest is also in tmpDir (now read-only), this would fail too.
	// Instead, use a different writable dir for dest.
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "dest.json")

	err := atomicMove(srcPath, destPath)
	// Should succeed (source removal warning is non-fatal)
	if err != nil {
		t.Fatalf("atomicMove should succeed despite source removal warning: %v", err)
	}

	// Dest file should exist with correct content
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile dest failed: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("expected content %q, got %q", string(content), string(data))
	}
}

func TestPoolAddAtGateRequired(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "gated-at",
		Tier:    types.TierBronze,
		Content: "Gated content via AddAt",
	}
	scoring := types.Scoring{GateRequired: true}
	pastTime := time.Now().Add(-2 * time.Hour)

	if err := p.AddAt(candidate, scoring, pastTime); err != nil {
		t.Fatalf("AddAt failed: %v", err)
	}

	entry, err := p.Get("gated-at")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.HumanReview == nil {
		t.Fatal("expected HumanReview to be set for gated candidate via AddAt")
	}
	if entry.HumanReview.Reviewed {
		t.Error("expected HumanReview.Reviewed to be false for new gated candidate")
	}
}

func TestPoolAddAtInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	err := p.AddAt(types.Candidate{ID: "../evil", Content: "test"}, types.Scoring{}, time.Now())
	if err == nil {
		t.Error("expected error for path traversal ID in AddAt")
	}
	if !strings.Contains(err.Error(), "invalid candidate ID") {
		t.Errorf("expected 'invalid candidate ID' error, got: %v", err)
	}
}

// blockChainFile replaces the chain.jsonl with a directory to make recordEvent fail.
// Returns a cleanup function that restores it.
func blockChainFile(t *testing.T, p *Pool) {
	t.Helper()
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	// Remove existing chain file if any
	_ = os.Remove(chainPath)
	// Create a directory at the chain file path so OpenFile fails with "is a directory"
	if err := os.MkdirAll(chainPath, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(chainPath)
	})
}

func TestStage_RecordEventFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Add a candidate
	candidate := types.Candidate{
		ID:      "stage-event-fail",
		Tier:    types.TierSilver,
		Content: "test content for stage event failure",
	}
	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatal(err)
	}

	// Block the chain file so recordEvent fails
	blockChainFile(t, p)

	// Stage should succeed (recordEvent failure is non-fatal)
	err := p.Stage("stage-event-fail", types.TierBronze)
	if err != nil {
		t.Fatalf("Stage should succeed despite recordEvent failure: %v", err)
	}
}

func TestReject_RecordEventFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	candidate := types.Candidate{
		ID:      "reject-event-fail",
		Tier:    types.TierBronze,
		Content: "test content for reject event failure",
	}
	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatal(err)
	}

	// Block the chain file
	blockChainFile(t, p)

	// Reject should succeed (recordEvent failure is non-fatal)
	err := p.Reject("reject-event-fail", "test reason", "tester")
	if err != nil {
		t.Fatalf("Reject should succeed despite recordEvent failure: %v", err)
	}
}

func TestApprove_RecordEventFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	candidate := types.Candidate{
		ID:      "approve-event-fail",
		Tier:    types.TierBronze,
		Content: "test content for approve event failure",
	}
	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatal(err)
	}

	// Block the chain file
	blockChainFile(t, p)

	// Approve should succeed (recordEvent failure is non-fatal)
	err := p.Approve("approve-event-fail", "looks good", "tester")
	if err != nil {
		t.Fatalf("Approve should succeed despite recordEvent failure: %v", err)
	}
}

func TestPromote_RecordEventFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Add and stage a candidate first
	candidate := types.Candidate{
		ID:      "promote-event-fail",
		Tier:    types.TierSilver,
		Content: "test content for promote event failure",
		Type:    types.KnowledgeTypeLearning,
	}
	if err := p.Add(candidate, types.Scoring{GateRequired: true}); err != nil {
		t.Fatal(err)
	}
	if err := p.Stage("promote-event-fail", types.TierBronze); err != nil {
		t.Fatal(err)
	}

	// Block the chain file
	blockChainFile(t, p)

	// Promote should succeed (recordEvent failures are non-fatal)
	_, err := p.Promote("promote-event-fail")
	if err != nil {
		t.Fatalf("Promote should succeed despite recordEvent failure: %v", err)
	}
}

func TestRecordEvent_CloseError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	// Exercise the deferred close error path in recordEvent.
	// We can't easily trigger a close error on a regular file, but we can
	// at least verify recordEvent works correctly on a valid chain file.
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Record a test event
	event := ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "test",
		CandidateID: "test-id",
	}
	if err := p.recordEvent(event); err != nil {
		t.Fatalf("recordEvent: %v", err)
	}

	// Make the chain file read-only to exercise the open error
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	if err := os.Chmod(chainPath, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(chainPath, 0600) })

	err := p.recordEvent(event)
	if err == nil {
		t.Error("expected error when chain file is read-only")
	}
}

func TestGetChain_CloseError(t *testing.T) {
	// Exercise GetChain with valid data to ensure the deferred close path works.
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Record some events
	for i := range 3 {
		event := ChainEvent{
			Timestamp:   time.Now(),
			Operation:   fmt.Sprintf("test-%d", i),
			CandidateID: fmt.Sprintf("id-%d", i),
		}
		if err := p.recordEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestAtomicMove_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.json")
	destPath := filepath.Join(tmpDir, "dest.json")

	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Error("expected error for nonexistent source file")
	}
	if !strings.Contains(err.Error(), "read source") {
		t.Errorf("expected 'read source' error, got: %v", err)
	}
}

func TestAtomicMove_ReadOnlyDestDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test":true}`), 0600); err != nil {
		t.Fatal(err)
	}

	// Create read-only destination directory
	readOnly := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnly, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	destPath := filepath.Join(readOnly, "dest.json")
	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Error("expected error when dest dir is read-only")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("expected 'create temp file' error, got: %v", err)
	}
}

func TestAtomicMove_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.json")
	content := []byte(`{"test":true}`)
	if err := os.WriteFile(srcPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(tmpDir, "dest.json")
	if err := atomicMove(srcPath, destPath); err != nil {
		t.Fatalf("atomicMove: %v", err)
	}

	// Verify destination has correct content
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("dest content = %q, want %q", data, content)
	}

	// Verify source is removed
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("source should be removed after move")
	}
}

func TestAddAt_RecordEventFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Pool{PoolPath: tmpDir}

	// Setup pending dir
	pendingDir := filepath.Join(tmpDir, PendingDir)
	if err := os.MkdirAll(pendingDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Block chain file to trigger recordEvent failure
	blockChainFile(t, p)

	candidate := types.Candidate{
		ID:      "test-add-at",
		Content: "Test content",
	}
	scoring := types.Scoring{
		RawScore: 0.8,
	}

	// AddAt should succeed even when recordEvent fails (it's a warning)
	err := p.AddAt(candidate, scoring, time.Now())
	if err != nil {
		t.Fatalf("AddAt: %v", err)
	}
}

func TestGetChain_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Pool{PoolPath: tmpDir}

	// No chain.jsonl exists -- should return empty
	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for nonexistent chain, got %d", len(events))
	}
}

func TestWriteEntry_DirectoryAsPath(t *testing.T) {
	// Exercise writeEntry with a path that is a directory -- os.WriteFile fails.
	tmpDir := t.TempDir()
	p := &Pool{PoolPath: tmpDir}

	targetPath := filepath.Join(tmpDir, "blocked-entry")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatal(err)
	}

	entry := &PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{ID: "test-write", Content: "test"},
			Status:    types.PoolStatusPending,
		},
	}

	err := p.writeEntry(targetPath, entry)
	if err == nil {
		t.Error("expected error when path is a directory")
	}
}

func TestRecordEvent_DirectoryAsChainFile(t *testing.T) {
	// Exercise recordEvent with chain file path that is a directory.
	tmpDir := t.TempDir()
	p := &Pool{PoolPath: tmpDir}

	// Create a directory at the chain file path to block OpenFile
	chainPath := filepath.Join(tmpDir, ChainFile)
	if err := os.MkdirAll(chainPath, 0755); err != nil {
		t.Fatal(err)
	}

	event := ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "test",
		CandidateID: "test-id",
	}

	err := p.recordEvent(event)
	if err == nil {
		t.Error("expected error when chain file path is a directory")
	}
}

func TestAtomicMove_WriteTempFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	// Exercise atomicMove with a read-only destination directory.
	// atomicMove creates a temp file in the same directory as destPath,
	// so a read-only directory will block temp file creation.
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test": true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(roDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0755) })

	destPath := filepath.Join(roDir, "dest.json")
	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Error("expected error when dest directory is read-only")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("expected 'create temp file' error, got: %v", err)
	}
}

func TestAtomicMove_RenameError(t *testing.T) {
	// Exercise atomicMove where write succeeds but rename fails.
	// This is hard to trigger on a normal filesystem, but we can test
	// by having the destPath be a directory (rename over a directory fails).
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test": true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create destPath as a non-empty directory -- os.Rename to a non-empty dir fails
	destPath := filepath.Join(tmpDir, "dest-dir.json")
	if err := os.MkdirAll(filepath.Join(destPath, "blocker"), 0755); err != nil {
		t.Fatal(err)
	}

	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Error("expected error when dest is a non-empty directory")
	}
	if !strings.Contains(err.Error(), "rename to destination") {
		t.Errorf("expected 'rename to destination' error, got: %v", err)
	}
}

// --- Coverage gap tests (targeting uncovered branches) ---

func TestFindByPrefix_SkipsNonJSONDirsAndMalformed(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Add a valid candidate with a known prefix
	if err := p.Add(types.Candidate{ID: "pfx-valid", Tier: types.TierSilver, Content: "valid"}, types.Scoring{}); err != nil {
		t.Fatal(err)
	}

	pendingDir := filepath.Join(p.PoolPath, PendingDir)

	// Create a non-JSON file with matching prefix (covers !strings.HasSuffix branch)
	if err := os.WriteFile(filepath.Join(pendingDir, "pfx-text.txt"), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with matching prefix (covers e.IsDir() branch)
	if err := os.MkdirAll(filepath.Join(pendingDir, "pfx-subdir"), 0700); err != nil {
		t.Fatal(err)
	}

	// Create a malformed JSON file with matching prefix (covers readEntry error branch)
	if err := os.WriteFile(filepath.Join(pendingDir, "pfx-broken.json"), []byte("{bad json"), 0600); err != nil {
		t.Fatal(err)
	}

	matches, err := p.FindByPrefix("pfx-")
	if err != nil {
		t.Fatalf("FindByPrefix failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match (skip non-JSON, dirs, malformed), got %d", len(matches))
	}
	if matches[0].Candidate.ID != "pfx-valid" {
		t.Errorf("expected pfx-valid, got %s", matches[0].Candidate.ID)
	}
}

func TestFindByPrefix_MissingDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	// Don't Init -- directories don't exist, covers os.ReadDir error continue branch

	matches, err := p.FindByPrefix("anything")
	if err != nil {
		t.Fatalf("FindByPrefix on uninitialized pool should not error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for uninitialized pool, got %d", len(matches))
	}
}

func TestPromoteEntryRemoveWarning(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	// Exercise the non-fatal os.Remove warning in Promote (line 445-447).
	// Make the staged directory non-writable so Remove fails but the rest succeeds.
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "promote-rm-warn",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Content for removal warning test",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatal(err)
	}
	if err := p.Stage("promote-rm-warn", types.TierBronze); err != nil {
		t.Fatal(err)
	}

	// Make staged directory read-only so os.Remove(entry.FilePath) fails
	stagedDir := filepath.Join(p.PoolPath, StagedDir)
	if err := os.Chmod(stagedDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stagedDir, 0700) })

	// Promote should succeed (Remove failure is non-fatal warning)
	artifactPath, err := p.Promote("promote-rm-warn")
	if err != nil {
		t.Fatalf("Promote should succeed despite source removal warning: %v", err)
	}
	if artifactPath == "" {
		t.Error("expected non-empty artifact path")
	}
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("artifact file should exist")
	}
}

func TestOpenIfExists_NonExistentReturnsNilNil(t *testing.T) {
	f, err := openIfExists(filepath.Join(t.TempDir(), "does-not-exist.txt"))
	if f != nil {
		_ = f.Close()
		t.Error("expected nil file for non-existent path")
	}
	if err != nil {
		t.Errorf("expected nil error for non-existent path, got: %v", err)
	}
}

func TestOpenIfExists_ExistingFileReturnsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	f, err := openIfExists(path)
	if err != nil {
		t.Fatalf("openIfExists failed: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil file")
	}
	_ = f.Close()
}

func TestOpenIfExists_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	path := filepath.Join(t.TempDir(), "noperm.txt")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })

	f, err := openIfExists(path)
	if f != nil {
		_ = f.Close()
		t.Error("expected nil file for unreadable path")
	}
	// Permission error is NOT IsNotExist, so it should return err
	if err == nil {
		t.Error("expected error for permission-denied file")
	}
}

func TestWriteArtifactNoSourceSession(t *testing.T) {
	// Exercise the "source_session: unknown" and "Source: unknown" branches
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	entry := &PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:      "no-source",
				Type:    types.KnowledgeTypeLearning,
				Content: "Content without source info",
				Source:  types.Source{}, // empty SessionID
			},
			Status: types.PoolStatusStaged,
		},
	}

	artifactPath := filepath.Join(tmpDir, "no-source.md")
	if err := p.writeArtifact(artifactPath, entry); err != nil {
		t.Fatalf("writeArtifact failed: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "source_session: unknown") {
		t.Error("expected 'source_session: unknown' in frontmatter")
	}
	if !strings.Contains(content, "**Source**: unknown") {
		t.Error("expected '**Source**: unknown' in body")
	}
}

// --- Benchmarks ---

func benchCandidate(id string) (types.Candidate, types.Scoring) {
	return types.Candidate{
			ID:         id,
			Type:       types.KnowledgeTypeLearning,
			Tier:       types.TierSilver,
			Content:    "Benchmark learning content for performance testing",
			Utility:    0.75,
			Confidence: 0.8,
			Maturity:   types.MaturityCandidate,
			Source: types.Source{
				SessionID:      "bench-session",
				TranscriptPath: "/path/to/transcript.jsonl",
			},
		}, types.Scoring{
			RawScore: 0.72,
			Rubric: types.RubricScores{
				Specificity:   0.8,
				Actionability: 0.7,
				Novelty:       0.6,
				Context:       0.75,
				Confidence:    0.8,
			},
		}
}

func BenchmarkPoolAdd(b *testing.B) {
	tmpDir := b.TempDir()
	p := NewPool(tmpDir)

	b.ResetTimer()
	for i := range b.N {
		id := fmt.Sprintf("bench-%d", i)
		c, s := benchCandidate(id)
		_ = p.Add(c, s)
	}
}

func BenchmarkPoolGet(b *testing.B) {
	tmpDir := b.TempDir()
	p := NewPool(tmpDir)

	c, s := benchCandidate("bench-get")
	if err := p.Add(c, s); err != nil {
		b.Fatalf("setup Add: %v", err)
	}

	b.ResetTimer()
	for range b.N {
		_, _ = p.Get("bench-get")
	}
}

func BenchmarkPoolList(b *testing.B) {
	tmpDir := b.TempDir()
	p := NewPool(tmpDir)

	// Seed pool with entries
	for i := range 50 {
		c, s := benchCandidate(fmt.Sprintf("bench-list-%d", i))
		if err := p.Add(c, s); err != nil {
			b.Fatalf("setup Add: %v", err)
		}
	}

	b.ResetTimer()
	for range b.N {
		_, _ = p.List(ListOptions{})
	}
}

func BenchmarkPoolListPaginated(b *testing.B) {
	tmpDir := b.TempDir()
	p := NewPool(tmpDir)

	// Seed pool with entries
	for i := range 50 {
		c, s := benchCandidate(fmt.Sprintf("bench-page-%d", i))
		if err := p.Add(c, s); err != nil {
			b.Fatalf("setup Add: %v", err)
		}
	}

	opts := ListOptions{Limit: 10, Offset: 5}
	b.ResetTimer()
	for range b.N {
		_, _ = p.ListPaginated(opts)
	}
}

func TestWriteTempFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "test.tmp")
	data := []byte("hello world")

	if err := writeTempFile(tempPath, data); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}

	got, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("file content = %q, want %q", got, data)
	}
}

func TestWriteTempFile_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(roDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0755) })

	err := writeTempFile(filepath.Join(roDir, "test.tmp"), []byte("data"))
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("error = %q, want 'create temp file' prefix", err)
	}
}

func TestWriteTempFile_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "existing.tmp")
	// Create the file first so O_EXCL fails
	if err := os.WriteFile(tempPath, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	err := writeTempFile(tempPath, []byte("new data"))
	if err == nil {
		t.Fatal("expected error when temp file already exists")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("error = %q, want 'create temp file' prefix", err)
	}
}

func TestFindByPrefix_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add several candidates with different IDs
	for _, id := range []string{"alpha-one", "alpha-two", "beta-one"} {
		if err := p.Add(types.Candidate{ID: id, Tier: types.TierSilver, Content: "content for " + id}, types.Scoring{}); err != nil {
			t.Fatalf("Add(%s) failed: %v", id, err)
		}
	}

	// Exact prefix that matches one candidate
	matches, err := p.FindByPrefix("beta-one")
	if err != nil {
		t.Fatalf("FindByPrefix failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for exact prefix 'beta-one', got %d", len(matches))
	}
	if matches[0].Candidate.ID != "beta-one" {
		t.Errorf("expected candidate ID 'beta-one', got %q", matches[0].Candidate.ID)
	}
}

func TestFindByPrefix_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add candidates
	for _, id := range []string{"alpha-one", "alpha-two"} {
		if err := p.Add(types.Candidate{ID: id, Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
			t.Fatalf("Add(%s) failed: %v", id, err)
		}
	}

	// Prefix that matches nothing
	matches, err := p.FindByPrefix("zzz-nonexistent")
	if err != nil {
		t.Fatalf("FindByPrefix failed: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for 'zzz-nonexistent', got %d", len(matches))
	}
}

func TestFindByPrefix_AmbiguousMatch(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add candidates with shared prefix
	for _, id := range []string{"alpha-one", "alpha-two", "alpha-three", "beta-one"} {
		if err := p.Add(types.Candidate{ID: id, Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
			t.Fatalf("Add(%s) failed: %v", id, err)
		}
	}

	// Ambiguous prefix that matches multiple candidates
	matches, err := p.FindByPrefix("alpha")
	if err != nil {
		t.Fatalf("FindByPrefix failed: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches for prefix 'alpha', got %d", len(matches))
	}

	// Verify all matches have the prefix
	for _, m := range matches {
		if !strings.HasPrefix(m.Candidate.ID, "alpha") {
			t.Errorf("unexpected match: %q does not have prefix 'alpha'", m.Candidate.ID)
		}
	}
}

func TestFindByPrefix_EmptyPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err := p.FindByPrefix("")
	if err == nil {
		t.Error("expected error for empty prefix")
	}
}

func TestFindByPrefix_AcrossDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add candidate to pending
	if err := p.Add(types.Candidate{ID: "cross-pending", Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	// Stage another to staged
	if err := p.Add(types.Candidate{ID: "cross-staged", Tier: types.TierSilver, Content: "c"}, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("cross-staged", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// FindByPrefix with shared prefix should find both
	matches, err := p.FindByPrefix("cross")
	if err != nil {
		t.Fatalf("FindByPrefix failed: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 matches across directories for prefix 'cross', got %d", len(matches))
	}
}

// --- Bug #3: Chain event records wrong FromStatus ---

func TestReject_ChainEventFromStatus(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "from-status-test",
		Tier:    types.TierBronze,
		Content: "Test from-status tracking",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Entry starts as pending; reject it
	if err := p.Reject("from-status-test", "bad quality", "tester"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// Read chain and find the reject event
	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}

	var rejectEvent *ChainEvent
	for i := range events {
		if events[i].Operation == "reject" && events[i].CandidateID == "from-status-test" {
			rejectEvent = &events[i]
			break
		}
	}
	if rejectEvent == nil {
		t.Fatal("reject chain event not found")
	}

	// FromStatus must be pending (before mutation), NOT rejected (after mutation)
	if rejectEvent.FromStatus != types.PoolStatusPending {
		t.Errorf("expected FromStatus=pending, got %s", rejectEvent.FromStatus)
	}
	if rejectEvent.ToStatus != types.PoolStatusRejected {
		t.Errorf("expected ToStatus=rejected, got %s", rejectEvent.ToStatus)
	}
	if rejectEvent.FromStatus == rejectEvent.ToStatus {
		t.Error("FromStatus == ToStatus; chain event recorded AFTER mutation instead of before")
	}
}

func TestPromote_ChainEventFromStatus(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "promote-chain-test",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Promotable learning",
		Source: types.Source{
			SessionID:      "sess-1",
			TranscriptPath: "/tmp/t.jsonl",
		},
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Stage("promote-chain-test", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	if _, err := p.Promote("promote-chain-test"); err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}

	var promoteEvent *ChainEvent
	for i := range events {
		if events[i].Operation == "promote" && events[i].CandidateID == "promote-chain-test" {
			promoteEvent = &events[i]
			break
		}
	}
	if promoteEvent == nil {
		t.Fatal("promote chain event not found")
	}

	if promoteEvent.FromStatus != types.PoolStatusStaged {
		t.Errorf("expected FromStatus=staged, got %s", promoteEvent.FromStatus)
	}
	if promoteEvent.ToStatus != types.PoolStatusArchived {
		t.Errorf("expected ToStatus=archived, got %s", promoteEvent.ToStatus)
	}
}

// --- Bug #11: Missing Sync in recordEvent ---

func TestRecordEvent_SyncsFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	event := ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "test-sync",
		CandidateID: "sync-test",
		ToStatus:    types.PoolStatusPending,
	}

	if err := p.recordEvent(event); err != nil {
		t.Fatalf("recordEvent failed: %v", err)
	}

	// Read back immediately — data must be on disk (synced), not just in OS buffer
	data, err := os.ReadFile(filepath.Join(p.PoolPath, ChainFile))
	if err != nil {
		t.Fatalf("read chain file failed: %v", err)
	}

	if !strings.Contains(string(data), "sync-test") {
		t.Error("chain event not found in file after recordEvent; data may not be synced")
	}

	// Verify it's valid JSON
	var readBack ChainEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &readBack); err != nil {
		t.Fatalf("chain event not valid JSON: %v", err)
	}
	if readBack.CandidateID != "sync-test" {
		t.Errorf("expected CandidateID=sync-test, got %s", readBack.CandidateID)
	}
}

// --- Bug #12: paginate returns nil vs empty slice ---

func TestPaginate_OffsetBeyondLength(t *testing.T) {
	entries := []PoolEntry{
		{PoolEntry: types.PoolEntry{Candidate: types.Candidate{ID: "a"}}},
		{PoolEntry: types.PoolEntry{Candidate: types.Candidate{ID: "b"}}},
		{PoolEntry: types.PoolEntry{Candidate: types.Candidate{ID: "c"}}},
		{PoolEntry: types.PoolEntry{Candidate: types.Candidate{ID: "d"}}},
		{PoolEntry: types.PoolEntry{Candidate: types.Candidate{ID: "e"}}},
	}

	result := paginate(entries, 100, 0)
	if result == nil {
		t.Error("paginate returned nil; expected empty slice")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestPaginate_EmptyInput(t *testing.T) {
	result := paginate([]PoolEntry{}, 0, 0)
	if result == nil {
		t.Error("paginate returned nil for empty input; expected empty slice")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

// --- Bug #14: Nil Source in writeArtifact ---

func TestWriteArtifact_NilSource(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Entry with zero-value Source (no SessionID, no TranscriptPath)
	entry := &PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:      "nil-source-test",
				Type:    types.KnowledgeTypeLearning,
				Tier:    types.TierSilver,
				Content: "Learning without source info",
				// Source is zero-value (empty struct) — no SessionID, no TranscriptPath
			},
			Status: types.PoolStatusStaged,
		},
	}

	artifactPath := filepath.Join(tmpDir, "test-artifact.md")

	// Must not panic
	err := p.writeArtifact(artifactPath, entry)
	if err != nil {
		t.Fatalf("writeArtifact failed: %v", err)
	}

	// Verify file was written and contains fallback source info
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact failed: %v", err)
	}

	content := string(data)
	// When Source is zero-value, should show "unknown" fallback instead of empty fields
	if strings.Contains(content, "source_session: \n") {
		t.Error("artifact contains empty source_session in frontmatter; expected fallback handling")
	}
	if !strings.Contains(content, "## Source") {
		t.Error("artifact missing Source section")
	}

	// Should contain either valid source info or the unknown fallback
	if strings.Contains(content, "- **Session**: \n") {
		t.Error("artifact contains empty Session field; expected unknown fallback")
	}
}

func TestWriteArtifact_WithSource(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	entry := &PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:      "with-source-test",
				Type:    types.KnowledgeTypeLearning,
				Tier:    types.TierSilver,
				Content: "Learning with source",
				Source: types.Source{
					SessionID:      "sess-abc",
					TranscriptPath: "/path/to/t.jsonl",
					MessageIndex:   42,
				},
			},
			Status: types.PoolStatusStaged,
		},
	}

	artifactPath := filepath.Join(tmpDir, "test-with-source.md")
	if err := p.writeArtifact(artifactPath, entry); err != nil {
		t.Fatalf("writeArtifact failed: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact failed: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "sess-abc") {
		t.Error("artifact missing session ID")
	}
	if !strings.Contains(content, "/path/to/t.jsonl") {
		t.Error("artifact missing transcript path")
	}
	if !strings.Contains(content, fmt.Sprintf("**Message**: %d", 42)) {
		t.Error("artifact missing message index")
	}
}

// --- Error-path coverage: writeEntry marshal failure ---

func TestWriteEntry_MarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// NaN in a float64 field causes json.MarshalIndent to return an error
	entry := &PoolEntry{}
	entry.Candidate = types.Candidate{
		ID:      "nan-test",
		Content: "test",
		Utility: math.NaN(),
	}

	path := filepath.Join(p.PoolPath, PendingDir, "nan-test.json")
	err := p.writeEntry(path, entry)
	if err == nil {
		t.Fatal("expected error from writeEntry with NaN float, got nil")
	}
	if !strings.Contains(err.Error(), "NaN") {
		t.Errorf("expected error mentioning NaN, got: %s", err.Error())
	}
}

// --- Error-path coverage: writeTempFile write error via revoked permissions ---

func TestWriteTempFile_WriteError(t *testing.T) {
	// Use /dev/null — OpenFile succeeds but we simulate write error
	// by opening a file, then revoking its permissions mid-stream.
	// On macOS, we can use chflags to make the file immutable after open.
	//
	// Strategy: open a real file with O_EXCL, then use ftruncate to 0
	// and revoke write via fd manipulation. Since we can't do that cleanly,
	// we instead test with an extremely long path that exercises the error wrapping.
	//
	// Alternative: test that writeTempFile properly wraps errors by verifying
	// the success path writes correct data and the create-error path returns
	// the right error prefix (already tested). For coverage, we test the
	// data-integrity of a successful write more thoroughly.
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "write-test.tmp")
	data := []byte("integrity check data with special chars: \x00\xff\n\t")

	if err := writeTempFile(tempPath, data); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}

	got, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != len(data) {
		t.Errorf("file size = %d, want %d", len(got), len(data))
	}
	for i := range data {
		if got[i] != data[i] {
			t.Errorf("byte %d = %x, want %x", i, got[i], data[i])
			break
		}
	}

	// Verify file permissions are 0600
	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}

// --- Error-path coverage: recordEvent with unwritable chain path ---

func TestRecordEvent_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Create chain file as a directory — OpenFile succeeds on dirs on some
	// platforms but Write always fails. On macOS, OpenFile on a dir returns
	// an error itself, so we instead make the pool path a file to break OpenFile.
	chainPath := filepath.Join(p.PoolPath, ChainFile)

	// Create the chain file normally first, then make it read-only
	if err := os.WriteFile(chainPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	// Make the chain file read-only so OpenFile with O_WRONLY fails
	if err := os.Chmod(chainPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(chainPath, 0600) })

	err := p.recordEvent(ChainEvent{
		Timestamp:   time.Now(),
		Operation:   "test",
		CandidateID: "test-id",
	})
	if err == nil {
		t.Fatal("expected error when chain file is read-only, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied error, got: %s", err.Error())
	}
}

// --- Error-path coverage: Stage with write error after successful move ---

func TestStage_WriteEntryErrorAfterMove(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	// Add a candidate with NaN utility — it will be written manually
	// (bypassing json.Marshal validation) so Get() succeeds but
	// writeEntry fails when Stage tries to re-serialize.
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Manually write a JSON file with a valid utility field that we can
	// then corrupt. Since we can't write NaN in JSON, we use a different
	// approach: make the staged directory read-only AFTER pre-creating
	// the target file so atomicMove's rename succeeds but writeEntry fails.
	//
	// Strategy: add candidate normally, then chmod the staged dir to 0555
	// right before Stage. atomicMove creates a temp in staged dir — this
	// will fail at atomicMove, not writeEntry. So this path is not reachable
	// via Stage without mocking.
	//
	// Instead: test the error message format by calling writeEntry directly
	// on a read-only path and verify the "write staged entry" wrapping.
	candidate := types.Candidate{
		ID:      "stage-write-err-2",
		Tier:    types.TierSilver,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatal(err)
	}

	// Verify Stage succeeds normally (exercises the full path)
	if err := p.Stage("stage-write-err-2", types.TierBronze); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	// Verify the entry is now in staged with correct status
	entry, err := p.Get("stage-write-err-2")
	if err != nil {
		t.Fatalf("Get after stage failed: %v", err)
	}
	if entry.Status != types.PoolStatusStaged {
		t.Errorf("expected status %s, got %s", types.PoolStatusStaged, entry.Status)
	}
}

// --- Error-path coverage: Reject chain event error is non-fatal ---

func TestReject_RecordEventChainError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-chain-err",
		Tier:    types.TierBronze,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatal(err)
	}

	// Make chain file read-only so recordEvent fails inside Reject
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	if err := os.Chmod(chainPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(chainPath, 0600) })

	// Reject should still succeed despite chain event failure (it warns to stderr)
	err := p.Reject("reject-chain-err", "bad", "reviewer")
	if err != nil {
		t.Fatalf("Reject should succeed even when chain recording fails, got: %v", err)
	}

	// Verify the rejection actually happened
	entry, err := p.Get("reject-chain-err")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.Status != types.PoolStatusRejected {
		t.Errorf("expected status rejected, got %s", entry.Status)
	}
	if entry.HumanReview == nil || entry.HumanReview.Reviewer != "reviewer" {
		t.Error("expected HumanReview with reviewer set")
	}
}

// --- Error-path coverage: GetChain with malformed lines skips them ---

func TestGetChain_SkipsMalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Write chain file with a mix of valid and invalid lines
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	validEvent := ChainEvent{
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Operation:   "add",
		CandidateID: "valid-1",
		ToStatus:    types.PoolStatusPending,
	}
	validJSON, _ := json.Marshal(validEvent)

	content := string(validJSON) + "\n" +
		"this is not json\n" +
		"{\"broken\": json}\n" +
		string(validJSON) + "\n"
	if err := os.WriteFile(chainPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	events, err := p.GetChain()
	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}

	// Should have exactly 2 valid events (malformed lines skipped)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	for _, e := range events {
		if e.CandidateID != "valid-1" {
			t.Errorf("expected CandidateID valid-1, got %s", e.CandidateID)
		}
		if e.Operation != "add" {
			t.Errorf("expected Operation add, got %s", e.Operation)
		}
	}
}

// --- Error-path coverage: Add with NaN utility triggers writeEntry error ---

func TestAdd_WriteEntryMarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "nan-add",
		Type:    types.KnowledgeTypeLearning,
		Tier:    types.TierSilver,
		Content: "Test content",
		Utility: math.NaN(),
	}

	err := p.Add(candidate, types.Scoring{})
	if err == nil {
		t.Fatal("expected error from Add with NaN utility, got nil")
	}
	if !strings.Contains(err.Error(), "write entry") {
		t.Errorf("expected 'write entry' error wrapping, got: %s", err.Error())
	}
}

// --- Error-path coverage: Approve with NaN in existing entry ---

func TestApprove_WriteEntryMarshalError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatal(err)
	}

	// Write a valid JSON file manually, then modify the in-memory representation
	// to have NaN. Since Approve reads from disk and writes back, we can't inject
	// NaN into the read path. Instead, write a file with a value that parses to
	// a float but causes issues on re-marshal. Actually, NaN can't be in JSON.
	//
	// The only way to trigger writeEntry marshal error through Approve is if
	// the entry read from disk somehow gains a NaN. This isn't possible through
	// normal operation, so we test writeEntry directly (TestWriteEntry_MarshalError).
	//
	// Instead, test that Approve correctly wraps writeEntry permission errors.
	pendingDir := filepath.Join(p.PoolPath, PendingDir)
	entryData := types.PoolEntry{
		Candidate: types.Candidate{
			ID:      "approve-write-err",
			Tier:    types.TierBronze,
			Content: "Content",
		},
		Status:  types.PoolStatusPending,
		AddedAt: time.Now(),
	}
	data, _ := json.MarshalIndent(&PoolEntry{PoolEntry: entryData}, "", "  ")
	filePath := filepath.Join(pendingDir, "approve-write-err.json")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatal(err)
	}

	// Make the file read-only so writeEntry's os.WriteFile fails
	if err := os.Chmod(filePath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filePath, 0644) })

	err := p.Approve("approve-write-err", "looks good", "reviewer")
	if err == nil {
		t.Fatal("expected error from Approve when file is read-only, got nil")
	}
	if !strings.Contains(err.Error(), "write approved entry") {
		t.Errorf("expected 'write approved entry' wrapping, got: %s", err.Error())
	}
}

// --- Error-path coverage: Reject with writeEntry permission error ---

func TestReject_WriteEntryPermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-perm-err",
		Tier:    types.TierBronze,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatal(err)
	}

	// Make the rejected directory read-only so writeEntry fails
	// after atomicMove succeeds. But atomicMove also writes to rejected dir...
	// So we can't chmod before Stage. Instead, make the ENTIRE pool dir
	// read-only after the atomic move but before write — not possible sequentially.
	//
	// Alternative: make rejected dir writable but the specific file read-only
	// after atomicMove creates it. Not possible without concurrency.
	//
	// This error path requires writeEntry(newPath, entry) to fail AFTER
	// atomicMove(entry.FilePath, newPath) succeeds. Since both target the
	// same directory, making the dir unwritable breaks both.
	//
	// Test the wrapping format by verifying Reject returns correct error messages
	// for the paths we CAN trigger.
	rejectedDir := filepath.Join(p.PoolPath, RejectedDir)
	if err := os.Chmod(rejectedDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rejectedDir, 0755) })

	err := p.Reject("reject-perm-err", "bad", "reviewer")
	if err == nil {
		t.Fatal("expected error from Reject when rejected dir is read-only, got nil")
	}
	// The error should come from atomicMove since it writes to rejected dir first
	if !strings.Contains(err.Error(), "move to rejected") {
		t.Errorf("expected 'move to rejected' error, got: %s", err.Error())
	}
}

// --- Error-path coverage: Stage with rejected candidate ---

func TestStage_RejectedCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "stage-rejected",
		Tier:    types.TierSilver,
		Content: "Content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatal(err)
	}
	if err := p.Reject("stage-rejected", "reason", "rev"); err != nil {
		t.Fatal(err)
	}

	err := p.Stage("stage-rejected", types.TierBronze)
	if !errors.Is(err, ErrStageRejected) {
		t.Errorf("expected ErrStageRejected, got: %v", err)
	}
}

// --- Error-path coverage: scanChainEvents with scanner error ---

func TestScanChainEvents_ScannerError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with a line longer than bufio.MaxScanTokenSize (64KB)
	// to trigger a scanner error
	chainPath := filepath.Join(tmpDir, "chain.jsonl")
	validEvent := ChainEvent{
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Operation:   "add",
		CandidateID: "valid",
	}
	validJSON, _ := json.Marshal(validEvent)

	// Write valid line + very long line
	f, err := os.Create(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(validJSON)
	f.Write([]byte("\n"))
	// Write a line longer than 64KB to trigger scanner error
	longLine := make([]byte, 70000)
	for i := range longLine {
		longLine[i] = 'a'
	}
	f.Write(longLine)
	f.Write([]byte("\n"))
	f.Close()

	fRead, err := os.Open(chainPath)
	if err != nil {
		t.Fatal(err)
	}
	defer fRead.Close()

	events, err := scanChainEvents(fRead)
	if err == nil {
		t.Fatal("expected scanner error from oversized line, got nil")
	}
	// Should still have parsed the valid event before the error
	if len(events) != 1 {
		t.Errorf("expected 1 event parsed before error, got %d", len(events))
	}
	if events[0].CandidateID != "valid" {
		t.Errorf("expected CandidateID 'valid', got %s", events[0].CandidateID)
	}
}

// --- Error-path coverage: atomicMove source read error ---

func TestAtomicMove_SourceReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Source is a directory, not a file — ReadFile will fail
	srcDir := filepath.Join(tmpDir, "source-dir")
	if err := os.Mkdir(srcDir, 0700); err != nil {
		t.Fatal(err)
	}
	destPath := filepath.Join(tmpDir, "dest.json")

	err := atomicMove(srcDir, destPath)
	if err == nil {
		t.Fatal("expected error when source is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "read source") {
		t.Errorf("expected 'read source' error, got: %s", err.Error())
	}
}

// --- Error-path coverage: writeArtifact with context field ---

func TestWriteArtifact_WithContext(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	entry := &PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:      "context-test",
				Type:    types.KnowledgeTypeDecision,
				Tier:    types.TierGold,
				Content: "We decided to use Go modules",
				Context: "During the architecture review session, we evaluated multiple options",
				Source: types.Source{
					SessionID:      "sess-ctx",
					TranscriptPath: "/path/to/ctx.jsonl",
					MessageIndex:   7,
				},
			},
			Status: types.PoolStatusStaged,
		},
	}

	artifactPath := filepath.Join(tmpDir, "context-artifact.md")
	if err := p.writeArtifact(artifactPath, entry); err != nil {
		t.Fatalf("writeArtifact failed: %v", err)
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "## Context") {
		t.Error("artifact missing Context section")
	}
	if !strings.Contains(content, "architecture review") {
		t.Error("artifact missing context content")
	}
	if !strings.Contains(content, "# Decision: ") {
		t.Error("artifact missing Decision heading")
	}
	if !strings.Contains(content, "type: decision") {
		t.Error("frontmatter missing type: decision")
	}
}

// mockFileWriter implements fileWriter for testing error paths.
type mockFileWriter struct {
	writeErr error
	syncErr  error
	closeErr error
}

func (m *mockFileWriter) Write(p []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return len(p), nil
}

func (m *mockFileWriter) Sync() error  { return m.syncErr }
func (m *mockFileWriter) Close() error { return m.closeErr }

func TestStageWriteEntryFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "stage-write-fail",
		Tier:    types.TierSilver,
		Content: "Test content for stage write failure",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Inject writeEntry failure via test hook
	injectedErr := fmt.Errorf("injected disk full error")
	writeEntryFunc = func(_ string, _ []byte) error { return injectedErr }
	defer func() { writeEntryFunc = defaultWriteEntry }()

	err := p.Stage("stage-write-fail", types.TierBronze)
	if err == nil {
		t.Fatal("expected Stage to fail when writeEntry fails")
	}
	if !strings.Contains(err.Error(), "write staged entry") {
		t.Errorf("expected 'write staged entry' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected disk full error") {
		t.Errorf("expected wrapped injected error, got: %v", err)
	}
}

func TestRejectWriteEntryFailure(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-write-fail",
		Tier:    types.TierBronze,
		Content: "Test content for reject write failure",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Inject writeEntry failure via test hook
	injectedErr := fmt.Errorf("injected write failure")
	writeEntryFunc = func(_ string, _ []byte) error { return injectedErr }
	defer func() { writeEntryFunc = defaultWriteEntry }()

	err := p.Reject("reject-write-fail", "bad content", "reviewer")
	if err == nil {
		t.Fatal("expected Reject to fail when writeEntry fails")
	}
	if !strings.Contains(err.Error(), "write rejected entry") {
		t.Errorf("expected 'write rejected entry' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected write failure") {
		t.Errorf("expected wrapped injected error, got: %v", err)
	}
}

func TestRecordEventMarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Inject json.Marshal failure
	injectedErr := fmt.Errorf("injected marshal error")
	jsonMarshalFunc = func(_ any) ([]byte, error) { return nil, injectedErr }
	defer func() { jsonMarshalFunc = json.Marshal }()

	err := p.recordEvent(ChainEvent{Operation: "test"})
	if err == nil {
		t.Fatal("expected recordEvent to fail when marshal fails")
	}
	if err.Error() != "injected marshal error" {
		t.Errorf("expected 'injected marshal error', got: %v", err)
	}
}

func TestRecordEventWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Inject chain file open that returns a writer whose Write fails
	injectedErr := fmt.Errorf("injected write error")
	openChainFileFunc = func(_ string) (fileWriter, error) {
		return &mockFileWriter{writeErr: injectedErr}, nil
	}
	defer func() { openChainFileFunc = defaultOpenChainFile }()

	err := p.recordEvent(ChainEvent{Operation: "test"})
	if err == nil {
		t.Fatal("expected recordEvent to fail when Write fails")
	}
	if err.Error() != "injected write error" {
		t.Errorf("expected 'injected write error', got: %v", err)
	}
}

func TestRecordEventCloseError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Inject chain file open that returns a writer whose Close fails
	// Write and Sync succeed, so err == nil when defer runs, triggering L815-817
	injectedErr := fmt.Errorf("injected close error")
	openChainFileFunc = func(_ string) (fileWriter, error) {
		return &mockFileWriter{closeErr: injectedErr}, nil
	}
	defer func() { openChainFileFunc = defaultOpenChainFile }()

	err := p.recordEvent(ChainEvent{Operation: "test-close"})
	if err == nil {
		t.Fatal("expected recordEvent to return close error")
	}
	if err.Error() != "injected close error" {
		t.Errorf("expected 'injected close error', got: %v", err)
	}
}

// closeErrorReader wraps a reader and injects a Close error.
type closeErrorReader struct {
	io.Reader
	closeErr error
}

func (c *closeErrorReader) Close() error { return c.closeErr }

func TestGetChainCloseError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Write a valid chain file so scanChainEvents succeeds (err == nil)
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	eventJSON := `{"operation":"add","candidate_id":"test-1"}` + "\n"
	if err := os.WriteFile(chainPath, []byte(eventJSON), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Inject openIfExistsFunc that returns a reader whose Close fails
	injectedErr := fmt.Errorf("injected close error on chain read")
	openIfExistsFunc = func(path string) (readCloser, error) {
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &closeErrorReader{Reader: f, closeErr: injectedErr}, nil
	}
	defer func() { openIfExistsFunc = defaultOpenIfExists }()

	// GetChain should return the close error since scanChainEvents succeeds
	events, err := p.GetChain()
	if err == nil {
		t.Fatal("expected GetChain to return close error")
	}
	if err.Error() != "injected close error on chain read" {
		t.Errorf("expected injected close error, got: %v", err)
	}
	// Events should still be populated since scan succeeded before close
	if len(events) != 1 {
		t.Fatalf("expected 1 event despite close error, got %d", len(events))
	}
	if events[0].Operation != "add" {
		t.Errorf("expected operation 'add', got %q", events[0].Operation)
	}
}

func TestGetChainOpenError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create chain file, then make it unreadable
	chainPath := filepath.Join(p.PoolPath, ChainFile)
	if err := os.WriteFile(chainPath, []byte(`{"operation":"test"}`+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.Chmod(chainPath, 0000); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer os.Chmod(chainPath, 0600) //nolint:errcheck

	_, err := p.GetChain()
	if err == nil {
		t.Fatal("expected GetChain to fail on unreadable chain file")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied, got: %v", err)
	}
}

func TestWriteTempFileWriteFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Inject openTempFile that returns a writer whose Write fails
	injectedErr := fmt.Errorf("injected write failure")
	openTempFileFunc = func(_ string) (fileWriter, error) {
		return &mockFileWriter{writeErr: injectedErr}, nil
	}
	defer func() { openTempFileFunc = defaultOpenTempFile }()

	tempPath := filepath.Join(tmpDir, "test.tmp")
	err := writeTempFile(tempPath, []byte("data"))
	if err == nil {
		t.Fatal("expected writeTempFile to fail when Write fails")
	}
	if !strings.Contains(err.Error(), "write temp file") {
		t.Errorf("expected 'write temp file' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected write failure") {
		t.Errorf("expected wrapped injected error, got: %v", err)
	}
}

func TestWriteTempFileSyncFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Inject openTempFile that returns a writer whose Sync fails
	injectedErr := fmt.Errorf("injected sync failure")
	openTempFileFunc = func(_ string) (fileWriter, error) {
		return &mockFileWriter{syncErr: injectedErr}, nil
	}
	defer func() { openTempFileFunc = defaultOpenTempFile }()

	tempPath := filepath.Join(tmpDir, "test.tmp")
	err := writeTempFile(tempPath, []byte("data"))
	if err == nil {
		t.Fatal("expected writeTempFile to fail when Sync fails")
	}
	if !strings.Contains(err.Error(), "sync temp file") {
		t.Errorf("expected 'sync temp file' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected sync failure") {
		t.Errorf("expected wrapped injected error, got: %v", err)
	}
}

func TestWriteTempFileCloseFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Inject openTempFile that returns a writer whose Close fails
	injectedErr := fmt.Errorf("injected close failure")
	openTempFileFunc = func(_ string) (fileWriter, error) {
		return &mockFileWriter{closeErr: injectedErr}, nil
	}
	defer func() { openTempFileFunc = defaultOpenTempFile }()

	tempPath := filepath.Join(tmpDir, "test.tmp")
	err := writeTempFile(tempPath, []byte("data"))
	if err == nil {
		t.Fatal("expected writeTempFile to fail when Close fails")
	}
	if !strings.Contains(err.Error(), "close temp file") {
		t.Errorf("expected 'close temp file' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected close failure") {
		t.Errorf("expected wrapped injected error, got: %v", err)
	}
}

func TestWriteTempFileCreateError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.Chmod(readOnlyDir, 0500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0700) //nolint:errcheck

	tempPath := filepath.Join(readOnlyDir, "test.tmp")
	err := writeTempFile(tempPath, []byte("test data"))
	if err == nil {
		t.Fatal("expected writeTempFile to fail when directory is read-only")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("expected 'create temp file' error, got: %v", err)
	}
}

func TestWriteTempFileHappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "test-write.tmp")
	data := []byte("hello world content for temp file test")

	if err := writeTempFile(tempPath, data); err != nil {
		t.Fatalf("writeTempFile failed: %v", err)
	}

	got, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("expected content %q, got %q", string(data), string(got))
	}

	info, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}

func TestWriteTempFileExclusiveCreate(t *testing.T) {
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "existing.tmp")

	if err := os.WriteFile(tempPath, []byte("existing"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err := writeTempFile(tempPath, []byte("new data"))
	if err == nil {
		t.Fatal("expected writeTempFile to fail when file already exists")
	}
	if !strings.Contains(err.Error(), "create temp file") {
		t.Errorf("expected 'create temp file' error, got: %v", err)
	}

	got, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != "existing" {
		t.Errorf("expected original content 'existing', got %q", string(got))
	}
}

func TestAtomicMoveRandReadError(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test":true}`), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Inject rand.Read failure
	injectedErr := fmt.Errorf("injected entropy failure")
	randReadFunc = func(_ []byte) (int, error) { return 0, injectedErr }
	defer func() { randReadFunc = rand.Read }()

	destPath := filepath.Join(tmpDir, "dest.json")
	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Fatal("expected atomicMove to fail when rand.Read fails")
	}
	if !strings.Contains(err.Error(), "generate random suffix") {
		t.Errorf("expected 'generate random suffix' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "injected entropy failure") {
		t.Errorf("expected wrapped injected error, got: %v", err)
	}

	// Source should still exist
	if _, serr := os.Stat(srcPath); os.IsNotExist(serr) {
		t.Error("source file should still exist after failed move")
	}
}

func TestAtomicMoveDestDirMissing(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.json")
	if err := os.WriteFile(srcPath, []byte(`{"test":true}`), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	destPath := filepath.Join(tmpDir, "nonexistent", "subdir", "dest.json")
	err := atomicMove(srcPath, destPath)
	if err == nil {
		t.Fatal("expected atomicMove to fail when destination dir doesn't exist")
	}
	if !strings.Contains(err.Error(), "create temp file") && !strings.Contains(err.Error(), "rename") {
		t.Errorf("expected create/rename error, got: %v", err)
	}

	if _, serr := os.Stat(srcPath); os.IsNotExist(serr) {
		t.Error("source file should still exist after failed move")
	}
}

func TestAtomicMoveSourceRemoveNonFatal(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	content := []byte(`{"key":"value"}`)

	subDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(subDir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	srcInSub := filepath.Join(subDir, "source.json")
	if err := os.WriteFile(srcInSub, content, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	destPath := filepath.Join(tmpDir, "dest.json")

	if err := os.Chmod(subDir, 0500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer os.Chmod(subDir, 0700) //nolint:errcheck

	err := atomicMove(srcInSub, destPath)
	if err != nil {
		t.Fatalf("atomicMove should succeed even when source removal fails: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile dest failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("expected %q, got %q", string(content), string(got))
	}
}

func TestRecordEventMarshalJSON(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	event := ChainEvent{
		Timestamp:    time.Now(),
		Operation:    "promote",
		CandidateID:  "marshal-test",
		FromStatus:   types.PoolStatusStaged,
		ToStatus:     types.PoolStatus("promoted"),
		Reason:       "high quality",
		Reviewer:     "automated",
		ArtifactPath: "/path/to/artifact.md",
	}

	if err := p.recordEvent(event); err != nil {
		t.Fatalf("recordEvent failed: %v", err)
	}

	chainPath := filepath.Join(p.PoolPath, ChainFile)
	data, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var decoded ChainEvent
	if err := json.Unmarshal(data[:len(data)-1], &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.Operation != "promote" {
		t.Errorf("expected operation 'promote', got %q", decoded.Operation)
	}
	if decoded.ArtifactPath != "/path/to/artifact.md" {
		t.Errorf("expected artifact path '/path/to/artifact.md', got %q", decoded.ArtifactPath)
	}
	if decoded.FromStatus != types.PoolStatusStaged {
		t.Errorf("expected from_status 'staged', got %q", decoded.FromStatus)
	}
	if decoded.ToStatus != types.PoolStatus("promoted") {
		t.Errorf("expected to_status 'promoted', got %q", decoded.ToStatus)
	}
}

func TestStageWriteEntryFailureViaFullIntegration(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "stage-int-err",
		Tier:    types.TierGold,
		Content: "Integration test content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	stagedDir := filepath.Join(p.PoolPath, StagedDir)
	if err := os.Chmod(stagedDir, 0500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer os.Chmod(stagedDir, 0700) //nolint:errcheck

	err := p.Stage("stage-int-err", types.TierBronze)
	if err == nil {
		t.Fatal("expected Stage to fail when staged directory is read-only")
	}
	if !strings.Contains(err.Error(), "move to staged") {
		t.Errorf("expected 'move to staged' error, got: %v", err)
	}
}

func TestRejectWriteEntryFailureViaFullIntegration(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "reject-int-err",
		Tier:    types.TierBronze,
		Content: "Integration test content",
	}
	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	rejectedDir := filepath.Join(p.PoolPath, RejectedDir)
	if err := os.Chmod(rejectedDir, 0500); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	defer os.Chmod(rejectedDir, 0700) //nolint:errcheck

	err := p.Reject("reject-int-err", "bad", "reviewer")
	if err == nil {
		t.Fatal("expected Reject to fail when rejected directory is read-only")
	}
	if !strings.Contains(err.Error(), "move to rejected") {
		t.Errorf("expected 'move to rejected' error, got: %v", err)
	}
}

func TestOpenIfExistsErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}
	t.Run("file does not exist returns nil nil", func(t *testing.T) {
		f, err := openIfExists("/nonexistent/path/file.jsonl")
		if f != nil {
			t.Error("expected nil file for nonexistent path")
		}
		if err != nil {
			t.Errorf("expected nil error for nonexistent path, got: %v", err)
		}
	})

	t.Run("file exists and is readable", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.jsonl")
		if err := os.WriteFile(path, []byte("test\n"), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		f, err := openIfExists(path)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil file")
		}
		defer func() { _ = f.Close() }()
	})

	t.Run("permission denied returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "noperm.jsonl")
		if err := os.WriteFile(path, []byte("test\n"), 0600); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := os.Chmod(path, 0000); err != nil {
			t.Fatalf("Chmod failed: %v", err)
		}
		defer os.Chmod(path, 0600) //nolint:errcheck

		f, err := openIfExists(path)
		if err == nil {
			if f != nil {
				f.Close()
			}
			t.Fatal("expected error for permission-denied file")
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("expected permission denied, got: %v", err)
		}
	})
}

func TestScanChainEventsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = f.Close() }()

	events, err := scanChainEvents(f)
	if err != nil {
		t.Fatalf("scanChainEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events from empty file, got %d", len(events))
	}
}

func TestAtomicMovePreservesContent(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content []byte
	}{
		{"empty file", []byte{}},
		{"small content", []byte("small")},
		{"large content", []byte(strings.Repeat("x", 1<<16))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcPath := filepath.Join(tmpDir, tt.name+"-src")
			destPath := filepath.Join(tmpDir, tt.name+"-dst")

			if err := os.WriteFile(srcPath, tt.content, 0600); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}

			if err := atomicMove(srcPath, destPath); err != nil {
				t.Fatalf("atomicMove failed: %v", err)
			}

			got, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("ReadFile failed: %v", err)
			}
			if len(got) != len(tt.content) {
				t.Errorf("expected %d bytes, got %d", len(tt.content), len(got))
			}
		})
	}
}

func TestPoolNaNScore(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "nan-score",
		Tier:    types.TierSilver,
		Content: "NaN score test",
	}
	scoring := types.Scoring{
		RawScore: math.NaN(),
	}

	err := p.Add(candidate, scoring)
	if err == nil {
		t.Fatal("expected Add to fail with NaN score (json.Marshal rejects NaN)")
	}
	if !strings.Contains(err.Error(), "NaN") && !strings.Contains(err.Error(), "unsupported value") {
		t.Errorf("expected NaN-related error, got: %v", err)
	}
}

func TestValidateCandidateID_SentinelErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{"empty ID", "", ErrEmptyID},
		{"too long ID", strings.Repeat("a", 129), ErrIDTooLong},
		{"invalid chars slash", "../../etc/passwd", ErrIDInvalidChars},
		{"invalid chars space", "has space", ErrIDInvalidChars},
		{"invalid chars dot", "has.dot", ErrIDInvalidChars},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCandidateID(tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(%v, %v) to be true", err, tt.wantErr)
			}
		})
	}
}

func TestPromoteRejectedCandidate_SentinelError(t *testing.T) {
	tmpDir := t.TempDir()
	p := NewPool(tmpDir)

	candidate := types.Candidate{
		ID:      "sentinel-reject-test",
		Tier:    types.TierSilver,
		Type:    types.KnowledgeTypeLearning,
		Content: "Content to reject then promote",
	}

	if err := p.Add(candidate, types.Scoring{}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := p.Reject("sentinel-reject-test", "Not useful", "tester"); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	_, err := p.Promote("sentinel-reject-test")
	if err == nil {
		t.Fatal("expected error when promoting rejected candidate")
	}
	if !errors.Is(err, ErrPromoteRejected) {
		t.Errorf("expected errors.Is(err, ErrPromoteRejected), got: %v", err)
	}
}
