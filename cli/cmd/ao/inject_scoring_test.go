package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/search"
)

func TestMaturityWeight_KnownLevels(t *testing.T) {
	tests := []struct {
		maturity string
		want     float64
	}{
		{"established", 1.3},
		{"candidate", 1.1},
		{"provisional", 1.0},
		{"anti-pattern", 0.4},
	}
	for _, tt := range tests {
		got := maturityWeight(tt.maturity)
		if got != tt.want {
			t.Errorf("maturityWeight(%q) = %v, want %v", tt.maturity, got, tt.want)
		}
	}
}

func TestMaturityWeight_Unknown(t *testing.T) {
	tests := []string{"", "unknown", "experimental"}
	for _, m := range tests {
		got := maturityWeight(m)
		if got != 1.0 {
			t.Errorf("maturityWeight(%q) = %v, want 1.0", m, got)
		}
	}
}

func TestApplyCompositeScoring_WithMaturity(t *testing.T) {
	// Two learnings with identical freshness/utility but different maturity.
	// Established should outrank provisional.
	established := &learning{
		FreshnessScore: 0.9,
		Utility:        0.7,
		Maturity:       "established",
	}
	provisional := &learning{
		FreshnessScore: 0.9,
		Utility:        0.7,
		Maturity:       "provisional",
	}
	antiPattern := &learning{
		FreshnessScore: 0.9,
		Utility:        0.7,
		Maturity:       "anti-pattern",
	}

	items := []scorable{established, provisional, antiPattern}
	applyCompositeScoringTo(items, 0.5)

	if established.CompositeScore <= provisional.CompositeScore {
		t.Errorf("established (%v) should outrank provisional (%v)",
			established.CompositeScore, provisional.CompositeScore)
	}
	if provisional.CompositeScore <= antiPattern.CompositeScore {
		t.Errorf("provisional (%v) should outrank anti-pattern (%v)",
			provisional.CompositeScore, antiPattern.CompositeScore)
	}
}

func TestKnowledgeFindingScorableInterface(t *testing.T) {
	f := &knowledgeFinding{
		FreshnessScore: 0.75,
		Utility:        0.6,
	}
	if got := f.GetFreshness(); got != 0.75 {
		t.Errorf("GetFreshness() = %v, want 0.75", got)
	}
	if got := f.GetUtility(); got != 0.6 {
		t.Errorf("GetUtility() = %v, want 0.6", got)
	}
	if got := f.GetMaturity(); got != "" {
		t.Errorf("GetMaturity() = %q, want empty", got)
	}
	f.SetComposite(1.5)
	if f.CompositeScore != 1.5 {
		t.Errorf("SetComposite(1.5) -> CompositeScore = %v, want 1.5", f.CompositeScore)
	}
}

func TestApplyCompositeScoring_NoMaturity(t *testing.T) {
	// Patterns don't have maturity — should get weight 1.0 (no change).
	p1 := &pattern{FreshnessScore: 0.8, Utility: 0.6}
	p2 := &pattern{FreshnessScore: 0.5, Utility: 0.4}
	p3 := &pattern{FreshnessScore: 0.3, Utility: 0.2}

	items := []scorable{p1, p2, p3}
	applyCompositeScoringTo(items, 0.5)

	// Fresher + higher utility should rank higher
	if p1.CompositeScore <= p2.CompositeScore {
		t.Errorf("p1 (%v) should outrank p2 (%v)", p1.CompositeScore, p2.CompositeScore)
	}
	if p2.CompositeScore <= p3.CompositeScore {
		t.Errorf("p2 (%v) should outrank p3 (%v)", p2.CompositeScore, p3.CompositeScore)
	}
}

