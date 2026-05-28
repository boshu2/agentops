// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// InMemoryClaimEvidenceBinder is a ClaimEvidenceBinderPort backed by
// a per-claim map. Idempotent re-bind preserves the binding; Level
// upgrade is allowed; downgrade is rejected. Thread-safe via mutex.
type InMemoryClaimEvidenceBinder struct {
	mu       sync.Mutex
	bindings map[ClaimID]EvidenceBinding
	order    []ClaimID // insertion order for List
}

// NewInMemoryClaimEvidenceBinder returns an empty adapter.
func NewInMemoryClaimEvidenceBinder() *InMemoryClaimEvidenceBinder {
	return &InMemoryClaimEvidenceBinder{
		bindings: map[ClaimID]EvidenceBinding{},
	}
}

// Bind creates or updates a binding. See port contract for
// idempotency + upgrade-only semantics.
func (a *InMemoryClaimEvidenceBinder) Bind(ctx context.Context, binding EvidenceBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if binding.Claim == "" {
		return errors.New("ports: EvidenceBinding.Claim required")
	}
	if binding.Path == "" {
		return errors.New("ports: EvidenceBinding.Path required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	prev, existed := a.bindings[binding.Claim]
	if existed && evidenceLevelRank(binding.Level) < evidenceLevelRank(prev.Level) {
		return fmt.Errorf("ports: ClaimEvidenceBinder.Bind cannot downgrade %s from %s to %s",
			binding.Claim, prev.Level, binding.Level)
	}
	if !existed {
		a.order = append(a.order, binding.Claim)
	}
	a.bindings[binding.Claim] = binding
	return nil
}

// List returns all bindings in most-recently-bound-first order.
func (a *InMemoryClaimEvidenceBinder) List(ctx context.Context) ([]EvidenceBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]EvidenceBinding, 0, len(a.order))
	for i := len(a.order) - 1; i >= 0; i-- {
		out = append(out, a.bindings[a.order[i]])
	}
	return out, nil
}

// evidenceLevelRank maps a level to a numeric ordering for compare.
// EvidenceLevelNone < PG1 < PG2 < PG3 < PG4.
func evidenceLevelRank(level EvidenceLevel) int {
	switch level {
	case EvidenceLevelPG1:
		return 1
	case EvidenceLevelPG2:
		return 2
	case EvidenceLevelPG3:
		return 3
	case EvidenceLevelPG4:
		return 4
	default:
		return 0
	}
}

// Compile-time assertion: InMemoryClaimEvidenceBinder satisfies the port.
var _ ClaimEvidenceBinderPort = (*InMemoryClaimEvidenceBinder)(nil)
