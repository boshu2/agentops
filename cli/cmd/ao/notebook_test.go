package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindMemoryFile_StandardPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create the expected Claude Code project path structure
	cwd := "/Users/testuser/projects/myrepo"
	normalizedPath := strings.ReplaceAll(cwd, "/", "-")
	memoryDir := filepath.Join(tmp, ".claude", "projects", normalizedPath, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	if err := os.WriteFile(memoryFile, []byte("# Test Memory\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := findMemoryFile(cwd)
	if err != nil {
		t.Fatalf("findMemoryFile() error: %v", err)
	}
	if got != memoryFile {
		t.Errorf("findMemoryFile() = %q, want %q", got, memoryFile)
	}
}

func TestFindMemoryFile_FallbackSearch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a project dir that contains the last path component
	memoryDir := filepath.Join(tmp, ".claude", "projects", "-Users-someone-projects-myrepo", "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		t.Fatal(err)
	}
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	if err := os.WriteFile(memoryFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use a different cwd that won't match exactly but has "myrepo" as last component
	got, err := findMemoryFile("/different/path/to/myrepo")
	if err != nil {
		t.Fatalf("findMemoryFile() error: %v", err)
	}
	if got != memoryFile {
		t.Errorf("findMemoryFile() = %q, want %q", got, memoryFile)
	}
}

func TestFindMemoryFile_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create the projects dir but no memory files
	if err := os.MkdirAll(filepath.Join(tmp, ".claude", "projects"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := findMemoryFile("/nonexistent/repo")
	if err == nil {
		t.Error("expected error when no MEMORY.md exists, got nil")
	}
}

func TestParseNotebookSections(t *testing.T) {
	content := `# My Memory

## Section One
- item 1
- item 2

## Section Two
- item A
- item B

## Section Three
- item X
`

	sections := parseSectionsFromString(content)

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	if sections[0].Heading != "# My Memory" {
		t.Errorf("section 0 heading = %q, want %q", sections[0].Heading, "# My Memory")
	}
	if sections[1].Heading != "## Section One" {
		t.Errorf("section 1 heading = %q, want %q", sections[1].Heading, "## Section One")
	}
	if sections[2].Heading != "## Section Two" {
		t.Errorf("section 2 heading = %q, want %q", sections[2].Heading, "## Section Two")
	}
	if sections[3].Heading != "## Section Three" {
		t.Errorf("section 3 heading = %q, want %q", sections[3].Heading, "## Section Three")
	}
}

func TestUpdateLastSessionSection(t *testing.T) {
	sections := []notebookSection{
		{Heading: "# Memory", Lines: []string{""}},
		{Heading: "## Last Session", Lines: []string{"- old data", ""}},
		{Heading: "## Patterns", Lines: []string{"- pattern 1", ""}},
	}

	newSession := notebookSection{
		Heading: "## Last Session",
		Lines:   []string{"- **Date:** 2026-02-25", "- **Summary:** new summary", ""},
	}

	result := upsertLastSession(sections, newSession)

	if len(result) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(result))
	}
	if result[1].Lines[0] != "- **Date:** 2026-02-25" {
		t.Errorf("Last Session not updated: %v", result[1].Lines)
	}
}

func TestUpdateLastSessionSection_Insert(t *testing.T) {
	sections := []notebookSection{
		{Heading: "# Memory", Lines: []string{""}},
		{Heading: "## Patterns", Lines: []string{"- pattern 1", ""}},
	}

	newSession := notebookSection{
		Heading: "## Last Session",
		Lines:   []string{"- **Date:** 2026-02-25", ""},
	}

	result := upsertLastSession(sections, newSession)

	if len(result) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(result))
	}
	if result[1].Heading != "## Last Session" {
		t.Errorf("Last Session not inserted at position 1: heading = %q", result[1].Heading)
	}
	if result[2].Heading != "## Patterns" {
		t.Errorf("Patterns should be at position 2: heading = %q", result[2].Heading)
	}
}

