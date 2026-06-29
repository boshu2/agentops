package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIngestClaude_WrapsPreservesAndTags verifies the ingest writes ao-learnings
// that (1) preserve the original silo content verbatim, (2) tag tier=machine +
// source=claude-memory, and (3) are idempotent (stable dest name; re-run does not
// duplicate). This is the behavioral contract for age-unified-agent-memory-nyfq.5.
func TestIngestClaude_WrapsPreservesAndTags(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()

	// A Claude per-project memory silo with a curated fact.
	memDir := filepath.Join(source, "-Users-bo-dev-agentops", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "---\nname: only-here-fact\n---\n\nA fact that lives ONLY in the Claude silo: zephyrwidget calibration.\n"
	if err := os.WriteFile(filepath.Join(memDir, "only-here-fact.md"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := claudeMemoryFiles(source)
	if err != nil || len(files) != 1 {
		t.Fatalf("claudeMemoryFiles = %v (err %v), want 1 file", files, err)
	}

	wrapped, err := wrapClaudeMemory(source, files[0])
	if err != nil {
		t.Fatalf("wrapClaudeMemory: %v", err)
	}
	for _, want := range []string{
		"tier: machine",
		"source: claude-memory",
		"type: learning",
		"zephyrwidget calibration", // ORIGINAL content preserved verbatim
		"only-here-fact",           // original frontmatter preserved in body
	} {
		if !strings.Contains(wrapped, want) {
			t.Errorf("wrapped output missing %q", want)
		}
	}

	// Idempotency: the dest name is stable for the same source file.
	n1 := claudeMemoryDestName(source, files[0])
	n2 := claudeMemoryDestName(source, files[0])
	if n1 != n2 {
		t.Errorf("dest name not stable: %q vs %q", n1, n2)
	}
	if !strings.HasPrefix(n1, "claude-memory--") {
		t.Errorf("dest name %q missing claude-memory-- provenance prefix", n1)
	}

	// Write twice → still one file in dest (idempotent overwrite, no dupes).
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(filepath.Join(dest, n1), []byte(wrapped), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 1 {
		t.Errorf("dest has %d files after two ingests, want 1 (idempotent)", len(entries))
	}
}

// TestIngestClaude_MissingSourceIsNoOp verifies a missing ~/.claude/projects is a
// clean no-op, not an error.
func TestIngestClaude_MissingSourceIsNoOp(t *testing.T) {
	files, err := claudeMemoryFiles(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing source returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("missing source returned %d files, want 0", len(files))
	}
}
