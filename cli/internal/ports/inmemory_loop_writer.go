// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryLoopWriter is a LoopWriterPort backed by an in-memory slice.
// Auto-assigns Number when entry.Number is 0 (next = max+1). Rejects
// duplicate Numbers. Thread-safe via mutex.
type InMemoryLoopWriter struct {
	mu      sync.Mutex
	entries []CycleEntry
}

// NewInMemoryLoopWriter returns an empty writer. SeedEntries can be
// supplied as the initial state (e.g. to combine with InMemoryLoopReader
// in tests that need a read-write fixture).
func NewInMemoryLoopWriter(seed []CycleEntry) *InMemoryLoopWriter {
	cp := make([]CycleEntry, len(seed))
	copy(cp, seed)
	return &InMemoryLoopWriter{entries: cp}
}

// Append records the entry. See port contract for Number-assignment +
// duplicate-rejection semantics.
func (w *InMemoryLoopWriter) Append(ctx context.Context, entry CycleEntry) (CycleEntry, error) {
	if err := ctx.Err(); err != nil {
		return CycleEntry{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if entry.Number == 0 {
		entry.Number = w.nextNumberLocked()
	} else {
		for _, e := range w.entries {
			if e.Number == entry.Number {
				return CycleEntry{}, fmt.Errorf("ports: LoopWriter.Append duplicate cycle Number %d", entry.Number)
			}
		}
	}
	w.entries = append(w.entries, entry)
	return entry, nil
}

// Snapshot returns a defensive copy of the current entries. Test-only
// helper; not part of the port contract.
func (w *InMemoryLoopWriter) Snapshot() []CycleEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]CycleEntry, len(w.entries))
	copy(out, w.entries)
	return out
}

// nextNumberLocked computes the next sequential Number. Assumes the
// caller holds w.mu.
func (w *InMemoryLoopWriter) nextNumberLocked() int {
	maxN := 0
	for _, e := range w.entries {
		if e.Number > maxN {
			maxN = e.Number
		}
	}
	return maxN + 1
}

// Compile-time assertion: InMemoryLoopWriter satisfies the port.
var _ LoopWriterPort = (*InMemoryLoopWriter)(nil)