func TestPruneNotebook_UnderLimit(t *testing.T) {
	sections := []notebookSection{
		{Heading: "# Memory", Lines: []string{""}},
		{Heading: "## Last Session", Lines: []string{"- date", ""}},
		{Heading: "## Lessons", Lines: []string{"- lesson 1", "- lesson 2", ""}},
	}

	result := pruneNotebook(sections, 190)

	// Should be unchanged — well under limit
	totalBefore := totalLines(sections)
	totalAfter := totalLines(result)
	if totalBefore != totalAfter {
		t.Errorf("pruneNotebook changed content when under limit: %d → %d", totalBefore, totalAfter)
	}
}

func TestPruneNotebook_OverLimit(t *testing.T) {
	// Create a notebook that's over the limit
	var longLines []string
	for i := 0; i < 100; i++ {
		longLines = append(longLines, "- bullet point "+strings.Repeat("x", 10))
	}

	sections := []notebookSection{
		{Heading: "# Memory", Lines: []string{""}},
		{Heading: "## Last Session", Lines: []string{"- date", ""}},
		{Heading: "## Big Section", Lines: longLines},
		{Heading: "## Small Section", Lines: []string{"- keep this", ""}},
	}

	maxLines := 20
	result := pruneNotebook(sections, maxLines)
	total := totalLines(result)

	if total > maxLines {
		t.Errorf("pruneNotebook result has %d lines, want <= %d", total, maxLines)
	}
}

