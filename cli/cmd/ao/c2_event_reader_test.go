package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRPIC2Events_ReadsRunEventLog(t *testing.T) {
	root := t.TempDir()
	runID := "run-abc123"
	runDir := filepath.Join(root, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	// Two valid events + a blank line that must be skipped.
	content := `{"schema_version":1,"event_id":"e1","run_id":"run-abc123","type":"phase_start","timestamp":"2026-06-04T00:00:00Z"}

{"schema_version":1,"event_id":"e2","run_id":"run-abc123","type":"error","message":"boom","timestamp":"2026-06-04T00:01:00Z"}
`
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write events.jsonl: %v", err)
	}

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "phase_start" || events[0].EventID != "e1" {
		t.Errorf("event[0] mismatch: got type=%q id=%q", events[0].Type, events[0].EventID)
	}
	if events[1].Type != "error" || events[1].Message != "boom" {
		t.Errorf("event[1] mismatch: got type=%q message=%q", events[1].Type, events[1].Message)
	}
}

func TestLoadRPIC2Events_EmptyRootReadsRelativeRunEventLog(t *testing.T) {
	root := t.TempDir()
	runID := "run-relative"
	runDir := filepath.Join(root, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	content := `{"schema_version":1,"event_id":"e1","run_id":"run-relative","type":"phase_start","timestamp":"2026-06-04T00:00:00Z"}
`
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write events.jsonl: %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	events, err := loadRPIC2Events("", runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].RunID != runID || events[0].EventID != "e1" {
		t.Errorf("event mismatch: got runID=%q id=%q", events[0].RunID, events[0].EventID)
	}
}

func TestLoadRPIC2Events_MissingFileIsEmpty(t *testing.T) {
	events, err := loadRPIC2Events(t.TempDir(), "no-such-run")
	if err != nil {
		t.Fatalf("expected nil error for missing log, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for missing log, got %d", len(events))
	}
}

func TestLoadRPIC2Events_EmptyRunIDReturnsNil(t *testing.T) {
	events, err := loadRPIC2Events(t.TempDir(), "")
	if err != nil || events != nil {
		t.Fatalf("expected (nil,nil) for empty runID, got (%v,%v)", events, err)
	}
}
