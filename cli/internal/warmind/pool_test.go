package warmind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPool_Init(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	pool := NewPool(tmpDir, cfg)

	t.Run("creates directory structure", func(t *testing.T) {
		err := pool.Init()
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		dirs := []string{
			filepath.Join(pool.PoolPath, PendingDir),
			filepath.Join(pool.PoolPath, StagedDir),
			filepath.Join(pool.PoolPath, RejectedDir),
		}

		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Errorf("Directory not created: %s", dir)
			}
		}
	})
}

func TestPool_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	pool := NewPool(tmpDir, cfg)

	t.Run("adds candidate to pending", func(t *testing.T) {
		candidate := Candidate{
			ID:          "test-001",
			Title:       "Test Learning",
			Content:     "This is test content",
			Author:      "alice",
			ContentHash: ContentHash("This is test content"),
			CreatedAt:   time.Now(),
		}

		err := pool.Add(candidate)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		// Verify file exists
		path := filepath.Join(pool.PoolPath, PendingDir, "test-001.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Candidate file not created")
		}
	})

	t.Run("rejects duplicate content", func(t *testing.T) {
		candidate := Candidate{
			ID:          "test-002",
			Title:       "Duplicate",
			Content:     "This is test content", // Same content as test-001
			Author:      "bob",
			ContentHash: ContentHash("This is test content"),
		}

		err := pool.Add(candidate)
		if err == nil {
			t.Error("Expected error for duplicate content")
		}
	})

	t.Run("rejects invalid candidate ID", func(t *testing.T) {
		candidate := Candidate{
			ID:          "../../../etc/passwd",
			Title:       "Malicious",
			Content:     "Bad content",
			ContentHash: ContentHash("Bad content"),
		}

		err := pool.Add(candidate)
		if err == nil {
			t.Error("Expected error for invalid candidate ID")
		}
	})
}

func TestPool_Score(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	pool := NewPool(tmpDir, cfg)

	// Add a pending candidate
	candidate := Candidate{
		ID:          "score-test",
		Title:       "Score Test",
		Content:     "Content for scoring",
		Author:      "alice",
		ContentHash: ContentHash("Content for scoring"),
	}
	pool.Add(candidate)

	t.Run("moves pending to staged on good score", func(t *testing.T) {
		scoring := ScoringResult{
			Novelty:        0.8,
			Specificity:    0.7,
			Actionability:  0.6,
			CompositeScore: 0.75,
			Tier:           TierSilver,
			ScoredAt:       time.Now(),
		}

		err := pool.Score("score-test", scoring)
		if err != nil {
			t.Fatalf("Score failed: %v", err)
		}

		// Should be in staged, not pending
		pendingPath := filepath.Join(pool.PoolPath, PendingDir, "score-test.json")
		stagedPath := filepath.Join(pool.PoolPath, StagedDir, "score-test.json")

		if _, err := os.Stat(pendingPath); !os.IsNotExist(err) {
			t.Error("Candidate should be removed from pending")
		}
		if _, err := os.Stat(stagedPath); os.IsNotExist(err) {
			t.Error("Candidate should be in staged")
		}
	})
}

func TestPool_ScoreRejectsDiscard(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	pool := NewPool(tmpDir, cfg)

	// Add a pending candidate
	candidate := Candidate{
		ID:          "discard-test",
		Title:       "Discard Test",
		Content:     "Low quality content",
		Author:      "alice",
		ContentHash: ContentHash("Low quality content"),
	}
	pool.Add(candidate)

	t.Run("rejects discard tier", func(t *testing.T) {
		scoring := ScoringResult{
			CompositeScore: 0.3,
			Tier:           TierDiscard,
			ScoredAt:       time.Now(),
		}

		err := pool.Score("discard-test", scoring)
		if err != nil {
			t.Fatalf("Score failed: %v", err)
		}

		// Should be in rejected
		rejectedPath := filepath.Join(pool.PoolPath, RejectedDir, "discard-test.json")
		if _, err := os.Stat(rejectedPath); os.IsNotExist(err) {
			t.Error("Candidate should be in rejected")
		}
	})
}

func TestPool_RecordCitation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.Promotion.SelfCitationAllowed = false
	pool := NewPool(tmpDir, cfg)

	// Add and stage a candidate
	candidate := Candidate{
		ID:          "cite-test",
		Title:       "Citation Test",
		Content:     "Content",
		Author:      "alice",
		AuthorEmail: "alice@example.com",
		ContentHash: ContentHash("Content"),
	}
	pool.Add(candidate)
	pool.Score("cite-test", ScoringResult{Tier: TierSilver, CompositeScore: 0.75})

	t.Run("increments citation count for non-self", func(t *testing.T) {
		err := pool.RecordCitation("cite-test", "bob", "bob@example.com", false)
		if err != nil {
			t.Fatalf("RecordCitation failed: %v", err)
		}

		entry, _ := pool.Get("cite-test")
		if entry.CitationCount != 1 {
			t.Errorf("CitationCount = %d, want 1", entry.CitationCount)
		}
	})

	t.Run("ignores self citation when not allowed", func(t *testing.T) {
		err := pool.RecordCitation("cite-test", "alice", "alice@example.com", true)
		if err != nil {
			t.Fatalf("RecordCitation failed: %v", err)
		}

		entry, _ := pool.Get("cite-test")
		if entry.CitationCount != 1 {
			t.Errorf("CitationCount = %d, want 1 (self citation should be ignored)", entry.CitationCount)
		}
	})
}

