package verdictledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Writer appends verdict ledger records without clobbering existing records.
// It writes the whole ledger document atomically (tmp + rename), mirroring the
// scenarioresults artifact-writer style.
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

// AppendIteration appends one iteration record for a directive and returns the
// updated ledger. Existing records are preserved in append order.
func (w Writer) AppendIteration(projectRoot string, in IterationInput) (*Ledger, error) {
	rec := newIterationRecord(in)
	if defect := validateRecord(rec); defect != "" {
		return nil, fmt.Errorf("invalid iteration record: %s", defect)
	}
	return w.appendRecord(projectRoot, rec)
}

// AppendCooldown appends one re-steer cooldown record and returns the updated
// ledger.
func (w Writer) AppendCooldown(projectRoot string, in CooldownInput) (*Ledger, error) {
	rec := newCooldownRecord(in)
	if defect := validateRecord(rec); defect != "" {
		return nil, fmt.Errorf("invalid cooldown record: %s", defect)
	}
	return w.appendRecord(projectRoot, rec)
}

// appendRecord loads the existing ledger, appends rec, and writes atomically.
func (w Writer) appendRecord(projectRoot string, rec Record) (*Ledger, error) {
	path := ledgerPath(projectRoot)

	existing, err := w.readExistingRecords(path)
	if err != nil {
		return nil, err
	}

	ledger := &Ledger{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   w.nowUTC().Format(time.RFC3339),
		Records:       append(existing, rec),
	}
	if err := writeLedger(path, ledger); err != nil {
		return nil, err
	}
	return ledger, nil
}

// readExistingRecords loads prior records, returning nil when the ledger is
// absent. A malformed ledger is a hard error so a writer never silently drops
// prior records.
func (w Writer) readExistingRecords(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read existing verdict ledger %s: %w", path, err)
	}
	var prior Ledger
	if err := json.Unmarshal(data, &prior); err != nil {
		return nil, fmt.Errorf("parse existing verdict ledger %s: %w", path, err)
	}
	return prior.Records, nil
}

// ledgerPath resolves the absolute ledger path for a project root.
func ledgerPath(projectRoot string) string {
	return filepath.Join(projectRoot, filepath.FromSlash(ArtifactRelPath))
}

// writeLedger serializes ledger to path atomically via tmp + rename.
func writeLedger(path string, ledger *Ledger) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create verdict ledger directory: %w", err)
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verdict ledger: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write verdict ledger tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename verdict ledger artifact: %w", err)
	}
	return nil
}
