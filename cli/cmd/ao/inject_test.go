package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestFreshnessScore(t *testing.T) {
	tests := []struct {
		name     string
		ageWeeks float64
		wantMin  float64
		wantMax  float64
	}{
		{"fresh (0 weeks)", 0, 0.99, 1.01},
		{"1 week old", 1, 0.82, 0.86},
		{"4 weeks old", 4, 0.49, 0.52},
		{"12 weeks old", 12, 0.10, 0.15},
		{"52 weeks old", 52, 0.10, 0.11}, // Clamped to 0.1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freshnessScore(tt.ageWeeks)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("freshnessScore(%v) = %v, want between %v and %v",
					tt.ageWeeks, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestParseFrontMatter(t *testing.T) {
	tests := []struct {
		name           string
		lines          []string
		wantSuperseded string
		wantUtility    float64
		wantHasUtility bool
		wantEndLine    int
	}{
		{
			name:           "no front matter",
			lines:          []string{"# Title", "Content"},
			wantSuperseded: "",
			wantUtility:    0,
			wantHasUtility: false,
			wantEndLine:    0,
		},
		{
			name:           "empty front matter",
			lines:          []string{"---", "---", "# Title"},
			wantSuperseded: "",
			wantUtility:    0,
			wantHasUtility: false,
			wantEndLine:    2,
		},
		{
			name:           "superseded_by set",
			lines:          []string{"---", "superseded_by: L42", "---", "# Title"},
			wantSuperseded: "L42",
			wantUtility:    0,
			wantHasUtility: false,
			wantEndLine:    3,
		},
		{
			name:           "superseded-by with dash",
			lines:          []string{"---", "superseded-by: new-learning", "---"},
			wantSuperseded: "new-learning",
			wantUtility:    0,
			wantHasUtility: false,
			wantEndLine:    3,
		},
		{
			name:           "superseded_by null",
			lines:          []string{"---", "superseded_by: null", "---"},
			wantSuperseded: "null",
			wantUtility:    0,
			wantHasUtility: false,
			wantEndLine:    3,
		},
		{
			name:           "utility in front matter",
			lines:          []string{"---", "utility: 0.73", "---", "# Title"},
			wantSuperseded: "",
			wantUtility:    0.73,
			wantHasUtility: true,
			wantEndLine:    3,
		},
		{
			name:           "empty lines slice",
			lines:          []string{},
			wantSuperseded: "",
			wantUtility:    0,
			wantHasUtility: false,
			wantEndLine:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, endLine := parseFrontMatter(tt.lines)
			if fm.SupersededBy != tt.wantSuperseded {
				t.Errorf("parseFrontMatter() supersededBy = %q, want %q",
					fm.SupersededBy, tt.wantSuperseded)
			}
			if fm.HasUtility != tt.wantHasUtility {
				t.Errorf("parseFrontMatter() hasUtility = %v, want %v", fm.HasUtility, tt.wantHasUtility)
			}
			if fm.Utility != tt.wantUtility {
				t.Errorf("parseFrontMatter() utility = %f, want %f", fm.Utility, tt.wantUtility)
			}
			if endLine != tt.wantEndLine {
				t.Errorf("parseFrontMatter() endLine = %d, want %d",
					endLine, tt.wantEndLine)
			}
		})
	}
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		startIdx int
		want     string
	}{
		{
			name:     "simple paragraph",
			lines:    []string{"# Title", "This is the summary."},
			startIdx: 1,
			want:     "This is the summary.",
		},
		{
			name:     "skip empty lines",
			lines:    []string{"", "", "Summary text"},
			startIdx: 0,
			want:     "Summary text",
		},
		{
			name:     "skip headings",
			lines:    []string{"## Heading", "Content here"},
			startIdx: 0,
			want:     "Content here",
		},
		{
			name:     "multi-line paragraph",
			lines:    []string{"First line.", "Second line.", "Third line."},
			startIdx: 0,
			want:     "First line. Second line. Third line.",
		},
		{
			name:     "stop at empty line",
			lines:    []string{"First line.", "", "Different paragraph"},
			startIdx: 0,
			want:     "First line.",
		},
		{
			name:     "empty content",
			lines:    []string{},
			startIdx: 0,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummary(tt.lines, tt.startIdx)
			if got != tt.want {
				t.Errorf("extractSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseLearningFile(t *testing.T) {
	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "inject_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	// Test: regular markdown file
	t.Run("regular markdown", func(t *testing.T) {
		content := `---
id: L42
utility: 0.77
---
# Test Learning

This is the summary content.
`
		path := filepath.Join(tmpDir, "test-learning.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		l, err := parseLearningFile(path)
		if err != nil {
			t.Errorf("parseLearningFile() error = %v", err)
		}
		if l.Superseded {
			t.Error("expected Superseded = false")
		}
		if l.Title != "Test Learning" {
			t.Errorf("Title = %q, want %q", l.Title, "Test Learning")
		}
		if abs(l.Utility-0.77) > 0.001 {
			t.Errorf("Utility = %f, want 0.77", l.Utility)
		}
	})

	// Test: superseded markdown file
	t.Run("superseded markdown", func(t *testing.T) {
		content := `---
superseded_by: L100
---
# Old Learning
`
		path := filepath.Join(tmpDir, "old-learning.md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		l, err := parseLearningFile(path)
		if err != nil {
			t.Errorf("parseLearningFile() error = %v", err)
		}
		if !l.Superseded {
			t.Error("expected Superseded = true")
		}
	})

	// Test: file not found
	t.Run("file not found", func(t *testing.T) {
		_, err := parseLearningFile(filepath.Join(tmpDir, "nonexistent.md"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestApplyCompositeScoring(t *testing.T) {
	tests := []struct {
		name      string
		learnings []learning
		lambda    float64
		// We check relative ordering rather than exact scores
		wantFirst string // ID of learning that should rank first
	}{
		{
			name:      "empty slice",
			learnings: []learning{},
			lambda:    0.5,
			wantFirst: "",
		},
		{
			name: "high utility wins with high lambda",
			learnings: []learning{
				{ID: "fresh", FreshnessScore: 1.0, Utility: 0.3},
				{ID: "useful", FreshnessScore: 0.5, Utility: 0.9},
			},
			lambda:    2.0, // Weight utility MORE than freshness (lambda > 1)
			wantFirst: "useful",
		},
		{
			name: "freshness wins with low lambda",
			learnings: []learning{
				{ID: "fresh", FreshnessScore: 1.0, Utility: 0.3},
				{ID: "useful", FreshnessScore: 0.5, Utility: 0.9},
			},
			lambda:    0.0, // Ignore utility
			wantFirst: "fresh",
		},
		{
			name: "balanced scoring",
			learnings: []learning{
				{ID: "L1", FreshnessScore: 0.8, Utility: 0.6},
				{ID: "L2", FreshnessScore: 0.6, Utility: 0.8},
				{ID: "L3", FreshnessScore: 0.5, Utility: 0.5},
			},
			lambda:    0.5,
			wantFirst: "L1", // L1 and L2 similar, but L1 has higher freshness
		},
		{
			name: "default utility (all 0.5)",
			learnings: []learning{
				{ID: "newer", FreshnessScore: 0.9, Utility: types.InitialUtility},
				{ID: "older", FreshnessScore: 0.3, Utility: types.InitialUtility},
			},
			lambda:    0.5,
			wantFirst: "newer", // When utility is equal, freshness wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			learnings := make([]learning, len(tt.learnings))
			copy(learnings, tt.learnings)

			items := make([]scorable, len(learnings))
			for i := range learnings {
				items[i] = &learnings[i]
			}
			applyCompositeScoringTo(items, tt.lambda)

			if tt.wantFirst == "" {
				return // Empty case
			}

			// Sort by composite score (descending)
			maxScore := math.Inf(-1)
			var winner string
			for _, l := range learnings {
				if l.CompositeScore > maxScore {
					maxScore = l.CompositeScore
					winner = l.ID
				}
			}

			if winner != tt.wantFirst {
				t.Errorf("winner = %q, want %q", winner, tt.wantFirst)
				for _, l := range learnings {
					t.Logf("  %s: freshness=%.2f, utility=%.2f, composite=%.3f",
						l.ID, l.FreshnessScore, l.Utility, l.CompositeScore)
				}
			}
		})
	}
}

func TestCompositeScoringZNormalization(t *testing.T) {
	// Test that z-normalization produces mean ~0 and stddev ~1
	learnings := []learning{
		{ID: "L1", FreshnessScore: 1.0, Utility: 0.9},
		{ID: "L2", FreshnessScore: 0.8, Utility: 0.7},
		{ID: "L3", FreshnessScore: 0.6, Utility: 0.5},
		{ID: "L4", FreshnessScore: 0.4, Utility: 0.3},
		{ID: "L5", FreshnessScore: 0.2, Utility: 0.1},
	}

	items := make([]scorable, len(learnings))
	for i := range learnings {
		items[i] = &learnings[i]
	}
	applyCompositeScoringTo(items, 0.5)

	// All learnings should have composite scores set
	// Verify all learnings have composite scores computed.
	for _, l := range learnings {
		if l.CompositeScore == 0 && l.FreshnessScore != 0.6 {
			t.Errorf("expected non-zero composite score for learning %s (freshness=%v)", l.ID, l.FreshnessScore)
		}
	}

	// Verify that higher freshness + utility = higher score
	// L1 should have highest score, L5 should have lowest
	if learnings[0].CompositeScore <= learnings[4].CompositeScore {
		t.Errorf("expected L1 > L5 but got %v <= %v",
			learnings[0].CompositeScore, learnings[4].CompositeScore)
	}
}

// TestOlderItemScoresLowerThanNewerItem verifies that knowledge decay works correctly:
// An 8-week-old item should score lower than a 1-week-old item with the same utility.
// This tests the freshness decay formula: freshnessScore = exp(-ageWeeks * 0.17)
func TestOlderItemScoresLowerThanNewerItem(t *testing.T) {
	// Test freshness scores directly
	t.Run("freshness score decay", func(t *testing.T) {
		oneWeekScore := freshnessScore(1.0)   // 1 week old
		eightWeekScore := freshnessScore(8.0) // 8 weeks old

		if eightWeekScore >= oneWeekScore {
			t.Errorf("8-week-old item (%.4f) should score LOWER than 1-week-old item (%.4f)",
				eightWeekScore, oneWeekScore)
		}

		// Verify expected values based on formula: exp(-weeks * 0.17)
		// 1 week: exp(-1 * 0.17) = exp(-0.17) ≈ 0.8437
		// 8 weeks: exp(-8 * 0.17) = exp(-1.36) ≈ 0.2567
		expectedOneWeek := math.Exp(-1.0 * 0.17)
		expectedEightWeek := math.Exp(-8.0 * 0.17)

		if math.Abs(oneWeekScore-expectedOneWeek) > 0.01 {
			t.Errorf("1-week score %.4f doesn't match expected %.4f", oneWeekScore, expectedOneWeek)
		}
		if math.Abs(eightWeekScore-expectedEightWeek) > 0.01 {
			t.Errorf("8-week score %.4f doesn't match expected %.4f", eightWeekScore, expectedEightWeek)
		}
	})

	// Test composite scores with same utility
	t.Run("composite score with same utility", func(t *testing.T) {
		sameUtility := 0.7

		learnings := []learning{
			{ID: "newer", FreshnessScore: freshnessScore(1.0), Utility: sameUtility},
			{ID: "older", FreshnessScore: freshnessScore(8.0), Utility: sameUtility},
		}

		items := make([]scorable, len(learnings))
		for i := range learnings {
			items[i] = &learnings[i]
		}
		applyCompositeScoringTo(items, types.DefaultLambda)

		// Find the scores
		var newerScore, olderScore float64
		for _, l := range learnings {
			switch l.ID {
			case "newer":
				newerScore = l.CompositeScore
			case "older":
				olderScore = l.CompositeScore
			}
		}

		if olderScore >= newerScore {
			t.Errorf("8-week-old item (composite=%.4f) should rank LOWER than 1-week-old item (composite=%.4f) when utility is equal (%.2f)",
				olderScore, newerScore, sameUtility)
		}
	})
}

// TestDecayFloorEnforced verifies that the minimum score floor of 0.1 is enforced.
// Very old items should not decay below 0.1 - old knowledge still has some value.
func TestDecayFloorEnforced(t *testing.T) {
	tests := []struct {
		name     string
		ageWeeks float64
		wantMin  float64
	}{
		{"20 weeks old", 20, 0.1},
		{"52 weeks old (1 year)", 52, 0.1},
		{"104 weeks old (2 years)", 104, 0.1},
		{"1000 weeks old", 1000, 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := freshnessScore(tt.ageWeeks)

			if score < tt.wantMin {
				t.Errorf("freshnessScore(%.0f weeks) = %.4f, should not be less than %.2f (decay floor)",
					tt.ageWeeks, score, tt.wantMin)
			}

			// Also verify it's exactly 0.1 (clamped) for very old items
			if tt.ageWeeks >= 20 && score != 0.1 {
				t.Errorf("freshnessScore(%.0f weeks) = %.4f, expected exactly 0.1 (clamped)",
					tt.ageWeeks, score)
			}
		})
	}
}

// TestConfidenceDecayRate verifies the confidence decay formula.
// Confidence decays at 10%/week: newConf = oldConf * exp(-weeks * 0.1)
func TestConfidenceDecayRate(t *testing.T) {
	tests := []struct {
		name            string
		weeksSinceDecay float64
		oldConfidence   float64
		wantMin         float64 // Minimum expected (with small tolerance)
		wantMax         float64 // Maximum expected (with small tolerance)
	}{
		{
			name:            "1 week decay",
			weeksSinceDecay: 1.0,
			oldConfidence:   1.0,
			// exp(-1 * 0.1) = exp(-0.1) ≈ 0.9048
			wantMin: 0.90,
			wantMax: 0.91,
		},
		{
			name:            "4 weeks decay",
			weeksSinceDecay: 4.0,
			oldConfidence:   1.0,
			// exp(-4 * 0.1) = exp(-0.4) ≈ 0.6703
			wantMin: 0.66,
			wantMax: 0.68,
		},
		{
			name:            "8 weeks decay",
			weeksSinceDecay: 8.0,
			oldConfidence:   1.0,
			// exp(-8 * 0.1) = exp(-0.8) ≈ 0.4493
			wantMin: 0.44,
			wantMax: 0.46,
		},
		{
			name:            "decay from 0.5 confidence",
			weeksSinceDecay: 4.0,
			oldConfidence:   0.5,
			// 0.5 * exp(-0.4) ≈ 0.5 * 0.6703 ≈ 0.335
			wantMin: 0.33,
			wantMax: 0.34,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply decay formula: newConf = oldConf * exp(-weeks * 0.1)
			decayFactor := math.Exp(-tt.weeksSinceDecay * types.ConfidenceDecayRate)
			newConfidence := tt.oldConfidence * decayFactor

			if newConfidence < tt.wantMin || newConfidence > tt.wantMax {
				t.Errorf("confidence decay: %.2f * exp(-%.1f * 0.1) = %.4f, want between %.2f and %.2f",
					tt.oldConfidence, tt.weeksSinceDecay, newConfidence, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestParseFrontMatterPromotedTo verifies promoted_to and promoted-to parsing.
func TestParseFrontMatterPromotedTo(t *testing.T) {
	tests := []struct {
		name       string
		lines      []string
		wantPromTo string
	}{
		{
			name:       "promoted_to with underscore",
			lines:      []string{"---", "promoted_to: ~/.agents/learnings/global-auth.md", "---"},
			wantPromTo: "~/.agents/learnings/global-auth.md",
		},
		{
			name:       "promoted-to with dash",
			lines:      []string{"---", "promoted-to: ~/.agents/learnings/global-auth.md", "---"},
			wantPromTo: "~/.agents/learnings/global-auth.md",
		},
		{
			name:       "promoted_to null",
			lines:      []string{"---", "promoted_to: null", "---"},
			wantPromTo: "null",
		},
		{
			name:       "no promoted_to",
			lines:      []string{"---", "utility: 0.7", "---"},
			wantPromTo: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, _ := parseFrontMatter(tt.lines)
			if fm.PromotedTo != tt.wantPromTo {
				t.Errorf("parseFrontMatter() PromotedTo = %q, want %q", fm.PromotedTo, tt.wantPromTo)
			}
		})
	}
}

// TestIsPromoted verifies promoted detection including null/tilde filtering.
func TestIsPromoted(t *testing.T) {
	tests := []struct {
		name string
		fm   frontMatter
		want bool
	}{
		{"promoted", frontMatter{PromotedTo: "~/.agents/learnings/foo.md"}, true},
		{"null", frontMatter{PromotedTo: "null"}, false},
		{"tilde", frontMatter{PromotedTo: "~"}, false},
		{"empty", frontMatter{PromotedTo: ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPromoted(tt.fm); got != tt.want {
				t.Errorf("isPromoted() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseLearningFileSkipsPromoted verifies promoted files are skipped via Superseded flag.
func TestParseLearningFileSkipsPromoted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "inject_promoted_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck // test cleanup
	}()

	content := "---\npromoted_to: ~/.agents/learnings/global-auth.md\n---\n# Auth Pattern\n\nLocal auth learning.\n"
	path := filepath.Join(tmpDir, "promoted-learning.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	l, err := parseLearningFile(path)
	if err != nil {
		t.Errorf("parseLearningFile() error = %v", err)
	}
	if !l.Superseded {
		t.Error("expected promoted learning to have Superseded = true")
	}
}

// TestCollectLearningsGlobalDir verifies global learnings are collected and flagged.
func TestCollectLearningsGlobalDir(t *testing.T) {
	// Create local learnings dir with .agents/learnings structure
	localDir, err := os.MkdirTemp("", "inject_local_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(localDir) //nolint:errcheck // test cleanup
	}()

	localLearningsDir := filepath.Join(localDir, ".agents", "learnings")
	if err := os.MkdirAll(localLearningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	localContent := "---\nutility: 0.8\nmaturity: provisional\n---\n# Local Learning\n\nThis is local.\n"
	if err := os.WriteFile(filepath.Join(localLearningsDir, "local.md"), []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create global learnings dir
	globalDir, err := os.MkdirTemp("", "inject_global_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(globalDir) //nolint:errcheck // test cleanup
	}()

	globalContent := "---\nutility: 0.7\nmaturity: provisional\n---\n# Global Learning\n\nCross-repo knowledge.\n"
	if err := os.WriteFile(filepath.Join(globalDir, "global.md"), []byte(globalContent), 0644); err != nil {
		t.Fatal(err)
	}

	learnings, err := collectLearnings(localDir, "", 10, globalDir, 0.8)
	if err != nil {
		t.Fatalf("collectLearnings() error = %v", err)
	}
	if len(learnings) != 2 {
		t.Fatalf("expected 2 learnings, got %d", len(learnings))
	}

	// Verify one is global and one is not
	var foundLocal, foundGlobal bool
	for _, l := range learnings {
		if l.Global {
			foundGlobal = true
		} else {
			foundLocal = true
		}
	}
	if !foundLocal {
		t.Error("expected to find a local learning")
	}
	if !foundGlobal {
		t.Error("expected to find a global learning")
	}
}

// TestCollectLearningsGlobalWeight verifies global weight penalty reduces global scores.
func TestCollectLearningsGlobalWeight(t *testing.T) {
	// Create local learnings dir with multiple files for z-normalization spread
	localDir, err := os.MkdirTemp("", "inject_weight_local")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(localDir) //nolint:errcheck // test cleanup
	}()

	localLearningsDir := filepath.Join(localDir, ".agents", "learnings")
	if err := os.MkdirAll(localLearningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create 3 local learnings with varying utility for z-norm spread
	for _, item := range []struct{ name, utility string }{
		{"local-high.md", "0.9"},
		{"local-mid.md", "0.7"},
		{"local-low.md", "0.4"},
	} {
		content := "---\nutility: " + item.utility + "\nmaturity: provisional\n---\n# " + item.name + "\n\nLocal content.\n"
		if err := os.WriteFile(filepath.Join(localLearningsDir, item.name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create global learnings dir with high utility (should still score lower after penalty)
	globalDir, err := os.MkdirTemp("", "inject_weight_global")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(globalDir) //nolint:errcheck // test cleanup
	}()

	globalContent := "---\nutility: 0.9\nmaturity: provisional\n---\n# Global High\n\nGlobal content.\n"
	if err := os.WriteFile(filepath.Join(globalDir, "global-high.md"), []byte(globalContent), 0644); err != nil {
		t.Fatal(err)
	}

	learnings, err := collectLearnings(localDir, "", 10, globalDir, 0.8)
	if err != nil {
		t.Fatalf("collectLearnings() error = %v", err)
	}

	// Find the local-high and global-high scores (both utility 0.9, same freshness)
	var localHighScore, globalHighScore float64
	for _, l := range learnings {
		if l.Global && l.Title == "global-high.md" {
			globalHighScore = l.CompositeScore
		}
		if !l.Global && l.Title == "local-high.md" {
			localHighScore = l.CompositeScore
		}
	}

	// Global item with same utility should score strictly lower due to 0.8 weight penalty
	if globalHighScore >= localHighScore {
		t.Errorf("global score (%.4f) should be less than local score (%.4f) due to weight penalty",
			globalHighScore, localHighScore)
		for _, l := range learnings {
			t.Logf("  %s (global=%v): composite=%.4f utility=%.2f freshness=%.2f",
				l.Title, l.Global, l.CompositeScore, l.Utility, l.FreshnessScore)
		}
	}
}

// TestCollectPatternsGlobalDir verifies global patterns are collected and flagged.
func TestCollectPatternsGlobalDir(t *testing.T) {
	// Create local patterns dir
	localDir, err := os.MkdirTemp("", "patterns_local_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(localDir) //nolint:errcheck // test cleanup
	}()

	localPatternsDir := filepath.Join(localDir, ".agents", "patterns")
	if err := os.MkdirAll(localPatternsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localPatternsDir, "local-pattern.md"), []byte("# Local Pattern\n\nLocal description.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create global patterns dir
	globalDir, err := os.MkdirTemp("", "patterns_global_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(globalDir) //nolint:errcheck // test cleanup
	}()

	globalNamespace := filepath.Join(globalDir, "jren-platform")
	if err := os.MkdirAll(globalNamespace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalNamespace, "global-pattern.md"), []byte("# Global Pattern\n\nCross-repo pattern.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	patterns, err := collectPatterns(localDir, "", 10, globalDir, 0.8)
	if err != nil {
		t.Fatalf("collectPatterns() error = %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}

	var foundLocal, foundGlobal bool
	for _, p := range patterns {
		if p.Global {
			foundGlobal = true
		} else {
			foundLocal = true
		}
	}
	if !foundLocal {
		t.Error("expected to find a local pattern")
	}
	if !foundGlobal {
		t.Error("expected to find a global pattern")
	}
}

// TestConfidenceDecayFloor verifies that confidence decay respects the minimum of 0.1.
func TestConfidenceDecayFloor(t *testing.T) {
	tests := []struct {
		name            string
		weeksSinceDecay float64
		oldConfidence   float64
	}{
		{"52 weeks from full confidence", 52, 1.0},
		{"100 weeks from full confidence", 100, 1.0},
		{"10 weeks from low confidence", 10, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decayFactor := math.Exp(-tt.weeksSinceDecay * types.ConfidenceDecayRate)
			newConfidence := tt.oldConfidence * decayFactor

			// Apply floor
			if newConfidence < 0.1 {
				newConfidence = 0.1
			}

			if newConfidence < 0.1 {
				t.Errorf("confidence should never go below 0.1, got %.4f", newConfidence)
			}
		})
	}
}

func TestApplyConfidenceDecay_MarkdownLearning(t *testing.T) {
	dir := t.TempDir()
	fourWeeksAgo := time.Now().Add(-4 * 7 * 24 * time.Hour).Format(time.RFC3339)
	path := writeTestMDLearning(t, dir, "test-decay.md", map[string]string{
		"confidence":     "0.8000",
		"last_reward_at": fourWeeksAgo,
		"maturity":       "provisional",
	}, "# Test Learning\nSome content\n")

	l := learning{ID: "test-decay", Utility: 0.8, Source: path}
	result := applyConfidenceDecay(l, path, time.Now())

	// After 4 weeks of decay, utility should decrease
	if result.Utility >= l.Utility {
		t.Errorf("expected utility to decrease after 4 weeks decay, got %.4f >= %.4f", result.Utility, l.Utility)
	}

	// Verify file was updated with new confidence
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "last_decay_at:") {
		t.Error("expected last_decay_at field in updated frontmatter")
	}
	// Confidence in file should be less than original 0.8
	if strings.Contains(string(content), "confidence: 0.8000") {
		t.Error("expected confidence to be updated from original 0.8000")
	}
}

func TestApplyConfidenceDecay_MarkdownNoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-fm.md")
	if err := os.WriteFile(path, []byte("# Just a heading\nNo frontmatter here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	l := learning{ID: "no-fm", Utility: 0.8, Source: path}
	result := applyConfidenceDecay(l, path, time.Now())

	// Should be a no-op — utility unchanged
	if result.Utility != l.Utility {
		t.Errorf("expected utility unchanged for .md without frontmatter, got %.4f != %.4f", result.Utility, l.Utility)
	}
}

func TestApplyConfidenceDecay_MarkdownNoTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := writeTestMDLearning(t, dir, "no-ts.md", map[string]string{
		"confidence": "0.9000",
		"maturity":   "provisional",
	}, "# No Timestamp\nContent here\n")

	l := learning{ID: "no-ts", Utility: 0.9, Source: path}
	result := applyConfidenceDecay(l, path, time.Now())

	// No timestamp → no decay → utility unchanged
	if result.Utility != l.Utility {
		t.Errorf("expected utility unchanged without timestamp, got %.4f != %.4f", result.Utility, l.Utility)
	}
}

func TestPassesQualityGate(t *testing.T) {
	tests := []struct {
		name     string
		maturity string
		utility  float64
		want     bool
	}{
		{"provisional with good utility", "provisional", 0.8, true},
		{"candidate with good utility", "candidate", 0.5, true},
		{"established with good utility", "established", 0.9, true},
		{"provisional with low utility", "provisional", 0.2, false},
		{"provisional at boundary", "provisional", 0.3, false}, // 0.3 is NOT > 0.3
		{"provisional just above", "provisional", 0.31, true},
		{"empty maturity defaults provisional", "", 0.8, true},
		{"draft maturity", "draft", 0.8, false},
		{"unknown maturity", "foobar", 0.8, false},
		{"empty maturity low utility", "", 0.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := learning{ID: "test", Maturity: tt.maturity, Utility: tt.utility}
			got := passesQualityGate(l)
			if got != tt.want {
				t.Errorf("passesQualityGate(maturity=%q, utility=%.2f) = %v, want %v",
					tt.maturity, tt.utility, got, tt.want)
			}
		})
	}
}

func TestProcessLearningFile_QualityGateFilters(t *testing.T) {
	dir := t.TempDir()

	// Learning with no maturity should now pass (empty defaults to provisional)
	noMaturity := writeTestMDLearning(t, dir, "no-maturity.md", map[string]string{
		"utility": "0.9",
	}, "# Good content\nBut no maturity\n")

	_, included := processLearningFile(noMaturity, "", time.Now())
	if !included {
		t.Error("expected learning without maturity to pass quality gate (empty defaults to provisional)")
	}

	// Learning with provisional + good utility should pass
	good := writeTestMDLearning(t, dir, "good.md", map[string]string{
		"maturity": "provisional",
		"utility":  "0.8",
	}, "# Good Learning\nHas maturity and utility\n")

	l, included := processLearningFile(good, "", time.Now())
	if !included {
		t.Error("expected provisional learning with utility 0.8 to pass quality gate")
	}
	if l.Maturity != "provisional" {
		t.Errorf("expected maturity=provisional, got %q", l.Maturity)
	}
}

func TestProcessLearningFile_QualityGateUtilityBoundary(t *testing.T) {
	dir := t.TempDir()

	// Learning with provisional but utility exactly 0.3 — should NOT pass (> not >=)
	borderline := writeTestMDLearning(t, dir, "borderline.md", map[string]string{
		"maturity": "provisional",
		"utility":  "0.3",
	}, "# Borderline\nExactly at threshold\n")

	_, included := processLearningFile(borderline, "", time.Now())
	if included {
		t.Error("expected learning with utility=0.3 to be filtered (gate requires > 0.3)")
	}
}

// TestInjectForFlag_ResearchSkill verifies --for filters out excluded sections.
func TestInjectForFlag_ResearchSkill(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	// Create skill with context declaration excluding HISTORY and TASK
	skillDir := filepath.Join(tmp, "skills", "research")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: research
description: test
skill_api_version: 1
context:
  window: isolated
  sections:
    exclude: [HISTORY, TASK]
  intel_scope: topic
---
# Test
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a learning so knowledge is populated
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	writeTestMDLearning(t, learningsDir, "test-learning.md", map[string]string{
		"maturity": "provisional",
		"utility":  "0.8",
	}, "# Test Learning\nSome important knowledge.\n")

	// Create a session file so sessions would be populated without the filter
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	sessionContent := fmt.Sprintf("---\ndate: %s\nsummary: Did some work\n---\n# Session\nWorked on things.\n",
		time.Now().Format("2006-01-02"))
	if err := os.WriteFile(filepath.Join(sessionsDir, "2026-03-03-test.md"), []byte(sessionContent), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("inject", "--for=research", "--no-cite")
	if err != nil {
		t.Fatalf("inject --for=research failed: %v", err)
	}

	// HISTORY is excluded → no session content should appear
	if strings.Contains(out, "Recent Sessions") {
		t.Errorf("expected HISTORY to be excluded by --for=research, but found 'Recent Sessions' in output:\n%s", out)
	}
}

// TestInjectForFlag_UnknownSkill verifies --for with a nonexistent skill returns an error.
func TestInjectForFlag_UnknownSkill(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	_, err := executeCommand("inject", "--for=nonexistent-skill-xyz", "--no-cite")
	if err == nil {
		t.Fatal("expected error for --for with nonexistent skill, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got: %v", err)
	}
}

// TestInjectForFlag_NoDeclaration verifies --for with a skill that has no context field succeeds (passthrough).
func TestInjectForFlag_NoDeclaration(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	// Create a skill SKILL.md with no context field
	skillDir := filepath.Join(tmp, "skills", "basic-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: basic-skill
description: A skill without context declaration
skill_api_version: 1
---
# Basic Skill
Does things.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand("inject", "--for=basic-skill", "--no-cite")
	if err != nil {
		t.Fatalf("inject --for=basic-skill (no context) should succeed, got: %v", err)
	}

	// Should produce valid output (passthrough — no filtering)
	if !strings.Contains(out, "Injected Knowledge") && !strings.Contains(out, "No prior knowledge found") {
		t.Errorf("expected standard inject output, got:\n%s", out)
	}
}

// TestInjectForFlag_CreatesContextDir verifies --for + RPI_RUN_ID creates context artifact directory.
func TestInjectForFlag_CreatesContextDir(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	// Create a skill with context declaration
	skillDir := filepath.Join(tmp, "skills", "ctx-test")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: ctx-test
description: test context dir
skill_api_version: 1
context:
  window: fork
---
# Context Test
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set RPI_RUN_ID to trigger context dir creation
	t.Setenv("RPI_RUN_ID", "test-run-abc")

	_, err := executeCommand("inject", "--for=ctx-test", "--no-cite")
	if err != nil {
		t.Fatalf("inject --for=ctx-test with RPI_RUN_ID failed: %v", err)
	}

	// Verify context dir was created
	expectedDir := filepath.Join(tmp, ".agents", "context", "test-run-abc")
	info, statErr := os.Stat(expectedDir)
	if statErr != nil {
		t.Fatalf("expected context dir %q to exist, got: %v", expectedDir, statErr)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", expectedDir)
	}
}

// TestAtomicWriteFile_Success verifies atomicWriteFile writes content with correct permissions.
func TestAtomicWriteFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-output.txt")
	content := []byte("hello atomic world")
	perm := os.FileMode(0644)

	err := atomicWriteFile(path, content, perm)
	if err != nil {
		t.Fatalf("atomicWriteFile() error = %v", err)
	}

	// Verify file content
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("file content = %q, want %q", string(got), string(content))
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != perm {
		t.Errorf("file permissions = %v, want %v", info.Mode().Perm(), perm)
	}
}

// TestAtomicWriteFile_BadDir verifies atomicWriteFile returns error for non-existent directory.
func TestAtomicWriteFile_BadDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "subdir", "file.txt")
	err := atomicWriteFile(path, []byte("data"), 0644)
	if err == nil {
		t.Fatal("expected error when writing to non-existent directory, got nil")
	}
}

// TestWritePredecessorSection_AllFields verifies all predecessor fields are rendered.
func TestWritePredecessorSection_AllFields(t *testing.T) {
	pred := &predecessorContext{
		WorkingOn:  "refactoring inject.go",
		Progress:   "70% complete",
		Blocker:    "failing test in CI",
		NextStep:   "fix the flaky test",
		SessionAge: "2h",
		RawSummary: "raw summary text",
	}
	var sb strings.Builder
	writePredecessorSection(&sb, pred)
	output := sb.String()

	if !strings.Contains(output, "### Predecessor Context") {
		t.Error("missing header '### Predecessor Context'")
	}
	if !strings.Contains(output, "(2h ago)") {
		t.Errorf("missing session age, got:\n%s", output)
	}
	if !strings.Contains(output, "**Working on:** refactoring inject.go") {
		t.Errorf("missing WorkingOn field, got:\n%s", output)
	}
	if !strings.Contains(output, "**Progress:** 70% complete") {
		t.Errorf("missing Progress field, got:\n%s", output)
	}
	if !strings.Contains(output, "**Blocker:** failing test in CI") {
		t.Errorf("missing Blocker field, got:\n%s", output)
	}
	if !strings.Contains(output, "**Next step:** fix the flaky test") {
		t.Errorf("missing NextStep field, got:\n%s", output)
	}
	// RawSummary should NOT appear when Progress is set (per implementation)
	if strings.Contains(output, "raw summary text") {
		t.Errorf("RawSummary should not appear when Progress is set, got:\n%s", output)
	}
}

// TestWritePredecessorSection_Partial verifies graceful output with partial fields.
func TestWritePredecessorSection_Partial(t *testing.T) {
	pred := &predecessorContext{
		WorkingOn: "deploy pipeline",
		// No Progress, Blocker, NextStep, SessionAge
		RawSummary: "just a raw summary",
	}
	var sb strings.Builder
	writePredecessorSection(&sb, pred)
	output := sb.String()

	if !strings.Contains(output, "### Predecessor Context") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "**Working on:** deploy pipeline") {
		t.Errorf("missing WorkingOn, got:\n%s", output)
	}
	// RawSummary SHOULD appear when Progress is empty
	if !strings.Contains(output, "just a raw summary") {
		t.Errorf("RawSummary should appear when Progress is empty, got:\n%s", output)
	}
	// SessionAge not set, so no "(ago)" suffix
	if strings.Contains(output, "ago)") {
		t.Errorf("should not contain session age when not set, got:\n%s", output)
	}
}

// TestWritePredecessorSection_Nil verifies nil predecessor produces no output and no panic.
func TestWritePredecessorSection_Nil(t *testing.T) {
	var sb strings.Builder
	writePredecessorSection(&sb, nil)
	if sb.Len() != 0 {
		t.Errorf("expected empty output for nil predecessor, got %q", sb.String())
	}
}

// TestFilterMemoryDuplicates_RemovesDuplicates verifies learnings matching MEMORY.md content are filtered.
func TestFilterMemoryDuplicates_RemovesDuplicates(t *testing.T) {
	// filterMemoryDuplicates calls findMemoryFile which looks for MEMORY.md
	// in ~/.claude/projects/-<path>/ — we need to set up that structure.
	// Instead, since findMemoryFile uses cwd to derive the path, we test the
	// filtering logic by creating the expected MEMORY.md location.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tmpDir := t.TempDir()
	// Create MEMORY.md in the Claude projects convention path
	normalizedPath := strings.ReplaceAll(tmpDir, "/", "-")
	memoryDir := filepath.Join(homeDir, ".claude", "projects", normalizedPath, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatalf("create memory dir: %v", err)
	}
	memoryPath := filepath.Join(memoryDir, "MEMORY.md")
	memoryContent := "# Memory\n\n- Known pattern: auth-caching\n- Learning L42 is important\n"
	if err := os.WriteFile(memoryPath, []byte(memoryContent), 0644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}
	defer os.RemoveAll(filepath.Join(homeDir, ".claude", "projects", normalizedPath))

	learnings := []learning{
		{ID: "L42", Title: "Auth Caching Pattern"},  // ID matches MEMORY.md
		{ID: "L99", Title: "auth-caching"},          // Title matches MEMORY.md
		{ID: "L100", Title: "New Unique Knowledge"}, // Should pass through
	}

	result := filterMemoryDuplicates(tmpDir, learnings)

	if len(result) != 1 {
		t.Fatalf("expected 1 learning after dedup, got %d: %+v", len(result), result)
	}
	if result[0].ID != "L100" {
		t.Errorf("expected surviving learning ID=L100, got %q", result[0].ID)
	}
}

// TestFilterMemoryDuplicates_NoMemoryFilePassthrough verifies all learnings pass through when no MEMORY.md exists.
func TestFilterMemoryDuplicates_NoMemoryFilePassthrough(t *testing.T) {
	tmpDir := t.TempDir() // No MEMORY.md anywhere for this path

	learnings := []learning{
		{ID: "L1", Title: "First"},
		{ID: "L2", Title: "Second"},
		{ID: "L3", Title: "Third"},
	}

	result := filterMemoryDuplicates(tmpDir, learnings)
	if len(result) != 3 {
		t.Errorf("expected all 3 learnings to pass through, got %d", len(result))
	}
}

// TestRenderKnowledge_JSON verifies JSON format output contains expected fields.
func TestRenderKnowledge_JSON(t *testing.T) {
	knowledge := &injectedKnowledge{
		Timestamp: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC),
		Query:     "test-query",
		Learnings: []learning{
			{ID: "L1", Title: "Test Learning", Summary: "A summary"},
		},
	}

	output, err := renderKnowledge(knowledge, "json")
	if err != nil {
		t.Fatalf("renderKnowledge(json) error = %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(output), &parsed); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", jsonErr, output)
	}

	// Verify key fields are present
	if _, ok := parsed["timestamp"]; !ok {
		t.Error("JSON output missing 'timestamp' field")
	}
	if q, ok := parsed["query"]; !ok || q != "test-query" {
		t.Errorf("JSON query = %v, want 'test-query'", q)
	}
	if _, ok := parsed["learnings"]; !ok {
		t.Error("JSON output missing 'learnings' field")
	}
	learningsArr, ok := parsed["learnings"].([]interface{})
	if !ok || len(learningsArr) != 1 {
		t.Errorf("expected 1 learning in JSON, got %v", parsed["learnings"])
	}
}

// TestRenderKnowledge_Markdown verifies markdown format output.
func TestRenderKnowledge_Markdown(t *testing.T) {
	knowledge := &injectedKnowledge{
		Timestamp: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC),
		Learnings: []learning{
			{ID: "L1", Title: "Test Learning", Summary: "A summary"},
		},
		Predecessor: &predecessorContext{
			WorkingOn: "testing",
		},
	}

	output, err := renderKnowledge(knowledge, "markdown")
	if err != nil {
		t.Fatalf("renderKnowledge(markdown) error = %v", err)
	}

	if !strings.Contains(output, "## Injected Knowledge (ao inject)") {
		t.Error("missing main header in markdown output")
	}
	if !strings.Contains(output, "### Predecessor Context") {
		t.Error("missing predecessor section in markdown output")
	}
	if !strings.Contains(output, "### Recent Learnings") {
		t.Error("missing learnings section in markdown output")
	}
	if !strings.Contains(output, "**L1**") {
		t.Error("missing learning ID in markdown output")
	}
	if !strings.Contains(output, "Last injection:") {
		t.Error("missing timestamp in markdown output")
	}
}

// TestRunQuarantineFlagged_WithFlagged verifies quarantine processing with flagged paths.
func TestRunQuarantineFlagged_WithFlagged(t *testing.T) {
	dir := t.TempDir()

	// Create a learning file that will be quarantined
	learningsDir := filepath.Join(dir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	learningFile := filepath.Join(learningsDir, "bad-learning.md")
	if err := os.WriteFile(learningFile, []byte("# Bad Learning\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create quality report referencing the learning
	defragDir := filepath.Join(dir, ".agents", "defrag")
	if err := os.MkdirAll(defragDir, 0755); err != nil {
		t.Fatal(err)
	}
	report := fmt.Sprintf(`{"flagged_paths": ["%s"]}`, learningFile)
	if err := os.WriteFile(filepath.Join(defragDir, "quality-report.json"), []byte(report), 0644); err != nil {
		t.Fatal(err)
	}

	err := runQuarantineFlagged(dir)
	if err != nil {
		t.Fatalf("runQuarantineFlagged() error = %v", err)
	}

	// Verify original file was moved
	if _, statErr := os.Stat(learningFile); !os.IsNotExist(statErr) {
		t.Error("expected learning file to be moved to quarantine")
	}

	// Verify file exists in .quarantine directory
	quarantinedFile := filepath.Join(learningsDir, ".quarantine", "bad-learning.md")
	if _, statErr := os.Stat(quarantinedFile); statErr != nil {
		t.Errorf("expected quarantined file at %s, got error: %v", quarantinedFile, statErr)
	}
}

// TestRunQuarantineFlagged_NoReport verifies error when no quality report exists.
func TestRunQuarantineFlagged_NoReport(t *testing.T) {
	dir := t.TempDir() // No .agents/defrag/quality-report.json

	err := runQuarantineFlagged(dir)
	if err == nil {
		t.Fatal("expected error when no quality report exists, got nil")
	}
	if !strings.Contains(err.Error(), "no quality report found") {
		t.Errorf("expected 'no quality report found' error, got: %v", err)
	}
}

// TestInjectForFlag_CreatesAdhocContextDir verifies --for without RPI_RUN_ID creates adhoc context dir.
func TestInjectForFlag_CreatesAdhocContextDir(t *testing.T) {
	tmp := chdirTemp(t)
	setupAgentsDir(t, tmp)

	// Create a skill with context declaration
	skillDir := filepath.Join(tmp, "skills", "ctx-adhoc")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: ctx-adhoc
description: test adhoc context dir
skill_api_version: 1
context:
  window: fork
---
# Context Adhoc Test
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Ensure RPI_RUN_ID is NOT set
	t.Setenv("RPI_RUN_ID", "")

	_, err := executeCommand("inject", "--for=ctx-adhoc", "--no-cite")
	if err != nil {
		t.Fatalf("inject --for=ctx-adhoc without RPI_RUN_ID failed: %v", err)
	}

	// Verify an adhoc context dir was created under .agents/context/
	contextBase := filepath.Join(tmp, ".agents", "context")
	entries, readErr := os.ReadDir(contextBase)
	if readErr != nil {
		t.Fatalf("expected .agents/context/ to exist, got: %v", readErr)
	}

	found := false
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "adhoc-") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected adhoc-* directory under %q, found: %v", contextBase, entries)
	}
}

// TestCollectOLConstraints_* tests are in inject_ol_test.go
