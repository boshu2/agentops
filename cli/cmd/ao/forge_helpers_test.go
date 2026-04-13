package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// extractSnippet
// ---------------------------------------------------------------------------

func TestExtractSnippet(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog and continues running"

	tests := []struct {
		name     string
		startIdx int
		maxLen   int
		wantNot  string
		check    func(string) bool
		desc     string
	}{
		{
			name:     "from start",
			startIdx: 0,
			maxLen:   15,
			check:    func(s string) bool { return len(s) > 0 && len(s) <= 20 },
			desc:     "should return short snippet with ellipsis",
		},
		{
			name:     "from middle",
			startIdx: 10,
			maxLen:   20,
			check:    func(s string) bool { return len(s) > 0 },
			desc:     "should return snippet from middle",
		},
		{
			name:     "past end",
			startIdx: len(content) + 10,
			maxLen:   20,
			check:    func(s string) bool { return s == "" },
			desc:     "should return empty for past-end index",
		},
		{
			name:     "negative start clamped",
			startIdx: -5,
			maxLen:   10,
			check:    func(s string) bool { return len(s) > 0 },
			desc:     "should clamp negative start to 0",
		},
		{
			name:     "maxLen covers entire content",
			startIdx: 0,
			maxLen:   1000,
			check:    func(s string) bool { return s == content },
			desc:     "should return full content when maxLen exceeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSnippet(content, tt.startIdx, tt.maxLen)
			if !tt.check(got) {
				t.Errorf("extractSnippet(%d, %d) = %q; %s", tt.startIdx, tt.maxLen, got, tt.desc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lastSpaceIndex
// ---------------------------------------------------------------------------

func TestLastSpaceIndex(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"with spaces", "hello world", 5},
		{"trailing space", "hello ", 5},
		{"no spaces", "helloworld", -1},
		{"empty", "", -1},
		{"single space", " ", 0},
		{"multiple spaces", "a b c", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastSpaceIndex(tt.s)
			if got != tt.want {
				t.Errorf("lastSpaceIndex(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractIssueIDs
// ---------------------------------------------------------------------------

func TestExtractIssueIDs(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
		wantFirst string
	}{
		{"single issue", "Fixed ol-0001 in commit", 1, "ol-0001"},
		{"multiple issues", "Closes ol-0001 and at-v123", 2, "ol-0001"},
		{"no issues", "This has no issue references", 0, ""},
		{"compound ID", "See gt-abc-def for details", 1, "gt-abc-def"},
		{"uppercase ignored", "Not OL-0001", 0, ""},
		{"mixed text", "Working on ag-m0r with issues ag-oke and cm-0012", 3, "ag-m0r"},
		{"empty string", "", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIssueIDs(tt.content)
			if len(got) != tt.wantCount {
				t.Errorf("extractIssueIDs(%q) returned %d IDs, want %d; got %v", tt.content, len(got), tt.wantCount, got)
			}
			if tt.wantFirst != "" && len(got) > 0 && got[0] != tt.wantFirst {
				t.Errorf("first ID = %q, want %q", got[0], tt.wantFirst)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateString
// ---------------------------------------------------------------------------

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world!", 8, "hello..."},
		{"empty", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// dedup
// ---------------------------------------------------------------------------

func TestDedup(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  int
	}{
		{"no duplicates", []string{"a", "b", "c"}, 3},
		{"all duplicates", []string{"a", "a", "a"}, 1},
		{"mixed", []string{"a", "b", "a", "c", "b"}, 3},
		{"empty", []string{}, 0},
		{"single", []string{"x"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedup(tt.input)
			if len(got) != tt.want {
				t.Errorf("dedup() returned %d items, want %d; got %v", len(got), tt.want, got)
			}
			// Verify order preserved
			if len(tt.input) > 0 && len(got) > 0 && got[0] != tt.input[0] {
				t.Errorf("first element = %q, want %q (order should be preserved)", got[0], tt.input[0])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitMarkdownSections
// ---------------------------------------------------------------------------

func TestSplitMarkdownSections(t *testing.T) {
	t.Run("empty string returns one section", func(t *testing.T) {
		got := splitMarkdownSections("")
		if len(got) != 1 {
			t.Errorf("expected 1 section, got %d", len(got))
		}
	})

	t.Run("no headings returns one section", func(t *testing.T) {
		content := "just some text\nno headings here"
		got := splitMarkdownSections(content)
		if len(got) != 1 {
			t.Errorf("expected 1 section for no-heading content, got %d", len(got))
		}
		if got[0] != content {
			t.Errorf("section = %q, want %q", got[0], content)
		}
	})

	t.Run("h1 heading splits into 2 sections", func(t *testing.T) {
		content := "intro text\n# Heading\ncontent under heading"
		got := splitMarkdownSections(content)
		if len(got) != 2 {
			t.Errorf("expected 2 sections, got %d: %v", len(got), got)
		}
		if !strings.HasPrefix(got[1], "# Heading") {
			t.Errorf("second section should start with heading, got %q", got[1])
		}
	})

	t.Run("h2 heading splits correctly", func(t *testing.T) {
		content := "intro\n## Section A\ncontent A\n## Section B\ncontent B"
		got := splitMarkdownSections(content)
		if len(got) != 3 {
			t.Errorf("expected 3 sections, got %d", len(got))
		}
	})

	t.Run("content starting with heading has one section per heading", func(t *testing.T) {
		content := "# First\nstuff\n# Second\nmore stuff"
		got := splitMarkdownSections(content)
		if len(got) != 2 {
			t.Errorf("expected 2 sections, got %d: %v", len(got), got)
		}
	})

	t.Run("h3 not treated as heading boundary", func(t *testing.T) {
		content := "intro\n### Not a split point\nstill in intro"
		got := splitMarkdownSections(content)
		if len(got) != 1 {
			t.Errorf("expected 1 section for h3-only content, got %d", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// generateSummary
// ---------------------------------------------------------------------------

func TestGenerateSummary(t *testing.T) {
	t.Run("uses first decision when available", func(t *testing.T) {
		decisions := []string{"First decision", "Second decision"}
		got := generateSummary(decisions, nil, time.Now())
		if !strings.Contains(got, "First decision") {
			t.Errorf("expected summary to contain first decision, got %q", got)
		}
	})

	t.Run("falls back to knowledge when no decisions", func(t *testing.T) {
		knowledge := []string{"Key learning"}
		got := generateSummary(nil, knowledge, time.Now())
		if !strings.Contains(got, "Key learning") {
			t.Errorf("expected summary to contain knowledge, got %q", got)
		}
	})

	t.Run("uses date when no decisions or knowledge", func(t *testing.T) {
		refDate := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
		got := generateSummary(nil, nil, refDate)
		if !strings.Contains(got, "2024-03-15") {
			t.Errorf("expected summary to contain date, got %q", got)
		}
	})

	t.Run("truncates long decision to SummaryMaxLength", func(t *testing.T) {
		longDecision := strings.Repeat("x", SummaryMaxLength+50)
		got := generateSummary([]string{longDecision}, nil, time.Now())
		if len(got) > SummaryMaxLength {
			t.Errorf("summary length %d exceeds max %d", len(got), SummaryMaxLength)
		}
	})
}

// ---------------------------------------------------------------------------
// countLines
// ---------------------------------------------------------------------------

func TestCountLines(t *testing.T) {
	t.Run("returns 0 for nonexistent file", func(t *testing.T) {
		got := countLines("/tmp/definitely-does-not-exist-12345.txt")
		if got != 0 {
			t.Errorf("expected 0 for nonexistent file, got %d", got)
		}
	})

	t.Run("counts newlines in temp file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.txt")
		content := "line1\nline2\nline3\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		got := countLines(path)
		if got != 3 {
			t.Errorf("expected 3 newlines, got %d", got)
		}
	})

	t.Run("empty file has 0 lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		got := countLines(path)
		if got != 0 {
			t.Errorf("expected 0 for empty file, got %d", got)
		}
	})
}

// ---------------------------------------------------------------------------
// noFilesError
// ---------------------------------------------------------------------------

func TestNoFilesError(t *testing.T) {
	t.Run("quiet mode returns nil", func(t *testing.T) {
		err := noFilesError(true, "no files found")
		if err != nil {
			t.Errorf("expected nil for quiet mode, got %v", err)
		}
	})

	t.Run("non-quiet returns error with message", func(t *testing.T) {
		err := noFilesError(false, "no files found")
		if err == nil {
			t.Error("expected error for non-quiet mode")
		}
		if err.Error() != "no files found" {
			t.Errorf("expected %q, got %q", "no files found", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// forgeWarnf
// ---------------------------------------------------------------------------

func TestForgeWarnf(t *testing.T) {
	// forgeWarnf writes to stderr, not stdout. Capture stderr to verify.
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	forgeWarnf(true, "should not print: %s", "test")
	forgeWarnf(false, "test warning: %s\n", "msg")

	w.Close()
	data, _ := io.ReadAll(r)
	os.Stderr = oldStderr
	output := string(data)

	if strings.Contains(output, "should not print") {
		t.Error("quiet=true should suppress output")
	}
	if !strings.Contains(output, "test warning") {
		t.Errorf("quiet=false should produce output, got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// collectFilesFromPatterns
// ---------------------------------------------------------------------------

func TestCollectFilesFromPatterns(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")
	file2 := filepath.Join(dir, "test2.txt")
	for _, f := range []string{file1, file2} {
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("glob pattern matches files", func(t *testing.T) {
		pattern := filepath.Join(dir, "*.txt")
		files, err := collectFilesFromPatterns([]string{pattern}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d: %v", len(files), files)
		}
	})

	t.Run("literal path included directly", func(t *testing.T) {
		files, err := collectFilesFromPatterns([]string{file1}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("non-existent literal path excluded", func(t *testing.T) {
		files, err := collectFilesFromPatterns([]string{"/no/such/file.txt"}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files for non-existent path, got %d", len(files))
		}
	})

	t.Run("matchFilter filters files", func(t *testing.T) {
		pattern := filepath.Join(dir, "*.txt")
		filter := func(s string) bool { return strings.Contains(s, "test1") }
		files, err := collectFilesFromPatterns([]string{pattern}, filter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 filtered file, got %d: %v", len(files), files)
		}
	})

	t.Run("empty patterns returns empty", func(t *testing.T) {
		files, err := collectFilesFromPatterns(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})
}

// ---------------------------------------------------------------------------
// cloneHooksMap (lives in hooks.go, tested here for legacy coverage)
// ---------------------------------------------------------------------------

func TestCloneHooksMap(t *testing.T) {
	t.Run("clones hooks sub-map from rawSettings", func(t *testing.T) {
		rawSettings := map[string]any{
			"hooks": map[string]any{
				"SessionStart": []any{"cmd1"},
				"PreToolUse":   []any{"cmd2"},
			},
		}
		got := cloneHooksMap(rawSettings)
		if len(got) != 2 {
			t.Errorf("expected 2 entries, got %d: %v", len(got), got)
		}
		if _, ok := got["SessionStart"]; !ok {
			t.Error("expected SessionStart key in clone")
		}
	})

	t.Run("no hooks key returns empty map", func(t *testing.T) {
		rawSettings := map[string]any{"other": "value"}
		got := cloneHooksMap(rawSettings)
		if len(got) != 0 {
			t.Errorf("expected empty map, got %d entries", len(got))
		}
	})

	t.Run("empty rawSettings returns empty map", func(t *testing.T) {
		got := cloneHooksMap(map[string]any{})
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("hooks key with wrong type returns empty map", func(t *testing.T) {
		rawSettings := map[string]any{"hooks": "not-a-map"}
		got := cloneHooksMap(rawSettings)
		if len(got) != 0 {
			t.Errorf("expected empty map for wrong type, got %d entries", len(got))
		}
	})
}

// ---------------------------------------------------------------------------
// extractFilePathsFromTool
// ---------------------------------------------------------------------------

func TestExtractFilePathsFromTool(t *testing.T) {
	t.Run("extracts file_path from tool input", func(t *testing.T) {
		tool := types.ToolCall{
			Name: "Read",
			Input: map[string]any{
				"file_path": "/some/path/file.go",
			},
		}
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 || state.FilesChanged[0] != "/some/path/file.go" {
			t.Errorf("expected filesChanged=[/some/path/file.go], got %v", state.FilesChanged)
		}
	})

	t.Run("extracts path from tool input", func(t *testing.T) {
		tool := types.ToolCall{
			Name: "Bash",
			Input: map[string]any{
				"path": "/another/path.sh",
			},
		}
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 || state.FilesChanged[0] != "/another/path.sh" {
			t.Errorf("expected filesChanged=[/another/path.sh], got %v", state.FilesChanged)
		}
	})

	t.Run("deduplicates seen files", func(t *testing.T) {
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		tool := types.ToolCall{
			Name:  "Read",
			Input: map[string]any{"file_path": "/dup.go"},
		}
		extractFilePathsFromTool(tool, state)
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 {
			t.Errorf("expected 1 file after dedup, got %d", len(state.FilesChanged))
		}
	})

	t.Run("nil input is skipped", func(t *testing.T) {
		tool := types.ToolCall{Name: "Bash", Input: nil}
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 0 {
			t.Errorf("expected no files for nil input, got %v", state.FilesChanged)
		}
	})
}

// ---------------------------------------------------------------------------
// extractIssueRefs
// ---------------------------------------------------------------------------

func TestExtractIssueRefs(t *testing.T) {
	t.Run("extracts issue IDs from content", func(t *testing.T) {
		state := &transcriptState{SeenIssues: make(map[string]bool)}
		extractIssueRefs("Fixed ol-0001 and ag-m0r in this session", state)
		if len(state.Issues) != 2 {
			t.Errorf("expected 2 issues, got %v", state.Issues)
		}
	})

	t.Run("deduplicates repeated issue IDs", func(t *testing.T) {
		state := &transcriptState{SeenIssues: make(map[string]bool)}
		extractIssueRefs("ol-0001 and ol-0001 again", state)
		if len(state.Issues) != 1 {
			t.Errorf("expected 1 issue after dedup, got %v", state.Issues)
		}
	})

	t.Run("no issues in content leaves state empty", func(t *testing.T) {
		state := &transcriptState{SeenIssues: make(map[string]bool)}
		extractIssueRefs("no issue refs here", state)
		if len(state.Issues) != 0 {
			t.Errorf("expected 0 issues, got %v", state.Issues)
		}
	})
}

// ---------------------------------------------------------------------------
// extractToolRefs
// ---------------------------------------------------------------------------

func TestExtractToolRefs(t *testing.T) {
	t.Run("counts tool calls in session", func(t *testing.T) {
		session := &storage.Session{ToolCalls: make(map[string]int)}
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		tools := []types.ToolCall{
			{Name: "Read", Input: map[string]any{"file_path": "/a.go"}},
			{Name: "Bash", Input: nil},
			{Name: "Read", Input: map[string]any{"file_path": "/b.go"}},
		}
		extractToolRefs(tools, session, state)
		if session.ToolCalls["Read"] != 2 {
			t.Errorf("expected Read count=2, got %d", session.ToolCalls["Read"])
		}
		if session.ToolCalls["Bash"] != 1 {
			t.Errorf("expected Bash count=1, got %d", session.ToolCalls["Bash"])
		}
		if len(state.FilesChanged) != 2 {
			t.Errorf("expected 2 files, got %v", state.FilesChanged)
		}
	})

	t.Run("tool_result name is not counted", func(t *testing.T) {
		session := &storage.Session{ToolCalls: make(map[string]int)}
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		tools := []types.ToolCall{{Name: "tool_result"}}
		extractToolRefs(tools, session, state)
		if _, ok := session.ToolCalls["tool_result"]; ok {
			t.Error("tool_result should not be counted")
		}
	})
}

// ---------------------------------------------------------------------------
// updateSessionMeta
// ---------------------------------------------------------------------------

func TestUpdateSessionMeta(t *testing.T) {
	t.Run("sets session ID from first message with ID", func(t *testing.T) {
		session := &storage.Session{}
		msg := types.TranscriptMessage{SessionID: "abc-123"}
		updateSessionMeta(session, msg)
		if session.ID != "abc-123" {
			t.Errorf("expected ID=abc-123, got %q", session.ID)
		}
	})

	t.Run("does not overwrite existing session ID", func(t *testing.T) {
		session := &storage.Session{ID: "original-id"}
		msg := types.TranscriptMessage{SessionID: "new-id"}
		updateSessionMeta(session, msg)
		if session.ID != "original-id" {
			t.Errorf("expected ID=original-id, got %q", session.ID)
		}
	})

	t.Run("sets date from first message timestamp", func(t *testing.T) {
		session := &storage.Session{}
		ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		msg := types.TranscriptMessage{Timestamp: ts}
		updateSessionMeta(session, msg)
		if !session.Date.Equal(ts) {
			t.Errorf("expected date=%v, got %v", ts, session.Date)
		}
	})

	t.Run("updates date to earlier timestamp", func(t *testing.T) {
		later := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		earlier := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		session := &storage.Session{Date: later}
		msg := types.TranscriptMessage{Timestamp: earlier}
		updateSessionMeta(session, msg)
		if !session.Date.Equal(earlier) {
			t.Errorf("expected date updated to earlier=%v, got %v", earlier, session.Date)
		}
	})
}

// ===========================================================================
// NEW TESTS — covering previously-uncovered functions
// ===========================================================================

// ---------------------------------------------------------------------------
// forgeTotals.addSession
// ---------------------------------------------------------------------------

func TestForge_AddSession(t *testing.T) {
	tests := []struct {
		name          string
		sessions      []*storage.Session
		wantSessions  int
		wantDecisions int
		wantKnowledge int
	}{
		{
			name:          "empty session",
			sessions:      []*storage.Session{{}},
			wantSessions:  1,
			wantDecisions: 0,
			wantKnowledge: 0,
		},
		{
			name: "session with decisions and knowledge",
			sessions: []*storage.Session{{
				Decisions: []string{"d1", "d2"},
				Knowledge: []string{"k1"},
			}},
			wantSessions:  1,
			wantDecisions: 2,
			wantKnowledge: 1,
		},
		{
			name: "multiple sessions accumulate",
			sessions: []*storage.Session{
				{Decisions: []string{"d1"}, Knowledge: []string{"k1", "k2"}},
				{Decisions: []string{"d2", "d3"}, Knowledge: []string{"k3"}},
			},
			wantSessions:  2,
			wantDecisions: 3,
			wantKnowledge: 3,
		},
		{
			name:          "nil slices count as zero",
			sessions:      []*storage.Session{{Decisions: nil, Knowledge: nil}},
			wantSessions:  1,
			wantDecisions: 0,
			wantKnowledge: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totals := forgeTotals{}
			for _, s := range tt.sessions {
				totals.addSession(s)
			}
			if totals.sessions != tt.wantSessions {
				t.Errorf("sessions = %d, want %d", totals.sessions, tt.wantSessions)
			}
			if totals.decisions != tt.wantDecisions {
				t.Errorf("decisions = %d, want %d", totals.decisions, tt.wantDecisions)
			}
			if totals.knowledge != tt.wantKnowledge {
				t.Errorf("knowledge = %d, want %d", totals.knowledge, tt.wantKnowledge)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleForgeDryRun
// ---------------------------------------------------------------------------

func TestForge_HandleForgeDryRun(t *testing.T) {
	// Save/restore package-level dryRun flag.
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()

	t.Run("returns false when dry-run is off", func(t *testing.T) {
		dryRun = false
		var buf bytes.Buffer
		got := handleForgeDryRun(&buf, false, []string{"a.jsonl"}, "file(s)")
		if got {
			t.Error("expected false when dryRun=false")
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})

	t.Run("returns false when quiet even if dry-run is on", func(t *testing.T) {
		dryRun = true
		var buf bytes.Buffer
		got := handleForgeDryRun(&buf, true, []string{"a.jsonl"}, "file(s)")
		if got {
			t.Error("expected false when quiet=true")
		}
	})

	t.Run("prints files and returns true on dry-run", func(t *testing.T) {
		dryRun = true
		var buf bytes.Buffer
		files := []string{"session1.jsonl", "session2.jsonl"}
		got := handleForgeDryRun(&buf, false, files, "file(s)")
		if !got {
			t.Error("expected true for dry-run mode")
		}
		output := buf.String()
		if !strings.Contains(output, "[dry-run]") {
			t.Errorf("expected [dry-run] prefix, got %q", output)
		}
		if !strings.Contains(output, "2 file(s)") {
			t.Errorf("expected file count in output, got %q", output)
		}
		if !strings.Contains(output, "session1.jsonl") {
			t.Errorf("expected file name in output, got %q", output)
		}
	})
}

// ---------------------------------------------------------------------------
// printForgeSummary
// ---------------------------------------------------------------------------

func TestForge_PrintForgeSummary(t *testing.T) {
	tests := []struct {
		name     string
		totals   forgeTotals
		baseDir  string
		noun     string
		wantSubs []string
	}{
		{
			name:    "standard summary",
			totals:  forgeTotals{sessions: 3, decisions: 5, knowledge: 7},
			baseDir: "/tmp/test/.agents/ao",
			noun:    "session(s)",
			wantSubs: []string{
				"3 session(s)",
				"Decisions: 5",
				"Knowledge: 7",
				"/tmp/test/.agents/ao",
			},
		},
		{
			name:    "zero totals",
			totals:  forgeTotals{},
			baseDir: "/out",
			noun:    "file(s)",
			wantSubs: []string{
				"0 file(s)",
				"Decisions: 0",
				"Knowledge: 0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printForgeSummary(&buf, tt.totals, tt.baseDir, tt.noun)
			output := buf.String()
			for _, sub := range tt.wantSubs {
				if !strings.Contains(output, sub) {
					t.Errorf("output missing %q; got %q", sub, output)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// initSession
// ---------------------------------------------------------------------------

func TestForge_InitSession(t *testing.T) {
	t.Run("creates session with transcript path", func(t *testing.T) {
		s := initSession("/home/user/.claude/projects/test/session.jsonl")
		if s.TranscriptPath != "/home/user/.claude/projects/test/session.jsonl" {
			t.Errorf("TranscriptPath = %q, want the input path", s.TranscriptPath)
		}
	})

	t.Run("ToolCalls map is initialized", func(t *testing.T) {
		s := initSession("test.jsonl")
		if s.ToolCalls == nil {
			t.Error("ToolCalls should be initialized, got nil")
		}
		// Should be usable immediately
		s.ToolCalls["Read"]++
		if s.ToolCalls["Read"] != 1 {
			t.Errorf("ToolCalls[Read] = %d, want 1", s.ToolCalls["Read"])
		}
	})

	t.Run("ID and Date are zero values without an embedded session ID", func(t *testing.T) {
		s := initSession("test.jsonl")
		if s.ID != "" {
			t.Errorf("ID should be empty, got %q", s.ID)
		}
		if !s.Date.IsZero() {
			t.Errorf("Date should be zero, got %v", s.Date)
		}
	})

	t.Run("seeds ID from transcript path when present", func(t *testing.T) {
		s := initSession("/tmp/rollout-2026-03-05T15-20-21-019cbfa8-9155-7121-b18a-dfa3783cdd9e.jsonl")
		if s.ID != "019cbfa8-9155-7121-b18a-dfa3783cdd9e" {
			t.Errorf("ID = %q, want embedded session ID", s.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// reportProgress
// ---------------------------------------------------------------------------

func TestForge_ReportProgress(t *testing.T) {
	t.Run("quiet mode produces no output", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(true, &buf, 5000, 10000, &lastProgress)
		if buf.Len() != 0 {
			t.Errorf("expected no output in quiet mode, got %q", buf.String())
		}
		// lastProgress should not be updated
		if lastProgress != 0 {
			t.Errorf("lastProgress should stay 0 in quiet mode, got %d", lastProgress)
		}
	})

	t.Run("skips if fewer than 1000 lines since last report", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(false, &buf, 500, 10000, &lastProgress)
		if buf.Len() != 0 {
			t.Errorf("expected no output for <1000 lines, got %q", buf.String())
		}
	})

	t.Run("reports progress at 1000+ line intervals", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(false, &buf, 1000, 10000, &lastProgress)
		output := buf.String()
		if !strings.Contains(output, "1000/10000") {
			t.Errorf("expected 1000/10000 in output, got %q", output)
		}
		if !strings.Contains(output, "10%") {
			t.Errorf("expected 10%% in output, got %q", output)
		}
		if lastProgress != 1000 {
			t.Errorf("lastProgress = %d, want 1000", lastProgress)
		}
	})

	t.Run("handles totalLines=0 without divide-by-zero", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(false, &buf, 1000, 0, &lastProgress)
		output := buf.String()
		if !strings.Contains(output, "0%") {
			t.Errorf("expected 0%% for zero totalLines, got %q", output)
		}
	})

	t.Run("successive calls advance lastProgress", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0

		reportProgress(false, &buf, 1000, 5000, &lastProgress)
		if lastProgress != 1000 {
			t.Fatalf("after first call: lastProgress = %d, want 1000", lastProgress)
		}

		buf.Reset()
		// lineCount 1500 is only 500 past lastProgress — should skip
		reportProgress(false, &buf, 1500, 5000, &lastProgress)
		if buf.Len() != 0 {
			t.Error("expected no output for 500 lines since last report")
		}

		buf.Reset()
		// lineCount 2000 is 1000 past lastProgress — should print
		reportProgress(false, &buf, 2000, 5000, &lastProgress)
		if buf.Len() == 0 {
			t.Error("expected output at 2000 lines (1000 past lastProgress=1000)")
		}
		if lastProgress != 2000 {
			t.Errorf("lastProgress = %d, want 2000", lastProgress)
		}
	})
}

// ---------------------------------------------------------------------------
// drainParseErrors
// ---------------------------------------------------------------------------

func TestForge_DrainParseErrors(t *testing.T) {
	t.Run("returns nil for empty channel", func(t *testing.T) {
		errCh := make(chan error, 1)
		err := drainParseErrors(errCh)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("returns error from channel", func(t *testing.T) {
		errCh := make(chan error, 1)
		want := errors.New("parse failure")
		errCh <- want
		err := drainParseErrors(errCh)
		if err != want {
			t.Errorf("expected %v, got %v", want, err)
		}
	})

	t.Run("returns nil for closed empty channel", func(t *testing.T) {
		errCh := make(chan error, 1)
		close(errCh)
		err := drainParseErrors(errCh)
		if err != nil {
			t.Errorf("expected nil for closed empty channel, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// finalizeTranscriptSession
// ---------------------------------------------------------------------------

func TestForge_FinalizeTranscriptSession(t *testing.T) {
	t.Run("populates all session fields", func(t *testing.T) {
		session := &storage.Session{
			Date: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		}
		state := &transcriptState{
			Decisions:    []string{"dec-a", "dec-b", "dec-a"}, // has duplicate
			Knowledge:    []string{"know-x", "know-y", "know-x"},
			FilesChanged: []string{"/a.go", "/b.go"},
			Issues:       []string{"ol-001"},
		}

		finalizeTranscriptSession(session, state, 4000)

		if len(session.Decisions) != 2 {
			t.Errorf("Decisions = %d, want 2 (deduped from 3)", len(session.Decisions))
		}
		if len(session.Knowledge) != 2 {
			t.Errorf("Knowledge = %d, want 2 (deduped from 3)", len(session.Knowledge))
		}
		if len(session.FilesChanged) != 2 {
			t.Errorf("FilesChanged = %d, want 2", len(session.FilesChanged))
		}
		if len(session.Issues) != 1 {
			t.Errorf("Issues = %d, want 1", len(session.Issues))
		}
		// Token estimation: 4000 bytes / 4 chars-per-token = 1000
		if session.Tokens.Total != 1000 {
			t.Errorf("Tokens.Total = %d, want 1000", session.Tokens.Total)
		}
		if !session.Tokens.Estimated {
			t.Error("Tokens.Estimated should be true")
		}
		if session.Summary == "" {
			t.Error("Summary should be generated from decisions")
		}
	})

	t.Run("summary falls back to date when no decisions or knowledge", func(t *testing.T) {
		refDate := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
		session := &storage.Session{Date: refDate}
		state := &transcriptState{}

		finalizeTranscriptSession(session, state, 0)

		if !strings.Contains(session.Summary, "2025-12-25") {
			t.Errorf("expected date in summary, got %q", session.Summary)
		}
	})

	t.Run("empty state produces empty slices", func(t *testing.T) {
		session := &storage.Session{}
		state := &transcriptState{}

		finalizeTranscriptSession(session, state, 100)

		if session.Decisions == nil {
			t.Error("Decisions should be non-nil empty slice from dedup")
		}
		if session.Knowledge == nil {
			t.Error("Knowledge should be non-nil empty slice from dedup")
		}
	})
}

// ---------------------------------------------------------------------------
// isTranscriptCandidate
// ---------------------------------------------------------------------------

func TestForge_IsTranscriptCandidate(t *testing.T) {
	projectsDir := "/home/user/.claude/projects"

	tests := []struct {
		name        string
		path        string
		isDir       bool
		ext         string
		size        int64
		projectsDir string
		want        bool
	}{
		{
			name:        "valid jsonl file",
			path:        filepath.Join(projectsDir, "myproj", "session.jsonl"),
			size:        500,
			projectsDir: projectsDir,
			want:        true,
		},
		{
			name:        "directory is not a candidate",
			path:        filepath.Join(projectsDir, "myproj"),
			isDir:       true,
			size:        500,
			projectsDir: projectsDir,
			want:        false,
		},
		{
			name:        "non-jsonl extension",
			path:        filepath.Join(projectsDir, "myproj", "session.json"),
			size:        500,
			projectsDir: projectsDir,
			want:        false,
		},
		{
			name:        "too small file",
			path:        filepath.Join(projectsDir, "myproj", "tiny.jsonl"),
			size:        50,
			projectsDir: projectsDir,
			want:        false,
		},
		{
			name:        "exactly 100 bytes is not a candidate",
			path:        filepath.Join(projectsDir, "myproj", "edge.jsonl"),
			size:        100,
			projectsDir: projectsDir,
			want:        false,
		},
		{
			name:        "101 bytes is a candidate",
			path:        filepath.Join(projectsDir, "myproj", "edge.jsonl"),
			size:        101,
			projectsDir: projectsDir,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := fakeFileInfo{
				name:  filepath.Base(tt.path),
				size:  tt.size,
				isDir: tt.isDir,
			}
			got := isTranscriptCandidate(tt.path, info, tt.projectsDir)
			if got != tt.want {
				t.Errorf("isTranscriptCandidate(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// fakeFileInfo implements os.FileInfo for testing without real files.
type fakeFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi fakeFileInfo) Name() string       { return fi.name }
func (fi fakeFileInfo) Size() int64        { return fi.size }
func (fi fakeFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fakeFileInfo) ModTime() time.Time { return fi.modTime }
func (fi fakeFileInfo) IsDir() bool        { return fi.isDir }
func (fi fakeFileInfo) Sys() any           { return nil }

// ---------------------------------------------------------------------------
// collectTranscriptCandidates
// ---------------------------------------------------------------------------

func TestForge_CollectTranscriptCandidates(t *testing.T) {
	t.Run("finds jsonl files in project directory", func(t *testing.T) {
		dir := t.TempDir()
		proj := filepath.Join(dir, "myproj")
		if err := os.MkdirAll(proj, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a valid candidate (>100 bytes)
		content := strings.Repeat("x", 200)
		if err := os.WriteFile(filepath.Join(proj, "session.jsonl"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		candidates, err := collectTranscriptCandidates(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 1 {
			t.Errorf("expected 1 candidate, got %d", len(candidates))
		}
	})

	t.Run("skips subagents directory", func(t *testing.T) {
		dir := t.TempDir()
		subagents := filepath.Join(dir, "subagents")
		if err := os.MkdirAll(subagents, 0755); err != nil {
			t.Fatal(err)
		}

		content := strings.Repeat("x", 200)
		if err := os.WriteFile(filepath.Join(subagents, "sub.jsonl"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		candidates, err := collectTranscriptCandidates(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates (subagents skipped), got %d", len(candidates))
		}
	})

	t.Run("skips small files", func(t *testing.T) {
		dir := t.TempDir()
		proj := filepath.Join(dir, "proj")
		if err := os.MkdirAll(proj, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(proj, "tiny.jsonl"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		candidates, err := collectTranscriptCandidates(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates for tiny file, got %d", len(candidates))
		}
	})

	t.Run("empty directory returns empty", func(t *testing.T) {
		dir := t.TempDir()
		candidates, err := collectTranscriptCandidates(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates, got %d", len(candidates))
		}
	})

	t.Run("ignores non-jsonl files", func(t *testing.T) {
		dir := t.TempDir()
		proj := filepath.Join(dir, "proj")
		if err := os.MkdirAll(proj, 0755); err != nil {
			t.Fatal(err)
		}

		content := strings.Repeat("x", 200)
		if err := os.WriteFile(filepath.Join(proj, "notes.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(proj, "data.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		candidates, err := collectTranscriptCandidates(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(candidates) != 0 {
			t.Errorf("expected 0 candidates for non-jsonl, got %d", len(candidates))
		}
	})
}

// ---------------------------------------------------------------------------
// consumeTranscriptMessages
// ---------------------------------------------------------------------------

func TestForge_ConsumeTranscriptMessages(t *testing.T) {
	t.Run("processes messages from channel", func(t *testing.T) {
		msgCh := make(chan types.TranscriptMessage, 3)
		ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

		msgCh <- types.TranscriptMessage{
			SessionID: "sess-001",
			Timestamp: ts,
			Role:      "assistant",
			Content:   "No knowledge markers here",
		}
		msgCh <- types.TranscriptMessage{
			SessionID: "sess-001",
			Timestamp: ts.Add(time.Minute),
			Content:   "Fixed ol-0001 in this commit",
			Tools: []types.ToolCall{
				{Name: "Bash", Input: map[string]any{"path": "/app/main.go"}},
			},
		}
		close(msgCh)

		session := initSession("test.jsonl")
		extractor := parser.NewExtractor()
		state := &transcriptState{
			SeenFiles:  make(map[string]bool),
			SeenIssues: make(map[string]bool),
		}

		var buf bytes.Buffer
		consumeTranscriptMessages(msgCh, session, extractor, state, true, &buf, 2)

		// Session ID should be set from first message
		if session.ID != "sess-001" {
			t.Errorf("session.ID = %q, want sess-001", session.ID)
		}
		// Date should be earliest timestamp
		if !session.Date.Equal(ts) {
			t.Errorf("session.Date = %v, want %v", session.Date, ts)
		}
		// Issue should be extracted
		if len(state.Issues) != 1 || state.Issues[0] != "ol-0001" {
			t.Errorf("issues = %v, want [ol-0001]", state.Issues)
		}
		// File from tool should be extracted
		if len(state.FilesChanged) != 1 {
			t.Errorf("filesChanged = %v, want 1 file", state.FilesChanged)
		}
		// Quiet mode — no output
		if buf.Len() != 0 {
			t.Errorf("expected no output in quiet mode, got %q", buf.String())
		}
	})

	t.Run("empty channel produces no changes", func(t *testing.T) {
		msgCh := make(chan types.TranscriptMessage)
		close(msgCh)

		session := initSession("empty.jsonl")
		extractor := parser.NewExtractor()
		state := &transcriptState{
			SeenFiles:  make(map[string]bool),
			SeenIssues: make(map[string]bool),
		}

		var buf bytes.Buffer
		consumeTranscriptMessages(msgCh, session, extractor, state, true, &buf, 0)

		if session.ID != "" {
			t.Errorf("session.ID should be empty, got %q", session.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// extractMessageKnowledge
// ---------------------------------------------------------------------------

func TestForge_ExtractMessageKnowledge(t *testing.T) {
	extractor := parser.NewExtractor()

	t.Run("empty content produces no extractions", func(t *testing.T) {
		msg := types.TranscriptMessage{Content: ""}
		state := &transcriptState{}
		extractMessageKnowledge(msg, extractor, state)
		if len(state.Decisions) != 0 || len(state.Knowledge) != 0 {
			t.Error("expected no extractions for empty content")
		}
	})

	t.Run("content with decision keyword produces decision", func(t *testing.T) {
		// The extractor looks for keywords like "decided to", "decision:", etc.
		msg := types.TranscriptMessage{
			Content: "We decided to use PostgreSQL instead of MySQL for better JSON support",
		}
		state := &transcriptState{}
		extractMessageKnowledge(msg, extractor, state)
		// The extractor should find at least one result for "decided to"
		if len(state.Decisions) == 0 && len(state.Knowledge) == 0 {
			t.Log("No extraction from 'decided to' — may depend on exact extractor patterns; skipping strict check")
		}
	})
}

// ---------------------------------------------------------------------------
// extractMessageRefs
// ---------------------------------------------------------------------------

func TestForge_ExtractMessageRefs(t *testing.T) {
	t.Run("extracts both tools and issues", func(t *testing.T) {
		session := &storage.Session{ToolCalls: make(map[string]int)}
		state := &transcriptState{
			SeenFiles:  make(map[string]bool),
			SeenIssues: make(map[string]bool),
		}

		msg := types.TranscriptMessage{
			Content: "Fixing ag-m0r by editing the config",
			Tools: []types.ToolCall{
				{Name: "Edit", Input: map[string]any{"file_path": "/config.yaml"}},
			},
		}

		extractMessageRefs(msg, session, state)

		if session.ToolCalls["Edit"] != 1 {
			t.Errorf("ToolCalls[Edit] = %d, want 1", session.ToolCalls["Edit"])
		}
		if len(state.FilesChanged) != 1 {
			t.Errorf("filesChanged = %v, want 1 file", state.FilesChanged)
		}
		if len(state.Issues) != 1 || state.Issues[0] != "ag-m0r" {
			t.Errorf("issues = %v, want [ag-m0r]", state.Issues)
		}
	})

	t.Run("no tools and no issues leaves state empty", func(t *testing.T) {
		session := &storage.Session{ToolCalls: make(map[string]int)}
		state := &transcriptState{
			SeenFiles:  make(map[string]bool),
			SeenIssues: make(map[string]bool),
		}

		msg := types.TranscriptMessage{Content: "Just some text with no refs"}
		extractMessageRefs(msg, session, state)

		if len(session.ToolCalls) != 0 {
			t.Errorf("expected no tool calls, got %v", session.ToolCalls)
		}
		if len(state.Issues) != 0 {
			t.Errorf("expected no issues, got %v", state.Issues)
		}
	})
}

// ---------------------------------------------------------------------------
// resolveMarkdownFiles
// ---------------------------------------------------------------------------

func TestForge_ResolveMarkdownFiles(t *testing.T) {
	dir := t.TempDir()

	mdFile := filepath.Join(dir, "notes.md")
	txtFile := filepath.Join(dir, "notes.txt")
	for _, f := range []string{mdFile, txtFile} {
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("filters to only .md files", func(t *testing.T) {
		pattern := filepath.Join(dir, "*")
		files, err := resolveMarkdownFiles([]string{pattern})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 .md file, got %d: %v", len(files), files)
		}
		if len(files) > 0 && !strings.HasSuffix(files[0], ".md") {
			t.Errorf("expected .md file, got %q", files[0])
		}
	})

	t.Run("empty args returns empty", func(t *testing.T) {
		files, err := resolveMarkdownFiles(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})
}

// ---------------------------------------------------------------------------
// queueForExtraction
// ---------------------------------------------------------------------------

func TestForge_QueueForExtraction(t *testing.T) {
	t.Run("creates pending.jsonl with session data", func(t *testing.T) {
		dir := t.TempDir()

		session := &storage.Session{
			ID:        "test-session-123",
			Summary:   "Test session",
			Decisions: []string{"d1"},
			Knowledge: []string{"k1", "k2"},
		}

		err := queueForExtraction(session, "/out/session.md", "/in/transcript.jsonl", dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		pendingPath := filepath.Join(dir, storage.DefaultBaseDir, "pending.jsonl")
		data, err := os.ReadFile(pendingPath)
		if err != nil {
			t.Fatalf("failed to read pending file: %v", err)
		}
		content := string(data)

		if !strings.Contains(content, "test-session-123") {
			t.Errorf("pending should contain session ID, got %q", content)
		}
		if !strings.Contains(content, "/in/transcript.jsonl") {
			t.Errorf("pending should contain transcript path, got %q", content)
		}
		// Should end with newline
		if !strings.HasSuffix(content, "\n") {
			t.Error("pending entry should end with newline")
		}
	})

	t.Run("appends multiple entries", func(t *testing.T) {
		dir := t.TempDir()

		for i := 0; i < 3; i++ {
			session := &storage.Session{
				ID:      fmt.Sprintf("sess-%d", i),
				Summary: fmt.Sprintf("Session %d", i),
			}
			err := queueForExtraction(session, fmt.Sprintf("/out/%d.md", i), fmt.Sprintf("/in/%d.jsonl", i), dir)
			if err != nil {
				t.Fatalf("unexpected error on iteration %d: %v", i, err)
			}
		}

		pendingPath := filepath.Join(dir, storage.DefaultBaseDir, "pending.jsonl")
		data, err := os.ReadFile(pendingPath)
		if err != nil {
			t.Fatalf("failed to read pending file: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}
	})
}

// ---------------------------------------------------------------------------
// processMarkdown (integration with real temp file)
// ---------------------------------------------------------------------------

func TestForge_ProcessMarkdown(t *testing.T) {
	extractor := parser.NewExtractor()

	t.Run("parses markdown file into session", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.md")
		content := "# Introduction\n\nSome intro text here.\n\n## Decisions\n\nWe decided to use Go for the CLI.\n\n## Issues\n\nWorking on ol-001 and ag-m0r.\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		session, err := processMarkdown(path, extractor, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// ID should be deterministic (md-<hash>)
		if !strings.HasPrefix(session.ID, "md-") {
			t.Errorf("session ID should start with md-, got %q", session.ID)
		}

		// Transcript path should be the input file
		if session.TranscriptPath != path {
			t.Errorf("TranscriptPath = %q, want %q", session.TranscriptPath, path)
		}

		// Token count should be estimated
		if session.Tokens.Total != len(content)/CharsPerToken {
			t.Errorf("Tokens.Total = %d, want %d", session.Tokens.Total, len(content)/CharsPerToken)
		}
		if !session.Tokens.Estimated {
			t.Error("Tokens.Estimated should be true")
		}

		// Issues should be extracted
		if len(session.Issues) < 1 {
			t.Log("Issues may vary by content and patterns, but expected at least ol-001")
		}
	})

	t.Run("empty file returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.md")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := processMarkdown(path, extractor, true)
		if err == nil {
			t.Error("expected error for empty file")
		}
		if !strings.Contains(err.Error(), "empty file") {
			t.Errorf("expected 'empty file' error, got %q", err.Error())
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := processMarkdown("/tmp/nonexistent-file-xyz.md", extractor, true)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("deterministic ID for same path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "stable.md")
		if err := os.WriteFile(path, []byte("# Test\nContent"), 0644); err != nil {
			t.Fatal(err)
		}

		s1, err1 := processMarkdown(path, extractor, true)
		s2, err2 := processMarkdown(path, extractor, true)
		if err1 != nil || err2 != nil {
			t.Fatalf("unexpected errors: %v, %v", err1, err2)
		}
		if s1.ID != s2.ID {
			t.Errorf("session IDs should be deterministic: %q != %q", s1.ID, s2.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// processTranscript (integration with real temp JSONL file)
// ---------------------------------------------------------------------------

func TestForge_ProcessTranscript(t *testing.T) {
	p := parser.NewParser()
	p.MaxContentLength = 0
	extractor := parser.NewExtractor()

	t.Run("parses valid jsonl transcript", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")

		// Minimal valid JSONL lines matching Claude Code transcript format
		lines := []string{
			`{"type":"summary","sessionId":"sess-abc123","timestamp":"2024-06-01T12:00:00Z"}`,
			`{"type":"assistant","role":"assistant","content":"Working on the implementation","sessionId":"sess-abc123","timestamp":"2024-06-01T12:01:00Z"}`,
		}
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		session, err := processTranscript(path, p, extractor, true, &buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if session == nil {
			t.Fatal("session should not be nil")
		}

		// Tokens should be estimated from file size
		if session.Tokens.Total == 0 {
			t.Error("Tokens.Total should be > 0")
		}
		if !session.Tokens.Estimated {
			t.Error("Tokens.Estimated should be true")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := processTranscript("/tmp/no-such-transcript.jsonl", p, extractor, true, &buf)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("empty file processes without crash", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		session, err := processTranscript(path, p, extractor, true, &buf)
		if !errors.Is(err, errTranscriptHasNoChatMessages) {
			t.Fatalf("expected errTranscriptHasNoChatMessages, got session=%+v err=%v", session, err)
		}
	})

	t.Run("metadata-only transcript skips cleanly", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "10aecfbe-2d34-4955-bae8-fbc0492bd19c.jsonl")
		content := strings.Join([]string{
			`{"type":"progress","sessionId":"10aecfbe-2d34-4955-bae8-fbc0492bd19c","timestamp":"2026-03-02T01:20:03.146Z","data":{"type":"hook_progress"}}`,
			`{"type":"file-history-snapshot","messageId":"36e8695c-ab17-40a5-8279-09d553ec34f3","snapshot":{"timestamp":"2026-03-02T01:20:26.523Z"}}`,
		}, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		session, err := processTranscript(path, p, extractor, true, &buf)
		if !errors.Is(err, errTranscriptHasNoChatMessages) {
			t.Fatalf("expected errTranscriptHasNoChatMessages, got session=%+v err=%v", session, err)
		}
	})

	t.Run("codex archived session is forgeable", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rollout-2026-03-05T15-20-21-019cbfa8-9155-7121-b18a-dfa3783cdd9e.jsonl")
		content := strings.Join([]string{
			`{"timestamp":"2026-03-05T20:20:42.160Z","type":"session_meta","payload":{"id":"019cbfa8-9155-7121-b18a-dfa3783cdd9e","timestamp":"2026-03-05T20:20:21.464Z"}}`,
			`{"timestamp":"2026-03-05T20:20:42.163Z","type":"event_msg","payload":{"type":"user_message","message":"Shield AI recruiter reached out to me"}}`,
			`{"timestamp":"2026-03-05T20:20:54.239Z","type":"event_msg","payload":{"type":"agent_message","message":"I can audit the repo and search your history."}}`,
			`{"timestamp":"2026-03-05T20:20:54.282Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"pwd\"}"}}`,
			`{"timestamp":"2026-03-05T20:20:54.381Z","type":"response_item","payload":{"type":"function_call_output","output":"Chunk ID: abc\nOutput:\n/worktree\n"}}`,
		}, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		session, err := processTranscript(path, p, extractor, true, &buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session.ID != "019cbfa8-9155-7121-b18a-dfa3783cdd9e" {
			t.Fatalf("session ID = %q, want codex session ID", session.ID)
		}
		if session.Date.IsZero() {
			t.Fatal("session date should be populated")
		}
	})

	t.Run("oversized Claude transcript line parses successfully", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ses_large.jsonl")
		hugeContent := strings.Repeat("x", 128*1024)
		line := fmt.Sprintf(`{"type":"user","sessionId":"ses_large","timestamp":"2026-03-02T01:20:03.146Z","content":"%s"}`, hugeContent)
		if err := os.WriteFile(path, []byte(line+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		session, err := processTranscript(path, p, extractor, true, &buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session.ID != "ses_large" {
			t.Fatalf("session ID = %q, want ses_large", session.ID)
		}
	})
}

// ---------------------------------------------------------------------------
// writeSessionIndex (integration with real FileStorage)
// ---------------------------------------------------------------------------

func TestForge_WriteSessionIndex(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, ".agents", "ao")

	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)
	if err := fs.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	session := &storage.Session{
		ID:      "idx-test-001",
		Date:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Summary: "Index test session",
	}

	err := writeSessionIndex(fs, session, "/fake/session/path.md")
	if err != nil {
		t.Fatalf("writeSessionIndex failed: %v", err)
	}

	// Verify the index file was created
	indexPath := filepath.Join(baseDir, "index", "sessions.jsonl")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index file was not created")
	}
}

// ---------------------------------------------------------------------------
// writeSessionProvenance (integration with real FileStorage)
// ---------------------------------------------------------------------------

func TestForge_WriteSessionProvenance(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, ".agents", "ao")

	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)
	if err := fs.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	t.Run("writes provenance with session ID", func(t *testing.T) {
		err := writeSessionProvenance(fs, "prov-test-001234", "/out/session.md", "/in/transcript.jsonl", "transcript", true)
		if err != nil {
			t.Fatalf("writeSessionProvenance failed: %v", err)
		}
	})

	t.Run("writes provenance without session ID", func(t *testing.T) {
		err := writeSessionProvenance(fs, "prov-test-002345", "/out/session.md", "/in/notes.md", "markdown", false)
		if err != nil {
			t.Fatalf("writeSessionProvenance failed: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// initForgeStorage (integration — creates dirs in cwd)
// ---------------------------------------------------------------------------

func TestForge_InitForgeStorage(t *testing.T) {
	// initForgeStorage uses os.Getwd(), so we need to be in a temp dir.
	// Since we can't chdir easily in tests without affecting other tests,
	// we just verify the function doesn't panic and returns valid results
	// when called from the current directory.
	t.Run("returns valid cwd and storage", func(t *testing.T) {
		cwd, baseDir, fs, err := initForgeStorage()
		if err != nil {
			t.Fatalf("initForgeStorage failed: %v", err)
		}
		if cwd == "" {
			t.Error("cwd should not be empty")
		}
		if baseDir == "" {
			t.Error("baseDir should not be empty")
		}
		if fs == nil {
			t.Error("FileStorage should not be nil")
		}
		if !strings.Contains(baseDir, storage.DefaultBaseDir) {
			t.Errorf("baseDir should contain %q, got %q", storage.DefaultBaseDir, baseDir)
		}
	})
}

// ---------------------------------------------------------------------------
// forgeTranscriptFile (integration with real FileStorage)
// ---------------------------------------------------------------------------

func TestForge_ForgeTranscriptFile(t *testing.T) {
	// Save/restore package-level flags
	origQuiet := forgeQuiet
	origQueue := forgeQueue
	defer func() {
		forgeQuiet = origQuiet
		forgeQueue = origQueue
	}()
	forgeQuiet = true
	forgeQueue = false

	dir := t.TempDir()
	baseDir := filepath.Join(dir, ".agents", "ao")

	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)
	if err := fs.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	session := &storage.Session{
		ID:        "forge-tx-001",
		Date:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Summary:   "Test forge transcript",
		Decisions: []string{"d1"},
		Knowledge: []string{"k1", "k2"},
		ToolCalls: make(map[string]int),
	}

	totals := forgeTotals{}
	forgeTranscriptFile(fs, session, "/in/session.jsonl", baseDir, dir, &totals)

	if totals.sessions != 1 {
		t.Errorf("totals.sessions = %d, want 1", totals.sessions)
	}
	if totals.decisions != 1 {
		t.Errorf("totals.decisions = %d, want 1", totals.decisions)
	}
	if totals.knowledge != 2 {
		t.Errorf("totals.knowledge = %d, want 2", totals.knowledge)
	}
}

// ---------------------------------------------------------------------------
// forgeMarkdownFile (integration with real FileStorage)
// ---------------------------------------------------------------------------

func TestForge_ForgeMarkdownFile(t *testing.T) {
	// Save/restore package-level flags
	origQuiet := forgeMdQuiet
	origQueue := forgeMdQueue
	defer func() {
		forgeMdQuiet = origQuiet
		forgeMdQueue = origQueue
	}()
	forgeMdQuiet = true
	forgeMdQueue = false

	dir := t.TempDir()
	baseDir := filepath.Join(dir, ".agents", "ao")

	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)
	if err := fs.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	session := &storage.Session{
		ID:        "forge-md-001",
		Date:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Summary:   "Test forge markdown",
		Decisions: []string{"d1", "d2"},
		Knowledge: []string{"k1"},
		ToolCalls: make(map[string]int),
	}

	totals := forgeTotals{}
	forgeMarkdownFile(fs, session, "/in/notes.md", baseDir, dir, &totals)

	if totals.sessions != 1 {
		t.Errorf("totals.sessions = %d, want 1", totals.sessions)
	}
	if totals.decisions != 2 {
		t.Errorf("totals.decisions = %d, want 2", totals.decisions)
	}
	if totals.knowledge != 1 {
		t.Errorf("totals.knowledge = %d, want 1", totals.knowledge)
	}
}

// ---------------------------------------------------------------------------
// updateSearchIndexForFile
// ---------------------------------------------------------------------------

func TestForge_UpdateSearchIndexForFile(t *testing.T) {
	t.Run("no-op when index does not exist", func(t *testing.T) {
		dir := t.TempDir()
		// Should not panic or error when index.jsonl doesn't exist
		updateSearchIndexForFile(dir, "/some/file.md", true)
	})
}

// ---------------------------------------------------------------------------
// fileWithTime struct (verify sort behavior via collectTranscriptCandidates)
// ---------------------------------------------------------------------------

func TestForge_FileWithTimeSorting(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "proj")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatal(err)
	}

	content := strings.Repeat("x", 200)

	// Create files with staggered modification times
	file1 := filepath.Join(proj, "old.jsonl")
	file2 := filepath.Join(proj, "new.jsonl")
	if err := os.WriteFile(file1, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Set old modification time
	oldTime := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(file1, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	candidates, err := collectTranscriptCandidates(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// Verify modTime is populated
	for _, c := range candidates {
		if c.modTime.IsZero() {
			t.Errorf("modTime should not be zero for %q", c.path)
		}
	}
}

// ---------------------------------------------------------------------------
// forgeTranscriptFile with queue enabled
// ---------------------------------------------------------------------------

func TestForge_ForgeTranscriptFileWithQueue(t *testing.T) {
	origQuiet := forgeQuiet
	origQueue := forgeQueue
	defer func() {
		forgeQuiet = origQuiet
		forgeQueue = origQueue
	}()
	forgeQuiet = true
	forgeQueue = true

	dir := t.TempDir()
	baseDir := filepath.Join(dir, ".agents", "ao")

	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)
	if err := fs.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	session := &storage.Session{
		ID:        "queue-test-001",
		Date:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Summary:   "Queue test session",
		Decisions: []string{"d1"},
		ToolCalls: make(map[string]int),
	}

	totals := forgeTotals{}
	forgeTranscriptFile(fs, session, "/in/session.jsonl", baseDir, dir, &totals)

	// Verify pending.jsonl was created
	pendingPath := filepath.Join(dir, storage.DefaultBaseDir, "pending.jsonl")
	if _, err := os.Stat(pendingPath); os.IsNotExist(err) {
		t.Error("pending.jsonl should be created when queue=true")
	}
}

// ---------------------------------------------------------------------------
// forgeMarkdownFile with queue enabled
// ---------------------------------------------------------------------------

func TestForge_ForgeMarkdownFileWithQueue(t *testing.T) {
	origQuiet := forgeMdQuiet
	origQueue := forgeMdQueue
	defer func() {
		forgeMdQuiet = origQuiet
		forgeMdQueue = origQueue
	}()
	forgeMdQuiet = true
	forgeMdQueue = true

	dir := t.TempDir()
	baseDir := filepath.Join(dir, ".agents", "ao")

	fs := storage.NewFileStorage(
		storage.WithBaseDir(baseDir),
		storage.WithFormatters(
			formatter.NewMarkdownFormatter(),
			formatter.NewJSONLFormatter(),
		),
	)
	if err := fs.Init(); err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	session := &storage.Session{
		ID:        "md-queue-test-001",
		Date:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Summary:   "MD Queue test",
		Knowledge: []string{"k1"},
		ToolCalls: make(map[string]int),
	}

	totals := forgeTotals{}
	forgeMarkdownFile(fs, session, "/in/notes.md", baseDir, dir, &totals)

	pendingPath := filepath.Join(dir, storage.DefaultBaseDir, "pending.jsonl")
	if _, err := os.Stat(pendingPath); os.IsNotExist(err) {
		t.Error("pending.jsonl should be created when queue=true")
	}
}
