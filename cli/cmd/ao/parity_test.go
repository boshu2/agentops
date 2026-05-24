// practices: [design-by-contract, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/boshu2/agentops/cli/internal/vibecheck"
)

// ---------------------------------------------------------------------------
// Test 1: Pool FindByPrefix round-trip
// ---------------------------------------------------------------------------

func TestPoolFindByPrefixRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	p := pool.NewPool(tmp)

	candidates := []types.Candidate{
		{ID: "cand-alpha-001", Type: "learning", Tier: types.TierSilver, Content: "Alpha content", Utility: 0.8, Confidence: 0.9, Maturity: "emerging"},
		{ID: "cand-alpha-002", Type: "learning", Tier: types.TierBronze, Content: "Alpha two content", Utility: 0.6, Confidence: 0.7, Maturity: "emerging"},
		{ID: "cand-beta-001", Type: "learning", Tier: types.TierGold, Content: "Beta content", Utility: 0.95, Confidence: 0.99, Maturity: "established"},
	}

	for _, c := range candidates {
		if err := p.Add(c, types.Scoring{RawScore: c.Utility, TierAssignment: c.Tier}); err != nil {
			t.Fatalf("add %s: %v", c.ID, err)
		}
	}

	// Unique prefix should return exactly 1 match.
	matches, err := p.FindByPrefix("cand-beta")
	if err != nil {
		t.Fatalf("FindByPrefix(cand-beta): %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("FindByPrefix(cand-beta) returned %d matches, want 1", len(matches))
	} else if matches[0].Candidate.ID != "cand-beta-001" {
		t.Errorf("FindByPrefix(cand-beta) returned ID %s, want cand-beta-001", matches[0].Candidate.ID)
	}

	// Ambiguous prefix should return multiple matches.
	matches, err = p.FindByPrefix("cand-alpha")
	if err != nil {
		t.Fatalf("FindByPrefix(cand-alpha): %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("FindByPrefix(cand-alpha) returned %d matches, want 2", len(matches))
	}

	// Non-matching prefix should return 0 matches.
	matches, err = p.FindByPrefix("cand-gamma")
	if err != nil {
		t.Fatalf("FindByPrefix(cand-gamma): %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("FindByPrefix(cand-gamma) returned %d matches, want 0", len(matches))
	}

	// Full ID as prefix should return exactly 1 match.
	matches, err = p.FindByPrefix("cand-alpha-001")
	if err != nil {
		t.Fatalf("FindByPrefix(cand-alpha-001): %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("FindByPrefix(cand-alpha-001) exact match returned %d, want 1", len(matches))
	}
}

// ---------------------------------------------------------------------------
// Test 3: Goals V1 loads with deprecation warning (no error)
// ---------------------------------------------------------------------------

func TestGoalsV1LoadsWithWarning(t *testing.T) {
	dir := t.TempDir()
	v1Content := `version: 1
goals:
  - id: test-goal
    description: A test goal
    check: echo ok
    weight: 5
`
	path := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(path, []byte(v1Content), 0o600); err != nil {
		t.Fatalf("write v1 goals: %v", err)
	}

	gf, err := goals.LoadGoals(path)
	if err != nil {
		t.Fatalf("LoadGoals returned error for v1: %v", err)
	}
	if gf.Version != 1 {
		t.Errorf("expected version 1, got %d", gf.Version)
	}
	if len(gf.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(gf.Goals))
	}
	if gf.Goals[0].ID != "test-goal" {
		t.Errorf("goal ID = %q, want %q", gf.Goals[0].ID, "test-goal")
	}
	// V1 goals should still default Type to "health"
	if gf.Goals[0].Type != goals.GoalTypeHealth {
		t.Errorf("goal Type = %q, want %q", gf.Goals[0].Type, goals.GoalTypeHealth)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Goals V1 migration preserves data
// ---------------------------------------------------------------------------

func TestGoalsV1MigrationPreservesData(t *testing.T) {
	v1Content := `version: 1
goals:
  - id: goal-alpha
    description: Alpha goal
    check: echo alpha
    weight: 3
  - id: goal-beta
    description: Beta goal
    check: echo beta
    weight: 7
`
	dir := t.TempDir()
	path := filepath.Join(dir, "GOALS.yaml")
	if err := os.WriteFile(path, []byte(v1Content), 0o600); err != nil {
		t.Fatalf("write v1 goals: %v", err)
	}

	gf, err := goals.LoadGoals(path)
	if err != nil {
		t.Fatalf("LoadGoals failed: %v", err)
	}

	// Capture original goals for comparison
	originalGoalIDs := make([]string, len(gf.Goals))
	originalDescriptions := make([]string, len(gf.Goals))
	originalChecks := make([]string, len(gf.Goals))
	originalWeights := make([]int, len(gf.Goals))
	for i, g := range gf.Goals {
		originalGoalIDs[i] = g.ID
		originalDescriptions[i] = g.Description
		originalChecks[i] = g.Check
		originalWeights[i] = g.Weight
	}

	// Migrate
	goals.MigrateV1ToV2(gf)

	// Assert version is now 2
	if gf.Version != 2 {
		t.Errorf("version after migration = %d, want 2", gf.Version)
	}

	// Assert mission was set
	if gf.Mission == "" {
		t.Error("expected non-empty mission after migration")
	}

	// Assert all goals preserved
	if len(gf.Goals) != len(originalGoalIDs) {
		t.Fatalf("goal count changed: was %d, now %d", len(originalGoalIDs), len(gf.Goals))
	}
	for i, g := range gf.Goals {
		if g.ID != originalGoalIDs[i] {
			t.Errorf("goal[%d].ID = %q, want %q", i, g.ID, originalGoalIDs[i])
		}
		if g.Description != originalDescriptions[i] {
			t.Errorf("goal[%d].Description = %q, want %q", i, g.Description, originalDescriptions[i])
		}
		if g.Check != originalChecks[i] {
			t.Errorf("goal[%d].Check = %q, want %q", i, g.Check, originalChecks[i])
		}
		if g.Weight != originalWeights[i] {
			t.Errorf("goal[%d].Weight = %d, want %d", i, g.Weight, originalWeights[i])
		}
		// Type should be defaulted to "health" after migration
		if g.Type != goals.GoalTypeHealth {
			t.Errorf("goal[%d].Type = %q, want %q", i, g.Type, goals.GoalTypeHealth)
		}
	}

	// Validate the migrated file passes validation
	errs := goals.ValidateGoals(gf)
	if len(errs) != 0 {
		t.Errorf("migrated goals have %d validation errors: %v", len(errs), errs)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Vibe check score bounds
// ---------------------------------------------------------------------------

func TestVibeCheckScoreBounds(t *testing.T) {
	tests := []struct {
		name    string
		metrics map[string]vibecheck.Metric
	}{
		{
			name:    "empty metrics",
			metrics: map[string]vibecheck.Metric{},
		},
		{
			name: "all pass",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 10, Threshold: 3, Passed: true},
				"rework":   {Name: "rework", Value: 5, Threshold: 30, Passed: true},
				"trust":    {Name: "trust", Value: 0.8, Threshold: 0.3, Passed: true},
				"spirals":  {Name: "spirals", Value: 0, Threshold: 0, Passed: true},
				"flow":     {Name: "flow", Value: 90, Threshold: 50, Passed: true},
			},
		},
		{
			name: "all fail",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 0, Threshold: 3, Passed: false},
				"rework":   {Name: "rework", Value: 100, Threshold: 30, Passed: false},
				"trust":    {Name: "trust", Value: 0, Threshold: 0.3, Passed: false},
				"spirals":  {Name: "spirals", Value: 5, Threshold: 0, Passed: false},
				"flow":     {Name: "flow", Value: 0, Threshold: 50, Passed: false},
			},
		},
		{
			name: "mixed results",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 5, Threshold: 3, Passed: true},
				"rework":   {Name: "rework", Value: 50, Threshold: 30, Passed: false},
				"trust":    {Name: "trust", Value: 0.1, Threshold: 0.3, Passed: false},
				"spirals":  {Name: "spirals", Value: 0, Threshold: 0, Passed: true},
				"flow":     {Name: "flow", Value: 25, Threshold: 50, Passed: false},
			},
		},
		{
			name: "extreme values",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 1000, Threshold: 3, Passed: true},
				"rework":   {Name: "rework", Value: 200, Threshold: 30, Passed: false},
				"trust":    {Name: "trust", Value: 100, Threshold: 0.3, Passed: true},
				"spirals":  {Name: "spirals", Value: 100, Threshold: 0, Passed: false},
				"flow":     {Name: "flow", Value: 999, Threshold: 50, Passed: true},
			},
		},
		{
			name: "single metric",
			metrics: map[string]vibecheck.Metric{
				"velocity": {Name: "velocity", Value: 5, Threshold: 3, Passed: true},
			},
		},
	}

	validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, grade := vibecheck.ComputeOverallRating(tt.metrics)

			if score < 0 || score > 100 {
				t.Errorf("score %.2f out of bounds [0, 100]", score)
			}
			if !validGrades[grade] {
				t.Errorf("grade %q not in valid set {A, B, C, D, F}", grade)
			}

			// Grade must be consistent with score
			switch {
			case score >= 80 && grade != "A":
				t.Errorf("score %.2f should be grade A, got %s", score, grade)
			case score >= 60 && score < 80 && grade != "B":
				t.Errorf("score %.2f should be grade B, got %s", score, grade)
			case score >= 40 && score < 60 && grade != "C":
				t.Errorf("score %.2f should be grade C, got %s", score, grade)
			case score >= 20 && score < 40 && grade != "D":
				t.Errorf("score %.2f should be grade D, got %s", score, grade)
			case score < 20 && grade != "F":
				t.Errorf("score %.2f should be grade F, got %s", score, grade)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 6: JSON truncation validity
// ---------------------------------------------------------------------------

func TestJSONTruncationValidity(t *testing.T) {
	t.Helper()

	// Build a knowledge struct large enough to require truncation.
	knowledge := &injectedKnowledge{
		Timestamp: time.Now(),
		Query:     "test-query",
	}

	// Add many learnings to inflate size.
	for i := range 20 {
		is := fmt.Sprintf("%d", i)
		knowledge.Learnings = append(knowledge.Learnings, learning{
			ID:      "learn-" + strings.Repeat("x", 20) + "-" + is,
			Title:   "Title " + strings.Repeat("word ", 30) + is,
			Summary: "Summary " + strings.Repeat("detail ", 50) + is,
		})
	}
	for i := range 10 {
		is := fmt.Sprintf("%d", i)
		knowledge.Patterns = append(knowledge.Patterns, pattern{
			Name:        "pattern-" + is,
			Description: "Description " + strings.Repeat("info ", 40) + is,
		})
	}
	for i := range 10 {
		is := fmt.Sprintf("%d", i)
		knowledge.Sessions = append(knowledge.Sessions, session{
			Date:    "2026-02-" + fmt.Sprintf("%d", i+1),
			Summary: "Session summary " + strings.Repeat("data ", 30) + is,
		})
	}

	// First verify the full JSON is larger than our test budget.
	fullJSON, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		t.Fatalf("marshal full knowledge: %v", err)
	}

	budgets := []int{500, 1000, 2000, 4000}
	for _, budget := range budgets {
		if len(fullJSON) <= budget {
			t.Skipf("full JSON (%d bytes) fits in budget %d; skipping truncation test", len(fullJSON), budget)
		}

		result := trimJSONToCharBudget(knowledge, budget)

		// 1. Result must be valid JSON.
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Errorf("budget %d: truncated output is not valid JSON: %v\nOutput (first 200 chars): %s", budget, err, truncateText(result, 200))
		}

		// 2. Result must fit within the budget.
		if len(result) > budget {
			t.Errorf("budget %d: truncated output is %d bytes (exceeds budget)", budget, len(result))
		}

		// 3. Result must contain the "truncated" field set to true.
		if truncated, ok := parsed["truncated"]; !ok {
			t.Errorf("budget %d: truncated output missing 'truncated' field", budget)
		} else if truncated != true {
			t.Errorf("budget %d: 'truncated' field = %v, want true", budget, truncated)
		}
	}
}

// TestJSONTruncationNoTruncationNeeded verifies that when JSON fits in budget,
// the function still returns valid JSON (this exercises the early-return path).
func TestJSONTruncationNoTruncationNeeded(t *testing.T) {
	knowledge := &injectedKnowledge{
		Timestamp: time.Now(),
		Query:     "small",
		Learnings: []learning{
			{ID: "l1", Title: "One", Summary: "Short"},
		},
	}

	fullJSON, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Use a generous budget that exceeds the full output.
	result := trimJSONToCharBudget(knowledge, len(fullJSON)+1000)

	// Should still be valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("non-truncated output is not valid JSON: %v", err)
	}

	// Should have truncated=true (the function always sets this).
	if truncated, ok := parsed["truncated"]; !ok || truncated != true {
		t.Errorf("expected truncated=true even when output fits budget, got %v", parsed["truncated"])
	}
}
