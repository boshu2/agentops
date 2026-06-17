// practices: [in-toto-provenance, design-by-contract]
package orchestration

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// LedgerWriter appends orchestration instrument events to the provenance ledger.
type LedgerWriter struct {
	RepoRoot string
	Store    *provenancegraph.Store
	Now      func() time.Time
}

// NewLedgerWriter returns a writer targeting docs/provenance/ledger.jsonl under repoRoot.
func NewLedgerWriter(repoRoot string) LedgerWriter {
	path := filepath.Join(repoRoot, provenancegraph.LedgerRelativePath)
	return LedgerWriter{
		RepoRoot: repoRoot,
		Store:    provenancegraph.NewStore(path),
		Now:      time.Now,
	}
}

// AppendInstrumentEvent writes an orchestration instrument event. Idempotency uses
// the full idempotency key as from_id. Returns skipped=true when duplicate.
func (w LedgerWriter) AppendInstrumentEvent(eventType, idempotencyKey string, result InstrumentResult) (skipped bool, err error) {
	if w.Store == nil {
		return false, fmt.Errorf("ledger store is nil")
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return false, fmt.Errorf("marshal instrument payload: %w", err)
	}
	target := result.Profile
	if target == "" {
		target = result.Session
	}
	if target == "" {
		target = "orchestration"
	}
	ts := ""
	if w.Now != nil {
		ts = w.Now().UTC().Format(time.RFC3339)
	}
	edge := provenancegraph.Edge{
		FromID:      idempotencyKey,
		FromType:    "agent",
		ToID:        target,
		ToType:      "artifact",
		Relation:    "wasAttributedTo",
		EvidenceRef: eventType + "|" + string(payload),
		TrustTier:   "authored",
		TS:          ts,
	}
	res, err := w.Store.Append(edge)
	if err != nil {
		return false, err
	}
	return res.Skipped, nil
}

// WriteInstrumentLedger attempts ledger append; on failure marks result ledger_unwritten.
func WriteInstrumentLedger(w LedgerWriter, eventType, idempotencyKey string, result *InstrumentResult) {
	if result == nil {
		return
	}
	result.LedgerEventType = eventType
	_, err := w.AppendInstrumentEvent(eventType, idempotencyKey, *result)
	if err != nil {
		ApplyLedgerFailure(result)
	}
}
