package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

// writeQueueFile is a small helper that writes the supplied entries to a
// freshly-created next-work.jsonl in a tmp dir and returns its path.
func writeQueueFile(t *testing.T, entries []cliRPI.NextWorkEntry) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")
	var b strings.Builder
	for _, e := range entries {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		b.WriteString(string(raw))
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write queue file: %v", err)
	}
	return path
}

func resetMarkProbedFlags() {
	rpiMarkProbedID = ""
	rpiMarkProbedBy = ""
	rpiMarkProbedQueue = ".agents/rpi/next-work.jsonl"
	rpiMarkProbedAt = ""
}

func TestRPIMarkProbed_StampsMatchingItemByID(t *testing.T) {
	defer resetMarkProbedFlags()

	queue := writeQueueFile(t, []cliRPI.NextWorkEntry{
		{
			SourceEpic: "test-epic",
			Timestamp:  "2026-04-26T00:00:00Z",
			Items: []cliRPI.NextWorkItem{
				{ID: "item-1", Title: "first", Type: "task", Severity: "low", Source: "council-finding", Description: "d1"},
				{ID: "item-2", Title: "second", Type: "task", Severity: "low", Source: "council-finding", Description: "d2"},
			},
		},
	})

	rpiMarkProbedID = "item-2"
	rpiMarkProbedBy = "nightly/2026-04-26-v3"
	rpiMarkProbedQueue = queue
	rpiMarkProbedAt = "2026-04-26T22:30:00Z"

	if err := runRPIMarkProbed(nil, nil); err != nil {
		t.Fatalf("runRPIMarkProbed: %v", err)
	}

	out, err := os.ReadFile(queue)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	var entry cliRPI.NextWorkEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &entry); err != nil {
		t.Fatalf("unmarshal queue line: %v\nraw=%s", err, out)
	}

	if entry.Items[0].ProbedStaleAt != nil {
		t.Errorf("first item should be untouched; got ProbedStaleAt=%v", entry.Items[0].ProbedStaleAt)
	}
	if entry.Items[1].ProbedStaleAt == nil || *entry.Items[1].ProbedStaleAt != "2026-04-26T22:30:00Z" {
		t.Errorf("second item ProbedStaleAt = %v, want %q", entry.Items[1].ProbedStaleAt, "2026-04-26T22:30:00Z")
	}
	if entry.Items[1].ProbedBy == nil || *entry.Items[1].ProbedBy != "nightly/2026-04-26-v3" {
		t.Errorf("second item ProbedBy = %v, want %q", entry.Items[1].ProbedBy, "nightly/2026-04-26-v3")
	}
}

func TestRPIMarkProbed_ErrorsWhenNoMatch(t *testing.T) {
	defer resetMarkProbedFlags()

	queue := writeQueueFile(t, []cliRPI.NextWorkEntry{
		{
			SourceEpic: "epic",
			Timestamp:  "2026-04-26T00:00:00Z",
			Items: []cliRPI.NextWorkItem{
				{ID: "real-id", Title: "t", Type: "task", Severity: "low", Source: "council-finding", Description: "d"},
			},
		},
	})

	rpiMarkProbedID = "missing-id"
	rpiMarkProbedBy = "test"
	rpiMarkProbedQueue = queue

	err := runRPIMarkProbed(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
	if !strings.Contains(err.Error(), "no item matched") {
		t.Errorf("error = %v, want it to mention no match", err)
	}
}

func TestRPIMarkProbed_RequiresIDAndBy(t *testing.T) {
	defer resetMarkProbedFlags()

	rpiMarkProbedID = ""
	rpiMarkProbedBy = "x"
	if err := runRPIMarkProbed(nil, nil); err == nil || !strings.Contains(err.Error(), "--id is required") {
		t.Errorf("missing --id: err=%v", err)
	}

	rpiMarkProbedID = "x"
	rpiMarkProbedBy = ""
	if err := runRPIMarkProbed(nil, nil); err == nil || !strings.Contains(err.Error(), "--by is required") {
		t.Errorf("missing --by: err=%v", err)
	}
}

func TestRPIMarkProbed_RejectsNonRFC3339At(t *testing.T) {
	defer resetMarkProbedFlags()

	queue := writeQueueFile(t, []cliRPI.NextWorkEntry{
		{
			SourceEpic: "epic",
			Timestamp:  "2026-04-26T00:00:00Z",
			Items: []cliRPI.NextWorkItem{
				{ID: "id", Title: "t", Type: "task", Severity: "low", Source: "council-finding", Description: "d"},
			},
		},
	})

	rpiMarkProbedID = "id"
	rpiMarkProbedBy = "test"
	rpiMarkProbedQueue = queue
	rpiMarkProbedAt = "2026/04/26 22:30:00"

	err := runRPIMarkProbed(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("expected RFC3339 error, got %v", err)
	}
}
