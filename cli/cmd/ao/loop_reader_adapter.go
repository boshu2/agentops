// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionLoopReader satisfies ports.LoopReaderPort by reading the
// local .agents/evolve/cycle-history.jsonl file. The path is supplied
// at construction time so tests can plant a fixture in t.TempDir()
// and operator code can construct with the real .agents/evolve/...
// path.
//
// The on-disk record shape includes fields beyond the port's
// CycleEntry struct (timestamps, validator_before/after, etc.); this
// adapter projects to just the CycleEntry fields and discards the
// rest. A future richer port type can be added per the cycle 70
// docs-local-only contract; for now CycleEntry is the read surface.
//
// Per docs/contracts/bc-ports-inventory.md "Per-BC Wire-Up Order"
// section, this is the first BC3 production adapter. Sibling shape:
// cycle 83 productionCitationAdapter (cli/cmd/ao/citation_port_adapter.go).
type productionLoopReader struct {
	path string
}

// newProductionLoopReader returns an adapter reading from path. Empty
// path is allowed (the adapter just returns empty slices and zero
// values — matching the in-memory adapter's nil-runs-is-safe shape).
func newProductionLoopReader(path string) *productionLoopReader {
	return &productionLoopReader{path: path}
}

// rawCycleRecord is the on-disk superset of CycleEntry. Other fields
// (timestamps, milestone, etc.) are accepted by the parser but not
// projected into CycleEntry — the port surface stays narrow.
type rawCycleRecord struct {
	Cycle     int    `json:"cycle"`
	Mode      string `json:"mode"`
	Result    string `json:"result"`
	Commit    string `json:"commit"`
	Milestone string `json:"milestone"`
}

// readEntries parses the on-disk file into []CycleEntry. Returns
// nil + nil error when the file does not exist.
func (r *productionLoopReader) readEntries(ctx context.Context) ([]ports.CycleEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.path == "" {
		return nil, nil
	}
	f, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("productionLoopReader open %q: %w", r.path, err)
	}
	defer f.Close()
	out := make([]ports.CycleEntry, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // tolerate large lines
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec rawCycleRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Tolerate malformed lines (operator may hand-edit) —
			// skip rather than fail the whole read.
			continue
		}
		out = append(out, ports.CycleEntry{
			Number:    rec.Cycle,
			Mode:      rec.Mode,
			Result:    rec.Result,
			Commit:    rec.Commit,
			Milestone: rec.Milestone,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("productionLoopReader scan %q: %w", r.path, err)
	}
	return out, nil
}

// Latest returns the entry with the highest Number.
func (r *productionLoopReader) Latest(ctx context.Context) (ports.CycleEntry, error) {
	entries, err := r.readEntries(ctx)
	if err != nil {
		return ports.CycleEntry{}, err
	}
	if len(entries) == 0 {
		return ports.CycleEntry{}, nil
	}
	best := entries[0]
	for _, e := range entries[1:] {
		if e.Number > best.Number {
			best = e
		}
	}
	return best, nil
}

// Range returns entries whose Number is in [start, end] (inclusive).
func (r *productionLoopReader) Range(ctx context.Context, start, end int) ([]ports.CycleEntry, error) {
	entries, err := r.readEntries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ports.CycleEntry, 0)
	for _, e := range entries {
		if e.Number >= start && e.Number <= end {
			out = append(out, e)
		}
	}
	return out, nil
}

// IdleStreak returns the trailing count of entries whose Result is
// "idle" or "unchanged". File-order is assumed to be Number-ascending
// (matching how the evolve loop appends).
func (r *productionLoopReader) IdleStreak(ctx context.Context) (int, error) {
	entries, err := r.readEntries(ctx)
	if err != nil {
		return 0, err
	}
	streak := 0
	for i := len(entries) - 1; i >= 0; i-- {
		switch entries[i].Result {
		case "idle", "unchanged":
			streak++
		default:
			return streak, nil
		}
	}
	return streak, nil
}

// Compile-time assertion: productionLoopReader satisfies the port.
var _ ports.LoopReaderPort = (*productionLoopReader)(nil)
