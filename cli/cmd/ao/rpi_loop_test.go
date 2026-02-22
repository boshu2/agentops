package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadUnconsumedItems_NoFile(t *testing.T) {
	items, err := readUnconsumedItems("/nonexistent/path/next-work.jsonl", "")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestReadUnconsumedItems_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestReadUnconsumedItems_ConsumedOnly(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-test",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Should be skipped", Severity: "high"},
		},
		Consumed: true,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items from consumed entry, got %d", len(items))
	}
}

func TestReadUnconsumedItems_UnconsumedWithItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-test",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Item A", Severity: "high"},
			{Title: "Item B", Severity: "low"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Title != "Item A" {
		t.Errorf("expected first item 'Item A', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_EmptyItemsArray(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-empty",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items:      []nextWorkItem{},
		Consumed:   false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items from empty items array, got %d", len(items))
	}
}

func TestReadUnconsumedItems_MultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	consumed := nextWorkEntry{
		SourceEpic: "ag-old",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items:      []nextWorkItem{{Title: "Old item", Severity: "low"}},
		Consumed:   true,
	}
	unconsumed := nextWorkEntry{
		SourceEpic: "ag-new",
		Timestamp:  "2026-02-10T01:00:00Z",
		Items:      []nextWorkItem{{Title: "New item", Severity: "medium"}},
		Consumed:   false,
	}

	d1, _ := json.Marshal(consumed)
	d2, _ := json.Marshal(unconsumed)
	content := string(d1) + "\n" + string(d2) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (only unconsumed), got %d", len(items))
	}
	if items[0].Title != "New item" {
		t.Errorf("expected 'New item', got %q", items[0].Title)
	}
}

func TestSelectHighestSeverityItem(t *testing.T) {
	tests := []struct {
		name     string
		items    []nextWorkItem
		expected string
	}{
		{
			name:     "empty",
			items:    nil,
			expected: "",
		},
		{
			name: "single item",
			items: []nextWorkItem{
				{Title: "Only one", Severity: "low"},
			},
			expected: "Only one",
		},
		{
			name: "high beats medium and low",
			items: []nextWorkItem{
				{Title: "Low item", Severity: "low"},
				{Title: "High item", Severity: "high"},
				{Title: "Medium item", Severity: "medium"},
			},
			expected: "High item",
		},
		{
			name: "medium beats low",
			items: []nextWorkItem{
				{Title: "Low item", Severity: "low"},
				{Title: "Medium item", Severity: "medium"},
			},
			expected: "Medium item",
		},
		{
			name: "unknown severity ranks lowest",
			items: []nextWorkItem{
				{Title: "Unknown", Severity: "critical"},
				{Title: "Low item", Severity: "low"},
			},
			expected: "Low item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectHighestSeverityItem(tt.items)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		severity string
		rank     int
	}{
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			if got := severityRank(tt.severity); got != tt.rank {
				t.Errorf("severityRank(%q) = %d, want %d", tt.severity, got, tt.rank)
			}
		})
	}
}