func TestApplyCompositeScoring_SmallPool(t *testing.T) {
	// n < 3 triggers the raw-value fallback: score = freshness + λ × utility
	// (z-normalization is skipped). Verify correct ordering is preserved.
	high := &learning{
		FreshnessScore: 0.9,
		Utility:        0.8,
	}
	low := &learning{
		FreshnessScore: 0.3,
		Utility:        0.2,
	}

	items := []scorable{high, low}
	applyCompositeScoringTo(items, 0.5)

	if high.CompositeScore <= low.CompositeScore {
		t.Errorf("high-scored learning (%v) should outrank low-scored learning (%v)",
			high.CompositeScore, low.CompositeScore)
	}
}

func TestWeightedSectionScore_EmptyTokens(t *testing.T) {
	score := weightedSectionScore(nil, "Heading", "Some content", 0, 1)
	if score != 1.0 {
		t.Errorf("weightedSectionScore(nil tokens) = %v, want 1.0", score)
	}
}

func TestWeightedSectionScore_SingleToken(t *testing.T) {
	tokens := []string{"authentication"}
	// Heading contains the token — should get high heading + substring + exact scores
	score := weightedSectionScore(tokens, "Authentication Setup", "Configure authentication for the service", 0, 3)
	if score < 0.5 {
		t.Errorf("weightedSectionScore with matching heading = %v, want >= 0.5", score)
	}
}

func TestWeightedSectionScore_HeadingBonus(t *testing.T) {
	tokens := []string{"auth", "token"}
	// Token in heading should score higher than same tokens only in content.
	// Use identical content so the only difference is heading match.
	content := "The auth token is configured here for the service"
	headingScore := weightedSectionScore(tokens, "Auth Token Management", content, 0, 3)
	noHeadingScore := weightedSectionScore(tokens, "General Configuration", content, 0, 3)
	if headingScore <= noHeadingScore {
		t.Errorf("heading match (%v) should outrank no-heading match (%v)", headingScore, noHeadingScore)
	}
}

func TestWeightedSectionScore_AdjacencyBonus(t *testing.T) {
	tokens := []string{"database", "migration"}
	// Adjacent tokens should score higher than tokens far apart
	adjacentScore := weightedSectionScore(tokens, "Overview", "Run the database migration script to update", 0, 1)
	distantScore := weightedSectionScore(tokens, "Overview", "The database is large and after many steps we run migration", 0, 1)
	if adjacentScore <= distantScore {
		t.Errorf("adjacent tokens (%v) should outrank distant tokens (%v)", adjacentScore, distantScore)
	}
}

func TestWeightedSectionScore_SectionProximity(t *testing.T) {
	tokens := []string{"overview"}
	// First section should get proximity bonus over last section
	firstScore := weightedSectionScore(tokens, "Overview", "This is the overview of the system", 0, 5)
	lastScore := weightedSectionScore(tokens, "Overview", "This is the overview of the system", 4, 5)
	if firstScore <= lastScore {
		t.Errorf("first section (%v) should outrank last section (%v)", firstScore, lastScore)
	}
}

func TestWeightedSectionScore_NoMatch(t *testing.T) {
	tokens := []string{"kubernetes", "deployment"}
	score := weightedSectionScore(tokens, "Cooking Recipes", "How to make pasta and bread", 0, 1)
	// Only proximity contributes (0.10 * 1.0 = 0.10)
	if score > 0.15 {
		t.Errorf("non-matching content score = %v, want <= 0.15", score)
	}
}

func TestWeightedSectionScore_BoundedOutput(t *testing.T) {
	tokens := []string{"test"}
	score := weightedSectionScore(tokens, "Test", "test test test test test", 0, 1)
	if score > 1.0 {
		t.Errorf("score = %v, must be <= 1.0", score)
	}
	if score < 0.0 {
		t.Errorf("score = %v, must be >= 0.0", score)
	}
}

func TestTokenizeWords(t *testing.T) {
	tests := []struct {
		input string
		want  int // minimum number of words
	}{
		{"hello world", 2},
		{"it's a test!", 2}, // "it" is dropped (< 2 chars), "a" dropped
		{"", 0},
		{"word1, word2; word3.", 3},
	}
	for _, tt := range tests {
		got := tokenizeWords(tt.input)
		if len(got) < tt.want {
			t.Errorf("tokenizeWords(%q) = %d words, want >= %d", tt.input, len(got), tt.want)
		}
	}
}