func TestNotebookUpdate_Idempotent(t *testing.T) {
	tmp := t.TempDir()

	// Create a MEMORY.md
	memoryFile := filepath.Join(tmp, "MEMORY.md")
	initialContent := "# Test Memory\n\n## Lessons\n- lesson 1\n"
	if err := os.WriteFile(memoryFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a pending.jsonl with one entry
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}
	entry := pendingEntry{
		SessionID: "test-123",
		Summary:   "Did some work",
		Decisions: []string{"Used approach A"},
		Knowledge: []string{"Next: implement feature B"},
		QueuedAt:  time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Run update twice
	sections1, _ := parseNotebookSections(memoryFile)
	sections1 = upsertLastSession(sections1, buildLastSessionSection(&entry))
	content1 := renderNotebook(sections1)
	if err := os.WriteFile(memoryFile, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	sections2, _ := parseNotebookSections(memoryFile)
	sections2 = upsertLastSession(sections2, buildLastSessionSection(&entry))
	content2 := renderNotebook(sections2)

	if content1 != content2 {
		t.Error("running update twice with same input produced different output")
		t.Logf("first:\n%s", content1)
		t.Logf("second:\n%s", content2)
	}
}

func TestNotebookUpdate_EmptyPending(t *testing.T) {
	tmp := t.TempDir()

	// No pending.jsonl exists
	entry, err := readLatestPendingEntry(tmp)
	if entry != nil {
		t.Errorf("expected nil entry, got %+v", entry)
	}
	// err is expected to be non-nil since file doesn't exist
	_ = err
}

func TestRunNotebookUpdateWithOptions_NoMemoryUsesWriter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	var out bytes.Buffer
	err := runNotebookUpdateWithOptions(notebookUpdateOptions{
		Cwd:    filepath.Join(tmp, "repo"),
		Writer: &out,
	})
	if err != nil {
		t.Fatalf("runNotebookUpdateWithOptions returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No MEMORY.md found") {
		t.Fatalf("expected no-memory message, got: %q", out.String())
	}
}

func TestRunNotebookUpdateWithOptions_UpdatesMemoryAndUsesWriter(t *testing.T) {
	tmp := t.TempDir()
	memoryFile := filepath.Join(tmp, "MEMORY.md")
	if err := os.WriteFile(memoryFile, []byte("# Test Memory\n\n## Lessons\n- keep this\n"), 0644); err != nil {
		t.Fatal(err)
	}

	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}
	entry := pendingEntry{
		SessionID: "notebook-writer-1",
		Summary:   "Updated the notebook through injected options",
		Decisions: []string{"Use an injected writer"},
		Knowledge: []string{"Next: keep command handlers thin"},
		QueuedAt:  time.Date(2026, 4, 24, 1, 15, 0, 0, time.UTC),
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := runNotebookUpdateWithOptions(notebookUpdateOptions{
		Cwd:        tmp,
		MemoryFile: memoryFile,
		Source:     "pending",
		MaxLines:   190,
		Writer:     &out,
	})
	if err != nil {
		t.Fatalf("runNotebookUpdateWithOptions returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Updated "+memoryFile) {
		t.Fatalf("expected update message, got: %q", out.String())
	}

	content, err := os.ReadFile(memoryFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Updated the notebook through injected options") {
		t.Fatalf("memory file was not updated with session summary:\n%s", string(content))
	}
	if got, err := readNotebookCursor(filepath.Join(aoDir, "notebook-cursor.json")); err != nil || got != entry.SessionID {
		t.Fatalf("cursor = %q, err = %v; want %q", got, err, entry.SessionID)
	}
}

func TestRunNotebookUpdateWithOptions_QuietSuppressesSuccessOutput(t *testing.T) {
	tmp := t.TempDir()
	memoryFile := filepath.Join(tmp, "MEMORY.md")
	if err := os.WriteFile(memoryFile, []byte("# Test Memory\n"), 0644); err != nil {
		t.Fatal(err)
	}

	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}
	entry := pendingEntry{SessionID: "quiet-1", Summary: "Quiet update"}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := runNotebookUpdateWithOptions(notebookUpdateOptions{
		Cwd:        tmp,
		MemoryFile: memoryFile,
		Quiet:      true,
		Source:     "pending",
		Writer:     &out,
	})
	if err != nil {
		t.Fatalf("runNotebookUpdateWithOptions returned error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("quiet update wrote output: %q", out.String())
	}
}

func TestNotebookUpdate_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	memoryFile := filepath.Join(tmp, "MEMORY.md")

	content := "# Test Memory\n\n## Section\n- data\n"
	if err := atomicWriteFile(memoryFile, []byte(content), 0644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	// Verify file is valid
	got, err := os.ReadFile(memoryFile)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != content {
		t.Errorf("content mismatch: got %q, want %q", string(got), content)
	}

	// Verify no temp files left behind
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".ao-tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestBuildLastSessionSection(t *testing.T) {
	entry := &pendingEntry{
		Summary:   "Implemented notebook command",
		Decisions: []string{"Use MEMORY.md as compound notebook"},
		Knowledge: []string{
			"Next: wire into SessionEnd hook",
			"Success: all tests passing",
			"Architecture uses atomic writes",
		},
		QueuedAt: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
	}

	section := buildLastSessionSection(entry)

	if section.Heading != "## Last Session" {
		t.Errorf("heading = %q, want %q", section.Heading, "## Last Session")
	}

	content := strings.Join(section.Lines, "\n")

	if !strings.Contains(content, "2026-02-25") {
		t.Error("missing date")
	}
	if !strings.Contains(content, "Implemented notebook command") {
		t.Error("missing summary")
	}
	if !strings.Contains(content, "Key decisions") {
		t.Error("missing decisions")
	}
	if !strings.Contains(content, "Next:") {
		t.Error("missing next steps")
	}
}

func TestTotalLines(t *testing.T) {
	sections := []notebookSection{
		{Heading: "# Title", Lines: []string{""}},          // 2 lines
		{Heading: "## Section", Lines: []string{"a", "b"}}, // 3 lines
	}

	got := totalLines(sections)
	if got != 5 {
		t.Errorf("totalLines = %d, want 5", got)
	}
}

func TestReadLatestPendingEntry(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write two entries — should read the last one
	entries := []pendingEntry{
		{SessionID: "old", Summary: "old session"},
		{SessionID: "new", Summary: "new session"},
	}
	var lines []string
	for _, e := range entries {
		data, _ := json.Marshal(e)
		lines = append(lines, string(data))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readLatestPendingEntry(tmp)
	if err != nil {
		t.Fatalf("readLatestPendingEntry error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil entry")
	}
	if got.SessionID != "new" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "new")
	}
}

func TestParseSectionsFromString_Empty(t *testing.T) {
	sections := parseSectionsFromString("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty input, got %d", len(sections))
	}
}

func TestParseSectionsFromString_NoHeadings(t *testing.T) {
	sections := parseSectionsFromString("just text\nno headings here")
	if len(sections) != 1 {
		t.Fatalf("expected 1 preamble section, got %d", len(sections))
	}
	if sections[0].Heading != "" {
		t.Errorf("expected empty heading for preamble, got %q", sections[0].Heading)
	}
}

func TestUpsertLastSession_RemovesDuplicates(t *testing.T) {
	sections := []notebookSection{
		{Heading: "# Memory", Lines: []string{""}},
		{Heading: "## Last Session", Lines: []string{"- old data 1", ""}},
		{Heading: "## Patterns", Lines: []string{"- pattern", ""}},
		{Heading: "## Last Session", Lines: []string{"- old data 2 (duplicate)", ""}},
	}

	newSession := notebookSection{
		Heading: "## Last Session",
		Lines:   []string{"- new data", ""},
	}

	result := upsertLastSession(sections, newSession)

	// Should have 3 sections (no duplicate Last Session)
	count := 0
	for _, s := range result {
		if s.Heading == "## Last Session" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 Last Session section, got %d", count)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 sections total, got %d", len(result))
	}
}

