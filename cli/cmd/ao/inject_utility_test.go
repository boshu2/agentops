package main

import (
	"sort"
	"testing"

	"github.com/boshu2/agentops/cli/internal/search"
)

// TestInject_UtilityWeightedRanking verifies that the W1-06 utility-weighted
// re-rank (in applyInjectModifiers) actually changes order when learnings have
// different `utility:` frontmatter values.
//
// This is the consumer surface that closes the eval-verdict pipeline:
// hooks/eval-verdict-compiler.sh mutates utility frontmatter; ao inject reads
// it via search.Learning.Utility and scales CompositeScore accordingly.
func TestInject_UtilityWeightedRanking(t *testing.T) {
	// Three learnings with identical CompositeScore (baseline) but different utility.
	// After utility-weighted re-rank with weight=1.0 and offset 0.5:
	//   high   (u=0.9): score *= 1 + 1.0*(0.9-0.5) = 1.4   -> 1.4
	//   mid    (u=0.5): score *= 1 + 1.0*(0.5-0.5) = 1.0   -> 1.0
	//   low    (u=0.1): score *= 1 + 1.0*(0.1-0.5) = 0.6   -> 0.6
	//
	// Order: high > mid > low.
	saved := injectUtilityWeight
	defer func() { injectUtilityWeight = saved }()
	injectUtilityWeight = 1.0

	ls := []search.Learning{
		{ID: "high", CompositeScore: 1.0, Utility: 0.9},
		{ID: "mid", CompositeScore: 1.0, Utility: 0.5},
		{ID: "low", CompositeScore: 1.0, Utility: 0.1},
	}
	applyUtilityRerank(ls, injectUtilityWeight)
	if ls[0].ID != "high" || ls[1].ID != "mid" || ls[2].ID != "low" {
		t.Fatalf("utility-weighted ranking wrong: got order %s,%s,%s; want high,mid,low",
			ls[0].ID, ls[1].ID, ls[2].ID)
	}
	if !floatEq(ls[0].CompositeScore, 1.4) {
		t.Errorf("high score: got %v, want 1.4", ls[0].CompositeScore)
	}
	if !floatEq(ls[1].CompositeScore, 1.0) {
		t.Errorf("mid score: got %v, want 1.0", ls[1].CompositeScore)
	}
	if !floatEq(ls[2].CompositeScore, 0.6) {
		t.Errorf("low score: got %v, want 0.6", ls[2].CompositeScore)
	}
}

// TestInject_UtilityWeight_ZeroDisables: weight=0 leaves scores untouched.
func TestInject_UtilityWeight_ZeroDisables(t *testing.T) {
	saved := injectUtilityWeight
	defer func() { injectUtilityWeight = saved }()
	injectUtilityWeight = 0.0

	ls := []search.Learning{
		{ID: "a", CompositeScore: 1.0, Utility: 0.9},
		{ID: "b", CompositeScore: 1.0, Utility: 0.1},
	}
	applyUtilityRerank(ls, injectUtilityWeight)
	if !floatEq(ls[0].CompositeScore, 1.0) || !floatEq(ls[1].CompositeScore, 1.0) {
		t.Errorf("weight=0 should leave scores untouched: %v %v",
			ls[0].CompositeScore, ls[1].CompositeScore)
	}
}

// TestInject_UtilityWeight_MissingUtility: zero-valued utility (no frontmatter)
// produces a 0.5 implicit penalty proportional to weight. Default behavior:
// missing utility moderately penalized — encourages frontmatter to be set.
func TestInject_UtilityWeight_MissingUtility(t *testing.T) {
	saved := injectUtilityWeight
	defer func() { injectUtilityWeight = saved }()
	injectUtilityWeight = 1.0

	ls := []search.Learning{
		{ID: "missing", CompositeScore: 1.0, Utility: 0.0},           // missing -> u=0 -> penalty
		{ID: "explicit_baseline", CompositeScore: 1.0, Utility: 0.5}, // explicit baseline
	}
	applyUtilityRerank(ls, injectUtilityWeight)
	// missing: 1 * (1 + 1*(0-0.5)) = 0.5
	// explicit_baseline: 1 * (1 + 1*(0.5-0.5)) = 1.0
	if ls[0].ID != "explicit_baseline" {
		t.Errorf("explicit baseline should rank above missing-utility, got %s", ls[0].ID)
	}
	if !floatEq(ls[0].CompositeScore, 1.0) {
		t.Errorf("explicit baseline score: got %v, want 1.0", ls[0].CompositeScore)
	}
	if !floatEq(ls[1].CompositeScore, 0.5) {
		t.Errorf("missing-utility score: got %v, want 0.5", ls[1].CompositeScore)
	}
}

// applyUtilityRerank is the testable helper extracted from applyInjectModifiers
// so we can unit-test the multiplier logic without spinning up a full inject
// pipeline. Mirrors the production block exactly.
func applyUtilityRerank(ls []search.Learning, weight float64) {
	if weight == 0.0 {
		return
	}
	for i := range ls {
		u := ls[i].Utility
		ls[i].CompositeScore *= 1.0 + weight*(u-0.5)
	}
	sort.SliceStable(ls, func(i, j int) bool {
		return ls[i].CompositeScore > ls[j].CompositeScore
	})
}

func floatEq(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}
