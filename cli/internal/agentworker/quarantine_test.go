package agentworker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQuarantineWriterWritesRecordWithSessionRefs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "quarantine", "agentworker")
	writer := QuarantineWriter{
		Dir: dir,
		Now: func() time.Time { return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC) },
	}
	path, err := writer.Write(QuarantineRecord{
		Kind:      "wiki_extraction",
		Reason:    "invalid_worker_output",
		Error:     "invalid wiki extraction JSON",
		JobID:     "wiki.forge:1",
		AttemptID: "attempt-1",
		RequestID: "req-1",
		Session: SessionRef{
			WorkerKind: WorkerKind("codex"),
			Provider:   ProviderGasCity,
			JobID:      "wiki.forge:1",
			AttemptID:  "attempt-1",
			RequestID:  "req-1",
			SessionID:  "sess_quarantine",
			Status:     StatusCompleted,
		},
		Terminal:  TerminalState{Status: StatusCompleted},
		Attempts:  2,
		RawOutput: `{"bad":true}`,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("path dir: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read quarantine: %v", err)
	}
	var record QuarantineRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode quarantine: %v", err)
	}
	if record.Session.SessionID != "sess_quarantine" || record.JobID != "wiki.forge:1" {
		t.Fatalf("record refs: %#v", record)
	}
	if record.Attempts != 2 || record.RawOutput == "" {
		t.Fatalf("record retry/raw: %#v", record)
	}
}
