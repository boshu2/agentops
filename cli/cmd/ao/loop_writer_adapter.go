// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionLoopWriter satisfies ports.LoopWriterPort by appending
// CycleEntry records to the local .agents/evolve/cycle-history.jsonl
// file (or any path the caller supplies). Sibling of cycle 108's
// productionLoopReader — together they let evolve's Step 0/6
// bookkeeping flow through typed Go entry points.
//
// File semantics:
//   - Append-only: each call adds one JSON line to the end of the
//     file. The file is created with 0o644 if it doesn't exist.
//   - Number assignment: when entry.Number == 0, the adapter reads
//     the existing file to find max(Number)+1 (matching the
//     InMemoryLoopWriter contract). Explicit Number is honored as-is
//     but the adapter does NOT scan the file for duplicate
//     prevention — file-backed appends trust the caller for that
//     check (the InMemoryLoopWriter still enforces it for tests).
//   - Thread safety: a process-local mutex serializes appends from
//     this adapter instance. Cross-process concurrent appends are
//     NOT safe — adapters that need that should layer a flock.
type productionLoopWriter struct {
	mu   sync.Mutex
	path string
}

// newProductionLoopWriter returns an adapter writing to path. Empty
// path returns an adapter whose Append always errors — matches the
// "fail loud" posture of the in-memory adapter's empty-claim
// rejection.
func newProductionLoopWriter(path string) *productionLoopWriter {
	return &productionLoopWriter{path: path}
}

// Append writes the entry as one JSON line. Auto-assigns Number when
// 0 by scanning the existing file for the current max.
func (w *productionLoopWriter) Append(ctx context.Context, entry ports.CycleEntry) (ports.CycleEntry, error) {
	if err := ctx.Err(); err != nil {
		return ports.CycleEntry{}, err
	}
	if w.path == "" {
		return ports.CycleEntry{}, fmt.Errorf("productionLoopWriter: path required")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if entry.Number == 0 {
		maxN, err := w.scanMaxNumberLocked()
		if err != nil {
			return ports.CycleEntry{}, err
		}
		entry.Number = maxN + 1
	}
	rec := loopWriterRecord{
		Cycle:     entry.Number,
		Mode:      entry.Mode,
		Result:    entry.Result,
		Commit:    entry.Commit,
		Milestone: entry.Milestone,
		StartedAt: entry.StartedAt,
		Title:     entry.Title,
		Trace:     entry.Trace,
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return ports.CycleEntry{}, fmt.Errorf("productionLoopWriter marshal: %w", err)
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return ports.CycleEntry{}, fmt.Errorf("productionLoopWriter open %q: %w", w.path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(payload, '\n')); err != nil {
		return ports.CycleEntry{}, fmt.Errorf("productionLoopWriter write: %w", err)
	}
	return entry, nil
}

// loopWriterRecord is the on-disk shape this adapter writes. Matches
// productionLoopReader's rawCycleRecord (cycle 108) so reader/writer
// round-trip works. Widened with StartedAt+Title in cycle 162 to
// follow the cycle-161 CycleEntry widening (soc-ckc4).
type loopWriterRecord struct {
	Cycle     int               `json:"cycle"`
	Mode      string            `json:"mode,omitempty"`
	Result    string            `json:"result,omitempty"`
	Commit    string            `json:"commit,omitempty"`
	Milestone string            `json:"milestone,omitempty"`
	StartedAt string            `json:"started_at,omitempty"`
	Title     string            `json:"title,omitempty"`
	Trace     *ports.CycleTrace `json:"trace,omitempty"`
}

// scanMaxNumberLocked reads the file to find max(Cycle). Returns 0
// for missing or empty files. Assumes the caller holds w.mu.
func (w *productionLoopWriter) scanMaxNumberLocked() (int, error) {
	f, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("productionLoopWriter scan-max open %q: %w", w.path, err)
	}
	defer func() { _ = f.Close() }()
	maxN := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var rec loopWriterRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.Cycle > maxN {
			maxN = rec.Cycle
		}
	}
	return maxN, scanner.Err()
}

// Compile-time assertion: productionLoopWriter satisfies the port.
var _ ports.LoopWriterPort = (*productionLoopWriter)(nil)
