// practices: [sre, distributed-tracing]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteLiveStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live-status.md")

	phases := []PhaseProgress{
		{Name: "Research", Elapsed: 12 * time.Second, ToolCount: 5, Tokens: 1200, CostUSD: 0.0034, CurrentAction: "tool: Read", RetryCount: 1, LastError: "temporary timeout", LastUpdate: time.Now()},
		{Name: "Plan", Elapsed: 3 * time.Second, ToolCount: 2, Tokens: 800, CostUSD: 0.0021, CurrentAction: "analyzing"},
		{Name: "Implement", Elapsed: 0, ToolCount: 0, Tokens: 0, CostUSD: 0},
	}

	if err := WriteLiveStatus(path, phases, 1); err != nil {
		t.Fatalf("WriteLiveStatus: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	// Verify header row exists
	for _, col := range []string{"Phase", "Status", "Elapsed", "Tools", "Tokens", "Cost", "Action", "Retries", "Last Error", "Updated"} {
		if !strings.Contains(content, col) {
			t.Errorf("missing column header %q", col)
		}
	}

	// Verify status values
	if !strings.Contains(content, "| done |") {
		t.Error("expected 'done' status for completed phase")
	}
	if !strings.Contains(content, "| running |") {
		t.Error("expected 'running' status for current phase")
	}
	if !strings.Contains(content, "| pending |") {
		t.Error("expected 'pending' status for future phase")
	}

	// Verify the tmp file was cleaned up (renamed away)
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after successful write")
	}
}

func TestLiveStatusFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live-status.md")

	phases := []PhaseProgress{
		{Name: "Research", Elapsed: 5 * time.Second, ToolCount: 3, Tokens: 600, CostUSD: 0.002},
		{Name: "Plan", Elapsed: 0, ToolCount: 0, Tokens: 0, CostUSD: 0},
	}

	if err := WriteLiveStatus(path, phases, 0); err != nil {
		t.Fatalf("WriteLiveStatus: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	// Must contain markdown table headers
	expectedHeaders := []string{"Phase", "Status", "Elapsed", "Tools", "Tokens", "Cost", "Action", "Retries", "Last Error", "Updated"}
	for _, h := range expectedHeaders {
		if !strings.Contains(content, h) {
			t.Errorf("output missing expected header %q", h)
		}
	}

	// Must contain the separator row
	if !strings.Contains(content, "|---") {
		t.Error("output missing markdown table separator row")
	}

	// Phase names must appear in output
	if !strings.Contains(content, "Research") {
		t.Error("output missing phase name 'Research'")
	}
	if !strings.Contains(content, "Plan") {
		t.Error("output missing phase name 'Plan'")
	}

	// First phase (index 0) should be running, second should be pending
	if !strings.Contains(content, "| running |") {
		t.Error("expected 'running' for current phase at index 0")
	}
	if !strings.Contains(content, "| pending |") {
		t.Error("expected 'pending' for phase after current")
	}
}

func TestLiveStatusAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live-status.md")

	phases := []PhaseProgress{
		{Name: "Test", Elapsed: 1 * time.Second, ToolCount: 1, Tokens: 100, CostUSD: 0.001},
	}

	if err := WriteLiveStatus(path, phases, 0); err != nil {
		t.Fatalf("WriteLiveStatus: %v", err)
	}

	// The final file must exist
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	// The .tmp file must NOT persist after a successful write
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf(".tmp file %q should not persist after successful write", tmpPath)
	}

	// Write a second time to ensure repeated atomic writes work
	if err := WriteLiveStatus(path, phases, 0); err != nil {
		t.Fatalf("second WriteLiveStatus: %v", err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf(".tmp file should not persist after second write")
	}
}

func TestWriteLiveStatus_WriteError(t *testing.T) {
	// Writing to a nonexistent directory should fail.
	path := filepath.Join(t.TempDir(), "no-such-dir", "sub", "live-status.md")
	phases := []PhaseProgress{
		{Name: "Research", Elapsed: 1 * time.Second},
	}
	err := WriteLiveStatus(path, phases, 0)
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory")
	}
	if !strings.Contains(err.Error(), "write tmp") {
		t.Errorf("expected 'write tmp' in error, got %q", err.Error())
	}
}

func TestNormalizeLiveStatusField_LongTruncation(t *testing.T) {
	// A string longer than 72 characters should be truncated with "..."
	long := strings.Repeat("x", 100)
	got := normalizeLiveStatusField(long)
	if len(got) != 72 {
		t.Errorf("expected truncated length 72, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncated string to end with '...', got %q", got[len(got)-5:])
	}
	if got != strings.Repeat("x", 69)+"..." {
		t.Errorf("unexpected truncation result: %q", got)
	}
}
