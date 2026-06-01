package provenancegraph

import "sort"

// CanonicalLess reports whether edge a sorts before edge b in the canonical
// export order. The order is the tuple (ts, from_id, to_id, relation), with
// from_type/to_type/trust_tier/evidence_ref as final tie-breakers so that two
// edges differing only in those fields still have a total, stable order. A
// total order is required for export to be byte-identical on re-run.
func CanonicalLess(a, b Edge) bool {
	switch {
	case a.TS != b.TS:
		return a.TS < b.TS
	case a.FromID != b.FromID:
		return a.FromID < b.FromID
	case a.ToID != b.ToID:
		return a.ToID < b.ToID
	case a.Relation != b.Relation:
		return a.Relation < b.Relation
	case a.FromType != b.FromType:
		return a.FromType < b.FromType
	case a.ToType != b.ToType:
		return a.ToType < b.ToType
	case a.TrustTier != b.TrustTier:
		return a.TrustTier < b.TrustTier
	default:
		return a.EvidenceRef < b.EvidenceRef
	}
}

// CanonicalSort returns a new slice with the edges ordered by CanonicalLess.
// The input slice is not mutated. The sort is stable so equal-tuple edges keep
// their relative input order.
func CanonicalSort(edges []Edge) []Edge {
	out := make([]Edge, len(edges))
	copy(out, edges)
	sort.SliceStable(out, func(i, j int) bool {
		return CanonicalLess(out[i], out[j])
	})
	return out
}

// ReChain canonically sorts the edges and re-seals them into a fresh, intact
// hash chain: each edge's prev_hash links to the prior edge's recomputed hash
// (genesis prev_hash = ""), and payload_hash/hash are recomputed from the
// edge's non-chain fields. Field validation runs on every edge, so a malformed
// edge fails the whole re-chain rather than producing a half-valid export.
//
// Because the chain is rebuilt from the canonical order, ReChain is the
// deterministic core of `ao provenance export`: identical input edges (in any
// order) produce an identical sealed slice, hence byte-identical export output.
func ReChain(edges []Edge) ([]Edge, error) {
	sorted := CanonicalSort(edges)
	out := make([]Edge, 0, len(sorted))
	prevHash := ""
	for _, e := range sorted {
		// Drop any inherited chain fields before re-sealing onto the new tip.
		e.PrevHash = ""
		e.PayloadHash = ""
		e.Hash = ""
		sealed, err := Seal(e, prevHash)
		if err != nil {
			return nil, err
		}
		out = append(out, sealed)
		prevHash = sealed.Hash
	}
	return out, nil
}
