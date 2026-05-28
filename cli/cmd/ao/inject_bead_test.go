// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBeadContext_EnvVars(t *testing.T) {
	t.Setenv("HOOK_BEAD_TITLE", "Fix auth token refresh")
	t.Setenv("HOOK_BEAD_LABELS", "auth, debugging, P1")
	t.Setenv("HOOK_BEAD_PHASE", "implement")

	ctx := resolveBeadContext("ag-7abc", t.TempDir())
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.ID != "ag-7abc" {
		t.Errorf("ID = %q, want %q", ctx.ID, "ag-7abc")
	}
	if ctx.Title != "Fix auth token refresh" {
		t.Errorf("Title = %q, want %q", ctx.Title, "Fix auth token refresh")
	}
	if len(ctx.Labels) != 3 {
		t.Errorf("Labels count = %d, want 3", len(ctx.Labels))
	}
	if ctx.Phase != "implement" {
		t.Errorf("Phase = %q, want %q", ctx.Phase, "implement")
	}
	if len(ctx.Keywords) == 0 {
		t.Error("expected keywords to be populated")
	}
}

func TestResolveBeadContext_EmptyBeadID(t *testing.T) {
	ctx := resolveBeadContext("", t.TempDir())
	if ctx != nil {
		t.Error("expected nil context for empty bead ID")
	}
}

