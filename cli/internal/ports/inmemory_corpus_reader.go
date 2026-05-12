package ports

import (
	"context"
	"sort"
	"strings"
)

// InMemoryCorpusReader is a CorpusReaderPort backed by a slice of
// CorpusItems passed at construction time. It is intended for tests
// and ephemeral in-process callers (e.g. CLI dry-runs). It is NOT
// safe for concurrent mutation; callers that need a thread-safe
// reader should wrap it.
//
// Lookup semantics:
//
//   - When opts.Query is empty, all items are candidates.
//   - When opts.Query is non-empty, an item is a candidate iff its
//     Title or Body contains the query (case-insensitive substring).
//   - Score is computed as a simple match-count proxy (occurrences of
//     the lowercased query in lowercased Title+Body, or 1 when the
//     query is empty); higher = more matches.
//   - Items are returned sorted by Score descending; ties keep the
//     adapter's slice order.
//   - opts.DecayApplied is accepted but ignored — this adapter has no
//     time signal to decay against. Adapters that DO honor decay
//     belong in the package that owns the persistence surface (e.g.
//     a future .agents/learnings/-backed adapter); this one stays
//     pure to keep its test surface trivial.
type InMemoryCorpusReader struct {
	items []CorpusItem
}

// NewInMemoryCorpusReader returns a reader over the given items. The
// caller's slice is retained (not copied) so callers must not mutate
// it after construction. Returns a non-nil reader even for a nil
// items argument so Lookup always returns a non-nil slice.
func NewInMemoryCorpusReader(items []CorpusItem) *InMemoryCorpusReader {
	return &InMemoryCorpusReader{items: items}
}

// Lookup returns matching items per the package-level contract.
func (r *InMemoryCorpusReader) Lookup(ctx context.Context, opts LookupOptions) ([]CorpusItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(opts.Query))
	out := make([]CorpusItem, 0, len(r.items))
	for _, item := range r.items {
		if q == "" {
			scored := item
			scored.Score = 1
			out = append(out, scored)
			continue
		}
		hay := strings.ToLower(item.Title + "\n" + item.Body)
		if !strings.Contains(hay, q) {
			continue
		}
		scored := item
		scored.Score = float64(strings.Count(hay, q))
		out = append(out, scored)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}

// Compile-time assertion: InMemoryCorpusReader satisfies the port.
var _ CorpusReaderPort = (*InMemoryCorpusReader)(nil)
