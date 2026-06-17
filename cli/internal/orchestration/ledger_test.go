package orchestration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerWriter_IdempotentAppend(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := osMkdirAll(filepath.Join(repo, "docs", "provenance"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := NewLedgerWriter(repo)
	result := InstrumentResult{
		SchemaVersion: InstrumentSchemaVersionV1,
		Command:       InstrumentCommandPreflight,
		Profile:       "tri-vendor",
		RunID:         "run-1",
		Verdict:       Verdict{Status: VerdictStatusPass, Confidence: VerdictConfidenceHigh},
	}
	key := IdempotencyKey(InstrumentCommandPreflight, "tri-vendor", "", "run-1")
	skipped1, err := w.AppendInstrumentEvent(LedgerEventPreflight, key, result)
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	if skipped1 {
		t.Fatal("first append should not skip")
	}
	skipped2, err := w.AppendInstrumentEvent(LedgerEventPreflight, key, result)
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if !skipped2 {
		t.Fatal("second append should be idempotent skip")
	}
}

func TestApplyLedgerFailure(t *testing.T) {
	r := &InstrumentResult{Verdict: Verdict{Status: VerdictStatusPass, Confidence: VerdictConfidenceHigh}}
	ApplyLedgerFailure(r)
	if !r.LedgerUnwritten || r.Verdict.Status != VerdictStatusWarn {
		t.Fatalf("got %+v", r)
	}
}

func osMkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