func TestComputeAdjacency_AdjacentTokens(t *testing.T) {
	tokens := []string{"hello", "world"}
	words := []string{"the", "hello", "world", "is", "great"}
	score := computeAdjacency(tokens, words)
	if score < 0.9 {
		t.Errorf("adjacent tokens score = %v, want >= 0.9", score)
	}
}

func TestComputeAdjacency_DistantTokens(t *testing.T) {
	tokens := []string{"hello", "world"}
	words := []string{"hello", "a", "b", "c", "d", "e", "f", "g", "h", "i", "world"}
	score := computeAdjacency(tokens, words)
	adjacentScore := computeAdjacency(tokens, []string{"hello", "world"})
	if score >= adjacentScore {
		t.Errorf("distant score (%v) should be less than adjacent score (%v)", score, adjacentScore)
	}
}

func TestComputeAdjacency_SingleToken(t *testing.T) {
	// Single token present → 1.0
	score := computeAdjacency([]string{"hello"}, []string{"the", "hello", "world"})
	if score != 1.0 {
		t.Errorf("single present token adjacency = %v, want 1.0", score)
	}
	// Single token absent → 0.0
	score = computeAdjacency([]string{"missing"}, []string{"the", "hello", "world"})
	if score != 0.0 {
		t.Errorf("single absent token adjacency = %v, want 0.0", score)
	}
}

func TestComputeAdjacency_NoTokens(t *testing.T) {
	score := computeAdjacency(nil, []string{"hello", "world"})
	if score != 0.0 {
		t.Errorf("no tokens adjacency = %v, want 0.0", score)
	}
}

func TestApplyCompositeScoring_GlobalWeightPenalty(t *testing.T) {
	// NOTE: The global weight penalty is NOT applied in applyCompositeScoringTo.
	// It is applied upstream in collectLearnings (inject_learnings.go:87-98).
	// This test verifies that applyCompositeScoringTo ignores the Global field
	// entirely — two learnings with identical freshness/utility/maturity must
	// receive equal CompositeScores regardless of their Global value.
	nonGlobal := &learning{
		FreshnessScore: 0.7,
		Utility:        0.5,
		Maturity:       "provisional",
		Global:         false,
	}
	global := &learning{
		FreshnessScore: 0.7,
		Utility:        0.5,
		Maturity:       "provisional",
		Global:         true,
	}

	items := []scorable{nonGlobal, global}
	applyCompositeScoringTo(items, 0.5)

	if nonGlobal.CompositeScore != global.CompositeScore {
		t.Errorf("applyCompositeScoringTo should ignore Global field: nonGlobal=%v global=%v",
			nonGlobal.CompositeScore, global.CompositeScore)
	}
}

// ---------------------------------------------------------------------------
// TestInjectScoring_ContentHashDedup
// ---------------------------------------------------------------------------

func TestInjectScoring_ContentHashDedup(t *testing.T) {
	sharedContent := "This is identical content for deduplication testing purposes. It has more than 50 chars."

	itemA := &learning{
		Title:          "Title A",
		FreshnessScore: 0.9,
		Utility:        0.8,
		BodyText:       sharedContent,
	}
	itemB := &learning{
		Title:          "Title B — different title, same content",
		FreshnessScore: 0.7,
		Utility:        0.6,
		BodyText:       sharedContent,
	}
	itemC := &learning{
		Title:          "Title C — unique content",
		FreshnessScore: 0.5,
		Utility:        0.4,
		BodyText:       "Completely different content that is unique to this item.",
	}

	items := []search.Scorable{itemA, itemB, itemC}
	result := deduplicateByContentHash(items, func(s search.Scorable) string {
		if l, ok := s.(*learning); ok {
			return l.BodyText
		}
		return ""
	})

	// itemA and itemB share content — only itemA (first) should survive.
	// itemC has unique content and should survive.
	if len(result) != 2 {
		t.Errorf("expected 2 items after dedup, got %d", len(result))
	}
	if result[0] != itemA {
		t.Errorf("first surviving item should be itemA (first seen with that content hash)")
	}
	if result[1] != itemC {
		t.Errorf("second surviving item should be itemC (unique content)")
	}
}

