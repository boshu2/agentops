package rpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeQueue marshals real NextWorkEntry structs to one JSONL line each (fixture
// fidelity: the production reader parses exactly what the production type emits).
func writeQueue(t *testing.T, path string, entries ...NextWorkEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadQueueEntries_ReturnsSelectableSkipsConsumed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")
	writeQueue(t, path,
		NextWorkEntry{SourceEpic: "age-a", Items: []NextWorkItem{{Title: "live", Severity: "high"}}},
		NextWorkEntry{SourceEpic: "age-b", Consumed: true, Items: []NextWorkItem{{Title: "done", Severity: "low"}}},
	)

	entries, err := ReadQueueEntries(path)
	if err != nil {
		t.Fatalf("ReadQueueEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].SourceEpic != "age-a" {
		t.Fatalf("expected only the unconsumed age-a entry, got %+v", entries)
	}
}

func TestReadQueueEntries_MissingFileIsNilNil(t *testing.T) {
	entries, err := ReadQueueEntries(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil || entries != nil {
		t.Fatalf("missing file must return nil,nil; got %v / %+v", err, entries)
	}
}

func TestReadUnconsumedItems_RepoFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "next-work.jsonl")
	writeQueue(t, path, NextWorkEntry{SourceEpic: "age-a", Items: []NextWorkItem{
		{Title: "for-agentops", TargetRepo: "agentops", Severity: "high"},
		{Title: "for-other", TargetRepo: "other-repo", Severity: "high"},
		{Title: "for-any", TargetRepo: "*", Severity: "high"},
	}})

	items, err := ReadUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("ReadUnconsumedItems: %v", err)
	}
	// agentops + wildcard pass; other-repo is filtered out.
	titles := map[string]bool{}
	for _, it := range items {
		titles[it.Title] = true
	}
	if !titles["for-agentops"] || !titles["for-any"] || titles["for-other"] {
		t.Fatalf("repo filter wrong: got %v", titles)
	}
}