func TestPool_CheckPromotion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	cfg.Promotion.SilverCitationThreshold = 1
	cfg.Promotion.BronzeCitationThreshold = 3
	pool := NewPool(tmpDir, cfg)

	t.Run("silver tier promotes with 1 citation", func(t *testing.T) {
		candidate := Candidate{
			ID:          "silver-promo",
			Title:       "Silver Promo",
			Content:     "Silver content",
			ContentHash: ContentHash("Silver content"),
		}
		pool.Add(candidate)
		pool.Score("silver-promo", ScoringResult{Tier: TierSilver, CompositeScore: 0.75})
		pool.RecordCitation("silver-promo", "bob", "bob@example.com", false)

		eligible, reason := pool.CheckPromotion("silver-promo")
		if !eligible {
			t.Errorf("Silver tier with 1 citation should be eligible, reason: %s", reason)
		}
	})

	t.Run("bronze tier needs 3 citations", func(t *testing.T) {
		candidate := Candidate{
			ID:          "bronze-promo",
			Title:       "Bronze Promo",
			Content:     "Bronze content",
			ContentHash: ContentHash("Bronze content"),
		}
		pool.Add(candidate)
		pool.Score("bronze-promo", ScoringResult{Tier: TierBronze, CompositeScore: 0.55})
		pool.RecordCitation("bronze-promo", "bob", "bob@example.com", false)

		eligible, _ := pool.CheckPromotion("bronze-promo")
		if eligible {
			t.Error("Bronze tier with 1 citation should not be eligible")
		}

		// Add more citations
		pool.RecordCitation("bronze-promo", "charlie", "charlie@example.com", false)
		pool.RecordCitation("bronze-promo", "dave", "dave@example.com", false)

		eligible, _ = pool.CheckPromotion("bronze-promo")
		if !eligible {
			t.Error("Bronze tier with 3 citations should be eligible")
		}
	})
}

func TestValidateCandidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid simple", "learn-001", false},
		{"valid with underscore", "learn_test_001", false},
		{"valid with dash", "learn-test-001", false},
		{"empty", "", true},
		{"path traversal", "../etc/passwd", true},
		{"special chars", "learn@test!", true},
		{"spaces", "learn test", true},
		{"too long", string(make([]byte, 200)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCandidateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCandidateID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	t.Run("same content produces same hash", func(t *testing.T) {
		h1 := ContentHash("test content")
		h2 := ContentHash("test content")
		if h1 != h2 {
			t.Errorf("Same content should produce same hash: %s vs %s", h1, h2)
		}
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		h1 := ContentHash("test content 1")
		h2 := ContentHash("test content 2")
		if h1 == h2 {
			t.Error("Different content should produce different hash")
		}
	})

	t.Run("hash is deterministic", func(t *testing.T) {
		content := "deterministic test"
		for i := 0; i < 10; i++ {
			h := ContentHash(content)
			if len(h) != 64 { // SHA256 hex = 64 chars
				t.Errorf("Hash length = %d, want 64", len(h))
			}
		}
	})
}

func TestPool_Get(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	pool := NewPool(tmpDir, cfg)

	candidate := Candidate{
		ID:          "get-test",
		Title:       "Get Test",
		Content:     "Content",
		ContentHash: ContentHash("Content"),
	}
	pool.Add(candidate)

	t.Run("retrieves existing candidate", func(t *testing.T) {
		entry, err := pool.Get("get-test")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if entry.Candidate.ID != "get-test" {
			t.Errorf("ID = %s, want get-test", entry.Candidate.ID)
		}
	})

	t.Run("returns error for nonexistent", func(t *testing.T) {
		_, err := pool.Get("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent candidate")
		}
	})
}

func TestPool_Reject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "warmind-pool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig()
	pool := NewPool(tmpDir, cfg)

	candidate := Candidate{
		ID:          "reject-test",
		Title:       "Reject Test",
		Content:     "Content",
		ContentHash: ContentHash("Content"),
	}
	pool.Add(candidate)

	t.Run("moves to rejected directory", func(t *testing.T) {
		err := pool.Reject("reject-test", "test rejection")
		if err != nil {
			t.Fatalf("Reject failed: %v", err)
		}

		rejectedPath := filepath.Join(pool.PoolPath, RejectedDir, "reject-test.json")
		if _, err := os.Stat(rejectedPath); os.IsNotExist(err) {
			t.Error("Candidate should be in rejected")
		}

		entry, _ := pool.Get("reject-test")
		if entry.Status != StatusRejected {
			t.Errorf("Status = %v, want rejected", entry.Status)
		}
		if entry.RejectionReason != "test rejection" {
			t.Errorf("RejectionReason = %q, want 'test rejection'", entry.RejectionReason)
		}
	})
}