func TestInjectScoring_ContentHashDedup_EmptyContentNotDeduped(t *testing.T) {
	// Items with empty content should never be de-duped against each other.
	itemA := &learning{Title: "A", BodyText: ""}
	itemB := &learning{Title: "B", BodyText: ""}

	items := []search.Scorable{itemA, itemB}
	result := deduplicateByContentHash(items, func(s search.Scorable) string {
		if l, ok := s.(*learning); ok {
			return l.BodyText
		}
		return ""
	})

	if len(result) != 2 {
		t.Errorf("empty-content items should both survive: got %d items", len(result))
	}
}

// ---------------------------------------------------------------------------
// TestInjectScoring_IndexMdBoost
// ---------------------------------------------------------------------------

func TestInjectScoring_IndexMdBoost(t *testing.T) {
	dir := t.TempDir()

	// Create .agents/INDEX.md with a wikilink to learnings/test-a.
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("mkdir .agents: %v", err)
	}
	indexContent := `# Knowledge Index

## Learnings
- [[learnings/test-a]] — important learning
- [[patterns/core-pattern]] — core pattern

## Other
Some other content.
`
	if err := os.WriteFile(filepath.Join(agentsDir, "INDEX.md"), []byte(indexContent), 0600); err != nil {
		t.Fatalf("write INDEX.md: %v", err)
	}

	// Parse INDEX.md wikilinks.
	links := indexMdWikilinks(dir)
	if len(links) == 0 {
		t.Fatal("expected wikilinks from INDEX.md, got none")
	}
	if !links["learnings/test-a"] {
		t.Errorf("expected link 'learnings/test-a' in parsed links, got: %v", links)
	}

	// Build two learnings: test-a (linked) and test-b (not linked).
	baseScore := 1.0
	learningA := learning{
		Title:          "Test A",
		Source:         filepath.Join(dir, ".agents", "learnings", "test-a.md"),
		CompositeScore: baseScore,
	}
	learningB := learning{
		Title:          "Test B",
		Source:         filepath.Join(dir, ".agents", "learnings", "test-b.md"),
		CompositeScore: baseScore,
	}

	learnings := []learning{learningA, learningB}
	applyIndexMdBoostToLearnings(learnings, links, 1.5)

	if learnings[0].CompositeScore <= learnings[1].CompositeScore {
		t.Errorf("test-a (INDEX.md linked) score %v should exceed test-b (unlinked) score %v",
			learnings[0].CompositeScore, learnings[1].CompositeScore)
	}

	expectedBoosted := baseScore * 1.5
	if learnings[0].CompositeScore != expectedBoosted {
		t.Errorf("test-a CompositeScore = %v, want %v (1.5x boost)", learnings[0].CompositeScore, expectedBoosted)
	}
	if learnings[1].CompositeScore != baseScore {
		t.Errorf("test-b CompositeScore = %v, want %v (no boost)", learnings[1].CompositeScore, baseScore)
	}
}

func TestInjectScoring_IndexMdBoost_MissingFile(t *testing.T) {
	// When INDEX.md doesn't exist, indexMdWikilinks returns nil and
	// applyIndexMdBoostToLearnings is a no-op.
	links := indexMdWikilinks("/nonexistent/path")
	if len(links) != 0 {
		t.Errorf("expected empty links for missing INDEX.md, got %v", links)
	}

	l := learning{Title: "X", CompositeScore: 1.0}
	learnings := []learning{l}
	applyIndexMdBoostToLearnings(learnings, links, 1.5)

	if learnings[0].CompositeScore != 1.0 {
		t.Errorf("score should be unchanged with no links, got %v", learnings[0].CompositeScore)
	}
}
