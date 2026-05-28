package agentworker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestValidateQuarantineRecord(t *testing.T) {
	validRecord := QuarantineRecord{
		SchemaVersion: QuarantineSchemaVersion,
		Kind:          "test_kind",
		Reason:        "test_reason",
		Error:         "test_error",
		RawOutput:     "raw output data",
		Attempts:      1,
		Session: SessionRef{
			WorkerKind: WorkerKind("codex"),
			Provider:   ProviderGasCity,
			SessionID:  "sess-1",
		},
	}

	if err := validateQuarantineRecord(validRecord); err != nil {
		t.Fatalf("valid record should pass: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(r *QuarantineRecord)
		errMsg string
	}{
		{
			name:   "wrong schema version",
			mutate: func(r *QuarantineRecord) { r.SchemaVersion = 99 },
			errMsg: "schema_version must be",
		},
		{
			name:   "empty kind",
			mutate: func(r *QuarantineRecord) { r.Kind = "" },
			errMsg: "kind is required",
		},
		{
			name:   "whitespace-only kind",
			mutate: func(r *QuarantineRecord) { r.Kind = "   " },
			errMsg: "kind is required",
		},
		{
			name:   "empty reason",
			mutate: func(r *QuarantineRecord) { r.Reason = "" },
			errMsg: "reason is required",
		},
		{
			name:   "empty error",
			mutate: func(r *QuarantineRecord) { r.Error = "" },
			errMsg: "error is required",
		},
		{
			name:   "empty raw_output",
			mutate: func(r *QuarantineRecord) { r.RawOutput = "" },
			errMsg: "raw_output is required",
		},
		{
			name:   "zero attempts",
			mutate: func(r *QuarantineRecord) { r.Attempts = 0 },
			errMsg: "attempts must be positive",
		},
		{
			name:   "negative attempts",
			mutate: func(r *QuarantineRecord) { r.Attempts = -1 },
			errMsg: "attempts must be positive",
		},
		{
			name:   "invalid session ref",
			mutate: func(r *QuarantineRecord) { r.Session.SessionID = "" },
			errMsg: "session",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord
			tt.mutate(&r)
			err := validateQuarantineRecord(r)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q should contain %q", err, tt.errMsg)
			}
		})
	}
}

func TestQuarantineFileName(t *testing.T) {
	tests := []struct {
		name   string
		record QuarantineRecord
		want   string
	}{
		{
			name: "uses session ID",
			record: QuarantineRecord{
				CreatedAt: time.Date(2026, 5, 27, 14, 30, 0, 0, time.UTC),
				Session:   SessionRef{SessionID: "my-session"},
			},
			want: "20260527T143000Z-my-session.json",
		},
		{
			name: "falls back to job ID",
			record: QuarantineRecord{
				CreatedAt: time.Date(2026, 5, 27, 14, 30, 0, 0, time.UTC),
				JobID:     "wiki.forge:1",
			},
			want: "20260527T143000Z-wiki-forge-1.json",
		},
		{
			name: "falls back to request ID",
			record: QuarantineRecord{
				CreatedAt: time.Date(2026, 5, 27, 14, 30, 0, 0, time.UTC),
				RequestID: "req-abc",
			},
			want: "20260527T143000Z-req-abc.json",
		},
		{
			name: "falls back to default",
			record: QuarantineRecord{
				CreatedAt: time.Date(2026, 5, 27, 14, 30, 0, 0, time.UTC),
			},
			want: "20260527T143000Z-worker-output.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quarantineFileName(tt.record)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeQuarantineFragment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple-name", "simple-name"},
		{"UPPER_case", "upper_case"},
		{"special!@#chars", "special---chars"},
		{"---leading-trailing---", "leading-trailing"},
		{"", "worker-output"},
		{"   ", "worker-output"},
		{"a/b/c", "a-b-c"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeQuarantineFragment(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeQuarantineFragment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuarantineWriterEmptyDir(t *testing.T) {
	writer := QuarantineWriter{Dir: "", Now: time.Now}
	_, err := writer.Write(QuarantineRecord{Kind: "test"})
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
	if !strings.Contains(err.Error(), "dir is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQuarantineWriterSetsDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "q")
	fixedTime := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	writer := QuarantineWriter{
		Dir: dir,
		Now: func() time.Time { return fixedTime },
	}

	path, err := writer.Write(QuarantineRecord{
		Kind:      "test",
		Reason:    "reason",
		Error:     "error",
		RawOutput: "output",
		Attempts:  1,
		Session: SessionRef{
			WorkerKind: "codex",
			Provider:   ProviderGasCity,
			SessionID:  "s1",
		},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record QuarantineRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != QuarantineSchemaVersion {
		t.Errorf("SchemaVersion: got %d, want %d", record.SchemaVersion, QuarantineSchemaVersion)
	}
	if !record.CreatedAt.Equal(fixedTime) {
		t.Errorf("CreatedAt: got %v, want %v", record.CreatedAt, fixedTime)
	}
}
