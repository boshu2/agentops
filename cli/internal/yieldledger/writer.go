package yieldledger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Writer appends yield-ledger events as append-only JSONL: ONE json line per
// event, written with O_APPEND so concurrent fungible lanes can each append
// without read-modify-write contention or a shared tmp collision (spec §D).
// Emission is fail-open observability — callers guard each append (`|| true`)
// so it can never block a merge.
type Writer struct {
	// Now supplies the generated_at timestamp; defaults to time.Now when nil.
	Now func() time.Time
}

// nowUTC returns the writer's clock in UTC, defaulting to the wall clock.
func (w Writer) nowUTC() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

// AppendAccept appends one accept event and returns the resulting ledger view.
func (w Writer) AppendAccept(projectRoot string, in AcceptInput) (*Ledger, error) {
	return w.appendValidated(projectRoot, newAcceptEvent(in), "accept")
}

// AppendGateVerdict appends one gate-verdict event and returns the resulting
// ledger view. Before appending it stamps the escape sentinels (EM.2.1): if in is
// an overturning-REFUTED (an escape), an empty domain becomes DomainUnclassified
// and an empty reason becomes ReasonUnspecified, so an escape is never recorded
// without BOTH. This runs at the writer chokepoint so EVERY emitter (CLI, tests,
// future Go callers) gets the guarantee, not just the producer scripts. Best-
// effort: a load failure (e.g. no ledger yet, so no prior CONFIRMED to overturn)
// just skips the stamp.
func (w Writer) AppendGateVerdict(projectRoot string, in GateVerdictInput) (*Ledger, error) {
	existing, loadErr := LoadPath(LedgerPath(projectRoot))
	// A load error here is a corrupt/unreadable EXISTING ledger (a missing ledger
	// loads as empty, not an error) — StampEscapeSentinels fails SAFE in that case
	// so a degraded ledger can never let an escape through without domain+reason.
	in, _ = StampEscapeSentinels(existing, loadErr, in)
	return w.appendValidated(projectRoot, newGateVerdictEvent(in), "gate-verdict")
}

// AppendUsage appends one usage event and returns the resulting ledger view.
func (w Writer) AppendUsage(projectRoot string, in UsageInput) (*Ledger, error) {
	return w.appendValidated(projectRoot, newUsageEvent(in), "usage")
}

// appendValidated validates ev, appends it as one JSONL line, and returns the
// full ledger re-read from disk so the caller sees the post-append state.
func (w Writer) appendValidated(projectRoot string, ev Event, kind string) (*Ledger, error) {
	if defect := validateEvent(ev); defect != "" {
		return nil, fmt.Errorf("invalid %s event: %s", kind, defect)
	}
	path := ledgerPath(projectRoot)
	if err := appendEventLine(path, ev); err != nil {
		return nil, err
	}
	return LoadPath(path)
}

// appendEventLine serializes ev to one canonical JSON line and appends it to the
// ledger with O_APPEND. A single Write of marshal(event)+"\n" keeps the append
// atomic for typical event sizes (well under PIPE_BUF) so concurrent lanes do
// not interleave bytes within a line.
func appendEventLine(path string, ev Event) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create yield ledger directory: %w", err)
	}
	line, err := ev.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal yield event: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open yield ledger for append: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("append yield event: %w", err)
	}
	return nil
}

// ledgerPath resolves the absolute ledger path for a project root.
func ledgerPath(projectRoot string) string {
	return filepath.Join(projectRoot, filepath.FromSlash(ArtifactRelPath))
}
