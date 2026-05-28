package ports

import (
	"context"
	"fmt"
	"strings"
)

// InMemoryCitation is a CitationPort backed by a fixed set of "known"
// references the adapter considers FRESH. Intended for tests and
// CLI dry-runs. Construct with NewInMemoryCitation and a slice of
// known-fresh raw citation strings (one entry per kind/raw pair).
//
// Verify semantics:
//
//   - Empty Raw returns UNKNOWN with reason "empty Raw".
//   - Whitespace-only Raw is treated as empty.
//   - Otherwise the adapter looks up Kind+Raw in its known-fresh set:
//     FRESH if present, STALE otherwise. Reason names the verdict
//     basis.
type InMemoryCitation struct {
	fresh map[citationKey]struct{}
}

type citationKey struct {
	kind CitationKind
	raw  string
}

// NewInMemoryCitation returns an adapter that considers each
// (kind, raw) pair in fresh as a FRESH citation. Construction-time
// cost is O(len(fresh)). The returned adapter is safe for concurrent
// reads; callers must not mutate the fresh slice after construction.
func NewInMemoryCitation(fresh []CitationRequest) *InMemoryCitation {
	set := make(map[citationKey]struct{}, len(fresh))
	for _, c := range fresh {
		set[citationKey{kind: c.Kind, raw: c.Raw}] = struct{}{}
	}
	return &InMemoryCitation{fresh: set}
}

// Verify returns a CitationVerdict per the package-level contract.
func (a *InMemoryCitation) Verify(ctx context.Context, req CitationRequest) (CitationVerdict, error) {
	if err := ctx.Err(); err != nil {
		return CitationVerdict{}, err
	}
	if strings.TrimSpace(req.Raw) == "" {
		return CitationVerdict{Status: CitationStatusUnknown, Reason: "empty Raw"}, nil
	}
	if _, fresh := a.fresh[citationKey{kind: req.Kind, raw: req.Raw}]; fresh {
		return CitationVerdict{
			Status:   CitationStatusFresh,
			Reason:   fmt.Sprintf("%s citation %q is in the known-fresh set", req.Kind, req.Raw),
			Resolved: req.Raw,
		}, nil
	}
	return CitationVerdict{
		Status: CitationStatusStale,
		Reason: fmt.Sprintf("%s citation %q is not in the known-fresh set", req.Kind, req.Raw),
	}, nil
}

// Compile-time assertion: InMemoryCitation satisfies the port.
var _ CitationPort = (*InMemoryCitation)(nil)
