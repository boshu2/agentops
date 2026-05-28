// practices: [microservices, sre]
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestPoolListEmpty(t *testing.T) {
	tmp := t.TempDir()
	p := pool.NewPool(tmp)

	entries, err := p.List(pool.ListOptions{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries=%d, want 0", len(entries))
	}
}

func TestPoolStagePromoteWorkflow(t *testing.T) {
	tmp := t.TempDir()
	// Create learnings dir for promote target
	if err := os.MkdirAll(tmp+"/.agents/learnings", 0755); err != nil {
		t.Fatal(err)
	}

	p := pool.NewPool(tmp)

	cand := types.Candidate{
		ID:         "cand-test-001",
		Type:       "learning",
		Tier:       types.TierSilver,
		Content:    "Test learning content",
		Utility:    0.8,
		Confidence: 0.9,
		Maturity:   "established",
		Source: types.Source{
			SessionID:      "session-abc",
			TranscriptPath: "/tmp/t.md",
			MessageIndex:   5,
		},
	}

	if err := p.Add(cand, types.Scoring{RawScore: 0.85, TierAssignment: types.TierSilver}); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Verify candidate is pending
	entries, _ := p.List(pool.ListOptions{Status: types.PoolStatusPending})
	if len(entries) != 1 {
		t.Fatalf("pending=%d, want 1", len(entries))
	}

	// Stage
	if err := p.Stage(cand.ID, types.TierBronze); err != nil {
		t.Fatalf("stage: %v", err)
	}

	// Verify staged
	entries, _ = p.List(pool.ListOptions{Status: types.PoolStatusStaged})
	if len(entries) != 1 {
		t.Fatalf("staged=%d, want 1", len(entries))
	}

	// Promote
	artifactPath, err := p.Promote(cand.ID)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}

	if _, err := os.Stat(artifactPath); err != nil {
		t.Errorf("artifact not created: %v", err)
	}
}

func TestPoolRejectRequiresCandidate(t *testing.T) {
	tmp := t.TempDir()
	p := pool.NewPool(tmp)

	// Reject a nonexistent candidate
	err := p.Reject("nonexistent-id", "test reason", "tester")
	if err == nil {
		t.Error("expected error rejecting nonexistent candidate")
	}
}

func TestPoolBulkApproveThreshold(t *testing.T) {
	tmp := t.TempDir()
	p := pool.NewPool(tmp)

	cand := types.Candidate{
		ID:         "cand-bulk-001",
		Type:       "learning",
		Tier:       types.TierSilver,
		Content:    "Bulk test content",
		Utility:    0.7,
		Confidence: 0.8,
		Maturity:   "emerging",
	}

	// Add with a past timestamp so the candidate qualifies for the 1h threshold
	pastTime := time.Now().Add(-2 * time.Hour)
	if err := p.AddAt(cand, types.Scoring{RawScore: 0.75, TierAssignment: types.TierSilver}, pastTime); err != nil {
		t.Fatalf("add: %v", err)
	}

	// BulkApprove with 1h threshold — candidate was added 2h ago so it qualifies
	approved, err := p.BulkApprove(time.Hour, "tester", false)
	if err != nil {
		t.Fatalf("bulk approve: %v", err)
	}
	if len(approved) != 1 {
		t.Errorf("approved=%d, want 1", len(approved))
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		max  int
		want string
	}{
		{"short id unchanged", "abc-123", 10, "abc-123"},
		{"exact length unchanged", "abc-123", 7, "abc-123"},
		{"truncated with ellipsis", "abcdefghij", 7, "abcd..."},
		{"single char max", "hello", 4, "h..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateID(tt.id, tt.max)
			if got != tt.want {
				t.Errorf("truncateID(%q, %d) = %q, want %q", tt.id, tt.max, got, tt.want)
			}
		})
	}
}

