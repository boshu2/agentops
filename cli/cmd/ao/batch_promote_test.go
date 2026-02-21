package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
)

func TestNormalizeContent(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		wantSame bool
	}{
		{
			name:     "exact duplicates",
			a:        "Lead-only commit eliminates merge conflicts",
			b:        "Lead-only commit eliminates merge conflicts",
			wantSame: true,
		},
		{
			name:     "case difference",
			a:        "Lead-Only Commit Pattern",
			b:        "lead-only commit pattern",
			wantSame: true,
		},
		{
			name:     "whitespace normalization",
			a:        "  Workers  should   never  commit  ",
			b:        "Workers should never commit",
			wantSame: true,
		},
		{
			name:     "distinct content with same 200-char prefix",
			a:        "This is a learning about topological wave decomposition that extracts maximum parallelism from dependency graphs by analyzing leaf nodes and grouping them into waves for concurrent execution — version alpha with special notes",
			b:        "This is a learning about topological wave decomposition that extracts maximum parallelism from dependency graphs by analyzing leaf nodes and grouping them into waves for concurrent execution — version beta with other notes",
			wantSame: false,
		},
		{
			name:     "completely different",
			a:        "Workers should never commit",
			b:        "Wave sizing follows dependency graph",
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyA := normalizeContent(tt.a)
			keyB := normalizeContent(tt.b)
			gotSame := keyA == keyB
			if gotSame != tt.wantSame {
				t.Errorf("normalizeContent(%q) == normalizeContent(%q): got %v, want %v",
					tt.a, tt.b, gotSame, tt.wantSame)
			}
		})
	}
}

func TestCheckPromotionCriteria(t *testing.T) {
	minAge := 24 * time.Hour

	tests := []struct {
		name       string
		entry      pool.PoolEntry
		citations  map[string]int
		promoted   map[string]bool
		wantReason string // "" means qualifies
	}{
		{
			name: "qualifies — old enough, cited, not duplicate",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-abc",
						Content: "unique learning content",
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"cand-abc": 2},
			promoted:   map[string]bool{},
			wantReason: "",
		},
		{
			name: "too young",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-young",
						Content: "fresh learning",
					},
				},
				Age:       12 * time.Hour,
				AgeString: "12h",
			},
			citations:  map[string]int{"cand-young": 1},
			promoted:   map[string]bool{},
			wantReason: "too young",
		},
		{
			name: "no citations",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-uncited",
						Content: "uncited learning",
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{},
			promoted:   map[string]bool{},
			wantReason: "no citations",
		},
		{
			name: "cited by file path",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-filepath",
						Content: "cited via file path",
					},
				},
				FilePath:  "/path/to/cand-filepath.json",
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"/path/to/cand-filepath.json": 1},
			promoted:   map[string]bool{},
			wantReason: "",
		},
		{
			name: "duplicate of promoted content",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-dup",
						Content: "Already Promoted Learning",
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"cand-dup": 1},
			promoted:   map[string]bool{normalizeContent("already promoted learning"): true},
			wantReason: "duplicate of already-promoted content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := checkPromotionCriteria("/tmp", tt.entry, minAge, tt.citations, tt.promoted)
			if tt.wantReason == "" {
				if reason != "" {
					t.Errorf("expected qualification, got skip reason: %q", reason)
				}
			} else {
				if reason == "" {
					t.Errorf("expected skip reason containing %q, got qualification", tt.wantReason)
				} else if !stringContains(reason, tt.wantReason) {
					t.Errorf("skip reason %q does not contain %q", reason, tt.wantReason)
				}
			}
		})
	}
}

func TestBuildCitationCounts(t *testing.T) {
	citations := []types.CitationEvent{
		{ArtifactPath: ".agents/pool/pending/cand-abc123.json"},
		{ArtifactPath: ".agents/pool/pending/cand-abc123.json"},
		{ArtifactPath: ".agents/learnings/pattern.md"},
		{ArtifactPath: ".agents/pool/pending/cand-def456.json"},
	}

	counts := buildCitationCounts(citations, "/tmp")

	if counts["cand-abc123"] != 2 {
		t.Errorf("cand-abc123 count = %d, want 2", counts["cand-abc123"])
	}

	if counts[".agents/pool/pending/cand-abc123.json"] != 2 {
		t.Errorf("path count = %d, want 2", counts[".agents/pool/pending/cand-abc123.json"])
	}

	if counts["cand-def456"] != 1 {
		t.Errorf("cand-def456 count = %d, want 1", counts["cand-def456"])
	}
}

func TestLoadPromotedContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "batch_promote_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(learningsDir, "pattern1.md"), []byte("Lead-only commit pattern"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "pattern2.md"), []byte("Wave decomposition strategy"), 0644); err != nil {
		t.Fatal(err)
	}

	content := loadPromotedContent(tmpDir)

	if len(content) != 2 {
		t.Errorf("got %d promoted entries, want 2", len(content))
	}

	key := normalizeContent("lead-only commit pattern")
	if !content[key] {
		t.Error("expected to find normalized 'lead-only commit pattern' in promoted content")
	}
}

func TestLoadPromotedContentMissingDir(t *testing.T) {
	content := loadPromotedContent("/nonexistent/path")
	if len(content) != 0 {
		t.Errorf("got %d entries for nonexistent dir, want 0", len(content))
	}
}

func TestOutputBatchResult(t *testing.T) {
	result := batchPromoteResult{
		Pending:  5,
		Promoted: 3,
		Skipped:  2,
		Reasons: []skipReason{
			{CandidateID: "cand-1", Reason: "too young"},
			{CandidateID: "cand-2", Reason: "no citations"},
		},
	}

	err := outputBatchResult(result)
	if err != nil {
		t.Errorf("outputBatchResult: %v", err)
	}
}

// stringContains checks if substr is in s.
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
