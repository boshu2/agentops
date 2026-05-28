// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// InMemoryHypothesisLedger is a HypothesisLedgerPort backed by an
// append-ordered in-memory slice. It is intended for tests and
// dry-run loop orchestration that needs durable-looking hypothesis
// behavior without touching .agents/evolve/hypotheses.jsonl.
type InMemoryHypothesisLedger struct {
	mu      sync.Mutex
	records []HypothesisRecord
	byID    map[string]int
}

// NewInMemoryHypothesisLedger returns an adapter seeded with the
// given records. Seed records are copied defensively; non-empty seed
// IDs are indexed so later Append calls still reject duplicates.
func NewInMemoryHypothesisLedger(seed []HypothesisRecord) *InMemoryHypothesisLedger {
	records := make([]HypothesisRecord, len(seed))
	byID := make(map[string]int, len(seed))
	for i, record := range seed {
		records[i] = cloneHypothesisRecord(record)
		if record.ID != "" {
			byID[record.ID] = i
		}
	}
	return &InMemoryHypothesisLedger{records: records, byID: byID}
}

// Append stores record at the end of the ledger, rejecting empty and
// duplicate IDs.
func (l *InMemoryHypothesisLedger) Append(ctx context.Context, record HypothesisRecord) (HypothesisRecord, error) {
	if err := ctx.Err(); err != nil {
		return HypothesisRecord{}, err
	}
	if record.ID == "" {
		return HypothesisRecord{}, errors.New("ports: HypothesisLedger.Append requires ID")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.byID[record.ID]; exists {
		return HypothesisRecord{}, fmt.Errorf("ports: HypothesisLedger.Append duplicate ID %q", record.ID)
	}

	stored := cloneHypothesisRecord(record)
	l.byID[stored.ID] = len(l.records)
	l.records = append(l.records, stored)
	return cloneHypothesisRecord(stored), nil
}

// List returns all records in append order.
func (l *InMemoryHypothesisLedger) List(ctx context.Context) ([]HypothesisRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]HypothesisRecord, len(l.records))
	for i, record := range l.records {
		out[i] = cloneHypothesisRecord(record)
	}
	return out, nil
}

// Find returns a record by ID. Unknown IDs are not errors.
func (l *InMemoryHypothesisLedger) Find(ctx context.Context, id string) (HypothesisRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return HypothesisRecord{}, false, err
	}
	if id == "" {
		return HypothesisRecord{}, false, errors.New("ports: HypothesisLedger.Find requires ID")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	idx, exists := l.byID[id]
	if !exists {
		return HypothesisRecord{}, false, nil
	}
	return cloneHypothesisRecord(l.records[idx]), true, nil
}

func cloneHypothesisRecord(record HypothesisRecord) HypothesisRecord {
	if record.Evidence != nil {
		record.Evidence = append([]string(nil), record.Evidence...)
	}
	return record
}

// Compile-time assertion: InMemoryHypothesisLedger satisfies the port.
var _ HypothesisLedgerPort = (*InMemoryHypothesisLedger)(nil)
