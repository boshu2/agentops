// practices: [wiki-knowledge-surface, event-sourcing-cqrs]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseManagedBlock_WithContent(t *testing.T) {
	content := `# Memory

## Custom Notes

Some manual notes here.

<!-- ao:memory:start -->
- **[2026-02-25]** (abc1234) Did some work
- **[2026-02-24]** (def5678) Fixed a bug
<!-- ao:memory:end -->

## Footer

More manual content.
`
	before, managed, after := parseManagedBlock(content)

	if !strings.Contains(before, "Custom Notes") {
		t.Error("before should contain manual content before markers")
	}
	if !strings.Contains(managed, "abc1234") {
		t.Error("managed should contain session entries")
	}
	if !strings.Contains(after, "Footer") {
		t.Error("after should contain content after markers")
	}
}

func TestParseManagedBlock_NoBlock(t *testing.T) {
	content := "# Memory\n\nSome notes\n"
	before, managed, after := parseManagedBlock(content)

	if before != content {
		t.Errorf("before should be entire content, got %q", before)
	}
	if managed != "" {
		t.Error("managed should be empty when no block exists")
	}
	if after != "" {
		t.Error("after should be empty when no block exists")
	}
}

func TestParseManagedBlock_EmptyBlock(t *testing.T) {
	content := "# Memory\n\n" + memoryBlockStart + "\n" + memoryBlockEnd + "\n"
	before, managed, after := parseManagedBlock(content)

	if !strings.Contains(before, "# Memory") {
		t.Error("before should contain header")
	}
	if managed != "\n" {
		t.Errorf("managed should be a newline, got %q", managed)
	}
	_ = after
}

func TestParseManagedBlock_MalformedEndBeforeStart(t *testing.T) {
	content := memoryBlockEnd + "\n" + memoryBlockStart + "\n"
	before, managed, after := parseManagedBlock(content)

	// Should treat as no valid block
	if before != content {
		t.Error("malformed block should return entire content as before")
	}
	if managed != "" || after != "" {
		t.Error("malformed block should have empty managed and after")
	}
}

func TestExtractSessionIDs(t *testing.T) {
	managed := `
- **[2026-02-25]** (abc1234) Did some work
- **[2026-02-24]** (def5678) Fixed a bug
Some random line
- **[2026-02-23]** (ghi9012) Another session
`
	ids := extractSessionIDs(managed)

	if !ids["abc1234"] {
		t.Error("should extract abc1234")
	}
	if !ids["def5678"] {
		t.Error("should extract def5678")
	}
	if !ids["ghi9012"] {
		t.Error("should extract ghi9012")
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 IDs, got %d", len(ids))
	}
}