func TestNotebookCursor_SkipsDuplicate(t *testing.T) {
	tmp := t.TempDir()
	cursorPath := filepath.Join(tmp, "notebook-cursor.json")

	// Write cursor with a session ID
	if err := writeNotebookCursor(cursorPath, "session-abc"); err != nil {
		t.Fatalf("writeNotebookCursor failed: %v", err)
	}

	// Read it back
	got, err := readNotebookCursor(cursorPath)
	if err != nil {
		t.Fatalf("readNotebookCursor failed: %v", err)
	}
	if got != "session-abc" {
		t.Errorf("cursor session_id = %q, want %q", got, "session-abc")
	}

	// Non-existent cursor returns empty
	got2, err := readNotebookCursor(filepath.Join(tmp, "nonexistent.json"))
	if err == nil {
		t.Error("expected error for nonexistent cursor")
	}
	if got2 != "" {
		t.Errorf("expected empty string for missing cursor, got %q", got2)
	}
}

func TestReadLatestSessionEntry_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two session files — should read the latest by filename sort
	old := `{"session_id":"old-001","date":"2026-02-24T10:00:00Z","summary":"old session","decisions":["d1"],"knowledge":["k1"]}`
	new := `{"session_id":"new-002","date":"2026-02-25T10:00:00Z","summary":"new session","decisions":["d2"],"knowledge":["k2"]}`

	if err := os.WriteFile(filepath.Join(sessionsDir, "2026-02-24-old-session-old-001.jsonl"), []byte(old+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-new-session-new-002.jsonl"), []byte(new+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Also create a .md file that should be ignored
	if err := os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-some-session-abc1234.md"), []byte("# ignored"), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := readLatestSessionEntry(tmp)
	if err != nil {
		t.Fatalf("readLatestSessionEntry error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.SessionID != "new-002" {
		t.Errorf("SessionID = %q, want %q", entry.SessionID, "new-002")
	}
	if entry.Summary != "new session" {
		t.Errorf("Summary = %q, want %q", entry.Summary, "new session")
	}
	if entry.QueuedAt.IsZero() {
		t.Error("QueuedAt should be mapped from session Date")
	}
}

func TestReadLatestSessionEntry_Empty(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	entry, err := readLatestSessionEntry(tmp)
	if entry != nil {
		t.Errorf("expected nil entry for empty dir, got %+v", entry)
	}
	if err == nil {
		t.Error("expected error for empty sessions dir")
	}
}

func TestReadLatestSessionEntry_NoDir(t *testing.T) {
	tmp := t.TempDir()
	entry, err := readLatestSessionEntry(tmp)
	if entry != nil {
		t.Errorf("expected nil entry, got %+v", entry)
	}
	if err == nil {
		t.Error("expected error when sessions dir doesn't exist")
	}
}

func TestReadSessionByID_Found(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	data := `{"session_id":"abc1234","date":"2026-02-25T10:00:00Z","summary":"target session"}`
	if err := os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-target-session-abc1234.jsonl"), []byte(data+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := readSessionByID(tmp, "abc1234")
	if err != nil {
		t.Fatalf("readSessionByID error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.SessionID != "abc1234" {
		t.Errorf("SessionID = %q, want %q", entry.SessionID, "abc1234")
	}
}

func TestReadSessionByID_NotFound(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := readSessionByID(tmp, "nonexistent")
	if err == nil {
		t.Error("expected error for missing session ID")
	}
}

func TestReadSessionByID_Ambiguous(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two files that both match the ID substring "abc"
	data1 := `{"session_id":"abc1111","date":"2026-02-25T10:00:00Z","summary":"session 1"}`
	data2 := `{"session_id":"abc2222","date":"2026-02-25T11:00:00Z","summary":"session 2"}`
	os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-session-abc1111.jsonl"), []byte(data1+"\n"), 0644)
	os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-session-abc2222.jsonl"), []byte(data2+"\n"), 0644)

	_, err := readSessionByID(tmp, "abc")
	if err == nil {
		t.Fatal("expected error for ambiguous session ID")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should mention ambiguity, got: %v", err)
	}
}

func TestPruneNotebook_IterationCap(t *testing.T) {
	// Create sections that resist pruning — all protected types
	sections := []notebookSection{
		{Heading: "# My Notebook", Lines: []string{"", "Main content"}},
		{Heading: "## Last Session", Lines: make([]string, 500)},
	}
	// Fill Last Session with non-empty lines
	for i := range sections[1].Lines {
		sections[1].Lines[i] = "line"
	}

	// With only protected sections, pruning can't make progress but should not loop forever
	result := pruneNotebook(sections, 10)
	// Should return without infinite loop — the iteration cap prevents it
	if result == nil {
		t.Error("pruneNotebook should return sections even when it can't prune")
	}
}

func TestResolveNotebookSource_AutoFallback(t *testing.T) {
	tmp := t.TempDir()

	// No sessions dir, but has pending.jsonl
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := `{"session_id":"pending-1","summary":"from pending"}`
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), []byte(data+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := resolveNotebookSource(tmp, "auto")
	if err != nil {
		t.Fatalf("resolveNotebookSource auto error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry from pending fallback")
	}
	if entry.SessionID != "pending-1" {
		t.Errorf("SessionID = %q, want %q (should fall back to pending)", entry.SessionID, "pending-1")
	}
}

func TestResolveNotebookSource_SessionsPreferred(t *testing.T) {
	tmp := t.TempDir()

	// Both sessions and pending exist — sessions should win
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	sessionData := `{"session_id":"session-1","date":"2026-02-25T10:00:00Z","summary":"from sessions"}`
	if err := os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-test-session-session.jsonl"), []byte(sessionData+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	aoDir := filepath.Join(tmp, ".agents", "ao")
	pendingData := `{"session_id":"pending-1","summary":"from pending"}`
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), []byte(pendingData+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	entry, err := resolveNotebookSource(tmp, "auto")
	if err != nil {
		t.Fatalf("resolveNotebookSource auto error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want %q (sessions should be preferred)", entry.SessionID, "session-1")
	}
}

func TestResolveNotebookSource_InvalidSource(t *testing.T) {
	tmp := t.TempDir()
	_, err := resolveNotebookSource(tmp, "garbage")
	if err == nil {
		t.Error("expected error for unknown source value")
	}
	if !strings.Contains(err.Error(), "unknown source") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPruneNotebook_SmallSections(t *testing.T) {
	// Test that pruning works even when all sections have <= 2 lines
	sections := []notebookSection{
		{Heading: "# Title", Lines: []string{""}},
		{Heading: "## Last Session", Lines: []string{"- date", ""}},
		{Heading: "## A", Lines: []string{"- a1", "- a2"}},
		{Heading: "## B", Lines: []string{"- b1", "- b2"}},
		{Heading: "## C", Lines: []string{"- c1", "- c2"}},
	}

	result := pruneNotebook(sections, 10)
	total := totalLines(result)

	if total > 10 {
		t.Errorf("pruneNotebook result has %d lines, want <= 10", total)
	}
}
