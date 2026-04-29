package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixedLedgerTime = "2026-04-28T12:00:00Z"

func TestLedgerAppendRead(t *testing.T) {
	store := NewStore(t.TempDir())

	first, err := store.AppendLedgerEvent(testLedgerEvent("evt-1", EventJobAccepted))
	if err != nil {
		t.Fatalf("append first event: %v", err)
	}
	second, err := store.AppendLedgerEvent(testLedgerEvent("evt-2", EventJobClaimed))
	if err != nil {
		t.Fatalf("append second event: %v", err)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("read %d events, want 2", len(events))
	}
	if events[0].EventID != first.EventID || events[1].EventID != second.EventID {
		t.Fatalf("events read out of append order: %#v", events)
	}
	if got := events[0].Payload["source"]; got != "test" {
		t.Fatalf("payload source = %#v, want test", got)
	}

	data, err := os.ReadFile(store.LedgerPath())
	if err != nil {
		t.Fatalf("read ledger file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("ledger file has %d jsonl records, want 2", len(lines))
	}
	for i, line := range lines {
		var event LedgerEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("ledger line %d is not valid json: %v", i+1, err)
		}
	}
}

func TestLedgerAppendRejectsInvalidWithoutPartialWrite(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.AppendLedgerEvent(testLedgerEvent("evt-1", EventJobAccepted)); err != nil {
		t.Fatalf("append valid event: %v", err)
	}
	before, err := os.ReadFile(store.LedgerPath())
	if err != nil {
		t.Fatalf("read ledger before invalid append: %v", err)
	}

	invalid := testLedgerEvent("evt-2", EventJobClaimed)
	invalid.Actor = ""
	if _, err := store.AppendLedgerEvent(invalid); err == nil {
		t.Fatal("invalid append succeeded")
	}

	after, err := os.ReadFile(store.LedgerPath())
	if err != nil {
		t.Fatalf("read ledger after invalid append: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid append changed ledger\nbefore: %s\nafter: %s", before, after)
	}
}

func TestLedgerIdempotentAppend(t *testing.T) {
	store := NewStore(t.TempDir())
	original := testLedgerEvent("evt-1", EventJobAccepted)
	first, err := store.AppendLedgerEvent(original)
	if err != nil {
		t.Fatalf("append original: %v", err)
	}

	duplicate := testLedgerEvent("evt-1", EventJobClaimed)
	duplicate.Payload["source"] = "duplicate"
	second, err := store.AppendLedgerEvent(duplicate)
	if err != nil {
		t.Fatalf("append duplicate: %v", err)
	}
	if second.EventType != first.EventType {
		t.Fatalf("duplicate append returned event type %q, want original %q", second.EventType, first.EventType)
	}
	if got := second.Payload["source"]; got != "test" {
		t.Fatalf("duplicate append returned payload source %#v, want original payload", got)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("read %d events after duplicate append, want 1", len(events))
	}
}

func TestReplayLedgerDeduplicatesDuplicateEventID(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	first := testLedgerEvent("evt-1", EventJobAccepted)
	duplicate := testLedgerEvent("evt-1", EventJobClaimed)
	duplicate.Payload["source"] = "duplicate"
	second := testLedgerEvent("evt-2", EventJobCompleted)
	data := strings.Join([]string{
		mustLedgerLine(t, first),
		mustLedgerLine(t, duplicate),
		mustLedgerLine(t, second),
		"",
	}, "\n")
	if err := os.WriteFile(store.LedgerPath(), []byte(data), 0600); err != nil {
		t.Fatalf("write ledger fixture: %v", err)
	}

	replay, err := store.ReplayLedger()
	if err != nil {
		t.Fatalf("replay ledger: %v", err)
	}
	if len(replay.Events) != 2 {
		t.Fatalf("replayed %d unique events, want 2", len(replay.Events))
	}
	if replay.Events[0].EventType != EventJobAccepted {
		t.Fatalf("duplicate event id did not preserve first event: %#v", replay.Events[0])
	}
	if len(replay.Corrupt) != 0 {
		t.Fatalf("replay marked valid duplicate fixture corrupt: %#v", replay.Corrupt)
	}
}

func TestCorruptLedgerRecordsAreQuarantined(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	valid := testLedgerEvent("evt-1", EventJobAccepted)
	missingActor := testLedgerEvent("evt-2", EventJobClaimed)
	missingActor.Actor = ""
	done := testLedgerEvent("evt-3", EventJobCompleted)
	data := strings.Join([]string{
		mustLedgerLine(t, valid),
		"{not-json",
		mustLedgerLine(t, missingActor),
		mustLedgerLine(t, done),
		"",
	}, "\n")
	if err := os.WriteFile(store.LedgerPath(), []byte(data), 0600); err != nil {
		t.Fatalf("write corrupt ledger fixture: %v", err)
	}

	replay, err := store.ReplayLedger()
	if err != nil {
		t.Fatalf("replay corrupt ledger: %v", err)
	}
	if len(replay.Events) != 2 {
		t.Fatalf("replayed %d valid events, want 2", len(replay.Events))
	}
	if len(replay.Corrupt) != 2 {
		t.Fatalf("found %d corrupt records, want 2: %#v", len(replay.Corrupt), replay.Corrupt)
	}
	for _, record := range replay.Corrupt {
		if record.QuarantinePath == "" {
			t.Fatalf("corrupt record missing quarantine path: %#v", record)
		}
		if _, err := os.Stat(record.QuarantinePath); err != nil {
			t.Fatalf("stat quarantine record %s: %v", record.QuarantinePath, err)
		}
		if filepath.Dir(record.QuarantinePath) != store.QuarantineDir() {
			t.Fatalf("quarantine path %s outside %s", record.QuarantinePath, store.QuarantineDir())
		}
	}
}

func testLedgerEvent(eventID string, eventType EventType) LedgerEvent {
	return LedgerEvent{
		SchemaVersion: LedgerSchemaVersion,
		EventID:       eventID,
		RequestID:     "req-1",
		JobID:         "job-1",
		EventType:     eventType,
		OccurredAt:    fixedLedgerTime,
		Actor:         "agentopsd",
		Payload: map[string]any{
			"source": "test",
		},
	}
}

func mustLedgerLine(t *testing.T, event LedgerEvent) string {
	t.Helper()
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal ledger fixture: %v", err)
	}
	return string(data)
}
