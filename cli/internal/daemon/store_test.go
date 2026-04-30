package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// TestLedgerRotation drives the size-cap path: append enough events that the
// active ledger crosses the configured threshold, assert that an archive
// appears, the active file shrinks, and ReplayLedger still returns every event
// in append order across the rotation boundary.
func TestLedgerRotation(t *testing.T) {
	store := NewStore(t.TempDir()).WithLedgerMaxBytes(1024)

	const totalEvents = 30
	for i := 0; i < totalEvents; i++ {
		ev := testLedgerEvent(fmt.Sprintf("evt-%03d", i), EventJobAccepted)
		if _, err := store.AppendLedgerEvent(ev); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	archives, err := store.LedgerArchivePaths()
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) == 0 {
		t.Fatalf("expected at least one archive after %d appends, got 0", totalEvents)
	}
	for _, a := range archives {
		if !strings.HasSuffix(a, ".jsonl.gz") {
			t.Fatalf("archive %s missing .jsonl.gz suffix", a)
		}
	}

	info, err := os.Stat(store.LedgerPath())
	if err != nil {
		t.Fatalf("stat active ledger: %v", err)
	}
	if info.Size() > 1024 {
		t.Fatalf("active ledger size %d > cap 1024 after rotation", info.Size())
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger across archives: %v", err)
	}
	if len(events) != totalEvents {
		t.Fatalf("read %d events, want %d (rotation lost data)", len(events), totalEvents)
	}
	for i, ev := range events {
		want := fmt.Sprintf("evt-%03d", i)
		if ev.EventID != want {
			t.Fatalf("events[%d].EventID = %q, want %q (rotation reordered)", i, ev.EventID, want)
		}
	}
}

// TestRotateWhileAppendingDoesNotLoseEvents stresses the lock contract:
// concurrent appends across multiple goroutines must produce a complete,
// deduplicated event stream after replay even when rotation triggers
// repeatedly mid-flight.
func TestRotateWhileAppendingDoesNotLoseEvents(t *testing.T) {
	store := NewStore(t.TempDir()).WithLedgerMaxBytes(512)

	const writers = 4
	const perWriter = 25
	totalUnique := writers * perWriter

	var wg sync.WaitGroup
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		w := w
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				ev := testLedgerEvent(fmt.Sprintf("w%d-%03d", w, i), EventJobAccepted)
				if _, err := store.AppendLedgerEvent(ev); err != nil {
					t.Errorf("writer %d append %d: %v", w, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	archives, err := store.LedgerArchivePaths()
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) == 0 {
		t.Fatalf("rotation never fired during concurrent stress")
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("replay after concurrent appends: %v", err)
	}
	if len(events) != totalUnique {
		t.Fatalf("replayed %d events, want %d (concurrent rotation lost data)", len(events), totalUnique)
	}
	seen := map[string]struct{}{}
	for _, ev := range events {
		if _, dup := seen[ev.EventID]; dup {
			t.Fatalf("duplicate event ID after rotation+replay: %s", ev.EventID)
		}
		seen[ev.EventID] = struct{}{}
	}
}

// TestReplayReadsArchivesInChronologicalOrder seeds two pre-rotated archive
// files plus a current ledger and asserts replay yields events in the
// archive-then-current order — protects against alphabetical re-sort regressions.
func TestReplayReadsArchivesInChronologicalOrder(t *testing.T) {
	store := NewStore(t.TempDir()).WithLedgerMaxBytes(0) // rotation disabled
	if err := os.MkdirAll(store.Dir(), 0700); err != nil {
		t.Fatalf("create dir: %v", err)
	}

	old := mustLedgerLine(t, testLedgerEvent("old-1", EventJobAccepted))
	mid := mustLedgerLine(t, testLedgerEvent("mid-1", EventJobClaimed))
	current := mustLedgerLine(t, testLedgerEvent("now-1", EventJobCompleted))

	oldPath := filepath.Join(store.Dir(), "ledger.20260101T000000.000000000Z.jsonl")
	midPath := filepath.Join(store.Dir(), "ledger.20260201T000000.000000000Z.jsonl")
	if err := os.WriteFile(oldPath, []byte(old+"\n"), 0600); err != nil {
		t.Fatalf("write old archive: %v", err)
	}
	if err := os.WriteFile(midPath, []byte(mid+"\n"), 0600); err != nil {
		t.Fatalf("write mid archive: %v", err)
	}
	if err := os.WriteFile(store.LedgerPath(), []byte(current+"\n"), 0600); err != nil {
		t.Fatalf("write current ledger: %v", err)
	}

	events, err := store.ReadLedger()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events across archives, want 3", len(events))
	}
	wantOrder := []string{"old-1", "mid-1", "now-1"}
	for i, want := range wantOrder {
		if events[i].EventID != want {
			t.Fatalf("events[%d].EventID = %q, want %q", i, events[i].EventID, want)
		}
	}
}
