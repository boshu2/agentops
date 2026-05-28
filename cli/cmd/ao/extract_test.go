// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// TestExtractSingleEntry tests default behavior (process most recent only).
func TestExtractSingleEntry(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	// Create test entries
	entries := []PendingExtraction{
		{
			SessionID:   "session-old",
			Summary:     "Old session",
			QueuedAt:    time.Now().Add(-2 * time.Hour),
			SessionPath: "/old",
		},
		{
			SessionID:   "session-recent",
			Summary:     "Recent session",
			QueuedAt:    time.Now().Add(-1 * time.Hour),
			SessionPath: "/recent",
		},
		{
			SessionID:   "session-latest",
			Summary:     "Latest session",
			QueuedAt:    time.Now(),
			SessionPath: "/latest",
		},
	}

	// Write entries to pending file
	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	// Change to temp dir
	t.Chdir(tempDir)

	// Run extract (without --all)
	extractAll = false
	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract failed: %v", err)
	}

	// Read remaining entries
	remaining, err := readPendingExtractions(pendingPath)
	if err != nil {
		t.Fatalf("read pending after extract: %v", err)
	}

	// Should have 2 remaining (oldest 2)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining entries, got %d", len(remaining))
	}

	// Verify the latest was removed
	for _, e := range remaining {
		if e.SessionID == "session-latest" {
			t.Errorf("latest session should have been removed")
		}
	}
}

// TestExtractAllSuccess tests --all flag with all entries succeeding.
func TestExtractAllSuccess(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	entries := []PendingExtraction{
		{SessionID: "session-1", Summary: "Session 1", QueuedAt: time.Now()},
		{SessionID: "session-2", Summary: "Session 2", QueuedAt: time.Now()},
		{SessionID: "session-3", Summary: "Session 3", QueuedAt: time.Now()},
	}

	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	t.Chdir(tempDir)

	// Run with --all
	extractAll = true
	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract --all failed: %v", err)
	}

	// File should be empty (all processed)
	remaining, err := readPendingExtractions(pendingPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read pending after extract: %v", err)
	}

	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining entries, got %d", len(remaining))
	}
}

// TestExtractAllEmpty tests --all with empty pending file.
func TestExtractAllEmpty(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	// Create empty pending file
	if err := writePendingFile(pendingPath, []PendingExtraction{}); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	t.Chdir(tempDir)

	extractAll = true
	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract --all on empty file should succeed: %v", err)
	}
}

// TestExtractAllDryRun tests --all --dry-run.
func TestExtractAllDryRun(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	entries := []PendingExtraction{
		{SessionID: "session-1", Summary: "Session 1", QueuedAt: time.Now()},
		{SessionID: "session-2", Summary: "Session 2", QueuedAt: time.Now()},
	}

	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	t.Chdir(tempDir)

	// Enable dry-run
	dryRun = true
	extractAll = true
	defer func() { dryRun = false }()

	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract --all --dry-run failed: %v", err)
	}

	// Entries should remain (dry-run doesn't modify)
	remaining, err := readPendingExtractions(pendingPath)
	if err != nil {
		t.Fatalf("read pending after dry-run: %v", err)
	}

	if len(remaining) != 2 {
		t.Fatalf("dry-run should not modify file, expected 2 entries, got %d", len(remaining))
	}
}

// TestExtractAllJSON tests --all with --json output.
func TestExtractAllJSON(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	entries := []PendingExtraction{
		{SessionID: "session-1", Summary: "Session 1", QueuedAt: time.Now()},
		{SessionID: "session-2", Summary: "Session 2", QueuedAt: time.Now()},
	}

	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	t.Chdir(tempDir)

	// Capture stdout to verify JSON output
	output = "json"
	extractAll = true
	defer func() { output = "table" }()

	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract --all --json failed: %v", err)
	}

	// Verify file was processed
	remaining, err := readPendingExtractions(pendingPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read pending after extract: %v", err)
	}

	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining after --all, got %d", len(remaining))
	}
}

