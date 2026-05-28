// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// InMemoryLoopReader is a LoopReaderPort backed by a fixed slice of
// CycleEntry records the adapter returns from the read methods. The
// entries SHOULD be ordered by Number ascending; the adapter does not
// re-sort, so callers that pass out-of-order entries will get
// surprising Latest results.
type InMemoryLoopReader struct {
	entries []CycleEntry
}

// NewInMemoryLoopReader returns an adapter over the given entries.
// The caller's slice is retained (not copied) so callers must not
// mutate it after construction. Nil entries is safe.
func NewInMemoryLoopReader(entries []CycleEntry) *InMemoryLoopReader {
	return &InMemoryLoopReader{entries: entries}
}

// Latest returns the entry with the highest Number. Empty ledger →
// zero-value CycleEntry + nil error.
func (r *InMemoryLoopReader) Latest(ctx context.Context) (CycleEntry, error) {
	if err := ctx.Err(); err != nil {
		return CycleEntry{}, err
	}
	if len(r.entries) == 0 {
		return CycleEntry{}, nil
	}
	best := r.entries[0]
	for _, e := range r.entries[1:] {
		if e.Number > best.Number {
			best = e
		}
	}
	return best, nil
}

// Range returns entries whose Number is in [start, end] (inclusive).
func (r *InMemoryLoopReader) Range(ctx context.Context, start, end int) ([]CycleEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]CycleEntry, 0)
	for _, e := range r.entries {
		if e.Number >= start && e.Number <= end {
			out = append(out, e)
		}
	}
	return out, nil
}

// IdleStreak returns the trailing count of entries (from the slice
// end) whose Result is "idle" or "unchanged". The slice is assumed
// to be Number-ascending so the "trailing" end is the most-recent
// entry. Mixed-order callers get undefined results.
func (r *InMemoryLoopReader) IdleStreak(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	streak := 0
	for i := len(r.entries) - 1; i >= 0; i-- {
		switch r.entries[i].Result {
		case "idle", "unchanged":
			streak++
		default:
			return streak, nil
		}
	}
	return streak, nil
}

// Compile-time assertion: InMemoryLoopReader satisfies the port.
var _ LoopReaderPort = (*InMemoryLoopReader)(nil)