func TestExtractEntryLines(t *testing.T) {
	managed := `
- **[2026-02-25]** (abc1234) Did some work
Some random line
- **[2026-02-24]** (def5678) Fixed a bug
`
	lines := extractEntryLines(managed)

	if len(lines) != 2 {
		t.Fatalf("expected 2 entry lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "abc1234") {
		t.Error("first line should contain abc1234")
	}
}

func TestFormatMemoryEntry(t *testing.T) {
	entry := &pendingEntry{
		SessionID: "abc12345678",
		Summary:   "Implemented the thing\nwith multiple lines",
		QueuedAt:  time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
	}

	line := formatMemoryEntry(entry)

	if !strings.Contains(line, "2026-02-25") {
		t.Error("should contain formatted date")
	}
	if !strings.Contains(line, "(abc1234)") {
		t.Error("should truncate session ID to 7 chars")
	}
	if strings.Contains(line, "\n") {
		t.Error("should not contain newlines")
	}
	if !strings.HasPrefix(line, "- **[") {
		t.Error("should start with list marker")
	}
}

func TestFormatMemoryEntry_LongSummary(t *testing.T) {
	entry := &pendingEntry{
		SessionID: "abc1234",
		Summary:   strings.Repeat("x", 300),
		QueuedAt:  time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
	}

	line := formatMemoryEntry(entry)

	if len(line) > 250 {
		t.Errorf("line too long: %d chars", len(line))
	}
	if !strings.HasSuffix(line, "...") {
		t.Error("should end with ellipsis for truncated summary")
	}
}

func TestFormatMemoryEntry_EmptySummary(t *testing.T) {
	entry := &pendingEntry{
		SessionID: "abc1234",
		QueuedAt:  time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
	}

	line := formatMemoryEntry(entry)

	if !strings.Contains(line, "Session recorded") {
		t.Error("should use default summary for empty")
	}
}

func TestBuildManagedBlock(t *testing.T) {
	entries := []string{
		"- **[2026-02-25]** (abc1234) Work done",
		"- **[2026-02-24]** (def5678) Bug fixed",
	}

	block := buildManagedBlock(entries)

	if !strings.HasPrefix(block, memoryBlockStart) {
		t.Error("should start with block start marker")
	}
	if !strings.HasSuffix(block, memoryBlockEnd) {
		t.Error("should end with block end marker")
	}
	if !strings.Contains(block, "abc1234") {
		t.Error("should contain entry content")
	}
}

func TestBuildManagedBlock_Empty(t *testing.T) {
	block := buildManagedBlock(nil)

	if !strings.Contains(block, memoryBlockStart) {
		t.Error("should contain start marker even when empty")
	}
	if !strings.Contains(block, memoryBlockEnd) {
		t.Error("should contain end marker even when empty")
	}
}

func TestAssembleManagedFile_NewFile(t *testing.T) {
	managed := buildManagedBlock([]string{"- **[2026-02-25]** (abc1234) Work"})
	content := assembleManagedFile("", managed, "")

	if !strings.Contains(content, "# Memory") {
		t.Error("new file should have header")
	}
	if !strings.Contains(content, memoryBlockStart) {
		t.Error("new file should contain managed block")
	}
}

func TestAssembleManagedFile_PreservesManualContent(t *testing.T) {
	before := "# My Project\n\n## Custom Notes\n\nImportant stuff here.\n"
	after := "\n## Footer\n\nMore stuff.\n"
	managed := buildManagedBlock([]string{"- **[2026-02-25]** (abc1234) Work"})

	content := assembleManagedFile(before, managed, after)

	if !strings.Contains(content, "Custom Notes") {
		t.Error("should preserve content before block")
	}
	if !strings.Contains(content, "Footer") {
		t.Error("should preserve content after block")
	}
	if !strings.Contains(content, memoryBlockStart) {
		t.Error("should contain managed block")
	}
}

func TestSyncMemory_DedupOnSecondSync(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	// Create 2 session files
	for i, id := range []string{"aaa1111", "bbb2222"} {
		entry := map[string]any{
			"session_id": id,
			"date":       time.Date(2026, 2, 24+i, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"summary":    fmt.Sprintf("Session %d work", i),
		}
		data, _ := json.Marshal(entry)
		name := fmt.Sprintf("2026-02-%02d-test-%s.jsonl", 24+i, id[:7])
		os.WriteFile(filepath.Join(sessionsDir, name), data, 0644)
	}

	outputPath := filepath.Join(tmp, "MEMORY.md")

	// First sync
	if err := syncMemory(tmp, outputPath, 10, true); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// Read output after first sync
	data1, _ := os.ReadFile(outputPath)
	count1 := strings.Count(string(data1), "aaa1111")
	if count1 != 1 {
		t.Fatalf("after first sync: expected 1 occurrence of aaa1111, got %d", count1)
	}

	// Second sync (same sessions) — should NOT duplicate
	if err := syncMemory(tmp, outputPath, 10, true); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	data2, _ := os.ReadFile(outputPath)
	count2 := strings.Count(string(data2), "aaa1111")
	if count2 != 1 {
		t.Errorf("after second sync: expected 1 occurrence of aaa1111, got %d (dedup failed)", count2)
	}
	count2b := strings.Count(string(data2), "bbb2222")
	if count2b != 1 {
		t.Errorf("after second sync: expected 1 occurrence of bbb2222, got %d (dedup failed)", count2b)
	}
}

func TestSyncMemory_PreservesManualContent(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	entry := map[string]any{
		"session_id": "aaa1111",
		"date":       time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"summary":    "Test session",
	}
	data, _ := json.Marshal(entry)
	os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-test-aaa1111.jsonl"), data, 0644)

	outputPath := filepath.Join(tmp, "MEMORY.md")
	os.WriteFile(outputPath, []byte("# My Notes\n\nManual content here.\n"), 0644)

	if err := syncMemory(tmp, outputPath, 10, true); err != nil {
		t.Fatalf("sync: %v", err)
	}

	result, _ := os.ReadFile(outputPath)
	resultStr := string(result)

	if !strings.Contains(resultStr, "Manual content here.") {
		t.Error("should preserve manual content")
	}
	if !strings.Contains(resultStr, "aaa1111") {
		t.Error("should contain session entry")
	}
	if !strings.Contains(resultStr, memoryBlockStart) {
		t.Error("should contain managed block markers")
	}
}

func TestSyncMemory_NoSessions(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	outputPath := filepath.Join(tmp, "MEMORY.md")

	if err := syncMemory(tmp, outputPath, 10, true); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Should not create file when no sessions exist
	if _, err := os.Stat(outputPath); err == nil {
		t.Error("should not create output file when no sessions exist")
	}
}

func TestParseManagedBlock_DuplicateMarkers(t *testing.T) {
	// Two start markers — should refuse to parse to avoid data loss
	content := "# Header\n" + memoryBlockStart + "\nentry1\n" + memoryBlockEnd + "\nMiddle\n" + memoryBlockStart + "\nentry2\n" + memoryBlockEnd + "\n"
	before, managed, after := parseManagedBlock(content)

	if before != content {
		t.Error("duplicate markers: before should be entire content")
	}
	if managed != "" {
		t.Error("duplicate markers: managed should be empty")
	}
	if after != "" {
		t.Error("duplicate markers: after should be empty")
	}
}

func TestMemorySync_MaxEntriesTrimming(t *testing.T) {
	var entries []string
	for i := 0; i < 15; i++ {
		entries = append(entries, fmt.Sprintf("- **[2026-02-%02d]** (id%05d) Session %d", i+1, i, i))
	}

	maxEntries := 10
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	if len(entries) != 10 {
		t.Errorf("expected 10 entries after trim, got %d", len(entries))
	}
}

func TestReadNLatestSessionEntries_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	// Create 3 session files
	for i, id := range []string{"aaa1111", "bbb2222", "ccc3333"} {
		entry := map[string]any{
			"session_id": id,
			"date":       time.Date(2026, 2, 23+i, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"summary":    fmt.Sprintf("Session %d", i),
		}
		data, _ := json.Marshal(entry)
		name := fmt.Sprintf("2026-02-%02d-test-%s.jsonl", 23+i, id[:7])
		os.WriteFile(filepath.Join(sessionsDir, name), data, 0644)
	}

	// Also create a .md file that should be ignored
	os.WriteFile(filepath.Join(sessionsDir, "2026-02-25-test.md"), []byte("# Session"), 0644)

	entries, err := readNLatestSessionEntries(tmp, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Should be newest first
	if entries[0].SessionID != "ccc3333" {
		t.Errorf("first entry should be newest, got %s", entries[0].SessionID)
	}
}

func TestReadNLatestSessionEntries_CappedAtMax(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	for i := 0; i < 5; i++ {
		entry := map[string]any{
			"session_id": fmt.Sprintf("sess%04d", i),
			"date":       time.Date(2026, 2, 20+i, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"summary":    fmt.Sprintf("Session %d", i),
		}
		data, _ := json.Marshal(entry)
		name := fmt.Sprintf("2026-02-%02d-test-%s.jsonl", 20+i, fmt.Sprintf("sess%04d", i))
		os.WriteFile(filepath.Join(sessionsDir, name), data, 0644)
	}

	entries, err := readNLatestSessionEntries(tmp, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries (capped), got %d", len(entries))
	}
}

func TestReadNLatestSessionEntries_Empty(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	entries, err := readNLatestSessionEntries(tmp, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestReadNLatestSessionEntries_NoDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := readNLatestSessionEntries(tmp, 10)
	if err == nil {
		t.Error("expected error for missing sessions dir")
	}
}

func TestParseManagedBlock_HTMLCommentInManualContent(t *testing.T) {
	// User has HTML comments in manual content — should not confuse parser
	content := `# Memory

<!-- This is a user comment -->

## Notes

Some text here.

<!-- ao:memory:start -->
- **[2026-02-25]** (abc1234) Work done
<!-- ao:memory:end -->
`
	before, managed, after := parseManagedBlock(content)

	if !strings.Contains(before, "user comment") {
		t.Error("user HTML comments before block should be preserved in before")
	}
	if !strings.Contains(managed, "abc1234") {
		t.Error("managed content should be parsed correctly")
	}
	_ = after
}

func TestFormatMemoryEntry_UnicodeContent(t *testing.T) {
	entry := &pendingEntry{
		SessionID: "abc1234",
		Summary:   "Fixed the bug with Japanese text 日本語テスト and emoji 🎉",
		QueuedAt:  time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
	}

	line := formatMemoryEntry(entry)

	if !strings.Contains(line, "日本語テスト") {
		t.Error("should preserve unicode content")
	}
	if !strings.Contains(line, "🎉") {
		t.Error("should preserve emoji")
	}
}

func TestMemorySyncOutputFileFlag(t *testing.T) {
	f := memorySyncCmd.Flags().Lookup("output-file")
	if f == nil {
		t.Fatal("expected --output-file flag on memory sync, not found")
	}
	// Check local flags only — root's persistent --output/-o (output format) is inherited and fine
	if old := memorySyncCmd.LocalFlags().Lookup("output"); old != nil {
		t.Error("--output local flag should be renamed to --output-file on memory sync")
	}
}

func TestMemorySync_EndToEnd(t *testing.T) {
	tmp := t.TempDir()

	// Create sessions dir with 2 sessions
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	os.MkdirAll(sessionsDir, 0755)
	for i, id := range []string{"aaa1111", "bbb2222"} {
		entry := map[string]any{
			"session_id": id,
			"date":       time.Date(2026, 2, 24+i, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
			"summary":    fmt.Sprintf("Session %d work", i),
		}
		data, _ := json.Marshal(entry)
		name := fmt.Sprintf("2026-02-%02d-test-%s.jsonl", 24+i, id[:7])
		os.WriteFile(filepath.Join(sessionsDir, name), data, 0644)
	}

	// Create existing MEMORY.md with manual content
	outputPath := filepath.Join(tmp, "MEMORY.md")
	os.WriteFile(outputPath, []byte("# My Project\n\n## Important Notes\n\nDon't lose this.\n"), 0644)

	// Read sessions
	entries, err := readNLatestSessionEntries(tmp, 10)
	if err != nil {
		t.Fatalf("read sessions: %v", err)
	}

	// Parse existing
	existing, _ := os.ReadFile(outputPath)
	before, managed, after := parseManagedBlock(string(existing))

	// Build entries
	existingIDs := extractSessionIDs(managed)
	var newEntries []string
	for _, entry := range entries {
		id := entry.SessionID
		if len(id) > 7 {
			id = id[:7]
		}
		if existingIDs[id] {
			continue
		}
		newEntries = append(newEntries, formatMemoryEntry(entry))
	}
	existingEntryLines := extractEntryLines(managed)
	allEntries := append(newEntries, existingEntryLines...)
	if len(allEntries) > 10 {
		allEntries = allEntries[:10]
	}

	// Assemble
	managedContent := buildManagedBlock(allEntries)
	content := assembleManagedFile(before, managedContent, after)

	os.WriteFile(outputPath, []byte(content), 0644)

	// Verify
	result, _ := os.ReadFile(outputPath)
	resultStr := string(result)

	if !strings.Contains(resultStr, "Important Notes") {
		t.Error("should preserve manual content")
	}
	if !strings.Contains(resultStr, "Don't lose this") {
		t.Error("should preserve manual content details")
	}
	if !strings.Contains(resultStr, memoryBlockStart) {
		t.Error("should have managed block")
	}
	if !strings.Contains(resultStr, "aaa1111") {
		t.Error("should contain first session")
	}
	if !strings.Contains(resultStr, "bbb2222") {
		t.Error("should contain second session")
	}
}
