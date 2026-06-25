package search

import (
	"math"
	"math/rand" // #nosec G404 nosemgrep -- ε-exploration sampler for retrieval ranking; not a security primitive.
	"os"
	"strconv"
)

// The ε-exploration floor (knowledge-field S1, cold-start).
//
// Retrieval ranks best-first and returns the top-K. With a flat default utility
// (every un-rewarded trail sits at the same low prior), new/under-explored trails
// live in the tail and are NEVER returned → never cited → never reinforced → the
// cold-start lock: the strong trails win forever and the field can't learn what
// the tail is worth. ExploreSelect reserves a fraction of the returned slots for
// tail trails so they get surfaced, cited, and can earn their way up — the ACO
// τ_min / exploration-noise analog. (Complement: optimistic-init, a later slice,
// boosts under-observed trails UP the ranking; this floor samples the tail.)

// retrievalEpsilonEnv tunes the exploration fraction at runtime.
const retrievalEpsilonEnv = "AO_RETRIEVAL_EPSILON"

// ResolveRetrievalEpsilon reads AO_RETRIEVAL_EPSILON (a float in [0,1]); defaults
// to 0 (no exploration → zero behavior change). Out-of-range/garbage → 0.
func ResolveRetrievalEpsilon() float64 {
	s := os.Getenv(retrievalEpsilonEnv)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1 {
		return 0
	}
	return f
}

// ExploreSelect returns `limit` items from a slice already ranked best-first:
// the top (limit - reserve) by rank (exploit) plus reserve = floor(epsilon*limit)
// items sampled WITHOUT replacement from the tail (rank >= keep) (explore).
//
//   - epsilon <= 0 (default): returns ranked[:limit] verbatim — zero behavior change.
//   - len(ranked) <= limit, or limit <= 0: returns ranked unchanged (nothing to trim).
//   - limit == 1: no exploration is possible without dropping the only exploit slot,
//     so the top item is always kept.
//
// rng nil uses the global source; callers pass a seeded *rand.Rand for determinism.
func ExploreSelect[T any](ranked []T, limit int, epsilon float64, rng *rand.Rand) []T {
	if limit <= 0 {
		return ranked[:0] // match `ranked[:limit]` semantics (limit 0 -> empty)
	}
	if len(ranked) <= limit {
		return ranked
	}
	if epsilon <= 0 {
		return ranked[:limit]
	}
	reserve := int(epsilon * float64(limit))
	if reserve < 1 {
		reserve = 1 // epsilon > 0 guarantees at least one exploration slot
	}
	if reserve >= limit {
		reserve = limit - 1 // always keep at least one exploit slot
	}
	keep := limit - reserve

	out := make([]T, 0, limit)
	out = append(out, ranked[:keep]...)

	// Sample exploration slots from the TRUE tail (indices >= limit — items that
	// were never in the top-K), so the floor actually surfaces previously-excluded
	// cold-start trails. ranked[keep:limit) are the displaced top-K items; use them
	// only as fallback fill to still return exactly `limit`.
	trueTail := make([]int, 0, len(ranked)-limit)
	for i := limit; i < len(ranked); i++ {
		trueTail = append(trueTail, i)
	}
	intn := func(n int) int {
		if rng != nil {
			return rng.Intn(n)
		}
		return rand.Intn(n) // #nosec G404 nosemgrep -- ε-exploration sampler; not a security primitive.
	}
	for r := 0; r < reserve && len(trueTail) > 0; r++ {
		j := intn(len(trueTail))
		out = append(out, ranked[trueTail[j]])
		trueTail = append(trueTail[:j], trueTail[j+1:]...)
	}
	// Fallback: if the true tail was smaller than reserve, fill from the displaced
	// top-K items in rank order so the result is exactly `limit`.
	for i := keep; i < limit && len(out) < limit; i++ {
		out = append(out, ranked[i])
	}
	return out
}