// TestExtractNoFile tests behavior when pending.jsonl doesn't exist.
func TestExtractNoFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Chdir(tempDir)

	// Should succeed silently (no error)
	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract with no file should succeed: %v", err)
	}
}

// TestExtractClear tests --clear flag.
func TestExtractClear(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	entries := []PendingExtraction{
		{SessionID: "session-1", Summary: "Session 1", QueuedAt: time.Now()},
	}

	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	t.Chdir(tempDir)

	extractClear = true
	defer func() { extractClear = false }()

	if err := runExtract(nil, nil); err != nil {
		t.Fatalf("runExtract --clear failed: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(pendingPath); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed by --clear")
	}
}

// TestReadPendingExtractions tests parsing of JSONL file.
func TestReadPendingExtractions(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, "test.jsonl")

	entries := []PendingExtraction{
		{SessionID: "s1", Summary: "Summary 1", QueuedAt: time.Now()},
		{SessionID: "s2", Summary: "Summary 2", QueuedAt: time.Now()},
	}

	if err := writePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("write pending file: %v", err)
	}

	// Add a malformed line
	f, _ := os.OpenFile(pendingPath, os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString("invalid json\n")
	_ = f.Close()

	// Read and verify
	read, err := readPendingExtractions(pendingPath)
	if err != nil {
		t.Fatalf("readPendingExtractions failed: %v", err)
	}

	// Should skip malformed line
	if len(read) != 2 {
		t.Fatalf("expected 2 valid entries, got %d", len(read))
	}

	if read[0].SessionID != "s1" || read[1].SessionID != "s2" {
		t.Errorf("unexpected entries: %+v", read)
	}
}

// TestRewritePendingFile tests concurrent-safe file rewriting.
func TestRewritePendingFile(t *testing.T) {
	tempDir := t.TempDir()
	pendingPath := filepath.Join(tempDir, storage.DefaultBaseDir, "pending.jsonl")

	entries := []PendingExtraction{
		{SessionID: "s1", Summary: "Summary 1", QueuedAt: time.Now()},
		{SessionID: "s2", Summary: "Summary 2", QueuedAt: time.Now()},
		{SessionID: "s3", Summary: "Summary 3", QueuedAt: time.Now()},
	}

	// Write initial
	if err := rewritePendingFile(pendingPath, entries); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Remove middle entry
	remaining := []PendingExtraction{entries[0], entries[2]}
	if err := rewritePendingFile(pendingPath, remaining); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	// Verify
	read, err := readPendingExtractions(pendingPath)
	if err != nil {
		t.Fatalf("read after rewrite: %v", err)
	}

	if len(read) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(read))
	}

	if read[0].SessionID != "s1" || read[1].SessionID != "s3" {
		t.Errorf("unexpected entries after rewrite: %+v", read)
	}
}

// TestOutputExtractionPrompt verifies prompt generation doesn't crash.
func TestOutputExtractionPrompt(t *testing.T) {
	tempDir := t.TempDir()

	extraction := PendingExtraction{
		SessionID:      "test-session",
		Summary:        "Test summary",
		Decisions:      []string{"Decision 1", "Decision 2"},
		Knowledge:      []string{"Knowledge 1"},
		QueuedAt:       time.Now(),
		SessionPath:    "/test/path",
		TranscriptPath: "/test/transcript",
	}

	// Capture stdout to prevent test output pollution
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputExtractionPrompt(extraction, tempDir, 3000)

	_ = w.Close()
	os.Stdout = oldStdout

	// Read output
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify key elements are present
	if !strings.Contains(output, "test-session") {
		t.Errorf("output missing session ID")
	}
	if !strings.Contains(output, "Test summary") {
		t.Errorf("output missing summary")
	}
	if !strings.Contains(output, "Decision 1") {
		t.Errorf("output missing decisions")
	}
}

// --- Helper Functions ---

// writePendingFile writes entries to a JSONL file.
func writePendingFile(path string, entries []PendingExtraction) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
	}

	return nil
}
