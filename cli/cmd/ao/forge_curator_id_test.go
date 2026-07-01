package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin writeJSONAtomic to the canonical fsync-safe storage writer
// (storage.AtomicWriteFile). The behavioral discriminators below (parent-dir
// creation, no temp leftover, exact indented content) all hold for the routed
// implementation and would fail against the old fsync-less body — in particular
// the old body's bare os.WriteFile(tmp,...) could not create a missing parent
// directory, so the missing-dir case is the load-bearing routing proof.

func TestWriteJSONAtomic_WritesExactIndentedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job.json")
	job := curatorJob{
		ID:         "20260701T000000Z-ingest-abcd1234",
		Kind:       "ingest-claude-session",
		CreatedAt:  "2026-07-01T00:00:00Z",
		MaxRetries: 3,
		Source:     &curatorJobSource{Path: "session.jsonl", ChunkStart: 0, ChunkEnd: 1},
	}
	if err := writeJSONAtomic(path, job); err != nil {
		t.Fatalf("writeJSONAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Exact on-disk shape: indented JSON with a trailing newline.
	want, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	want = append(want, '\n')
	if string(got) != string(want) {
		t.Fatalf("on-disk content mismatch:\n got: %q\nwant: %q", got, want)
	}

	// Round-trips back to the same struct.
	var back curatorJob
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.ID != job.ID || back.Kind != job.Kind || back.Source == nil || back.Source.Path != "session.jsonl" {
		t.Fatalf("round-trip mismatch: got %+v", back)
	}
}

func TestWriteJSONAtomic_CreatesMissingParentDir(t *testing.T) {
	// Routing proof: storage.AtomicWriteFile os.MkdirAll's the parent, so a write
	// into a not-yet-existing nested directory succeeds. The old fsync-less body
	// (bare os.WriteFile on <path>.tmp) would fail here with a "no such file or
	// directory" error, so this passing asserts the routed implementation.
	dir := t.TempDir()
	path := filepath.Join(dir, "queue", "nested", "job.json")
	if err := writeJSONAtomic(path, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("writeJSONAtomic into missing dir: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), `"k": "v"`) {
		t.Fatalf("content = %q, want it to contain the indented pair", got)
	}
}

func TestWriteJSONAtomic_NoTempLeftover(t *testing.T) {
	// storage.AtomicWriteFile removes its temp file on every path; after a
	// successful write the directory contains exactly the destination file and no
	// stray ".tmp" artifact.
	dir := t.TempDir()
	path := filepath.Join(dir, "job.json")
	if err := writeJSONAtomic(path, map[string]int{"n": 1}); err != nil {
		t.Fatalf("writeJSONAtomic: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory entry count: got %d, want exactly 1 (no temp leftover)", len(entries))
	}
	if entries[0].Name() != "job.json" {
		t.Fatalf("unexpected leftover entry %q", entries[0].Name())
	}
}

func TestWriteJSONAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job.json")
	if err := writeJSONAtomic(path, map[string]string{"v": "old"}); err != nil {
		t.Fatalf("seed writeJSONAtomic: %v", err)
	}
	if err := writeJSONAtomic(path, map[string]string{"v": "new"}); err != nil {
		t.Fatalf("overwrite writeJSONAtomic: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), `"v": "new"`) || strings.Contains(string(got), "old") {
		t.Fatalf("content = %q, want only the new value", got)
	}
}
