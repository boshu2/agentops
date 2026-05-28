package notebook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTruncate_RuneSafe(t *testing.T) {
	got := Truncate("aébbbb", 5)
	if got != "aé..." {
		t.Fatalf("Truncate unicode boundary = %q, want %q", got, "aé...")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("Truncate returned invalid UTF-8: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"short string passes through", "hello", 10, "hello"},
		{"exact length passes through", "hello", 5, "hello"},
		{"truncates with ellipsis", "hello world", 8, "hello..."},
		{"zero n", "hello", 0, ""},
		{"negative n", "hello", -1, ""},
		{"n=1", "hello", 1, "."},
		{"n=2", "hello", 2, ".."},
		{"n=3", "hello", 3, "..."},
		{"n=4", "hello world", 4, "h..."},
		{"empty string", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

func TestParseSectionsFromString(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantHead string
	}{
		{
			name:     "basic sections",
			content:  "# Title\nIntro\n\n## Section A\nContent A\n\n## Section B\nContent B\n",
			wantLen:  3,
			wantHead: "# Title",
		},
		{
			name:    "empty content",
			content: "",
			wantLen: 0,
		},
		{
			name:     "single heading",
			content:  "# Just Title\nSome content\n",
			wantLen:  1,
			wantHead: "# Just Title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := ParseSectionsFromString(tt.content)
			if len(sections) != tt.wantLen {
				t.Fatalf("got %d sections, want %d", len(sections), tt.wantLen)
			}
			if tt.wantLen > 0 && sections[0].Heading != tt.wantHead {
				t.Errorf("first heading: got %q, want %q", sections[0].Heading, tt.wantHead)
			}
		})
	}
}

func TestCategorizeKnowledge(t *testing.T) {
	items := []string{
		"Worked: refactored the parser",
		"Next: add more tests",
		"Todo: clean up docs",
		"Follow-up: check performance",
		"Success: all tests pass",
		"Resolved: fixed the bug",
		"General insight about architecture",
		"Another observation",
	}
	worked, next, other := CategorizeKnowledge(items)

	if len(worked) != 3 {
		t.Errorf("worked: got %d, want 3: %v", len(worked), worked)
	}
	if len(next) != 3 {
		t.Errorf("next: got %d, want 3: %v", len(next), next)
	}
	if len(other) != 2 {
		t.Errorf("other: got %d, want 2: %v", len(other), other)
	}

	emptyWorked, emptyNext, emptyOther := CategorizeKnowledge(nil)
	if len(emptyWorked) != 0 || len(emptyNext) != 0 || len(emptyOther) != 0 {
		t.Error("nil input should return empty slices")
	}
}

func TestTotalLines(t *testing.T) {
	sections := []Section{
		{Heading: "## A", Lines: []string{"line1", "line2"}},
		{Heading: "## B", Lines: []string{"line3"}},
		{Heading: "", Lines: []string{"preamble"}},
	}
	got := TotalLines(sections)
	if got != 6 {
		t.Errorf("TotalLines = %d, want 6", got)
	}

	if TotalLines(nil) != 0 {
		t.Error("nil sections should return 0")
	}
}

func TestRender(t *testing.T) {
	sections := []Section{
		{Heading: "# Title", Lines: []string{"Intro line"}},
		{Heading: "## Section", Lines: []string{"Content here", ""}},
	}
	got := Render(sections)
	if !strings.HasPrefix(got, "# Title\n") {
		t.Error("should start with title heading")
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("should end with newline")
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Error("should not end with double newline")
	}
}

func TestUpsertLastSession(t *testing.T) {
	existing := []Section{
		{Heading: "# Title", Lines: []string{"Intro"}},
		{Heading: "## Last Session", Lines: []string{"old data"}},
		{Heading: "## Other", Lines: []string{"kept"}},
	}
	newSession := Section{Heading: "## Last Session", Lines: []string{"new data"}}

	result := UpsertLastSession(existing, newSession)
	found := false
	for _, s := range result {
		if s.Heading == "## Last Session" {
			if found {
				t.Error("duplicate Last Session sections")
			}
			found = true
			if len(s.Lines) != 1 || s.Lines[0] != "new data" {
				t.Errorf("Last Session content: %v", s.Lines)
			}
		}
	}
	if !found {
		t.Error("Last Session section not found")
	}

	noExisting := UpsertLastSession(nil, newSession)
	if len(noExisting) != 1 {
		t.Errorf("insert into empty: got %d sections", len(noExisting))
	}

	noLastSession := []Section{
		{Heading: "# Title", Lines: []string{"Intro"}},
		{Heading: "## Other", Lines: []string{"data"}},
	}
	inserted := UpsertLastSession(noLastSession, newSession)
	if len(inserted) != 3 {
		t.Errorf("insert new: got %d sections, want 3", len(inserted))
	}
	if inserted[1].Heading != "## Last Session" {
		t.Errorf("inserted at wrong position: %v", inserted[1].Heading)
	}
}

func TestBuildLastSessionSection(t *testing.T) {
	entry := &PendingEntry{
		SessionID: "test-123",
		Summary:   "Did some work on tests",
		Decisions: []string{"Use table-driven tests"},
		Knowledge: []string{"Worked: refactored parser", "Next: add coverage"},
		QueuedAt:  time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
	}
	section := BuildLastSessionSection(entry, Truncate)
	if section.Heading != "## Last Session" {
		t.Errorf("heading: got %q", section.Heading)
	}
	content := strings.Join(section.Lines, "\n")
	if !strings.Contains(content, "2026-05-27") {
		t.Error("missing date")
	}
	if !strings.Contains(content, "Did some work") {
		t.Error("missing summary")
	}
	if !strings.Contains(content, "table-driven") {
		t.Error("missing decision")
	}
}

func TestAppendBulletSection(t *testing.T) {
	lines := []string{"existing"}
	result := AppendBulletSection(lines, "- **Items:**", []string{"item1", "item2"}, 100, Truncate)
	if len(result) != 4 {
		t.Fatalf("got %d lines, want 4", len(result))
	}
	if result[1] != "- **Items:**" {
		t.Errorf("heading: got %q", result[1])
	}
	if result[2] != "  - item1" {
		t.Errorf("item: got %q", result[2])
	}

	empty := AppendBulletSection(lines, "- **Items:**", nil, 100, Truncate)
	if len(empty) != 1 {
		t.Errorf("empty items should not add lines, got %d", len(empty))
	}
}

func TestPrune(t *testing.T) {
	sections := []Section{
		{Heading: "# Title", Lines: []string{"Intro"}},
		{Heading: "## Last Session", Lines: []string{"data"}},
		{Heading: "## Long Section", Lines: []string{"a", "b", "c", "d", "e"}},
	}
	pruned := Prune(sections, 5)
	total := TotalLines(pruned)
	if total > 5 {
		t.Errorf("pruned to %d lines, want <= 5", total)
	}
	for _, s := range pruned {
		if s.Heading == "## Last Session" && len(s.Lines) == 0 {
			t.Error("Last Session should not be pruned")
		}
	}
}

func TestSessionEntryToPendingEntry(t *testing.T) {
	entry := SessionEntry{
		SessionID: "sess-1",
		Date:      time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		Summary:   "test",
		Decisions: []string{"d1"},
		Knowledge: []string{"k1"},
	}
	pending := entry.ToPendingEntry()
	if pending.SessionID != "sess-1" {
		t.Errorf("SessionID: got %q", pending.SessionID)
	}
	if pending.Summary != "test" {
		t.Errorf("Summary: got %q", pending.Summary)
	}
	if len(pending.Decisions) != 1 || pending.Decisions[0] != "d1" {
		t.Errorf("Decisions: got %v", pending.Decisions)
	}
	if pending.QueuedAt != entry.Date {
		t.Error("QueuedAt should equal Date")
	}
}

func TestReadLatestPendingEntry(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := []PendingEntry{
		{SessionID: "s1", Summary: "first"},
		{SessionID: "s2", Summary: "second"},
	}
	var lines []string
	for _, e := range entries {
		b, _ := json.Marshal(e)
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(filepath.Join(aoDir, "pending.jsonl"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadLatestPendingEntry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SessionID != "s2" {
		t.Errorf("expected last entry s2, got %q", got.SessionID)
	}
}

func TestReadSessionFile(t *testing.T) {
	dir := t.TempDir()
	entry := SessionEntry{
		SessionID: "test-session",
		Date:      time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		Summary:   "test session summary",
	}
	data, _ := json.Marshal(entry)
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SessionID != "test-session" {
		t.Errorf("SessionID: got %q", got.SessionID)
	}
	if got.Summary != "test session summary" {
		t.Errorf("Summary: got %q", got.Summary)
	}

	emptyPath := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(emptyPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ReadSessionFile(emptyPath)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadWriteCursor(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "cursor.json")

	if err := WriteCursor(cursorPath, "session-abc"); err != nil {
		t.Fatalf("WriteCursor: %v", err)
	}
	got, err := ReadCursor(cursorPath)
	if err != nil {
		t.Fatalf("ReadCursor: %v", err)
	}
	if got != "session-abc" {
		t.Errorf("ReadCursor: got %q, want %q", got, "session-abc")
	}

	_, err = ReadCursor(filepath.Join(dir, "nonexistent.json"))
	if err == nil {
		t.Error("expected error for missing cursor file")
	}
}

func TestResolveSource(t *testing.T) {
	_, err := ResolveSource(t.TempDir(), "invalid-source")
	if err == nil {
		t.Error("expected error for unknown source")
	}
	if !strings.Contains(err.Error(), "unknown source") {
		t.Errorf("error should mention unknown source: %v", err)
	}
}