func TestRepeat(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"repeat 0 times", "abc", 0, ""},
		{"repeat 1 time", "abc", 1, "abc"},
		{"repeat 3 times", "ab", 3, "ababab"},
		{"empty string repeated", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repeat(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("repeat(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pool.go — outputPoolList
// ---------------------------------------------------------------------------

func TestPool_outputPoolList_emptyTable(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputPoolList(nil, 0, 50, 0)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPoolList: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "No pool entries found") {
		t.Errorf("expected 'No pool entries found', got: %s", out)
	}
}

func TestPool_outputPoolList_jsonMode(t *testing.T) {
	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	entries := []pool.PoolEntry{
		{
			PoolEntry: types.PoolEntry{
				Candidate: types.Candidate{
					ID:         "cand-test-001",
					Tier:       types.TierSilver,
					Utility:    0.75,
					Confidence: 0.90,
				},
				Status: types.PoolStatusPending,
			},
			AgeString: "2h",
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputPoolList(entries, 0, 50, 1)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPoolList: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "cand-test-001") {
		t.Errorf("expected JSON output with candidate ID, got: %s", out)
	}
}

func TestPool_outputPoolList_paginationMessage(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	// Temporarily widen to avoid truncation issues
	oldPoolWide := poolWide
	poolWide = true
	defer func() { poolWide = oldPoolWide }()

	entries := []pool.PoolEntry{
		{
			PoolEntry: types.PoolEntry{
				Candidate: types.Candidate{
					ID:         "cand-page-001",
					Tier:       types.TierBronze,
					Utility:    0.60,
					Confidence: 0.55,
				},
				Status: types.PoolStatusPending,
			},
			AgeString: "1h",
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// total > len(entries) triggers pagination message
	err := outputPoolList(entries, 0, 50, 10)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPoolList: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "Showing") {
		t.Errorf("expected pagination message with 'Showing', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// pool.go — outputPoolShow
// ---------------------------------------------------------------------------

func TestPool_outputPoolShow_textMode(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	entry := &pool.PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:          "show-test-001",
				Type:        types.KnowledgeTypeLearning,
				Tier:        types.TierGold,
				Content:     "Test learning content for show",
				Utility:     0.85,
				Confidence:  0.92,
				Maturity:    types.MaturityEstablished,
				RewardCount: 5,
				Source: types.Source{
					SessionID:      "session-show-test",
					TranscriptPath: "/tmp/show.jsonl",
					MessageIndex:   10,
				},
			},
			ScoringResult: types.Scoring{
				RawScore: 0.87,
				Rubric: types.RubricScores{
					Specificity:   0.90,
					Actionability: 0.85,
					Novelty:       0.80,
					Context:       0.88,
					Confidence:    0.91,
				},
			},
			Status: types.PoolStatusStaged,
		},
		AgeString: "3d",
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputPoolShow(entry)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPoolShow: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	checks := []string{
		"show-test-001",
		"gold",
		"staged",
		"Test learning content for show",
		"MemRL Metrics:",
		"Provenance:",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("expected output to contain %q, got:\n%s", check, out)
		}
	}
}

func TestPool_outputPoolShow_withHumanReview(t *testing.T) {
	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	reviewedAt := time.Now()
	entry := &pool.PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:      "review-show-001",
				Tier:    types.TierBronze,
				Content: "Reviewed content",
				Source:  types.Source{SessionID: "s1", TranscriptPath: "/t.jsonl"},
			},
			Status: types.PoolStatusPending,
			HumanReview: &types.HumanReview{
				Reviewed:   true,
				Approved:   true,
				Reviewer:   "test-user",
				Notes:      "Looks good",
				ReviewedAt: reviewedAt,
			},
			ScoringResult: types.Scoring{},
		},
		AgeString: "1h",
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputPoolShow(entry)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputPoolShow: %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	_ = r.Close()
	out := string(buf[:n])

	if !strings.Contains(out, "Human Review:") {
		t.Errorf("expected 'Human Review:' section, got:\n%s", out)
	}
	if !strings.Contains(out, "test-user") {
		t.Errorf("expected reviewer name, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// outputPoolAutoPromoteResult
// ---------------------------------------------------------------------------

func TestOutputPoolAutoPromoteResult_Human_NoPromotions(t *testing.T) {
	origOutput := output
	origDryRun := dryRun
	output = "table"
	dryRun = false
	defer func() { output = origOutput; dryRun = origDryRun }()

	result := poolAutoPromotePromoteResult{
		Threshold: "0.7",
		Promoted:  0,
	}

	out, err := captureStdout(t, func() error {
		return outputPoolAutoPromoteResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No candidates eligible") {
		t.Errorf("missing no-candidates message, got: %q", out)
	}
}

func TestOutputPoolAutoPromoteResult_Human_WithPromotions(t *testing.T) {
	origOutput := output
	origDryRun := dryRun
	output = "table"
	dryRun = false
	defer func() { output = origOutput; dryRun = origDryRun }()

	result := poolAutoPromotePromoteResult{
		Threshold: "0.8",
		Promoted:  2,
		Artifacts: []string{"/tmp/a.md", "/tmp/b.md"},
	}

	out, err := captureStdout(t, func() error {
		return outputPoolAutoPromoteResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Promoted 2 candidate(s):") {
		t.Errorf("missing promoted message, got: %q", out)
	}
	if !strings.Contains(out, "/tmp/a.md") {
		t.Errorf("missing first artifact, got: %q", out)
	}
}

func TestOutputPoolAutoPromoteResult_DryRun(t *testing.T) {
	origOutput := output
	origDryRun := dryRun
	output = "table"
	dryRun = true
	defer func() { output = origOutput; dryRun = origDryRun }()

	result := poolAutoPromotePromoteResult{
		Threshold: "0.7",
		Promoted:  3,
	}

	out, err := captureStdout(t, func() error {
		return outputPoolAutoPromoteResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("missing dry-run marker, got: %q", out)
	}
}

func TestOutputPoolAutoPromoteResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	result := poolAutoPromotePromoteResult{
		Threshold: "0.7",
		Promoted:  1,
		Artifacts: []string{"/tmp/test.md"},
	}

	out, err := captureStdout(t, func() error {
		return outputPoolAutoPromoteResult(result)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed poolAutoPromotePromoteResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Promoted != 1 {
		t.Errorf("Promoted = %d, want 1", parsed.Promoted)
	}
}
