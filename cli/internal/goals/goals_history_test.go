package goals_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/goals"
)

func TestGoalsHistory_NoHistoryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	historyPath := filepath.Join(dir, ".agents/ao/goals/history.jsonl")

	var stdout bytes.Buffer
	// Should print "no history" message, not error
	err := goals.RunHistory(goals.HistoryOptions{
		GoalID:      "",
		Since:       "",
		JSON:        false,
		HistoryPath: historyPath,
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatalf("history returned error: %v", err)
	}
}

// TestGoalsHistory_JSONEmptyStore_Decoder asserts that --json over an empty
// store emits exactly one JSON document `[]` on stdout (jq-safe), with any human
// hint on stderr and exit 0. RED before the fix: empty-state prose is printed to
// stdout, so json.Decoder fails to parse it.
func TestGoalsHistory_JSONEmptyStore_Decoder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	historyPath := filepath.Join(dir, ".agents/ao/goals/history.jsonl")

	var stdout, stderr bytes.Buffer
	err := goals.RunHistory(goals.HistoryOptions{
		JSON:        true,
		HistoryPath: historyPath,
		Stdout:      &stdout,
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("history returned error: %v", err)
	}

	// Shape-exact: the empty store must print the literal `[]` and nothing
	// else on stdout. A byte-exact check rejects `null` (which decodes into a
	// nil slice and would pass len==0) and any second document, without leaning
	// on Decoder.More() for the one-document guarantee.
	if got := strings.TrimSpace(stdout.String()); got != "[]" {
		t.Fatalf("stdout = %q, want exactly \"[]\"", got)
	}
	// The human hint belongs on stderr, never stdout.
	if !strings.Contains(stderr.String(), "No history entries found") {
		t.Errorf("stderr = %q, want the 'No history entries found' hint", stderr.String())
	}
}

func TestGoalsHistory_WithEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	historyDir := filepath.Join(dir, ".agents/ao/goals")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []goals.HistoryEntry{
		{Timestamp: "2025-06-01T12:00:00Z", GoalsPassing: 3, GoalsTotal: 5, Score: 60.0, GitSHA: "abc1234"},
		{Timestamp: "2025-06-02T12:00:00Z", GoalsPassing: 4, GoalsTotal: 5, Score: 80.0, GitSHA: "def5678"},
	}

	historyPath := filepath.Join(historyDir, "history.jsonl")
	f, err := os.Create(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, _ := json.Marshal(e)
		_, _ = f.Write(data)
		_, _ = f.Write([]byte("\n"))
	}
	_ = f.Close()

	var stdout bytes.Buffer
	err = goals.RunHistory(goals.HistoryOptions{
		GoalID:      "",
		Since:       "",
		JSON:        false,
		HistoryPath: historyPath,
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatalf("history returned error: %v", err)
	}
}

func TestGoalsHistory_JSONOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	historyDir := filepath.Join(dir, ".agents/ao/goals")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []goals.HistoryEntry{
		{Timestamp: "2025-06-01T12:00:00Z", GoalsPassing: 5, GoalsTotal: 5, Score: 100.0, GitSHA: "abc1234"},
	}

	historyPath := filepath.Join(historyDir, "history.jsonl")
	f, err := os.Create(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, _ := json.Marshal(e)
		_, _ = f.Write(data)
		_, _ = f.Write([]byte("\n"))
	}
	_ = f.Close()

	var stdout bytes.Buffer
	err = goals.RunHistory(goals.HistoryOptions{
		GoalID:      "",
		Since:       "",
		JSON:        true,
		HistoryPath: historyPath,
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatalf("history returned error: %v", err)
	}

	var decoded []goals.HistoryEntry
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON output: %v (raw: %s)", err, stdout.String())
	}
	if len(decoded) != 1 {
		t.Errorf("expected 1 entry, got %d", len(decoded))
	}
	if decoded[0].Score != 100.0 {
		t.Errorf("Score = %f, want 100.0", decoded[0].Score)
	}
}

func TestGoalsHistory_SinceFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	historyDir := filepath.Join(dir, ".agents/ao/goals")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []goals.HistoryEntry{
		{Timestamp: "2025-01-01T12:00:00Z", GoalsPassing: 1, GoalsTotal: 5, Score: 20.0, GitSHA: "old1"},
		{Timestamp: "2025-06-15T12:00:00Z", GoalsPassing: 4, GoalsTotal: 5, Score: 80.0, GitSHA: "new1"},
		{Timestamp: "2025-07-01T12:00:00Z", GoalsPassing: 5, GoalsTotal: 5, Score: 100.0, GitSHA: "new2"},
	}

	historyPath := filepath.Join(historyDir, "history.jsonl")
	f, err := os.Create(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, _ := json.Marshal(e)
		_, _ = f.Write(data)
		_, _ = f.Write([]byte("\n"))
	}
	_ = f.Close()

	var stdout bytes.Buffer
	err = goals.RunHistory(goals.HistoryOptions{
		GoalID:      "",
		Since:       "2025-06-01",
		JSON:        true,
		HistoryPath: historyPath,
		Stdout:      &stdout,
	})
	if err != nil {
		t.Fatalf("history returned error: %v", err)
	}

	var decoded []goals.HistoryEntry
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(decoded) != 2 {
		t.Errorf("expected 2 entries after --since 2025-06-01, got %d", len(decoded))
	}
}

func TestGoalsHistory_InvalidSinceDate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	historyDir := filepath.Join(dir, ".agents/ao/goals")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write at least one entry so we don't hit the "no entries" path
	historyPath := filepath.Join(historyDir, "history.jsonl")
	entry := goals.HistoryEntry{Timestamp: "2025-06-01T12:00:00Z", GoalsPassing: 1, GoalsTotal: 1, Score: 100.0}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(historyPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := goals.RunHistory(goals.HistoryOptions{
		GoalID:      "",
		Since:       "not-a-date",
		JSON:        false,
		HistoryPath: historyPath,
		Stdout:      &stdout,
	})
	if err == nil {
		t.Fatal("expected error for invalid --since date")
	}
	if !strings.Contains(err.Error(), "invalid --since date") {
		t.Errorf("error = %q, want 'invalid --since date'", err.Error())
	}
}
