package ports

import "context"

// CorpusItem is the minimum shape a CorpusReaderPort emits for each
// match. Concrete adapters may project additional fields onto a richer
// internal type; the port stays narrow on purpose. Score is a port
// concept (higher = more relevant); adapters that lack a meaningful
// ranking signal MAY return 0 for every item.
type CorpusItem struct {
	Path  string
	Title string
	Body  string
	Score float64
}

// LookupOptions configures a CorpusReaderPort lookup. Zero values are
// valid: Query empty + Limit 0 means "return everything the adapter
// considers ranked" (callers should always pass a meaningful Limit in
// production code). DecayApplied true requests that the adapter
// apply its decay-ranking heuristic; adapters that have no decay
// concept may ignore this flag, but documenting whether decay is
// honored is part of the adapter's contract.
type LookupOptions struct {
	Query        string
	Limit        int
	DecayApplied bool
}

// CorpusReaderPort is the read-side of BC1 Corpus. Callers — most
// notably evolve/Step-0 knowledge retrieval and the `ao lookup` /
// `ao inject` paths — depend on this port so they can be tested
// against in-memory adapters without standing up the full
// .agents/learnings/ + retrieval-index machinery.
//
// Contract:
//
//   - Lookup MUST return a non-nil (possibly empty) slice on success.
//     A nil slice + nil error pair is an adapter bug.
//   - The returned slice MUST respect opts.Limit when > 0.
//   - Items SHOULD be sorted by Score descending. Adapters with no
//     ranking signal MAY return insertion order with all Scores = 0.
//   - Context cancellation MUST be honored on a best-effort basis;
//     adapters that complete synchronously and quickly may ignore it.
//
// See docs/contracts/ubiquitous-language.md (BC1 row) for the
// canonical Corpus context surface.
type CorpusReaderPort interface {
	Lookup(ctx context.Context, opts LookupOptions) ([]CorpusItem, error)
}