func TestResolveBeadContext_CacheFile(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cache := BeadContext{
		ID:     "ag-9xyz",
		Title:  "Refactor pool scoring",
		Labels: []string{"refactoring", "pool"},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(filepath.Join(aoDir, beadContextCacheFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Clear env vars
	t.Setenv("HOOK_BEAD_TITLE", "")
	t.Setenv("HOOK_BEAD_LABELS", "")

	ctx := resolveBeadContext("ag-9xyz", tmp)
	if ctx == nil {
		t.Fatal("expected non-nil context from cache")
	}
	if ctx.Title != "Refactor pool scoring" {
		t.Errorf("Title = %q, want %q", ctx.Title, "Refactor pool scoring")
	}
	if len(ctx.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(ctx.Labels))
	}
}

func TestResolveBeadContext_CacheMismatch(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cache := BeadContext{ID: "ag-OTHER", Title: "Wrong bead"}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(filepath.Join(aoDir, beadContextCacheFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOOK_BEAD_TITLE", "")
	t.Setenv("HOOK_BEAD_LABELS", "")

	ctx := resolveBeadContext("ag-9xyz", tmp)
	if ctx == nil {
		t.Fatal("expected non-nil context (minimal fallback)")
	}
	// Should not get the cached title since bead IDs don't match
	if ctx.Title != "" {
		t.Errorf("Title = %q, want empty (cache mismatch)", ctx.Title)
	}
}

func TestResolveBeadContext_MinimalFallback(t *testing.T) {
	t.Setenv("HOOK_BEAD_TITLE", "")
	t.Setenv("HOOK_BEAD_LABELS", "")
	t.Setenv("HOOK_BEAD_PHASE", "")

	ctx := resolveBeadContext("ag-abc", t.TempDir())
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.ID != "ag-abc" {
		t.Errorf("ID = %q, want %q", ctx.ID, "ag-abc")
	}
}

func TestApplyBeadBoost_DirectMatch(t *testing.T) {
	bead := &BeadContext{ID: "ag-7abc", Title: "Auth bug"}
	l := learning{
		SourceBead:     "ag-7abc",
		CompositeScore: 1.0,
	}

	applyBeadBoost(&l, bead)
	if l.CompositeScore != BeadScoreMatchDirect {
		t.Errorf("CompositeScore = %f, want %f", l.CompositeScore, BeadScoreMatchDirect)
	}
}

func TestApplyBeadBoost_LabelMatch(t *testing.T) {
	bead := &BeadContext{
		ID:     "ag-7abc",
		Labels: []string{"auth", "debugging"},
	}
	l := learning{
		Title:          "Auth token validation",
		CompositeScore: 1.0,
	}

	applyBeadBoost(&l, bead)
	if l.CompositeScore != BeadScoreMatchLabel {
		t.Errorf("CompositeScore = %f, want %f", l.CompositeScore, BeadScoreMatchLabel)
	}
}

func TestApplyBeadBoost_NoMatch(t *testing.T) {
	bead := &BeadContext{
		ID:     "ag-7abc",
		Labels: []string{"network"},
	}
	l := learning{
		Title:          "Database pooling strategy",
		Summary:        "Use connection pools for efficiency",
		CompositeScore: 1.0,
	}

	applyBeadBoost(&l, bead)
	// No match — should check keyword match from title
	// "network" doesn't appear in "Database pooling strategy" or "Use connection pools..."
	// CompositeScore stays 1.0 if no keyword match either
	if l.CompositeScore < 0.99 || l.CompositeScore > 1.01 {
		t.Errorf("CompositeScore = %f, want ~1.0 (no match)", l.CompositeScore)
	}
}

func TestApplyBeadBoost_NilBead(t *testing.T) {
	l := learning{CompositeScore: 1.0}
	applyBeadBoost(&l, nil)
	if l.CompositeScore != 1.0 {
		t.Errorf("CompositeScore = %f, want 1.0 (nil bead)", l.CompositeScore)
	}
}

func TestSplitLabels(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"auth, debugging, P1", 3},
		{"single", 1},
		{"", 0},
		{" , , ", 0},
		{"a,b", 2},
	}
	for _, tt := range tests {
		got := splitLabels(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitLabels(%q) = %d items, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestBeadScopedRanking_TaggedLearningsRankHigher(t *testing.T) {
	bead := &BeadContext{
		ID:       "ag-7abc",
		Title:    "Auth token refresh",
		Labels:   []string{"auth"},
		Keywords: buildKeywords(&BeadContext{Title: "Auth token refresh", Labels: []string{"auth"}}),
	}

	// 5 learnings with similar base scores: 2 tagged with the bead, 3 unrelated.
	// With 1.5x boost, tagged learnings should rise above untagged peers.
	learnings := []learning{
		{ID: "L1", Title: "Database indexing", Summary: "Add indexes for queries", CompositeScore: 1.5, SourceBead: ""},
		{ID: "L2", Title: "Auth RS256 validation", Summary: "Token signing uses RS256", CompositeScore: 1.4, SourceBead: "ag-7abc"},
		{ID: "L3", Title: "CI pipeline caching", Summary: "Cache Go modules", CompositeScore: 1.3, SourceBead: ""},
		{ID: "L4", Title: "Token TTL edge case", Summary: "Clock skew causes failure", CompositeScore: 1.2, SourceBead: "ag-7abc"},
		{ID: "L5", Title: "Logging best practices", Summary: "Use structured logs", CompositeScore: 1.1, SourceBead: ""},
	}

	// Apply bead boost: L2 → 1.4*1.5=2.1, L4 → 1.2*1.5=1.8
	// After: L2(2.1) > L4(1.8) > L1(1.5) > L3(1.3) > L5(1.1)
	for i := range learnings {
		applyBeadBoost(&learnings[i], bead)
	}
	resortLearnings(learnings)

	// Tagged learnings (L2, L4) should be in top 2 positions
	if learnings[0].ID != "L2" {
		t.Errorf("Top learning should be L2 (bead-tagged), got ID=%s score=%.2f", learnings[0].ID, learnings[0].CompositeScore)
	}
	if learnings[1].ID != "L4" {
		t.Errorf("Second learning should be L4 (bead-tagged), got ID=%s score=%.2f", learnings[1].ID, learnings[1].CompositeScore)
	}
}

func TestBuildKeywords(t *testing.T) {
	ctx := &BeadContext{
		Title:  "Fix auth token refresh",
		Labels: []string{"auth", "debugging"},
	}
	kw := buildKeywords(ctx)
	if len(kw) == 0 {
		t.Fatal("expected keywords")
	}

	// Should contain words from title and labels (deduplicated)
	found := make(map[string]bool)
	for _, k := range kw {
		found[k] = true
	}
	if !found["auth"] {
		t.Error("expected 'auth' keyword")
	}
	if !found["token"] {
		t.Error("expected 'token' keyword")
	}
	if !found["debugging"] {
		t.Error("expected 'debugging' keyword")
	}
}
