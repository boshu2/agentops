// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// InMemoryCIStatus is a CIStatusPort backed by a fixed list of CIRun
// records the adapter returns from Latest/Recent. Intended for tests
// and CLI dry-runs of CI-aware classifier logic without depending on
// the `gh` CLI or GitHub API.
//
// The adapter holds a copy of the supplied runs slice — callers may
// mutate their original slice after construction without affecting
// the adapter's state. Recent returns runs in reverse-insertion
// order (most-recent first), respecting the supplied limit.
type InMemoryCIStatus struct {
	runs []CIRun
}

// NewInMemoryCIStatus returns an adapter over the given runs. Runs
// SHOULD be supplied in chronological order (oldest first); Recent
// will reverse them to return most-recent first. Nil/empty runs is
// safe; Latest then returns a zero-value CIRun for any sha.
func NewInMemoryCIStatus(runs []CIRun) *InMemoryCIStatus {
	copy_ := make([]CIRun, len(runs))
	copy(copy_, runs)
	return &InMemoryCIStatus{runs: copy_}
}

// Latest returns the most recent CIRun for the given sha. Zero-value
// CIRun + nil error when no run for that sha is found.
func (a *InMemoryCIStatus) Latest(ctx context.Context, sha string) (CIRun, error) {
	if err := ctx.Err(); err != nil {
		return CIRun{}, err
	}
	if sha == "" {
		return CIRun{}, errors.New("ports: sha required for CIStatusPort.Latest")
	}
	for i := len(a.runs) - 1; i >= 0; i-- {
		if a.runs[i].Sha == sha {
			return a.runs[i], nil
		}
	}
	return CIRun{}, nil
}

// Recent returns up to `limit` most-recent runs. limit==0 returns the
// full slice (capped implicitly at len(a.runs)).
func (a *InMemoryCIStatus) Recent(ctx context.Context, limit int) ([]CIRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reversed := make([]CIRun, len(a.runs))
	for i, r := range a.runs {
		reversed[len(a.runs)-1-i] = r
	}
	if limit > 0 && limit < len(reversed) {
		reversed = reversed[:limit]
	}
	return reversed, nil
}

// Compile-time assertion: InMemoryCIStatus satisfies the port.
var _ CIStatusPort = (*InMemoryCIStatus)(nil)
