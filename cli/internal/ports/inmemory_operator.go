// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"sync"
)

// InMemoryOperator is an OperatorPort backed by an in-memory slice.
// Thread-safe via mutex. List returns most-recent first.
type InMemoryOperator struct {
	mu      sync.Mutex
	intents []OperatorIntent
}

// NewInMemoryOperator returns an empty adapter.
func NewInMemoryOperator() *InMemoryOperator {
	return &InMemoryOperator{}
}

// Record appends intent. Empty Kind is rejected.
func (a *InMemoryOperator) Record(ctx context.Context, intent OperatorIntent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if intent.Kind == "" {
		return errors.New("ports: OperatorIntent.Kind required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.intents = append(a.intents, intent)
	return nil
}

// List returns all recorded intents, most-recent first.
func (a *InMemoryOperator) List(ctx context.Context) ([]OperatorIntent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]OperatorIntent, 0, len(a.intents))
	for i := len(a.intents) - 1; i >= 0; i-- {
		out = append(out, a.intents[i])
	}
	return out, nil
}

// Compile-time assertion: InMemoryOperator satisfies the port.
var _ OperatorPort = (*InMemoryOperator)(nil)
