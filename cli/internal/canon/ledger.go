// practices: [ddd-bounded-context, evidence-ledger]

package canon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ledger is an append-only JSONL store of attestation records of type T
// (citations, verifications). It mirrors the .agents/ao/citations.jsonl
// convention: one JSON object per line, malformed lines skipped on read so a
// single corrupt record can never strand the whole ledger.
type ledger[T any] struct {
	path string
}

func newLedger[T any](path string) *ledger[T] {
	return &ledger[T]{path: path}
}

// append writes one record as a JSON line, creating the parent directory and
// file on first write.
func (l *ledger[T]) append(record T) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("create ledger dir: %w", err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write record: %w", err)
	}
	return nil
}

// distinctStrings returns the input with duplicates removed, preserving first-
// seen order.
func distinctStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// load replays every record. A missing ledger is an empty ledger, not an error.
func (l *ledger[T]) load() ([]T, error) {
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []T
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record T
		if err := json.Unmarshal(line, &record); err != nil {
			continue // skip malformed line; do not strand the ledger
		}
		out = append(out, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ledger: %w", err)
	}
	return out, nil
}
