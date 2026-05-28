// practices: [ai-assisted-dev, distributed-tracing]
package main

import (
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func TestRerankContextBundleForPhase_PrefersWidelyReusedLearning(t *testing.T) {
	tmp := t.TempDir()
	if err := writeCitations(tmp, []types.CitationEvent{
		{ArtifactPath: tmp + "/.agents/learnings/reused.md", WorkspacePath: tmp, SessionID: "s1", CitedAt: time.Now().Add(-2 * time.Hour), CitationType: "applied", FeedbackGiven: true, FeedbackReward: 1},
		{ArtifactPath: tmp + "/.agents/learnings/reused.md", WorkspacePath: tmp, SessionID: "s2", CitedAt: time.Now().Add(-1 * time.Hour), CitationType: "reference"},
	}); err != nil {
		t.Fatal(err)
	}

	bundle := rankedContextBundle{
		CWD:   tmp,
		Query: "auth startup",
		Learnings: []learning{
			{ID: "L-fresh", Title: "Fresh auth note", Summary: "Recent auth startup note", Source: tmp + "/.agents/learnings/fresh.md", AgeWeeks: 1, CompositeScore: 0.9},
			{ID: "L-reused", Title: "Reused auth fix", Summary: "Auth startup fix used before", Source: tmp + "/.agents/learnings/reused.md", AgeWeeks: 3, CompositeScore: 0.6},
		},
	}

	ranked := rerankContextBundleForPhase(tmp, "auth startup", "startup", bundle)
	if got, want := ranked.Learnings[0].ID, "L-reused"; got != want {
		t.Fatalf("first learning = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// phaseFitWeight
// ---------------------------------------------------------------------------

func TestPhaseFitWeight_KnownPhases(t *testing.T) {
	tests := []struct {
		class string
		phase string
		want  int
	}{
		{"learning", "startup", 6},
		{"learning", "planning", 6},
		{"learning", "pre-mortem", -2},
		{"learning", "validation", 6},
		{"learning", "unknown-phase", 4},
		{"pattern", "startup", 6},
		{"finding", "pre-mortem", 6},
		{"recent-session", "startup", 6},
		{"next-work", "planning", 6},
		{"research", "planning", 6},
		{"discovery-notes", "startup", -2},
	}
	for _, tt := range tests {
		t.Run(tt.class+"_"+tt.phase, func(t *testing.T) {
			got := phaseFitWeight(tt.class, tt.phase)
			if got != tt.want {
				t.Errorf("phaseFitWeight(%q, %q) = %d, want %d", tt.class, tt.phase, got, tt.want)
			}
		})
	}
}

func TestPhaseFitWeight_UnknownClass(t *testing.T) {
	got := phaseFitWeight("nonexistent-class", "startup")
	if got != 0 {
		t.Errorf("phaseFitWeight(unknown class) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// modTimeWeight
// ---------------------------------------------------------------------------

func TestModTimeWeight(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"recent (1 day old)", now.Add(-24 * time.Hour).Format(time.RFC3339), 3},
		{"medium (15 days old)", now.Add(-15 * 24 * time.Hour).Format(time.RFC3339), 1},
		{"old (60 days old)", now.Add(-60 * 24 * time.Hour).Format(time.RFC3339), 0},
		{"empty string", "", 0},
		{"whitespace only", "   ", 0},
		{"invalid format", "not-a-date", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modTimeWeight(tt.raw)
			if got != tt.want {
				t.Errorf("modTimeWeight(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// scorePatternForRuntime
// ---------------------------------------------------------------------------

func TestScorePatternForRuntime(t *testing.T) {
	ctx := runtimeRelevanceContext{
		cwd:     "/tmp/repo",
		repo:    "testrepo",
		phase:   "startup",
		needles: []string{"auth", "login"},
	}

	p := pattern{
		Name:           "Auth retry pattern",
		Description:    "Handles auth login retries",
		FilePath:       "/tmp/repo/.agents/patterns/auth-retry.md",
		AgeWeeks:       1,
		CompositeScore: 0.8,
	}

	score := scorePatternForRuntime(ctx, p)
	// Score must include trustTierWeight + phaseFitWeight + lexical + repo + freshness + composite
	// All components should be positive for this matching case
	if score <= 0 {
		t.Errorf("scorePatternForRuntime returned %d, expected positive for matching pattern", score)
	}

	// A non-matching pattern should score lower
	noMatch := pattern{
		Name:           "Database migration",
		Description:    "Schema migration steps",
		FilePath:       "/other/repo/.agents/patterns/db.md",
		AgeWeeks:       52,
		CompositeScore: 0.1,
	}
	noMatchScore := scorePatternForRuntime(ctx, noMatch)
	if noMatchScore >= score {
		t.Errorf("non-matching pattern scored %d >= matching pattern %d", noMatchScore, score)
	}
}

// ---------------------------------------------------------------------------
// scoreFindingForRuntime
// ---------------------------------------------------------------------------

func TestScoreFindingForRuntime(t *testing.T) {
	ctx := runtimeRelevanceContext{
		cwd:     "/tmp/repo",
		repo:    "testrepo",
		phase:   "pre-mortem",
		needles: []string{"security", "auth"},
		scorecard: stigmergicScorecard{
			PromotedFindings: 5,
		},
	}

	finding := knowledgeFinding{
		ID:             "F-001",
		Title:          "Security auth bypass",
		Summary:        "Auth token validation skipped",
		Source:         "/tmp/repo/.agents/findings/F-001.md",
		SourceSkill:    "vibe",
		ScopeTags:      []string{"security"},
		ApplicableWhen: []string{"auth handling"},
		AgeWeeks:       2,
		CompositeScore: 0.7,
		Status:         "active",
	}
	appliedIDs := []string{"F-001"}

	score := scoreFindingForRuntime(ctx, finding, appliedIDs)
	if score <= 0 {
		t.Errorf("scoreFindingForRuntime returned %d, expected positive for matching finding", score)
	}

	// Same finding without being in appliedIDs should score lower
	scoreNotApplied := scoreFindingForRuntime(ctx, finding, nil)
	if scoreNotApplied >= score {
		t.Errorf("non-applied finding scored %d >= applied finding %d", scoreNotApplied, score)
	}
}

// ---------------------------------------------------------------------------
// scoreNextWorkForRuntime
// ---------------------------------------------------------------------------

func TestScoreNextWorkForRuntime(t *testing.T) {
	ctx := runtimeRelevanceContext{
		cwd:     "/tmp/repo",
		repo:    "testrepo",
		phase:   "planning",
		needles: []string{"refactor", "complexity"},
	}

	available := nextWorkItem{
		Title:       "Refactor complexity hotspot",
		Description: "Reduce cyclomatic complexity in handler",
		Evidence:    "CC=25 in handler.go",
		Source:      "/tmp/repo/.agents/work/item1.md",
		Severity:    "high",
		ClaimStatus: "available",
	}

	claimed := nextWorkItem{
		Title:       "Refactor complexity hotspot",
		Description: "Reduce cyclomatic complexity in handler",
		Evidence:    "CC=25 in handler.go",
		Source:      "/tmp/repo/.agents/work/item1.md",
		Severity:    "high",
		ClaimStatus: "claimed",
	}

	scoreAvail := scoreNextWorkForRuntime(ctx, available)
	scoreClaimed := scoreNextWorkForRuntime(ctx, claimed)

	if scoreAvail <= scoreClaimed {
		t.Errorf("available item scored %d <= claimed item %d, expected available to score higher", scoreAvail, scoreClaimed)
	}
}

// ---------------------------------------------------------------------------
// scoreResearchForRuntime
// ---------------------------------------------------------------------------

func TestScoreResearchForRuntime(t *testing.T) {
	now := time.Now()
	ctx := runtimeRelevanceContext{
		cwd:     "/tmp/repo",
		repo:    "testrepo",
		phase:   "planning",
		needles: []string{"helm", "chart"},
	}

	recent := codexArtifactRef{
		Title:      "Helm chart analysis",
		Path:       "/tmp/repo/.agents/research/helm.md",
		ModifiedAt: now.Add(-1 * 24 * time.Hour).Format(time.RFC3339),
	}

	old := codexArtifactRef{
		Title:      "Unrelated old research",
		Path:       "/other/repo/.agents/research/old.md",
		ModifiedAt: now.Add(-90 * 24 * time.Hour).Format(time.RFC3339),
	}

	recentScore := scoreResearchForRuntime(ctx, recent)
	oldScore := scoreResearchForRuntime(ctx, old)

	if recentScore <= oldScore {
		t.Errorf("recent matching research scored %d <= old non-matching %d", recentScore, oldScore)
	}
}

// ---------------------------------------------------------------------------
// freshnessWeight
// ---------------------------------------------------------------------------

func TestFreshnessWeight(t *testing.T) {
	tests := []struct {
		ageWeeks float64
		want     int
	}{
		{0.5, 4},
		{1.0, 4},
		{2.0, 3},
		{4.0, 3},
		{8.0, 1},
		{12.0, 1},
		{13.0, 0},
		{52.0, 0},
	}
	for _, tt := range tests {
		got := freshnessWeight(tt.ageWeeks)
		if got != tt.want {
			t.Errorf("freshnessWeight(%v) = %d, want %d", tt.ageWeeks, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// compositeWeight
// ---------------------------------------------------------------------------

func TestCompositeWeight(t *testing.T) {
	tests := []struct {
		score float64
		want  int
	}{
		{0.0, 0},
		{0.5, 3},
		{1.0, 6},
		{0.75, 5},
	}
	for _, tt := range tests {
		got := compositeWeight(tt.score)
		if got != tt.want {
			t.Errorf("compositeWeight(%v) = %d, want %d", tt.score, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// cassHitWeight
// ---------------------------------------------------------------------------

func TestCassHitWeight(t *testing.T) {
	t.Run("nil map returns 0", func(t *testing.T) {
		got := cassHitWeight(nil, "some/path")
		if got != 0 {
			t.Errorf("cassHitWeight(nil, ...) = %d, want 0", got)
		}
	})

	t.Run("empty path returns 0", func(t *testing.T) {
		hits := map[string]float64{"key": 5.0}
		got := cassHitWeight(hits, "")
		if got != 0 {
			t.Errorf("cassHitWeight(hits, \"\") = %d, want 0", got)
		}
	})

	t.Run("matching path returns scaled score", func(t *testing.T) {
		key := canonicalArtifactKey("", "sessions/test.jsonl")
		hits := map[string]float64{key: 2.5}
		got := cassHitWeight(hits, "sessions/test.jsonl")
		// 2.5 * 4 = 10
		if got != 10 {
			t.Errorf("cassHitWeight = %d, want 10", got)
		}
	})
}

// ---------------------------------------------------------------------------
// usageSignalWeight
// ---------------------------------------------------------------------------

func TestUsageSignalWeight(t *testing.T) {
	t.Run("zero signal", func(t *testing.T) {
		got := usageSignalWeight(artifactUsageSignal{})
		if got != 0 {
			t.Errorf("usageSignalWeight(zero) = %d, want 0", got)
		}
	})

	t.Run("positive signal", func(t *testing.T) {
		sig := artifactUsageSignal{
			UniqueSessions:   3,
			UniqueWorkspaces: 2,
			AppliedCount:     1,
			ReferenceCount:   1,
			FeedbackCount:    1,
			MeanReward:       1.0,
		}
		got := usageSignalWeight(sig)
		// 3*2 + 2*2 + 1*2 + 1 + round(1.0*3) = 6+4+2+1+3 = 16
		if got != 16 {
			t.Errorf("usageSignalWeight = %d, want 16", got)
		}
	})
}

// ---------------------------------------------------------------------------
// lexicalSignalWeight
// ---------------------------------------------------------------------------

func TestLexicalSignalWeight_EmptyNeedles(t *testing.T) {
	got := lexicalSignalWeight(nil, "some", "text")
	if got != 0 {
		t.Errorf("lexicalSignalWeight(nil needles) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// repoPathWeight
// ---------------------------------------------------------------------------

func TestRepoPathWeight(t *testing.T) {
	t.Run("empty path returns 0", func(t *testing.T) {
		got := repoPathWeight("/tmp/repo", "")
		if got != 0 {
			t.Errorf("repoPathWeight(\"/tmp/repo\", \"\") = %d, want 0", got)
		}
	})
}

// ---------------------------------------------------------------------------
// runtimeCassHitScores
// ---------------------------------------------------------------------------

func TestRuntimeCassHitScores_EmptyQuery(t *testing.T) {
	got := runtimeCassHitScores("/tmp", "", 8, nil)
	if got != nil {
		t.Errorf("runtimeCassHitScores with empty query = %v, want nil", got)
	}
}

func TestRuntimeCassHitScores_ZeroLimit(t *testing.T) {
	got := runtimeCassHitScores("/tmp", "query", 0, nil)
	if got != nil {
		t.Errorf("runtimeCassHitScores with zero limit = %v, want nil", got)
	}
}

func TestRuntimeCassHitScores_WithMockSearch(t *testing.T) {
	mockSearch := func(query string, limit int) ([]searchResult, error) {
		return []searchResult{
			{Path: "/tmp/repo/.agents/ao/sessions/matched.jsonl", Score: 9},
		}, nil
	}
	got := runtimeCassHitScores("/tmp/repo", "startup context", 8, mockSearch)
	if got == nil {
		t.Fatal("runtimeCassHitScores returned nil, want scores map")
	}
	key := canonicalArtifactKey("/tmp/repo", "/tmp/repo/.agents/ao/sessions/matched.jsonl")
	if score, ok := got[key]; !ok || score != 9 {
		t.Errorf("expected score 9 for key %q, got %v (ok=%v)", key, score, ok)
	}
}
