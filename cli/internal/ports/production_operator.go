// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// ProductionOperator satisfies OperatorPort by appending
// OperatorIntent records to a local JSONL file (default path is
// supplied at construction; .agents/operator/intents.jsonl is the
// expected convention).
//
// Same on-disk shape as the loop reader/writer pair (cycles 108-109):
// one JSON record per line, tolerate malformed lines on read,
// append-only on write. Process-local mutex serializes Append; cross-
// process concurrent writes are NOT safe (callers needing that
// should layer a flock).
type ProductionOperator struct {
	mu   sync.Mutex
	path string
}

// NewProductionOperator returns an adapter at path. Empty path makes
// the adapter fail-loud on every method call — matches the cycle 109
// loop writer's empty-path posture.
func NewProductionOperator(path string) *ProductionOperator {
	return &ProductionOperator{path: path}
}

// Record appends the intent. Empty Kind is rejected (port contract).
func (a *ProductionOperator) Record(ctx context.Context, intent OperatorIntent) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if intent.Kind == "" {
		return errors.New("ports: OperatorIntent.Kind required")
	}
	if a.path == "" {
		return errors.New("ProductionOperator: path required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	payload, err := json.Marshal(operatorIntentRecord(intent))
	if err != nil {
		return fmt.Errorf("ProductionOperator marshal: %w", err)
	}
	f, err := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("ProductionOperator open %q: %w", a.path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if _, err := f.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("ProductionOperator write: %w", err)
	}
	return nil
}

// List returns all recorded intents, most-recent first. Missing file
// → empty (non-nil) slice. Malformed lines are tolerated (skipped).
func (a *ProductionOperator) List(ctx context.Context) ([]OperatorIntent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]OperatorIntent, 0)
	if a.path == "" {
		return out, nil
	}
	f, err := os.Open(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("ProductionOperator open %q: %w", a.path, err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	in := make([]OperatorIntent, 0)
	for scanner.Scan() {
		var rec operatorIntentRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		in = append(in, OperatorIntent(rec))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ProductionOperator scan: %w", err)
	}
	// Reverse to most-recent first (file is append-order).
	for i := len(in) - 1; i >= 0; i-- {
		out = append(out, in[i])
	}
	return out, nil
}

// operatorIntentRecord is the on-disk shape. Kept narrow — matches
// the port's OperatorIntent struct field-for-field.
type operatorIntentRecord struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Compile-time assertion: ProductionOperator satisfies the port.
var _ OperatorPort = (*ProductionOperator)(nil)
