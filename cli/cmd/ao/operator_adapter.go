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

// productionOperator satisfies ports.OperatorPort by appending
// OperatorIntent records to a local JSONL file (default path is
// supplied at construction; .agents/operator/intents.jsonl is the
// expected convention).
//
// Same on-disk shape as the loop reader/writer pair (cycles 108-109):
// one JSON record per line, tolerate malformed lines on read,
// append-only on write. Process-local mutex serializes Append; cross-
// process concurrent writes are NOT safe (callers needing that
// should layer a flock).
type productionOperator struct {
	mu   sync.Mutex
	path string
}

// newProductionOperator returns an adapter at path. Empty path makes
// the adapter fail-loud on every method call — matches the cycle 109
// loop writer's empty-path posture.
func newProductionOperator(path string) *productionOperator {
	return &productionOperator{path: path}
}

// Record appends the intent. Empty Kind is rejected (port contract).
func (a *productionOperator) Record(ctx context.Context, intent ports.OperatorIntent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if intent.Kind == "" {
		return errors.New("ports: OperatorIntent.Kind required")
	}
	if a.path == "" {
		return errors.New("productionOperator: path required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	payload, err := json.Marshal(operatorIntentRecord{
		Kind:    intent.Kind,
		Subject: intent.Subject,
		Note:    intent.Note,
	})
	if err != nil {
		return fmt.Errorf("productionOperator marshal: %w", err)
	}
	f, err := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("productionOperator open %q: %w", a.path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("productionOperator write: %w", err)
	}
	return nil
}

// List returns all recorded intents, most-recent first. Missing file
// → empty (non-nil) slice. Malformed lines are tolerated (skipped).
func (a *productionOperator) List(ctx context.Context) ([]ports.OperatorIntent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]ports.OperatorIntent, 0)
	if a.path == "" {
		return out, nil
	}
	f, err := os.Open(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("productionOperator open %q: %w", a.path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	in := make([]ports.OperatorIntent, 0)
	for scanner.Scan() {
		var rec operatorIntentRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		in = append(in, ports.OperatorIntent{
			Kind:    rec.Kind,
			Subject: rec.Subject,
			Note:    rec.Note,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("productionOperator scan: %w", err)
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

// Compile-time assertion: productionOperator satisfies the port.
var _ ports.OperatorPort = (*productionOperator)(nil)
