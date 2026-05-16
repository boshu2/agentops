// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionHypothesisLedger satisfies ports.HypothesisLedgerPort by
// appending and reading records from .agents/evolve/hypotheses.jsonl
// or any path supplied by tests. It is the production sibling of
// InMemoryHypothesisLedger and follows the LoopReader/LoopWriter JSONL
// adapter shape.
//
// File semantics:
//   - Append-only: each Append writes one JSON line.
//   - Duplicate IDs are rejected by scanning valid existing records
//     before append.
//   - List tolerates empty, missing, and hand-edited files by skipping
//     malformed JSON lines.
//   - Thread safety is process-local. Cross-process appends need a
//     higher-level file lock if they become a real concurrent path.
type productionHypothesisLedger struct {
	mu   sync.Mutex
	path string
}

func newProductionHypothesisLedger(path string) *productionHypothesisLedger {
	return &productionHypothesisLedger{path: path}
}

func (l *productionHypothesisLedger) Append(ctx context.Context, record ports.HypothesisRecord) (ports.HypothesisRecord, error) {
	if err := ctx.Err(); err != nil {
		return ports.HypothesisRecord{}, err
	}
	if l.path == "" {
		return ports.HypothesisRecord{}, fmt.Errorf("productionHypothesisLedger: path required")
	}
	if record.ID == "" {
		return ports.HypothesisRecord{}, errors.New("productionHypothesisLedger: ID required")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	records, err := l.readRecords(ctx)
	if err != nil {
		return ports.HypothesisRecord{}, err
	}
	for _, existing := range records {
		if existing.ID == record.ID {
			return ports.HypothesisRecord{}, fmt.Errorf("productionHypothesisLedger: duplicate ID %q", record.ID)
		}
	}

	stored := cloneProductionHypothesisRecord(record)
	payload, err := json.Marshal(stored)
	if err != nil {
		return ports.HypothesisRecord{}, fmt.Errorf("productionHypothesisLedger marshal: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return ports.HypothesisRecord{}, fmt.Errorf("productionHypothesisLedger open %q: %w", l.path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(payload, '\n')); err != nil {
		return ports.HypothesisRecord{}, fmt.Errorf("productionHypothesisLedger write: %w", err)
	}
	return cloneProductionHypothesisRecord(stored), nil
}

func (l *productionHypothesisLedger) List(ctx context.Context) ([]ports.HypothesisRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.readRecords(ctx)
}

func (l *productionHypothesisLedger) Find(ctx context.Context, id string) (ports.HypothesisRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return ports.HypothesisRecord{}, false, err
	}
	if id == "" {
		return ports.HypothesisRecord{}, false, errors.New("productionHypothesisLedger: ID required")
	}

	records, err := l.List(ctx)
	if err != nil {
		return ports.HypothesisRecord{}, false, err
	}
	for _, record := range records {
		if record.ID == id {
			return cloneProductionHypothesisRecord(record), true, nil
		}
	}
	return ports.HypothesisRecord{}, false, nil
}

func (l *productionHypothesisLedger) readRecords(ctx context.Context) ([]ports.HypothesisRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.path == "" {
		return nil, nil
	}
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("productionHypothesisLedger open %q: %w", l.path, err)
	}
	defer f.Close()

	out := make([]ports.HypothesisRecord, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record ports.HypothesisRecord
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if record.ID == "" {
			continue
		}
		out = append(out, cloneProductionHypothesisRecord(record))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("productionHypothesisLedger scan %q: %w", l.path, err)
	}
	return out, nil
}

func cloneProductionHypothesisRecord(record ports.HypothesisRecord) ports.HypothesisRecord {
	if record.Evidence != nil {
		record.Evidence = append([]string(nil), record.Evidence...)
	}
	return record
}

// Compile-time assertion: productionHypothesisLedger satisfies the port.
var _ ports.HypothesisLedgerPort = (*productionHypothesisLedger)(nil)
