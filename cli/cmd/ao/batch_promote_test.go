package main

import (
	"os"
	"path/filepath"
	"strings"
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
			name: "qualifies — old enough, cited, sufficient utility, not duplicate",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-abc",
						Content: "unique learning content",
						Utility: 0.6,
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
						Utility: 0.6,
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{},
			promoted:   map[string]bool{},
			wantReason: "insufficient citations",
		},
		{
			name: "cited by file path with sufficient citations and utility",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-filepath",
						Content: "cited via file path",
						Utility: 0.7,
					},
				},
				FilePath:  "/path/to/cand-filepath.json",
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"/path/to/cand-filepath.json": 3},
			promoted:   map[string]bool{},
			wantReason: "",
		},
		{
			name: "cited by file path but insufficient citations",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-filepath-low",
						Content: "cited via file path low",
						Utility: 0.7,
					},
				},
				FilePath:  "/path/to/cand-filepath-low.json",
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"/path/to/cand-filepath-low.json": 1},
			promoted:   map[string]bool{},
			wantReason: "insufficient citations",
		},
		{
			name: "utility too low",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-lowutil",
						Content: "low utility learning",
						Utility: 0.3,
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"cand-lowutil": 5},
			promoted:   map[string]bool{},
			wantReason: "utility too low",
		},
		{
			name: "duplicate of promoted content",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-dup",
						Content: "Already Promoted Learning",
						Utility: 0.8,
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			citations:  map[string]int{"cand-dup": 3},
			promoted:   map[string]bool{normalizeContent("already promoted learning"): true},
			wantReason: "duplicate of already-promoted content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := checkPromotionCriteria("/tmp", tt.entry, minAge, tt.citations, tt.promoted, true)
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

// TestCheckPromotionCriteria_RequireCitationsFalse covers the flywheel close-loop path:
// freshly-scored silver/gold candidates have no citations yet (the loop hasn't surfaced them),
// so requireCitations=false lets them through on utility + non-duplicate alone. The other
// gates (age, utility, duplicate) still apply.
func TestCheckPromotionCriteria_RequireCitationsFalse(t *testing.T) {
	minAge := 24 * time.Hour

	tests := []struct {
		name       string
		entry      pool.PoolEntry
		promoted   map[string]bool
		wantReason string
	}{
		{
			name: "uncited fresh candidate qualifies without citations",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-fresh",
						Content: "new high-scoring learning",
						Utility: 0.6,
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			promoted:   map[string]bool{},
			wantReason: "",
		},
		{
			name: "age gate still enforced",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-young",
						Content: "too fresh",
						Utility: 0.8,
					},
				},
				Age:       1 * time.Hour,
				AgeString: "1h",
			},
			promoted:   map[string]bool{},
			wantReason: "too young",
		},
		{
			name: "utility gate still enforced",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-lowutil",
						Content: "weak signal",
						Utility: 0.2,
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			promoted:   map[string]bool{},
			wantReason: "utility too low",
		},
		{
			name: "duplicate gate still enforced",
			entry: pool.PoolEntry{
				PoolEntry: types.PoolEntry{
					Candidate: types.Candidate{
						ID:      "cand-dup",
						Content: "Already Promoted",
						Utility: 0.8,
					},
				},
				Age:       48 * time.Hour,
				AgeString: "48h",
			},
			promoted:   map[string]bool{normalizeContent("already promoted"): true},
			wantReason: "duplicate of already-promoted content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := checkPromotionCriteria("/tmp", tt.entry, minAge, map[string]int{}, tt.promoted, false)
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

func TestRunBatchPromote_NoPendingCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	oldOutput := output
	output = "text"
	oldMinAge := batchPromoteMinAge
	oldForce := batchPromoteForce
	batchPromoteMinAge = "24h"
	batchPromoteForce = false
	t.Cleanup(func() {
		output = oldOutput
		batchPromoteMinAge = oldMinAge
		batchPromoteForce = oldForce
	})

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = runBatchPromote(nil, nil)
	w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("runBatchPromote: %v", err)
	}

	var buf [512]byte
	n, _ := r.Read(buf[:])
	r.Close()
	if !strings.Contains(string(buf[:n]), "No pending candidates found") {
		t.Fatalf("expected no-pending message, got: %s", string(buf[:n]))
	}
}

func TestTryPromoteEntry_RecordsSkipOnError(t *testing.T) {
	tmpDir := t.TempDir()
	p := pool.NewPool(tmpDir)
	result := &batchPromoteResult{}
	entry := pool.PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{ID: "missing-candidate"},
		},
	}

	err := tryPromoteEntry(p, entry, result)
	if err == nil {
		t.Fatal("expected promotion error for missing candidate")
	}
	if result.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", result.Skipped)
	}
	if len(result.Reasons) != 1 || !strings.Contains(result.Reasons[0].Reason, "error:") {
		t.Fatalf("expected error reason recorded, got %+v", result.Reasons)
	}
}

func TestPromoteEntry_DryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = oldDryRun })

	entry := pool.PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:   "cand-dry-run",
				Tier: types.TierSilver,
			},
		},
		AgeString: "48h",
	}
	result := &batchPromoteResult{}

	stdout, err := captureStdout(t, func() error {
		return promoteEntry(nil, entry, result)
	})
	if err != nil {
		t.Fatalf("promoteEntry: %v", err)
	}
	if result.Promoted != 1 {
		t.Fatalf("expected promoted=1, got %d", result.Promoted)
	}
	if !strings.Contains(stdout, "[dry-run] Would promote: cand-dry-run (tier=silver, age=48h)") {
		t.Fatalf("expected dry-run output, got %q", stdout)
	}
}

func TestProcessPromotionCandidate_TooYoungSkips(t *testing.T) {
	oldForce := batchPromoteForce
	batchPromoteForce = false
	t.Cleanup(func() { batchPromoteForce = oldForce })

	result := &batchPromoteResult{}
	entry := pool.PoolEntry{
		PoolEntry: types.PoolEntry{
			Candidate: types.Candidate{
				ID:      "cand-too-young",
				Content: "some content",
			},
		},
		Age:       1 * time.Hour,
		AgeString: "1h",
	}

	promoted := processPromotionCandidate(nil, entry, t.TempDir(), 24*time.Hour, map[string]int{}, map[string]bool{}, result)
	if promoted {
		t.Fatal("expected non-promoted candidate for too-young entry")
	}
	if result.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", result.Skipped)
	}
	if len(result.Reasons) != 1 || !strings.Contains(result.Reasons[0].Reason, "too young") {
		t.Fatalf("expected too-young reason, got %+v", result.Reasons)
	}
}

// stringContains checks if substr is in s.
func stringContains(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
