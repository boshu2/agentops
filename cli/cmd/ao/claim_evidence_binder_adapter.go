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

// productionClaimEvidenceBinder satisfies ports.ClaimEvidenceBinderPort
// by appending EvidenceBinding records to a JSONL file (typically
// .agents/findings/evidence-bindings.jsonl).
//
// Semantics:
//   - Append-only on disk. List replays the file and folds it into the
//     current view (latest binding per Claim+Path wins; level can only
//     go up).
//   - Bind enforces idempotency + upgrade-only by reading the current
//     file once under lock, checking the existing binding for the same
//     (Claim, Path) pair, rejecting downgrades, and skipping the write
//     when the level is unchanged AND Anchors are identical.
//   - Empty Claim or empty Path is a structural-rejection error per
//     port contract.
//   - List returns bindings most-recently-bound first (replayed file
//     reversed).
//
// Shape: same JSONL pattern proven across cycles 108-110.
type productionClaimEvidenceBinder struct {
	mu   sync.Mutex
	path string
}

func newProductionClaimEvidenceBinder(path string) *productionClaimEvidenceBinder {
	return &productionClaimEvidenceBinder{path: path}
}

// Bind appends a binding after the upgrade-only check.
func (b *productionClaimEvidenceBinder) Bind(ctx context.Context, binding ports.EvidenceBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if binding.Claim == "" {
		return errors.New("productionClaimEvidenceBinder: Claim required")
	}
	if binding.Path == "" {
		return errors.New("productionClaimEvidenceBinder: Path required")
	}
	if b.path == "" {
		return errors.New("productionClaimEvidenceBinder: file path required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	existing, err := b.scanLatestFor(binding.Claim, binding.Path)
	if err != nil {
		return err
	}
	if existing != nil {
		if evidenceLevelRank(binding.Level) < evidenceLevelRank(existing.Level) {
			return fmt.Errorf("productionClaimEvidenceBinder: refusing downgrade %s → %s for claim %q path %q",
				existing.Level, binding.Level, binding.Claim, binding.Path)
		}
		if binding.Level == existing.Level && anchorsEqual(binding.Anchors, existing.Anchors) {
			return nil // pure idempotent no-op
		}
	}

	payload, err := json.Marshal(evidenceBindingRecord{
		Claim:   string(binding.Claim),
		Path:    binding.Path,
		Level:   string(binding.Level),
		Anchors: binding.Anchors,
	})
	if err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder marshal: %w", err)
	}
	f, err := os.OpenFile(b.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder open %q: %w", b.path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("productionClaimEvidenceBinder write: %w", err)
	}
	return nil
}

// List returns all bindings, most-recent first.
func (b *productionClaimEvidenceBinder) List(ctx context.Context) ([]ports.EvidenceBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	all, err := b.scanAllLocked()
	if err != nil {
		return nil, err
	}
	out := make([]ports.EvidenceBinding, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		out = append(out, all[i])
	}
	return out, nil
}

// scanLatestFor walks the file and returns the most recent binding
// for the given (claim, path), or nil if none. Assumes caller holds
// b.mu.
func (b *productionClaimEvidenceBinder) scanLatestFor(claim ports.ClaimID, path string) (*ports.EvidenceBinding, error) {
	all, err := b.scanAllLocked()
	if err != nil {
		return nil, err
	}
	var found *ports.EvidenceBinding
	for i := range all {
		if all[i].Claim == claim && all[i].Path == path {
			cp := all[i]
			found = &cp
		}
	}
	return found, nil
}

// scanAllLocked reads the file in order. Missing file → empty slice.
func (b *productionClaimEvidenceBinder) scanAllLocked() ([]ports.EvidenceBinding, error) {
	out := make([]ports.EvidenceBinding, 0)
	f, err := os.Open(b.path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("productionClaimEvidenceBinder open %q: %w", b.path, err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var rec evidenceBindingRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		out = append(out, ports.EvidenceBinding{
			Claim:   ports.ClaimID(rec.Claim),
			Path:    rec.Path,
			Level:   ports.EvidenceLevel(rec.Level),
			Anchors: rec.Anchors,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("productionClaimEvidenceBinder scan: %w", err)
	}
	return out, nil
}

// evidenceBindingRecord is the on-disk shape.
type evidenceBindingRecord struct {
	Claim   string   `json:"claim"`
	Path    string   `json:"path"`
	Level   string   `json:"level,omitempty"`
	Anchors []string `json:"anchors,omitempty"`
}

// evidenceLevelRank gives EvidenceLevel an integer ordering for the
// upgrade-only check. None < PG1 < PG2 < PG3 < PG4.
func evidenceLevelRank(l ports.EvidenceLevel) int {
	switch l {
	case ports.EvidenceLevelPG1:
		return 1
	case ports.EvidenceLevelPG2:
		return 2
	case ports.EvidenceLevelPG3:
		return 3
	case ports.EvidenceLevelPG4:
		return 4
	}
	return 0
}

func anchorsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Compile-time assertion: productionClaimEvidenceBinder satisfies the port.
var _ ports.ClaimEvidenceBinderPort = (*productionClaimEvidenceBinder)(nil)