func TestReadUnconsumedItems_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-valid",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items:      []nextWorkItem{{Title: "Valid", Severity: "high"}},
		Consumed:   false,
	}
	data, _ := json.Marshal(entry)
	content := "not json at all\n" + string(data) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (skip malformed), got %d", len(items))
	}
	if items[0].Title != "Valid" {
		t.Errorf("expected 'Valid', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_Match(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For agentops", Severity: "high", TargetRepo: "agentops"},
			{Title: "For olympus", Severity: "medium", TargetRepo: "olympus"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item matching repo filter, got %d", len(items))
	}
	if items[0].Title != "For agentops" {
		t.Errorf("expected 'For agentops', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_Exclude(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For olympus only", Severity: "high", TargetRepo: "olympus"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items (filtered out), got %d", len(items))
	}
}

func TestReadUnconsumedItems_RepoFilter_Wildcard(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For all repos", Severity: "high", TargetRepo: "*"},
			{Title: "For olympus", Severity: "low", TargetRepo: "olympus"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (wildcard passes, olympus excluded), got %d", len(items))
	}
	if items[0].Title != "For all repos" {
		t.Errorf("expected 'For all repos', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_Legacy(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	// Legacy items have no target_repo field (empty string after deserialization)
	entry := nextWorkEntry{
		SourceEpic: "ag-legacy",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Legacy item", Severity: "medium"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Legacy items (no target_repo) should pass any filter
	items, err := readUnconsumedItems(path, "agentops")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (legacy passes all filters), got %d", len(items))
	}
	if items[0].Title != "Legacy item" {
		t.Errorf("expected 'Legacy item', got %q", items[0].Title)
	}
}

func TestReadUnconsumedItems_RepoFilter_EmptyFilter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-repo",
		Timestamp:  "2026-02-10T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "For agentops", Severity: "high", TargetRepo: "agentops"},
			{Title: "For olympus", Severity: "medium", TargetRepo: "olympus"},
			{Title: "Legacy", Severity: "low"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty filter means no filtering - all items pass
	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items (no filter), got %d", len(items))
	}
}

// ---- Queue mark semantics ----

func writeJSONL(t *testing.T, path string, entries []nextWorkEntry) {
	t.Helper()
	var out strings.Builder
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		out.Write(data)
		out.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(out.String()), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readJSONLEntries(t *testing.T, path string) []nextWorkEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var entries []nextWorkEntry
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e nextWorkEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestQueueMarkConsumed_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-1", Timestamp: "2026-02-10T00:00:00Z", Items: []nextWorkItem{{Title: "Item 1", Severity: "high"}}, Consumed: false},
		{SourceEpic: "ag-2", Timestamp: "2026-02-10T01:00:00Z", Items: []nextWorkItem{{Title: "Item 2", Severity: "low"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	if err := markEntryConsumed(path, 0, "test-runner"); err != nil {
		t.Fatalf("markEntryConsumed: %v", err)
	}

	got := readJSONLEntries(t, path)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if !got[0].Consumed {
		t.Errorf("entry 0: expected Consumed=true")
	}
	if got[0].ConsumedAt == nil {
		t.Errorf("entry 0: expected ConsumedAt to be set")
	}
	if got[0].ConsumedBy == nil || *got[0].ConsumedBy != "test-runner" {
		t.Errorf("entry 0: expected ConsumedBy=test-runner")
	}
	// Entry 1 should be untouched.
	if got[1].Consumed {
		t.Errorf("entry 1: should not be consumed")
	}
}

func TestQueueMarkConsumed_SecondEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-1", Items: []nextWorkItem{{Title: "First"}}, Consumed: false},
		{SourceEpic: "ag-2", Items: []nextWorkItem{{Title: "Second"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	if err := markEntryConsumed(path, 1, "loop"); err != nil {
		t.Fatalf("markEntryConsumed: %v", err)
	}

	got := readJSONLEntries(t, path)
	if got[0].Consumed {
		t.Errorf("entry 0 should not be consumed")
	}
	if !got[1].Consumed {
		t.Errorf("entry 1 should be consumed")
	}
}

func TestQueueMarkFailed_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-fail", Items: []nextWorkItem{{Title: "Failing item"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	if err := markEntryFailed(path, 0); err != nil {
		t.Fatalf("markEntryFailed: %v", err)
	}

	got := readJSONLEntries(t, path)
	if got[0].Consumed {
		t.Errorf("failed entry should not be marked consumed")
	}
	if got[0].FailedAt == nil {
		t.Errorf("expected FailedAt to be set")
	}
}

func TestQueueMarkFailed_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-fail", Items: []nextWorkItem{{Title: "Failing item"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	if err := markEntryFailed(path, 0); err != nil {
		t.Fatalf("first markEntryFailed: %v", err)
	}
	first := readJSONLEntries(t, path)
	firstTime := *first[0].FailedAt

	// Mark again (idempotent - updates timestamp but remains non-consumed).
	if err := markEntryFailed(path, 0); err != nil {
		t.Fatalf("second markEntryFailed: %v", err)
	}
	second := readJSONLEntries(t, path)
	if second[0].Consumed {
		t.Errorf("should not be consumed after double-failure")
	}
	// Second call may update the timestamp; it should still be a valid timestamp.
	if second[0].FailedAt == nil {
		t.Errorf("FailedAt should still be set after second call")
	}
	_ = firstTime // both are valid
}

func TestQueueMarkConsumed_MissingFile(t *testing.T) {
	// Missing file returns an error (callers distinguish missing queue from no-op).
	err := markEntryConsumed("/nonexistent/path/next-work.jsonl", 0, "loop")
	if err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
}

func TestQueueMarkFailed_MissingFile(t *testing.T) {
	// Missing file is a no-op for markEntryFailed (best-effort warning semantics).
	err := markEntryFailed("/nonexistent/path/next-work.jsonl", 0)
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
}

// ---- readQueueEntries ----

func TestReadQueueEntries_SkipsConsumed(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-1", Items: []nextWorkItem{{Title: "Consumed"}}, Consumed: true},
		{SourceEpic: "ag-2", Items: []nextWorkItem{{Title: "Open"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].SourceEpic != "ag-2" {
		t.Errorf("expected ag-2, got %q", got[0].SourceEpic)
	}
}

func TestReadQueueEntries_SkipsFailed(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	failedAt := "2026-02-10T00:00:00Z"
	entries := []nextWorkEntry{
		{SourceEpic: "ag-fail", Items: []nextWorkItem{{Title: "Failed item"}}, Consumed: false, FailedAt: &failedAt},
		{SourceEpic: "ag-open", Items: []nextWorkItem{{Title: "Open item"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry (failed skipped), got %d", len(got))
	}
	if got[0].SourceEpic != "ag-open" {
		t.Errorf("expected ag-open, got %q", got[0].SourceEpic)
	}
}

func TestReadQueueEntries_SkipsEmptyItems(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{SourceEpic: "ag-empty", Items: []nextWorkItem{}, Consumed: false},
		{SourceEpic: "ag-ok", Items: []nextWorkItem{{Title: "Has items"}}, Consumed: false},
	}
	writeJSONL(t, path, entries)

	got, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry (empty-items skipped), got %d", len(got))
	}
	if got[0].SourceEpic != "ag-ok" {
		t.Errorf("expected ag-ok, got %q", got[0].SourceEpic)
	}
}

// ---- selectHighestSeverityEntry ----

func TestSelectHighestSeverityEntry_Empty(t *testing.T) {
	sel := selectHighestSeverityEntry(nil, "")
	if sel != nil {
		t.Errorf("expected nil for empty entries")
	}
}

func TestSelectHighestSeverityEntry_PicksHighest(t *testing.T) {
	entries := []nextWorkEntry{
		{Items: []nextWorkItem{{Title: "Low item", Severity: "low"}}},
		{Items: []nextWorkItem{{Title: "High item", Severity: "high"}}},
		{Items: []nextWorkItem{{Title: "Medium item", Severity: "medium"}}},
	}
	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "High item" {
		t.Errorf("expected 'High item', got %q", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_RepoFilter(t *testing.T) {
	entries := []nextWorkEntry{
		{Items: []nextWorkItem{
			{Title: "For olympus", Severity: "high", TargetRepo: "olympus"},
			{Title: "For agentops", Severity: "medium", TargetRepo: "agentops"},
		}},
	}
	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.Item.Title != "For agentops" {
		t.Errorf("expected 'For agentops' (filtered by repo), got %q", sel.Item.Title)
	}
}

func TestSelectHighestSeverityEntry_RepoFilter_NoneMatch(t *testing.T) {
	entries := []nextWorkEntry{
		{Items: []nextWorkItem{
			{Title: "For olympus", Severity: "high", TargetRepo: "olympus"},
		}},
	}
	sel := selectHighestSeverityEntry(entries, "agentops")
	if sel != nil {
		t.Errorf("expected nil (no matching items), got %+v", sel)
	}
}

func TestSelectHighestSeverityEntry_EntryIndexCorrect(t *testing.T) {
	entries := []nextWorkEntry{
		{SourceEpic: "ag-0", QueueIndex: 0, Items: []nextWorkItem{{Title: "Entry 0", Severity: "low"}}},
		{SourceEpic: "ag-1", QueueIndex: 1, Items: []nextWorkItem{{Title: "Entry 1", Severity: "high"}}},
	}
	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.EntryIndex != 1 {
		t.Errorf("expected EntryIndex=1 (high severity), got %d", sel.EntryIndex)
	}
}

func TestSelectHighestSeverityEntry_UsesParseableQueueIndex(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	consumed := nextWorkEntry{
		SourceEpic: "ag-consumed",
		Items:      []nextWorkItem{{Title: "Consumed", Severity: "low"}},
		Consumed:   true,
	}
	open := nextWorkEntry{
		SourceEpic: "ag-open",
		Items:      []nextWorkItem{{Title: "Open", Severity: "high"}},
		Consumed:   false,
	}
	consumedData, _ := json.Marshal(consumed)
	openData, _ := json.Marshal(open)
	content := string(consumedData) + "\nnot-json\n" + string(openData) + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	entries, err := readQueueEntries(path)
	if err != nil {
		t.Fatalf("readQueueEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one eligible entry, got %d", len(entries))
	}
	if entries[0].QueueIndex != 1 {
		t.Fatalf("expected parseable queue index 1, got %d", entries[0].QueueIndex)
	}

	sel := selectHighestSeverityEntry(entries, "")
	if sel == nil {
		t.Fatal("expected selection, got nil")
	}
	if sel.EntryIndex != 1 {
		t.Fatalf("expected queue entry index 1, got %d", sel.EntryIndex)
	}
}

// ---- RPILoop dry-run ----

func TestRPILoop_DryRun_ExplicitGoal(t *testing.T) {
	// The loop should not call the phased engine in dry-run mode.
	// It should print what it would do and return nil.
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 0
	defer func() { rpiMaxCycles = prevMaxCycles }()

	// Provide an explicit goal so we don't need a next-work.jsonl file.
	err := runRPILoop(nil, []string{"test goal"})
	if err != nil {
		t.Errorf("expected nil error in dry-run, got: %v", err)
	}
}

func TestRPILoop_DryRun_EmptyQueue(t *testing.T) {
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	// No next-work.jsonl in temp dir, so queue is empty.
	// Loop should detect empty queue before dry-run branch is reached.
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	err := runRPILoop(nil, nil)
	if err != nil {
		t.Errorf("expected nil error for empty queue, got: %v", err)
	}
}

func TestRPILoop_DryRun_FromQueue(t *testing.T) {
	prevDryRun := dryRun
	dryRun = true
	defer func() { dryRun = prevDryRun }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 0
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create a queue with one item.
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entry := nextWorkEntry{
		SourceEpic: "ag-dryrun",
		Items:      []nextWorkItem{{Title: "Dry run goal", Severity: "high"}},
		Consumed:   false,
	}
	data, _ := json.Marshal(entry)
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	if err := os.WriteFile(queuePath, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write queue: %v", err)
	}

	err := runRPILoop(nil, nil)
	if err != nil {
		t.Errorf("expected nil error in dry-run, got: %v", err)
	}

	// In dry-run, the queue entry should NOT be marked consumed.
	after := readJSONLEntries(t, queuePath)
	if after[0].Consumed {
		t.Errorf("queue entry should not be consumed in dry-run mode")
	}
}

func TestRPILoop_InfraFailure_DoesNotMarkQueueFailed(t *testing.T) {
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-infra",
		Items:      []nextWorkItem{{Title: "Infra failing goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 1
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	attempts := 0
	runRPISupervisedCycleFn = func(_ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		attempts++
		return wrapCycleFailure(cycleFailureInfrastructure, "landing", fmt.Errorf("transient network"))
	}

	err := runRPILoop(nil, nil)
	if err == nil {
		t.Fatal("expected loop to fail under failure-policy=stop")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts (1 retry), got %d", attempts)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt != nil {
		t.Fatal("infra failures should not mark queue entry failed")
	}
	if after[0].Consumed {
		t.Fatal("infra failures should not mark queue entry consumed")
	}
}

func TestRPILoop_InfraFailure_ContinuePolicy_RetriesUntilMaxCycles(t *testing.T) {
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 2
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-infra-continue",
		Items:      []nextWorkItem{{Title: "Infra continue goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyContinue
	rpiCycleRetries = 1
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	attempts := 0
	runRPISupervisedCycleFn = func(_ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		attempts++
		return wrapCycleFailure(cycleFailureInfrastructure, "landing", fmt.Errorf("simulated rebase conflict"))
	}

	if err := runRPILoop(nil, nil); err != nil {
		t.Fatalf("expected nil error under failure-policy=continue, got: %v", err)
	}
	if attempts != 4 {
		t.Fatalf("expected 4 attempts (2 cycles x 2 attempts), got %d", attempts)
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt != nil {
		t.Fatal("infra failures should not mark queue entry failed under continue policy")
	}
	if after[0].Consumed {
		t.Fatal("infra failures should not mark queue entry consumed under continue policy")
	}
}

func TestRPILoop_TaskFailure_MarksQueueFailed(t *testing.T) {
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	prevMaxCycles := rpiMaxCycles
	rpiMaxCycles = 1
	defer func() { rpiMaxCycles = prevMaxCycles }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	queuePath := setupSingleQueueEntry(t, tmpDir, nextWorkEntry{
		SourceEpic: "ag-task",
		Items:      []nextWorkItem{{Title: "Task failing goal", Severity: "high"}},
		Consumed:   false,
	})

	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	runRPISupervisedCycleFn = func(_ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		return wrapCycleFailure(cycleFailureTask, "phased engine", fmt.Errorf("validation failed"))
	}

	err := runRPILoop(nil, nil)
	if err == nil {
		t.Fatal("expected loop to fail under failure-policy=stop")
	}

	after := readJSONLEntries(t, queuePath)
	if after[0].FailedAt == nil {
		t.Fatal("task failures should mark queue entry failed")
	}
	if after[0].Consumed {
		t.Fatal("failed queue entry should remain unconsumed")
	}
}

func TestRPILoop_ExplicitGoalReportsExecutedCycles(t *testing.T) {
	prevGlobals := snapshotLoopSupervisorGlobals()
	defer restoreLoopSupervisorGlobals(prevGlobals)

	prevDryRun := dryRun
	dryRun = false
	defer func() { dryRun = prevDryRun }()

	prevRunCycle := runRPISupervisedCycleFn
	defer func() { runRPISupervisedCycleFn = prevRunCycle }()

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	rpiMaxCycles = 0
	rpiSupervisor = false
	rpiFailurePolicy = loopFailurePolicyStop
	rpiCycleRetries = 0
	rpiRetryBackoff = 0
	rpiCycleDelay = 0
	rpiLease = false
	rpiLeaseTTL = 2 * time.Minute
	rpiGatePolicy = loopGatePolicyOff
	rpiLandingPolicy = loopLandingPolicyOff
	rpiBDSyncPolicy = loopBDSyncPolicyAuto
	rpiAutoCleanStaleAfter = 24 * time.Hour
	rpiCommandTimeout = time.Minute

	runRPISupervisedCycleFn = func(_ string, _ string, _ int, _ int, _ rpiLoopSupervisorConfig) error {
		return nil
	}

	output, err := captureStdoutWithError(func() error {
		return runRPILoop(nil, []string{"count cycles"})
	})
	if err != nil {
		t.Fatalf("runRPILoop returned error: %v", err)
	}
	if !strings.Contains(output, "Explicit goal completed.") {
		t.Fatalf("expected explicit goal completion message, got:\n%s", output)
	}
	if !strings.Contains(output, "RPI loop finished after 1 cycle(s).") {
		t.Fatalf("expected cycle count message, got:\n%s", output)
	}
}

func captureStdoutWithError(fn func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	runErr := fn()
	_ = w.Close()
	os.Stdout = oldStdout

	data, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		return "", readErr
	}
	return string(data), runErr
}

func setupSingleQueueEntry(t *testing.T, tmpDir string, entry nextWorkEntry) string {
	t.Helper()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	queuePath := filepath.Join(rpiDir, "next-work.jsonl")
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(queuePath, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write queue: %v", err)
	}
	return queuePath
}

// ---- phasedEngineOptions defaults ----

func TestDefaultPhasedEngineOptions(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	if opts.From != "discovery" {
		t.Errorf("expected From=discovery, got %q", opts.From)
	}
	if opts.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", opts.MaxRetries)
	}
	if !opts.SwarmFirst {
		t.Errorf("expected SwarmFirst=true")
	}
	if opts.PhaseTimeout == 0 {
		t.Errorf("expected non-zero PhaseTimeout")
	}
}
