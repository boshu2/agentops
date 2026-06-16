package search

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestExploreSelect_EpsilonZeroIsTopK(t *testing.T) {
	in := []int{9, 8, 7, 6, 5, 4} // ranked best-first
	got := ExploreSelect(in, 3, 0, nil)
	if !reflect.DeepEqual(got, []int{9, 8, 7}) {
		t.Errorf("epsilon=0 must be top-K (zero behavior change), got %v", got)
	}
}

func TestExploreSelect_ShortSliceUnchanged(t *testing.T) {
	in := []int{3, 2}
	if got := ExploreSelect(in, 5, 0.5, nil); !reflect.DeepEqual(got, in) {
		t.Errorf("len<=limit must return unchanged, got %v", got)
	}
}

func TestExploreSelect_ReservesTailSlots(t *testing.T) {
	in := []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0} // 10 items, ranked
	rng := rand.New(rand.NewSource(42))
	got := ExploreSelect(in, 4, 0.5, rng) // reserve = floor(0.5*4)=2, keep=2
	if len(got) != 4 {
		t.Fatalf("must return exactly limit=4, got %d (%v)", len(got), got)
	}
	// top-2 (exploit) always present and first
	if got[0] != 9 || got[1] != 8 {
		t.Errorf("top-2 exploit slots must be the best ranked, got %v", got)
	}
	// the other 2 must come from the TRUE tail (rank >= limit=4 → values <= 5),
	// NOT the displaced top-K items (values 7,6) — locks the tail-boundary fix.
	for _, v := range got[2:] {
		if v > 5 {
			t.Errorf("explore slot %d came from the displaced top-K, not the true tail", v)
		}
	}
	// no duplicates
	seen := map[int]bool{}
	for _, v := range got {
		if seen[v] {
			t.Errorf("duplicate %d in selection %v", v, got)
		}
		seen[v] = true
	}
}

func TestExploreSelect_Limit1KeepsTop(t *testing.T) {
	in := []int{9, 8, 7}
	if got := ExploreSelect(in, 1, 0.9, rand.New(rand.NewSource(1))); !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("limit=1 must keep the top item (no room to explore), got %v", got)
	}
}

func TestExploreSelect_Deterministic(t *testing.T) {
	in := []int{9, 8, 7, 6, 5, 4, 3, 2}
	a := ExploreSelect(in, 4, 0.5, rand.New(rand.NewSource(7)))
	b := ExploreSelect(in, 4, 0.5, rand.New(rand.NewSource(7)))
	if !reflect.DeepEqual(a, b) {
		t.Errorf("same seed must be deterministic: %v vs %v", a, b)
	}
}

func TestExploreSelect_LimitZeroIsEmpty(t *testing.T) {
	// match ranked[:limit] semantics: limit 0 -> empty (not the whole slice)
	if got := ExploreSelect([]int{9, 8, 7}, 0, 0.5, nil); len(got) != 0 {
		t.Errorf("limit<=0 must return empty (matches [:0]), got %v", got)
	}
}

func TestExploreSelect_FallbackFillsToLimit(t *testing.T) {
	// true tail smaller than reserve → fill from displaced to still return limit
	in := []int{9, 8, 7, 6, 5}                                    // len 5
	got := ExploreSelect(in, 4, 0.5, rand.New(rand.NewSource(3))) // reserve=2, keep=2, trueTail=[idx4]=1 item
	if len(got) != 4 {
		t.Fatalf("must return exactly limit=4 even when true tail < reserve, got %d (%v)", len(got), got)
	}
	if got[0] != 9 || got[1] != 8 {
		t.Errorf("exploit slots must be the top-2, got %v", got)
	}
	seen := map[int]bool{}
	for _, v := range got {
		if seen[v] {
			t.Errorf("duplicate %d in %v", v, got)
		}
		seen[v] = true
	}
	if !seen[5] { // the lone true-tail item (idx4) must be included
		t.Errorf("true-tail item not surfaced: %v", got)
	}
}

func TestResolveRetrievalEpsilon(t *testing.T) {
	t.Setenv(retrievalEpsilonEnv, "")
	if ResolveRetrievalEpsilon() != 0 {
		t.Error("default epsilon must be 0")
	}
	t.Setenv(retrievalEpsilonEnv, "0.2")
	if ResolveRetrievalEpsilon() != 0.2 {
		t.Error("0.2 should parse")
	}
	for _, bad := range []string{"-0.1", "1.5", "abc", "NaN", "nan", "Inf", "+Inf"} {
		t.Setenv(retrievalEpsilonEnv, bad)
		if ResolveRetrievalEpsilon() != 0 {
			t.Errorf("out-of-range/garbage %q must clamp to 0", bad)
		}
	}
}
