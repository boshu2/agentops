package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// collectFilesFromPatterns
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// detectSessionTypeFromContent
// ---------------------------------------------------------------------------

func TestDetectSessionTypeFromContent(t *testing.T) {
	tests := []struct {
		name      string
		summary   string
		knowledge []string
		decisions []string
		want      string
	}{
		{"career from summary", "preparing for career interview", nil, nil, "career"},
		{"career from knowledge", "", []string{"updated resume with new skills"}, nil, "career"},
		{"career salary", "negotiating salary offer", nil, nil, "career"},
		{"debug from summary", "debug stack trace in production", nil, nil, "debug"},
		{"debug broken", "found broken endpoint", nil, nil, "debug"},
		{"debug error log", "analyzed error log for root cause", nil, nil, "debug"},
		{"brainstorm from summary", "brainstorm ideas for new feature", nil, nil, "brainstorm"},
		{"brainstorm what-if", "what if we used a different approach", nil, nil, "brainstorm"},
		{"brainstorm option a vs", "comparing option a vs option b", nil, nil, "brainstorm"},
		{"research from summary", "research available frameworks", nil, nil, "research"},
		{"research explore", "explore alternative architectures", nil, nil, "research"},
		{"implement from summary", "implement new feature with go test", nil, nil, "implement"},
		{"implement git commit", "ran git commit for the changes", nil, nil, "implement"},
		{"implement feat(", "feat(cli): add new command", nil, nil, "implement"},
		{"general fallback", "discussed random topics", nil, nil, "general"},
		{"empty inputs", "", nil, nil, "general"},
		{"knowledge triggers type", "", []string{"found the debug issue"}, nil, "debug"},
		{"decisions trigger type", "", nil, []string{"decided to research alternatives"}, "research"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectSessionTypeFromContent(tt.summary, tt.knowledge, tt.decisions)
			if got != tt.want {
				t.Errorf("detectSessionTypeFromContent(%q, ...) = %q, want %q", tt.summary, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitMarkdownSections
// ---------------------------------------------------------------------------

func TestSplitMarkdownSections_MultipleHeadings(t *testing.T) {
	content := "# Heading 1\nContent 1\n## Heading 2\nContent 2\n# Heading 3\nContent 3"
	sections := splitMarkdownSections(content)
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d: %v", len(sections), sections)
	}
	if sections[0] != "# Heading 1\nContent 1" {
		t.Errorf("section 0 = %q", sections[0])
	}
	if sections[1] != "## Heading 2\nContent 2" {
		t.Errorf("section 1 = %q", sections[1])
	}
	if sections[2] != "# Heading 3\nContent 3" {
		t.Errorf("section 2 = %q", sections[2])
	}
}

func TestSplitMarkdownSections_NoHeadings(t *testing.T) {
	content := "Just some text\nwith no headings"
	sections := splitMarkdownSections(content)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0] != content {
		t.Errorf("section = %q, want %q", sections[0], content)
	}
}

func TestSplitMarkdownSections_EmptyContent(t *testing.T) {
	sections := splitMarkdownSections("")
	if len(sections) != 1 {
		t.Fatalf("expected 1 section for empty content, got %d", len(sections))
	}
}

func TestSplitMarkdownSections_OnlyH1(t *testing.T) {
	content := "# Only heading\nSome body"
	sections := splitMarkdownSections(content)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
}

// ---------------------------------------------------------------------------
// isTranscriptCandidate
// ---------------------------------------------------------------------------

func TestIsTranscriptCandidate(t *testing.T) {
	tmp := t.TempDir()
	// Create a valid candidate
	validPath := filepath.Join(tmp, "project", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(validPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validPath, []byte(strings.Repeat("x", 200)), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(validPath)
	if !isTranscriptCandidate(validPath, info, tmp) {
		t.Error("expected valid .jsonl to be a candidate")
	}

	// Non-jsonl file should not be a candidate
	txtPath := filepath.Join(tmp, "project", "notes.txt")
	if err := os.WriteFile(txtPath, []byte(strings.Repeat("x", 200)), 0644); err != nil {
		t.Fatal(err)
	}
	txtInfo, _ := os.Stat(txtPath)
	if isTranscriptCandidate(txtPath, txtInfo, tmp) {
		t.Error("expected .txt file not to be a candidate")
	}

	// Tiny file should not be a candidate
	tinyPath := filepath.Join(tmp, "project", "tiny.jsonl")
	if err := os.WriteFile(tinyPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	tinyInfo, _ := os.Stat(tinyPath)
	if isTranscriptCandidate(tinyPath, tinyInfo, tmp) {
		t.Error("expected tiny file not to be a candidate")
	}
}

// ---------------------------------------------------------------------------
// collectTranscriptCandidates
// ---------------------------------------------------------------------------

func TestCollectTranscriptCandidates_SkipsSubagents(t *testing.T) {
	tmp := t.TempDir()
	// Normal transcript
	normalDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(normalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(normalDir, "main.jsonl"), []byte(strings.Repeat("x", 200)), 0644); err != nil {
		t.Fatal(err)
	}
	// Subagents transcript (should be skipped)
	subDir := filepath.Join(tmp, "subagents", "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "agent.jsonl"), []byte(strings.Repeat("x", 200)), 0644); err != nil {
		t.Fatal(err)
	}

	candidates, err := collectTranscriptCandidates(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate (subagents excluded), got %d", len(candidates))
	}
}

func TestCollectTranscriptCandidates_NonexistentDir(t *testing.T) {
	candidates, err := collectTranscriptCandidates("/nonexistent/dir/xyz")
	if err == nil && len(candidates) != 0 {
		t.Errorf("expected empty/error for nonexistent dir, got %d candidates", len(candidates))
	}
}

// ---------------------------------------------------------------------------
// collectFilesFromPatterns
// ---------------------------------------------------------------------------

func TestForge_collectFilesFromPatterns_LiteralPath(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "example.jsonl")
	if err := os.WriteFile(f, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := collectFilesFromPatterns([]string{f}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0] != f {
		t.Errorf("expected [%s], got %v", f, files)
	}
}

func TestForge_collectFilesFromPatterns_GlobPattern(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.jsonl", "b.jsonl", "c.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	pattern := filepath.Join(tmp, "*.jsonl")
	files, err := collectFilesFromPatterns([]string{pattern}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestForge_collectFilesFromPatterns_WithFilter(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("# hi"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	pattern := filepath.Join(tmp, "*")
	mdFilter := func(path string) bool {
		return filepath.Ext(path) == ".md"
	}

	files, err := collectFilesFromPatterns([]string{pattern}, mdFilter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 .md files, got %d: %v", len(files), files)
	}
}

func TestForge_collectFilesFromPatterns_InvalidPattern(t *testing.T) {
	_, err := collectFilesFromPatterns([]string{"[invalid"}, nil)
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

func TestForge_collectFilesFromPatterns_NonexistentLiteralPath(t *testing.T) {
	files, err := collectFilesFromPatterns([]string{"/nonexistent/file.jsonl"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for nonexistent literal path, got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// resolveMarkdownFiles
// ---------------------------------------------------------------------------

func TestForge_resolveMarkdownFiles_FiltersMd(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"notes.md", "data.jsonl", "readme.md"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	pattern := filepath.Join(tmp, "*")
	files, err := resolveMarkdownFiles([]string{pattern})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 .md files, got %d: %v", len(files), files)
	}
	for _, f := range files {
		if filepath.Ext(f) != ".md" {
			t.Errorf("non-.md file included: %s", f)
		}
	}
}

// ---------------------------------------------------------------------------
// handleForgeDryRun
// ---------------------------------------------------------------------------

func TestForge_handleForgeDryRun_Active(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	var buf bytes.Buffer
	files := []string{"a.jsonl", "b.jsonl"}
	result := handleForgeDryRun(&buf, false, files, "file(s)")
	if !result {
		t.Error("expected true return when dry-run is active")
	}
	out := buf.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run message, got %q", out)
	}
	if !strings.Contains(out, "2 file(s)") {
		t.Errorf("expected '2 file(s)' in output, got %q", out)
	}
}

func TestForge_handleForgeDryRun_Quiet(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	var buf bytes.Buffer
	result := handleForgeDryRun(&buf, true, []string{"a.jsonl"}, "file(s)")
	if result {
		t.Error("expected false return when quiet is true (even if dry-run)")
	}
}

func TestForge_handleForgeDryRun_NotDryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = false
	defer func() { dryRun = oldDryRun }()

	var buf bytes.Buffer
	result := handleForgeDryRun(&buf, false, []string{"a.jsonl"}, "file(s)")
	if result {
		t.Error("expected false return when dry-run is off")
	}
}

// ---------------------------------------------------------------------------
// noFilesError
// ---------------------------------------------------------------------------

func TestForge_noFilesError_Quiet(t *testing.T) {
	err := noFilesError(true, "some error")
	if err != nil {
		t.Errorf("expected nil error when quiet, got %v", err)
	}
}

func TestForge_noFilesError_NotQuiet(t *testing.T) {
	err := noFilesError(false, "no files found")
	if err == nil {
		t.Error("expected error when not quiet")
	}
	if err.Error() != "no files found" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// forgeTotals
// ---------------------------------------------------------------------------

func TestForge_forgeTotals_addSession(t *testing.T) {
	totals := forgeTotals{}
	session := &storage.Session{
		Decisions: []string{"decision1", "decision2"},
		Knowledge: []string{"knowledge1"},
	}

	totals.addSession(session)
	if totals.sessions != 1 {
		t.Errorf("sessions = %d, want 1", totals.sessions)
	}
	if totals.decisions != 2 {
		t.Errorf("decisions = %d, want 2", totals.decisions)
	}
	if totals.knowledge != 1 {
		t.Errorf("knowledge = %d, want 1", totals.knowledge)
	}

	// Add another session
	totals.addSession(&storage.Session{
		Decisions: []string{"d3"},
		Knowledge: []string{"k2", "k3"},
	})
	if totals.sessions != 2 {
		t.Errorf("sessions = %d, want 2", totals.sessions)
	}
	if totals.decisions != 3 {
		t.Errorf("decisions = %d, want 3", totals.decisions)
	}
	if totals.knowledge != 3 {
		t.Errorf("knowledge = %d, want 3", totals.knowledge)
	}
}

// ---------------------------------------------------------------------------
// printForgeSummary
// ---------------------------------------------------------------------------

func TestForge_printForgeSummary(t *testing.T) {
	var buf bytes.Buffer
	totals := forgeTotals{sessions: 3, decisions: 5, knowledge: 7}
	printForgeSummary(&buf, totals, "/tmp/out", "session(s)")

	out := buf.String()
	if !strings.Contains(out, "3 session(s)") {
		t.Errorf("expected '3 session(s)' in output, got %q", out)
	}
	if !strings.Contains(out, "Decisions: 5") {
		t.Errorf("expected 'Decisions: 5' in output, got %q", out)
	}
	if !strings.Contains(out, "Knowledge: 7") {
		t.Errorf("expected 'Knowledge: 7' in output, got %q", out)
	}
	if !strings.Contains(out, "/tmp/out") {
		t.Errorf("expected output dir in summary, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// splitMarkdownSections
// ---------------------------------------------------------------------------

func TestForge_splitMarkdownSections(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		checkIdx int
		contains string
	}{
		{
			name:     "two h2 sections",
			content:  "## First\nSome content\n## Second\nMore content",
			wantLen:  2,
			checkIdx: 1,
			contains: "## Second",
		},
		{
			name:     "h1 and h2 mix",
			content:  "# Title\nIntro\n## Section\nBody",
			wantLen:  2,
			checkIdx: 0,
			contains: "# Title",
		},
		{
			name:     "no headings",
			content:  "Just plain text\nwith lines",
			wantLen:  1,
			checkIdx: 0,
			contains: "Just plain text",
		},
		{
			name:     "empty content",
			content:  "",
			wantLen:  1,
			checkIdx: 0,
			contains: "",
		},
		{
			name:     "heading at start followed by content then heading",
			content:  "## A\nline1\nline2\n## B\nline3",
			wantLen:  2,
			checkIdx: 0,
			contains: "line1",
		},
		{
			name:     "three sections",
			content:  "# One\na\n# Two\nb\n# Three\nc",
			wantLen:  3,
			checkIdx: 2,
			contains: "# Three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := splitMarkdownSections(tt.content)
			if len(sections) != tt.wantLen {
				t.Errorf("got %d sections, want %d; sections=%v", len(sections), tt.wantLen, sections)
			}
			if tt.checkIdx < len(sections) && !strings.Contains(sections[tt.checkIdx], tt.contains) {
				t.Errorf("section[%d] = %q, want to contain %q", tt.checkIdx, sections[tt.checkIdx], tt.contains)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// initSession
// ---------------------------------------------------------------------------

func TestForge_initSession(t *testing.T) {
	session := initSession("/path/to/transcript.jsonl")
	if session.TranscriptPath != "/path/to/transcript.jsonl" {
		t.Errorf("TranscriptPath = %q, want %q", session.TranscriptPath, "/path/to/transcript.jsonl")
	}
	if session.ToolCalls == nil {
		t.Error("ToolCalls map should be initialized")
	}
	if session.ID != "" {
		t.Errorf("ID should be empty when path has no embedded session identifier, got %q", session.ID)
	}
}

func TestForge_inferSessionIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/tmp/ses_37c81fcb6ffeurJmCkdLL1k2qZ.jsonl", want: "ses_37c81fcb6ffeurJmCkdLL1k2qZ"},
		{path: "/tmp/10aecfbe-2d34-4955-bae8-fbc0492bd19c.jsonl", want: "10aecfbe-2d34-4955-bae8-fbc0492bd19c"},
		{path: "/tmp/rollout-2026-03-05T15-20-21-019cbfa8-9155-7121-b18a-dfa3783cdd9e.jsonl", want: "019cbfa8-9155-7121-b18a-dfa3783cdd9e"},
		{path: "/tmp/no-session-id.jsonl", want: ""},
	}

	for _, tc := range tests {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			if got := inferSessionIDFromPath(tc.path); got != tc.want {
				t.Fatalf("inferSessionIDFromPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// updateSessionMeta
// ---------------------------------------------------------------------------

func TestForge_updateSessionMeta(t *testing.T) {
	t.Run("sets ID from first message", func(t *testing.T) {
		session := &storage.Session{}
		msg := types.TranscriptMessage{SessionID: "sess-123"}
		updateSessionMeta(session, msg)
		if session.ID != "sess-123" {
			t.Errorf("ID = %q, want %q", session.ID, "sess-123")
		}
	})

	t.Run("does not overwrite existing ID", func(t *testing.T) {
		session := &storage.Session{ID: "first-id"}
		msg := types.TranscriptMessage{SessionID: "second-id"}
		updateSessionMeta(session, msg)
		if session.ID != "first-id" {
			t.Errorf("ID = %q, want %q (should not overwrite)", session.ID, "first-id")
		}
	})

	t.Run("picks earliest timestamp", func(t *testing.T) {
		t1 := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
		t2 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
		session := &storage.Session{Date: t1}

		msg := types.TranscriptMessage{Timestamp: t2}
		updateSessionMeta(session, msg)
		if !session.Date.Equal(t2) {
			t.Errorf("Date = %v, want %v (earlier)", session.Date, t2)
		}
	})

	t.Run("ignores zero timestamp", func(t *testing.T) {
		t1 := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
		session := &storage.Session{Date: t1}

		msg := types.TranscriptMessage{}
		updateSessionMeta(session, msg)
		if !session.Date.Equal(t1) {
			t.Errorf("Date should remain unchanged, got %v", session.Date)
		}
	})
}

// ---------------------------------------------------------------------------
// extractFilePathsFromTool
// ---------------------------------------------------------------------------

func TestForge_extractFilePathsFromTool(t *testing.T) {
	t.Run("file_path input", func(t *testing.T) {
		state := &transcriptState{
			SeenFiles: make(map[string]bool),
		}
		tool := types.ToolCall{
			Name:  "Read",
			Input: map[string]any{"file_path": "/src/main.go"},
		}
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 || state.FilesChanged[0] != "/src/main.go" {
			t.Errorf("expected [/src/main.go], got %v", state.FilesChanged)
		}
	})

	t.Run("path input", func(t *testing.T) {
		state := &transcriptState{
			SeenFiles: make(map[string]bool),
		}
		tool := types.ToolCall{
			Name:  "Glob",
			Input: map[string]any{"path": "/src/"},
		}
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 {
			t.Errorf("expected 1 file, got %d", len(state.FilesChanged))
		}
	})

	t.Run("filePath input", func(t *testing.T) {
		state := &transcriptState{
			SeenFiles: make(map[string]bool),
		}
		tool := types.ToolCall{
			Name:  "Edit",
			Input: map[string]any{"filePath": "/src/camel.go"},
		}
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 || state.FilesChanged[0] != "/src/camel.go" {
			t.Errorf("expected [/src/camel.go], got %v", state.FilesChanged)
		}
	})

	t.Run("deduplicates paths", func(t *testing.T) {
		state := &transcriptState{
			SeenFiles: make(map[string]bool),
		}
		tool := types.ToolCall{
			Name:  "Read",
			Input: map[string]any{"file_path": "/src/main.go"},
		}
		extractFilePathsFromTool(tool, state)
		extractFilePathsFromTool(tool, state)
		if len(state.FilesChanged) != 1 {
			t.Errorf("expected 1 file after dedup, got %d", len(state.FilesChanged))
		}
	})

	t.Run("nil input is safe", func(t *testing.T) {
		state := &transcriptState{SeenFiles: make(map[string]bool)}
		tool := types.ToolCall{Name: "test", Input: nil}
		extractFilePathsFromTool(tool, state) // should not panic
	})
}

// ---------------------------------------------------------------------------
// extractIssueRefs
// ---------------------------------------------------------------------------

func TestForge_extractIssueRefs(t *testing.T) {
	state := &transcriptState{SeenIssues: make(map[string]bool)}

	extractIssueRefs("Working on ol-0001 and ag-abc", state)
	if len(state.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d: %v", len(state.Issues), state.Issues)
	}

	// Calling again with same IDs should not duplicate
	extractIssueRefs("ol-0001 again", state)
	if len(state.Issues) != 2 {
		t.Errorf("expected 2 issues after dedup, got %d", len(state.Issues))
	}
}

// ---------------------------------------------------------------------------
// generateSummary
// ---------------------------------------------------------------------------

func TestForge_generateSummary(t *testing.T) {
	t.Run("from decisions", func(t *testing.T) {
		summary := generateSummary([]string{"Use middleware for auth"}, nil, time.Now())
		if summary != "Use middleware for auth" {
			t.Errorf("unexpected summary: %q", summary)
		}
	})

	t.Run("from knowledge when no decisions", func(t *testing.T) {
		summary := generateSummary(nil, []string{"Found a fix for race condition"}, time.Now())
		if summary != "Found a fix for race condition" {
			t.Errorf("unexpected summary: %q", summary)
		}
	})

	t.Run("date fallback when no content", func(t *testing.T) {
		date := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
		summary := generateSummary(nil, nil, date)
		if !strings.Contains(summary, "2026-02-10") {
			t.Errorf("expected date in summary, got %q", summary)
		}
	})

	t.Run("truncates long decisions", func(t *testing.T) {
		long := strings.Repeat("a", 200)
		summary := generateSummary([]string{long}, nil, time.Now())
		if len(summary) > SummaryMaxLength {
			t.Errorf("summary too long: %d > %d", len(summary), SummaryMaxLength)
		}
		if !strings.HasSuffix(summary, "...") {
			t.Error("expected ellipsis on truncated summary")
		}
	})
}

// ---------------------------------------------------------------------------
// countLines
// ---------------------------------------------------------------------------

func TestForge_countLines(t *testing.T) {
	tmp := t.TempDir()

	t.Run("normal file", func(t *testing.T) {
		path := filepath.Join(tmp, "lines.txt")
		if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644); err != nil {
			t.Fatal(err)
		}
		count := countLines(path)
		if count != 3 {
			t.Errorf("countLines = %d, want 3", count)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmp, "empty.txt")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		count := countLines(path)
		if count != 0 {
			t.Errorf("countLines = %d, want 0", count)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		count := countLines("/nonexistent/file.txt")
		if count != 0 {
			t.Errorf("countLines = %d, want 0 for nonexistent file", count)
		}
	})

	t.Run("no trailing newline", func(t *testing.T) {
		path := filepath.Join(tmp, "notail.txt")
		if err := os.WriteFile(path, []byte("line1\nline2"), 0644); err != nil {
			t.Fatal(err)
		}
		count := countLines(path)
		if count != 1 {
			t.Errorf("countLines = %d, want 1 (only one \\n)", count)
		}
	})
}

// ---------------------------------------------------------------------------
// extractIssueIDs
// ---------------------------------------------------------------------------

func TestForge_extractIssueIDs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"single issue", "Working on ol-0001", 1},
		{"multiple issues", "Fixed ag-abc and gs-def", 2},
		{"no issues", "No issues here", 0},
		{"issue in sentence", "The issue ol-1234 was resolved", 1},
		{"hyphenated issue ID", "Task gt-abc-def completed", 1},
		{"empty string", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := extractIssueIDs(tt.content)
			if len(ids) != tt.want {
				t.Errorf("extractIssueIDs(%q) returned %d IDs, want %d; ids=%v",
					tt.content, len(ids), tt.want, ids)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lastSpaceIndex
// ---------------------------------------------------------------------------

func TestForge_lastSpaceIndex(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"has spaces", "hello world foo", 11},
		{"single space", "a b", 1},
		{"no spaces", "nospace", -1},
		{"empty", "", -1},
		{"trailing space", "hello ", 5},
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
// dedup
// ---------------------------------------------------------------------------

func TestForge_dedup(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want int
	}{
		{"no duplicates", []string{"a", "b", "c"}, 3},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, 3},
		{"all same", []string{"x", "x", "x"}, 1},
		{"empty", []string{}, 0},
		{"nil", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedup(tt.in)
			if len(got) != tt.want {
				t.Errorf("dedup(%v) returned %d items, want %d", tt.in, len(got), tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateString
// ---------------------------------------------------------------------------

func TestForge_truncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world!", 8, "hello..."},
		{"empty", "", 10, ""},
		{"maxLen 4", "abcdef", 4, "a..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// queueForExtraction
// ---------------------------------------------------------------------------

func TestForge_queueForExtraction(t *testing.T) {
	tmp := t.TempDir()

	session := &storage.Session{
		ID:        "test-session-1234567890",
		Summary:   "Test summary",
		Decisions: []string{"decision1"},
		Knowledge: []string{"knowledge1"},
	}

	err := queueForExtraction(session, "/path/session.md", "/path/transcript.jsonl", tmp)
	if err != nil {
		t.Fatalf("queueForExtraction failed: %v", err)
	}

	pendingPath := filepath.Join(tmp, storage.DefaultBaseDir, "pending.jsonl")
	content, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatalf("read pending file: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "test-session-1234567890") {
		t.Error("expected session_id in pending output")
	}
	if !strings.Contains(text, "Test summary") {
		t.Error("expected summary in pending output")
	}
	if !strings.Contains(text, "decision1") {
		t.Error("expected decisions in pending output")
	}
}

func TestRunForgeTier1_EnqueuesDreamCuratorJobWhenWorkerConfigured(t *testing.T) {
	tmp := t.TempDir()
	workerDir := filepath.Join(tmp, "dream-worker")
	sourcePath := filepath.Join(tmp, "session.jsonl")
	if err := os.WriteFile(sourcePath, []byte(`{"type":"message","role":"user","content":"summarize me"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(tmp, "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", workerDir)

	origQuiet := forgeQuiet
	origModel := forgeTier1Model
	defer func() {
		forgeQuiet = origQuiet
		forgeTier1Model = origModel
	}()
	forgeQuiet = true
	forgeTier1Model = ""

	if err := runForgeTier1(io.Discard, []string{sourcePath}); err != nil {
		t.Fatalf("runForgeTier1: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(workerDir, "queue"))
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("queue entries = %d, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(workerDir, "queue", entries[0].Name()))
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	var job curatorJob
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatalf("parse job: %v\n%s", err, string(data))
	}
	if job.Kind != "ingest-claude-session" {
		t.Fatalf("job kind = %q, want ingest-claude-session", job.Kind)
	}
	if job.Source == nil || job.Source.Path != sourcePath {
		t.Fatalf("job source = %+v, want path %q", job.Source, sourcePath)
	}
	if job.Source.ChunkStart != 0 || job.Source.ChunkEnd != 1 {
		t.Fatalf("job chunk = %d..%d, want 0..1", job.Source.ChunkStart, job.Source.ChunkEnd)
	}
}

// ---------------------------------------------------------------------------
// finalizeTranscriptSession
// ---------------------------------------------------------------------------

func TestForge_finalizeTranscriptSession(t *testing.T) {
	session := &storage.Session{}
	state := &transcriptState{
		Decisions:    []string{"d1", "d2", "d1"}, // contains duplicate
		Knowledge:    []string{"k1"},
		FilesChanged: []string{"/a.go", "/b.go"},
		Issues:       []string{"ol-001"},
	}

	finalizeTranscriptSession(session, state, 4000)

	if len(session.Decisions) != 2 {
		t.Errorf("expected 2 unique decisions, got %d", len(session.Decisions))
	}
	if len(session.Knowledge) != 1 {
		t.Errorf("expected 1 knowledge, got %d", len(session.Knowledge))
	}
	if len(session.FilesChanged) != 2 {
		t.Errorf("expected 2 files changed, got %d", len(session.FilesChanged))
	}
	if len(session.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(session.Issues))
	}
	if session.Tokens.Total != 4000/CharsPerToken {
		t.Errorf("Tokens.Total = %d, want %d", session.Tokens.Total, 4000/CharsPerToken)
	}
	if !session.Tokens.Estimated {
		t.Error("expected Tokens.Estimated = true")
	}
	if session.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

// ---------------------------------------------------------------------------
// isTranscriptCandidate
// ---------------------------------------------------------------------------

func TestForge_isTranscriptCandidate(t *testing.T) {
	tmp := t.TempDir()
	projectsDir := tmp

	t.Run("valid jsonl file", func(t *testing.T) {
		path := filepath.Join(tmp, "session.jsonl")
		if err := os.WriteFile(path, make([]byte, 200), 0644); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		if !isTranscriptCandidate(path, info, projectsDir) {
			t.Error("expected valid transcript candidate")
		}
	})

	t.Run("too small file", func(t *testing.T) {
		path := filepath.Join(tmp, "tiny.jsonl")
		if err := os.WriteFile(path, make([]byte, 10), 0644); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		if isTranscriptCandidate(path, info, projectsDir) {
			t.Error("expected tiny file to be rejected")
		}
	})

	t.Run("non-jsonl file", func(t *testing.T) {
		path := filepath.Join(tmp, "notes.txt")
		if err := os.WriteFile(path, make([]byte, 200), 0644); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		if isTranscriptCandidate(path, info, projectsDir) {
			t.Error("expected non-jsonl file to be rejected")
		}
	})

	t.Run("directory", func(t *testing.T) {
		subDir := filepath.Join(tmp, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(subDir)
		if isTranscriptCandidate(subDir, info, projectsDir) {
			t.Error("expected directory to be rejected")
		}
	})
}

// ---------------------------------------------------------------------------
// reportProgress
// ---------------------------------------------------------------------------

func TestForge_reportProgress(t *testing.T) {
	t.Run("quiet suppresses output", func(t *testing.T) {
		var buf bytes.Buffer
		last := 0
		reportProgress(true, &buf, 2000, 5000, &last)
		if buf.Len() != 0 {
			t.Error("expected no output in quiet mode")
		}
	})

	t.Run("skips when under threshold", func(t *testing.T) {
		var buf bytes.Buffer
		last := 0
		reportProgress(false, &buf, 500, 5000, &last)
		if buf.Len() != 0 {
			t.Error("expected no output when lineCount - lastProgress < 1000")
		}
	})

	t.Run("reports when over threshold", func(t *testing.T) {
		var buf bytes.Buffer
		last := 0
		reportProgress(false, &buf, 1000, 5000, &last)
		out := buf.String()
		if !strings.Contains(out, "Processing") {
			t.Errorf("expected progress message, got %q", out)
		}
		if last != 1000 {
			t.Errorf("lastProgress not updated: got %d, want 1000", last)
		}
	})
}

// ---------------------------------------------------------------------------
// drainParseErrors
// ---------------------------------------------------------------------------

func TestForge_drainParseErrors(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		ch := make(chan error, 1)
		err := drainParseErrors(ch)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("with error", func(t *testing.T) {
		ch := make(chan error, 1)
		ch <- os.ErrNotExist
		err := drainParseErrors(ch)
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// forgeWarnf
// ---------------------------------------------------------------------------

func TestForge_forgeWarnf_QuietSuppressesOutput(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	forgeWarnf(true, "should not appear: %s\n", "test")

	w.Close()
	data, _ := io.ReadAll(r)
	os.Stderr = oldStderr
	if strings.Contains(string(data), "should not appear") {
		t.Error("quiet=true should suppress stderr output")
	}
}

// ---------------------------------------------------------------------------
// extractToolRefs
// ---------------------------------------------------------------------------

func TestForge_extractToolRefs(t *testing.T) {
	session := &storage.Session{
		ToolCalls: make(map[string]int),
	}
	state := &transcriptState{
		SeenFiles: make(map[string]bool),
	}

	tools := []types.ToolCall{
		{Name: "Read", Input: map[string]any{"file_path": "/a.go"}},
		{Name: "Write", Input: map[string]any{"file_path": "/b.go"}},
		{Name: "Read", Input: map[string]any{"file_path": "/a.go"}}, // duplicate
		{Name: "tool_result"}, // should be skipped
	}

	extractToolRefs(tools, session, state)

	if session.ToolCalls["Read"] != 2 {
		t.Errorf("Read count = %d, want 2", session.ToolCalls["Read"])
	}
	if session.ToolCalls["Write"] != 1 {
		t.Errorf("Write count = %d, want 1", session.ToolCalls["Write"])
	}
	if _, exists := session.ToolCalls["tool_result"]; exists {
		t.Error("tool_result should not be counted")
	}
	if len(state.FilesChanged) != 2 {
		t.Errorf("expected 2 unique files, got %d", len(state.FilesChanged))
	}
}

// ---------------------------------------------------------------------------
// extractSnippet
// ---------------------------------------------------------------------------

func TestForgeCoverage_extractSnippet_NegativeStart(t *testing.T) {
	content := "hello world"
	got := extractSnippet(content, -10, 100)
	if got != content {
		t.Errorf("expected full content for negative start, got %q", got)
	}
}

func TestForgeCoverage_extractSnippet_PastEnd(t *testing.T) {
	got := extractSnippet("hello", 100, 50)
	if got != "" {
		t.Errorf("expected empty for past-end, got %q", got)
	}
}

func TestForgeCoverage_extractSnippet_ExactLength(t *testing.T) {
	content := "abcdef"
	got := extractSnippet(content, 0, 100)
	if got != content {
		t.Errorf("expected full content when maxLen exceeds content, got %q", got)
	}
}

func TestForgeCoverage_extractSnippet_Truncated(t *testing.T) {
	content := "the quick brown fox jumps over the lazy dog and keeps on running"
	got := extractSnippet(content, 0, 30)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis at end of truncated snippet, got %q", got)
	}
	if len(got) > 35 { // 30 + "..." + some tolerance for word boundary
		t.Errorf("snippet too long: %d chars", len(got))
	}
}

func TestForgeCoverage_extractSnippet_FromMiddle(t *testing.T) {
	content := "abcdefghij klmnopqrst uvwxyz"
	got := extractSnippet(content, 11, 10)
	if !strings.HasPrefix(got, "klmnopqrst") {
		t.Errorf("expected snippet starting from index 11, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// lastSpaceIndex
// ---------------------------------------------------------------------------

func TestForgeCoverage_lastSpaceIndex(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty string", "", -1},
		{"no spaces", "abcdef", -1},
		{"trailing space", "abc ", 3},
		{"middle space", "ab cd ef", 5},
		{"leading space", " abc", 0},
		{"multiple spaces", "a b c d", 5},
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

func TestForgeCoverage_extractIssueIDs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "no issues",
			content: "no issue ids here",
			want:    nil,
		},
		{
			name:    "single issue",
			content: "fixed issue ol-0001",
			want:    []string{"ol-0001"},
		},
		{
			name:    "multiple issues",
			content: "working on ag-abc and ol-def",
			want:    []string{"ag-abc", "ol-def"},
		},
		{
			name:    "issue with hyphen suffix",
			content: "see gt-abc-def for details",
			want:    []string{"gt-abc-def"},
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIssueIDs(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("extractIssueIDs(%q) returned %d issues, want %d", tt.content, len(got), len(tt.want))
				return
			}
			for i, id := range got {
				if id != tt.want[i] {
					t.Errorf("extractIssueIDs(%q)[%d] = %q, want %q", tt.content, i, id, tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncateString
// ---------------------------------------------------------------------------

func TestForgeCoverage_truncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// dedup
// ---------------------------------------------------------------------------

func TestForgeCoverage_dedup(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  []string
	}{
		{"empty", []string{}, []string{}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"all same", []string{"x", "x", "x"}, []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedup(tt.items)
			if len(got) != len(tt.want) {
				t.Errorf("dedup() returned %d items, want %d", len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("dedup()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// countLines
// ---------------------------------------------------------------------------

func TestForgeCoverage_countLines(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty file", "", 0},
		{"single line no newline", "hello", 0},
		{"single line with newline", "hello\n", 1},
		{"three lines", "a\nb\nc\n", 3},
		{"multiple lines no trailing newline", "a\nb\nc", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmp, tt.name+".txt")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}
			got := countLines(path)
			if got != tt.want {
				t.Errorf("countLines() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestForgeCoverage_countLines_NonexistentFile(t *testing.T) {
	got := countLines("/nonexistent/file.txt")
	if got != 0 {
		t.Errorf("expected 0 for nonexistent file, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// generateSummary
// ---------------------------------------------------------------------------

func TestForgeCoverage_generateSummary(t *testing.T) {
	date := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		decisions []string
		knowledge []string
		wantHas   string
	}{
		{
			name:      "with decisions",
			decisions: []string{"decided to use Go"},
			knowledge: []string{"learned testing"},
			wantHas:   "decided to use Go",
		},
		{
			name:      "no decisions with knowledge",
			decisions: []string{},
			knowledge: []string{"learned testing patterns"},
			wantHas:   "learned testing",
		},
		{
			name:      "no decisions no knowledge",
			decisions: []string{},
			knowledge: []string{},
			wantHas:   "2026-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateSummary(tt.decisions, tt.knowledge, date)
			if !strings.Contains(got, tt.wantHas) {
				t.Errorf("generateSummary() = %q, expected to contain %q", got, tt.wantHas)
			}
		})
	}
}

func TestForgeCoverage_generateSummary_LongDecision(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := generateSummary([]string{long}, nil, time.Now())
	if len(got) > SummaryMaxLength {
		t.Errorf("summary too long: %d > %d", len(got), SummaryMaxLength)
	}
}

// ---------------------------------------------------------------------------
// initSession
// ---------------------------------------------------------------------------

func TestForgeCoverage_initSession(t *testing.T) {
	session := initSession("/path/to/transcript.jsonl")
	if session.TranscriptPath != "/path/to/transcript.jsonl" {
		t.Errorf("unexpected transcript path: %s", session.TranscriptPath)
	}
	if session.ToolCalls == nil {
		t.Error("expected ToolCalls map to be initialized")
	}
	if session.ID != "" {
		t.Error("expected empty session ID")
	}
}

// ---------------------------------------------------------------------------
// updateSessionMeta
// ---------------------------------------------------------------------------

func TestForgeCoverage_updateSessionMeta(t *testing.T) {
	session := &storage.Session{}
	now := time.Now()

	// First message sets ID and date
	msg1 := types.TranscriptMessage{
		SessionID: "sess-123",
		Timestamp: now,
	}
	updateSessionMeta(session, msg1)
	if session.ID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got %q", session.ID)
	}
	if session.Date != now {
		t.Error("expected date to be set from first message")
	}

	// Second message with earlier timestamp should update date
	earlier := now.Add(-time.Hour)
	msg2 := types.TranscriptMessage{
		SessionID: "sess-456",
		Timestamp: earlier,
	}
	updateSessionMeta(session, msg2)
	if session.ID != "sess-123" {
		t.Error("session ID should not change once set")
	}
	if session.Date != earlier {
		t.Error("date should be updated to earlier timestamp")
	}

	// Message with zero timestamp should not update date
	msg3 := types.TranscriptMessage{
		SessionID: "",
	}
	updateSessionMeta(session, msg3)
	if session.Date != earlier {
		t.Error("date should not change for zero timestamp")
	}
}

func TestForgeCoverage_updateSessionMeta_EmptySessionID(t *testing.T) {
	session := &storage.Session{}
	msg := types.TranscriptMessage{
		SessionID: "",
		Timestamp: time.Now(),
	}
	updateSessionMeta(session, msg)
	if session.ID != "" {
		t.Error("session ID should remain empty when message has no session ID")
	}
}

// ---------------------------------------------------------------------------
// extractMessageKnowledge
// ---------------------------------------------------------------------------

func TestForgeCoverage_extractMessageKnowledge_EmptyContent(t *testing.T) {
	extractor := parser.NewExtractor()
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	msg := types.TranscriptMessage{Content: ""}
	extractMessageKnowledge(msg, extractor, state)
	// Should be a no-op
	if len(state.Decisions) != 0 || len(state.Knowledge) != 0 {
		t.Error("expected no extractions from empty content")
	}
}

func TestForgeCoverage_extractMessageKnowledge_WithContent(t *testing.T) {
	extractor := parser.NewExtractor()
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	msg := types.TranscriptMessage{
		Content: "We decided to use PostgreSQL because it supports JSON indexing. The solution was to add a retry loop around the API calls.",
	}
	extractMessageKnowledge(msg, extractor, state)
	// Verify state maps remain valid (not nil or corrupted) after extraction
	if state.SeenFiles == nil || state.SeenIssues == nil {
		t.Error("extractMessageKnowledge corrupted state maps")
	}
	// With "decided to" pattern, extractor should find at least one decision
	if len(state.Decisions) == 0 && len(state.Knowledge) == 0 {
		t.Log("extractor found no patterns — verify extractor recognizes 'decided to' pattern")
	}
}

// ---------------------------------------------------------------------------
// extractMessageRefs
// ---------------------------------------------------------------------------

func TestForgeCoverage_extractMessageRefs(t *testing.T) {
	session := &storage.Session{ToolCalls: make(map[string]int)}
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	msg := types.TranscriptMessage{
		Content: "working on ag-abc and ol-def",
		Tools: []types.ToolCall{
			{
				Name:  "Write",
				Input: map[string]any{"file_path": "/test/file.go"},
			},
		},
	}
	extractMessageRefs(msg, session, state)
	if len(state.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(state.Issues))
	}
	if len(state.FilesChanged) != 1 {
		t.Errorf("expected 1 file changed, got %d", len(state.FilesChanged))
	}
}

// ---------------------------------------------------------------------------
// extractToolRefs
// ---------------------------------------------------------------------------

func TestForgeCoverage_extractToolRefs(t *testing.T) {
	session := &storage.Session{ToolCalls: make(map[string]int)}
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	tools := []types.ToolCall{
		{Name: "Write", Input: map[string]any{"file_path": "/a.go"}},
		{Name: "Read", Input: map[string]any{"path": "/b.go"}},
		{Name: "Write", Input: map[string]any{"file_path": "/c.go"}},
		{Name: "tool_result", Input: nil}, // should be skipped for counting
		{Name: "", Input: nil},            // empty name skipped
	}
	extractToolRefs(tools, session, state)
	if session.ToolCalls["Write"] != 2 {
		t.Errorf("expected Write=2, got %d", session.ToolCalls["Write"])
	}
	if session.ToolCalls["Read"] != 1 {
		t.Errorf("expected Read=1, got %d", session.ToolCalls["Read"])
	}
	if _, ok := session.ToolCalls["tool_result"]; ok {
		t.Error("tool_result should not be counted")
	}
	if len(state.FilesChanged) != 3 {
		t.Errorf("expected 3 files changed, got %d", len(state.FilesChanged))
	}
}

// ---------------------------------------------------------------------------
// extractFilePathsFromTool
// ---------------------------------------------------------------------------

func TestForgeCoverage_extractFilePathsFromTool(t *testing.T) {
	tests := []struct {
		name      string
		tool      types.ToolCall
		wantFiles int
	}{
		{
			name:      "nil input",
			tool:      types.ToolCall{Input: nil},
			wantFiles: 0,
		},
		{
			name: "file_path key",
			tool: types.ToolCall{
				Input: map[string]any{"file_path": "/test.go"},
			},
			wantFiles: 1,
		},
		{
			name: "path key",
			tool: types.ToolCall{
				Input: map[string]any{"path": "/test.go"},
			},
			wantFiles: 1,
		},
		{
			name: "filePath key",
			tool: types.ToolCall{
				Input: map[string]any{"filePath": "/camel.go"},
			},
			wantFiles: 1,
		},
		{
			name: "both keys same file",
			tool: types.ToolCall{
				Input: map[string]any{"file_path": "/same.go", "path": "/same.go"},
			},
			wantFiles: 1, // deduped by SeenFiles
		},
		{
			name: "both keys different files",
			tool: types.ToolCall{
				Input: map[string]any{"file_path": "/a.go", "path": "/b.go"},
			},
			wantFiles: 2,
		},
		{
			name: "non-string file_path",
			tool: types.ToolCall{
				Input: map[string]any{"file_path": 123},
			},
			wantFiles: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &transcriptState{
				SeenFiles:  make(map[string]bool),
				SeenIssues: make(map[string]bool),
			}
			extractFilePathsFromTool(tt.tool, state)
			if len(state.FilesChanged) != tt.wantFiles {
				t.Errorf("expected %d files, got %d", tt.wantFiles, len(state.FilesChanged))
			}
		})
	}
}

func TestForgeCoverage_extractFilePathsFromTool_Dedup(t *testing.T) {
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	tool := types.ToolCall{
		Input: map[string]any{"file_path": "/same.go"},
	}
	extractFilePathsFromTool(tool, state)
	extractFilePathsFromTool(tool, state)
	if len(state.FilesChanged) != 1 {
		t.Errorf("expected 1 file (deduped), got %d", len(state.FilesChanged))
	}
}

// ---------------------------------------------------------------------------
// extractIssueRefs
// ---------------------------------------------------------------------------

func TestForgeCoverage_extractIssueRefs(t *testing.T) {
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	extractIssueRefs("working on ag-abc and ol-def", state)
	if len(state.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(state.Issues))
	}
	// Calling again should dedup
	extractIssueRefs("also see ag-abc", state)
	if len(state.Issues) != 2 {
		t.Errorf("expected 2 issues after dedup, got %d", len(state.Issues))
	}
}

// ---------------------------------------------------------------------------
// splitMarkdownSections
// ---------------------------------------------------------------------------

func TestForgeCoverage_splitMarkdownSections(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLen  int
		wantDesc string
	}{
		{
			name:    "no headings",
			content: "just some text\nno headings here",
			wantLen: 1,
		},
		{
			name:    "single h1",
			content: "# Title\ncontent here",
			wantLen: 1,
		},
		{
			name:    "two h2 sections",
			content: "## Section 1\ncontent 1\n## Section 2\ncontent 2",
			wantLen: 2,
		},
		{
			name:    "preamble plus h2",
			content: "preamble text\n## Section 1\ncontent 1\n## Section 2\ncontent 2",
			wantLen: 3,
		},
		{
			name:    "h1 and h2 mixed",
			content: "# Title\nintro\n## Section 1\ncontent\n# Another\nmore",
			wantLen: 3,
		},
		{
			name:    "empty content",
			content: "",
			wantLen: 1, // empty content returns [""]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitMarkdownSections(tt.content)
			if len(got) != tt.wantLen {
				t.Errorf("splitMarkdownSections() returned %d sections, want %d. Sections: %v", len(got), tt.wantLen, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// noFilesError
// ---------------------------------------------------------------------------

func TestForgeCoverage_noFilesError(t *testing.T) {
	// Quiet mode returns nil
	err := noFilesError(true, "no files found")
	if err != nil {
		t.Errorf("expected nil error in quiet mode, got %v", err)
	}

	// Non-quiet returns error
	err = noFilesError(false, "no files found")
	if err == nil {
		t.Error("expected error in non-quiet mode")
	}
	if err.Error() != "no files found" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// forgeWarnf
// ---------------------------------------------------------------------------

func TestForgeCoverage_forgeWarnf_Quiet(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	forgeWarnf(true, "test warning: %s\n", "detail")

	w.Close()
	data, _ := io.ReadAll(r)
	os.Stderr = oldStderr
	if strings.Contains(string(data), "test warning") {
		t.Error("quiet mode should suppress output")
	}
}

func TestForgeCoverage_forgeWarnf_NotQuiet(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	forgeWarnf(false, "test warning: %s\n", "detail")

	w.Close()
	data, _ := io.ReadAll(r)
	os.Stderr = oldStderr
	if !strings.Contains(string(data), "test warning") {
		t.Errorf("expected warning on stderr, got: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// forgeTotals.addSession
// ---------------------------------------------------------------------------

func TestForgeCoverage_forgeTotals_addSession(t *testing.T) {
	totals := forgeTotals{}
	session := &storage.Session{
		Decisions: []string{"d1", "d2"},
		Knowledge: []string{"k1"},
	}
	totals.addSession(session)
	if totals.sessions != 1 {
		t.Errorf("expected 1 session, got %d", totals.sessions)
	}
	if totals.decisions != 2 {
		t.Errorf("expected 2 decisions, got %d", totals.decisions)
	}
	if totals.knowledge != 1 {
		t.Errorf("expected 1 knowledge, got %d", totals.knowledge)
	}

	// Add another session
	session2 := &storage.Session{
		Decisions: []string{"d3"},
		Knowledge: []string{"k2", "k3"},
	}
	totals.addSession(session2)
	if totals.sessions != 2 {
		t.Errorf("expected 2 sessions, got %d", totals.sessions)
	}
	if totals.decisions != 3 {
		t.Errorf("expected 3 decisions, got %d", totals.decisions)
	}
	if totals.knowledge != 3 {
		t.Errorf("expected 3 knowledge, got %d", totals.knowledge)
	}
}

// ---------------------------------------------------------------------------
// handleForgeDryRun
// ---------------------------------------------------------------------------

func TestForgeCoverage_handleForgeDryRun(t *testing.T) {
	oldDryRun := dryRun
	defer func() { dryRun = oldDryRun }()

	t.Run("not dry run", func(t *testing.T) {
		dryRun = false
		var buf bytes.Buffer
		result := handleForgeDryRun(&buf, false, []string{"a.jsonl", "b.jsonl"}, "file(s)")
		if result {
			t.Error("expected false when not in dry-run mode")
		}
	})

	t.Run("dry run active", func(t *testing.T) {
		dryRun = true
		var buf bytes.Buffer
		result := handleForgeDryRun(&buf, false, []string{"a.jsonl", "b.jsonl"}, "file(s)")
		if !result {
			t.Error("expected true when in dry-run mode")
		}
		output := buf.String()
		if !strings.Contains(output, "2 file(s)") {
			t.Errorf("expected file count in output, got %q", output)
		}
		if !strings.Contains(output, "a.jsonl") {
			t.Errorf("expected file name in output, got %q", output)
		}
	})

	t.Run("quiet suppresses output", func(t *testing.T) {
		dryRun = true
		var buf bytes.Buffer
		result := handleForgeDryRun(&buf, true, []string{"a.jsonl"}, "file(s)")
		if result {
			t.Error("expected false when quiet is true")
		}
	})
}

// ---------------------------------------------------------------------------
// printForgeSummary
// ---------------------------------------------------------------------------

func TestForgeCoverage_printForgeSummary(t *testing.T) {
	var buf bytes.Buffer
	totals := forgeTotals{sessions: 3, decisions: 5, knowledge: 10}
	printForgeSummary(&buf, totals, "/path/to/output", "session(s)")
	output := buf.String()
	if !strings.Contains(output, "3 session(s)") {
		t.Errorf("expected session count in output, got %q", output)
	}
	if !strings.Contains(output, "Decisions: 5") {
		t.Errorf("expected decisions count, got %q", output)
	}
	if !strings.Contains(output, "Knowledge: 10") {
		t.Errorf("expected knowledge count, got %q", output)
	}
	if !strings.Contains(output, "/path/to/output") {
		t.Errorf("expected base dir in output, got %q", output)
	}
}

// ---------------------------------------------------------------------------
// reportProgress
// ---------------------------------------------------------------------------

func TestForgeCoverage_reportProgress(t *testing.T) {
	t.Run("quiet mode", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(true, &buf, 2000, 5000, &lastProgress)
		if buf.Len() > 0 {
			t.Error("expected no output in quiet mode")
		}
	})

	t.Run("not enough lines since last", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 500
		reportProgress(false, &buf, 600, 5000, &lastProgress)
		if buf.Len() > 0 {
			t.Error("expected no output when less than 1000 lines since last progress")
		}
	})

	t.Run("enough lines to report", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(false, &buf, 1000, 5000, &lastProgress)
		if buf.Len() == 0 {
			t.Error("expected progress output")
		}
		if lastProgress != 1000 {
			t.Errorf("expected lastProgress=1000, got %d", lastProgress)
		}
	})

	t.Run("zero total lines", func(t *testing.T) {
		var buf bytes.Buffer
		lastProgress := 0
		reportProgress(false, &buf, 1000, 0, &lastProgress)
		output := buf.String()
		if !strings.Contains(output, "0%") {
			t.Errorf("expected 0%% in output for zero total, got %q", output)
		}
	})
}

// ---------------------------------------------------------------------------
// drainParseErrors
// ---------------------------------------------------------------------------

func TestForgeCoverage_drainParseErrors(t *testing.T) {
	t.Run("empty channel", func(t *testing.T) {
		ch := make(chan error, 1)
		err := drainParseErrors(ch)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("channel with error", func(t *testing.T) {
		ch := make(chan error, 1)
		ch <- os.ErrNotExist
		err := drainParseErrors(ch)
		if err != os.ErrNotExist {
			t.Errorf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("channel with nil", func(t *testing.T) {
		ch := make(chan error, 1)
		ch <- nil
		err := drainParseErrors(ch)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// finalizeTranscriptSession
// ---------------------------------------------------------------------------

func TestForgeCoverage_finalizeTranscriptSession(t *testing.T) {
	session := &storage.Session{}
	state := &transcriptState{
		Decisions:    []string{"d1", "d1", "d2"}, // has duplicates
		Knowledge:    []string{"k1", "k2", "k1"}, // has duplicates
		FilesChanged: []string{"/a.go", "/b.go"},
		Issues:       []string{"ag-abc"},
	}

	finalizeTranscriptSession(session, state, 4000)

	if len(session.Decisions) != 2 {
		t.Errorf("expected 2 decisions after dedup, got %d", len(session.Decisions))
	}
	if len(session.Knowledge) != 2 {
		t.Errorf("expected 2 knowledge after dedup, got %d", len(session.Knowledge))
	}
	if len(session.FilesChanged) != 2 {
		t.Errorf("expected 2 files, got %d", len(session.FilesChanged))
	}
	if len(session.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(session.Issues))
	}
	if session.Tokens.Total != 1000 { // 4000 / CharsPerToken(4)
		t.Errorf("expected 1000 tokens, got %d", session.Tokens.Total)
	}
	if !session.Tokens.Estimated {
		t.Error("expected estimated=true")
	}
	if session.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

// ---------------------------------------------------------------------------
// transcriptState
// ---------------------------------------------------------------------------

func TestForgeCoverage_transcriptState_Init(t *testing.T) {
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}
	if len(state.Decisions) != 0 {
		t.Error("expected empty decisions")
	}
	if len(state.Knowledge) != 0 {
		t.Error("expected empty knowledge")
	}
	if len(state.FilesChanged) != 0 {
		t.Error("expected empty filesChanged")
	}
	if len(state.Issues) != 0 {
		t.Error("expected empty issues")
	}
}

// ---------------------------------------------------------------------------
// collectFilesFromPatterns
// ---------------------------------------------------------------------------

func TestForgeCoverage_collectFilesFromPatterns(t *testing.T) {
	tmp := t.TempDir()

	// Create test files
	for _, name := range []string{"a.jsonl", "b.jsonl", "c.md", "d.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("glob pattern", func(t *testing.T) {
		files, err := collectFilesFromPatterns([]string{filepath.Join(tmp, "*.jsonl")}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})

	t.Run("literal path", func(t *testing.T) {
		files, err := collectFilesFromPatterns([]string{filepath.Join(tmp, "c.md")}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file, got %d", len(files))
		}
	})

	t.Run("with filter", func(t *testing.T) {
		pattern := filepath.Join(tmp, "*")
		files, err := collectFilesFromPatterns([]string{pattern}, func(path string) bool {
			return filepath.Ext(path) == ".md"
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 file (filtered to .md), got %d", len(files))
		}
	})

	t.Run("nonexistent literal path", func(t *testing.T) {
		files, err := collectFilesFromPatterns([]string{filepath.Join(tmp, "nonexistent.jsonl")}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 files for nonexistent path, got %d", len(files))
		}
	})

	t.Run("invalid glob pattern", func(t *testing.T) {
		_, err := collectFilesFromPatterns([]string{"[invalid"}, nil)
		if err == nil {
			t.Error("expected error for invalid glob pattern")
		}
	})
}

// ---------------------------------------------------------------------------
// resolveMarkdownFiles
// ---------------------------------------------------------------------------

func TestForgeCoverage_resolveMarkdownFiles(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := resolveMarkdownFiles([]string{filepath.Join(tmp, "*")})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 markdown files, got %d", len(files))
	}
}

// ---------------------------------------------------------------------------
// resolveTranscriptFiles
// ---------------------------------------------------------------------------

func TestForgeCoverage_resolveTranscriptFiles_WithArgs(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	oldLastSession := forgeLastSession
	defer func() { forgeLastSession = oldLastSession }()
	forgeLastSession = false

	files, err := resolveTranscriptFiles([]string{filepath.Join(tmp, "*.jsonl")}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestForgeCoverage_resolveTranscriptFiles_LastSessionQuiet(t *testing.T) {
	oldLastSession := forgeLastSession
	defer func() { forgeLastSession = oldLastSession }()
	forgeLastSession = true

	t.Setenv("HOME", t.TempDir())

	// Should return nil, nil in quiet mode when no sessions found
	files, err := resolveTranscriptFiles(nil, true)
	if err != nil {
		t.Fatalf("expected nil error in quiet mode, got %v", err)
	}
	if files != nil {
		t.Error("expected nil files in quiet mode when no sessions")
	}
}

// ---------------------------------------------------------------------------
// isTranscriptCandidate
// ---------------------------------------------------------------------------

func TestForgeCoverage_isTranscriptCandidate(t *testing.T) {
	tmp := t.TempDir()
	projectsDir := tmp

	// Create a valid candidate
	path := filepath.Join(tmp, "session.jsonl")
	content := strings.Repeat("x", 200)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if !isTranscriptCandidate(path, info, projectsDir) {
		t.Error("expected valid .jsonl file to be a candidate")
	}

	// Directory is not a candidate
	dirPath := filepath.Join(tmp, "subdir")
	os.MkdirAll(dirPath, 0755)
	dirInfo, _ := os.Stat(dirPath)
	if isTranscriptCandidate(dirPath, dirInfo, projectsDir) {
		t.Error("expected directory to not be a candidate")
	}

	// Non-jsonl file
	txtPath := filepath.Join(tmp, "file.txt")
	os.WriteFile(txtPath, []byte(content), 0644)
	txtInfo, _ := os.Stat(txtPath)
	if isTranscriptCandidate(txtPath, txtInfo, projectsDir) {
		t.Error("expected .txt file to not be a candidate")
	}

	// Too small file
	smallPath := filepath.Join(tmp, "small.jsonl")
	os.WriteFile(smallPath, []byte("x"), 0644)
	smallInfo, _ := os.Stat(smallPath)
	if isTranscriptCandidate(smallPath, smallInfo, projectsDir) {
		t.Error("expected small file to not be a candidate")
	}
}

// ---------------------------------------------------------------------------
// collectTranscriptCandidates
// ---------------------------------------------------------------------------

func TestForgeCoverage_collectTranscriptCandidates(t *testing.T) {
	tmp := t.TempDir()
	content := strings.Repeat("x", 200)

	// Create valid candidates
	if err := os.WriteFile(filepath.Join(tmp, "session1.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "session2.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Non-candidate
	if err := os.WriteFile(filepath.Join(tmp, "readme.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subagents directory (should be skipped)
	subagentsDir := filepath.Join(tmp, "subagents")
	os.MkdirAll(subagentsDir, 0755)
	os.WriteFile(filepath.Join(subagentsDir, "sub.jsonl"), []byte(content), 0644)

	candidates, err := collectTranscriptCandidates(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates (subagents skipped), got %d", len(candidates))
	}
}

func TestForgeCoverage_findLastSession_FindsMostRecent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectsDir := filepath.Join(home, ".claude", "projects")
	os.MkdirAll(projectsDir, 0755)

	content := strings.Repeat("x", 200)

	// Create two transcript files with different modification times
	oldPath := filepath.Join(projectsDir, "old.jsonl")
	newPath := filepath.Join(projectsDir, "new.jsonl")

	os.WriteFile(oldPath, []byte(content), 0644)
	oldTime := time.Now().Add(-time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)

	os.WriteFile(newPath, []byte(content), 0644)
	// newPath will have the current time

	result, err := findLastSession()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != newPath {
		t.Errorf("expected most recent file %s, got %s", newPath, result)
	}
}

// ---------------------------------------------------------------------------
// fileWithTime type
// ---------------------------------------------------------------------------

func TestForgeCoverage_fileWithTime(t *testing.T) {
	f := fileWithTime{
		path:    "/test/path.jsonl",
		modTime: time.Now(),
	}
	if f.path != "/test/path.jsonl" {
		t.Error("unexpected path")
	}
}

// ---------------------------------------------------------------------------
// queueForExtraction
// ---------------------------------------------------------------------------

func TestForgeCoverage_queueForExtraction(t *testing.T) {
	tmp := t.TempDir()
	session := &storage.Session{
		ID:        "test-session-123",
		Summary:   "test summary",
		Decisions: []string{"d1"},
		Knowledge: []string{"k1"},
	}

	err := queueForExtraction(session, "/path/to/session.md", "/path/to/transcript.jsonl", tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pending.jsonl was created
	pendingPath := filepath.Join(tmp, storage.DefaultBaseDir, "pending.jsonl")
	data, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatalf("pending file not created: %v", err)
	}

	// Verify content is valid JSONL
	var parsed map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil { // trim trailing newline
		t.Fatalf("invalid JSONL: %v", err)
	}
	if parsed["session_id"] != "test-session-123" {
		t.Errorf("unexpected session_id: %v", parsed["session_id"])
	}
}

func TestForgeCoverage_queueForExtraction_Append(t *testing.T) {
	tmp := t.TempDir()
	session1 := &storage.Session{ID: "sess-001", Summary: "first"}
	session2 := &storage.Session{ID: "sess-002", Summary: "second"}

	_ = queueForExtraction(session1, "path1", "trans1", tmp)
	_ = queueForExtraction(session2, "path2", "trans2", tmp)

	pendingPath := filepath.Join(tmp, storage.DefaultBaseDir, "pending.jsonl")
	data, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines in pending file, got %d", len(lines))
	}
}

// ---------------------------------------------------------------------------
// writeSessionIndex
// ---------------------------------------------------------------------------

func TestForgeCoverage_writeSessionIndex(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".agents", "ao")
	fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
	if err := fs.Init(); err != nil {
		t.Fatal(err)
	}

	session := &storage.Session{
		ID:      "test-123",
		Date:    time.Now(),
		Summary: "test session",
	}
	err := writeSessionIndex(fs, session, "sessions/test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeSessionProvenance
// ---------------------------------------------------------------------------

func TestForgeCoverage_writeSessionProvenance(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".agents", "ao")
	fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
	if err := fs.Init(); err != nil {
		t.Fatal(err)
	}

	err := writeSessionProvenance(fs, "test-session-id", "sessions/test.md", "/path/to/transcript.jsonl", "transcript", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForgeCoverage_writeSessionProvenance_NoSessionID(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".agents", "ao")
	fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
	if err := fs.Init(); err != nil {
		t.Fatal(err)
	}

	err := writeSessionProvenance(fs, "abcdefg", "sessions/test.md", "/path/to/file.md", "markdown", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// updateSearchIndexForFile
// ---------------------------------------------------------------------------

func TestForgeCoverage_updateSearchIndexForFile_NoIndex(t *testing.T) {
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.md")
	// No index file — should be a no-op, not create index
	updateSearchIndexForFile(tmp, testFile, false)
	indexPath := filepath.Join(tmp, ".agents", "ao", "index")
	entries, _ := os.ReadDir(indexPath)
	if len(entries) > 0 {
		t.Errorf("no-index mode should not create index entries, found %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// consumeTranscriptMessages
// ---------------------------------------------------------------------------

func TestForgeCoverage_consumeTranscriptMessages(t *testing.T) {
	session := initSession("/test.jsonl")
	extractor := parser.NewExtractor()
	state := &transcriptState{
		SeenFiles:  make(map[string]bool),
		SeenIssues: make(map[string]bool),
	}

	msgCh := make(chan types.TranscriptMessage, 3)
	msgCh <- types.TranscriptMessage{
		SessionID: "s1",
		Content:   "hello world",
		Timestamp: time.Now(),
	}
	msgCh <- types.TranscriptMessage{
		Content: "working on ag-abc",
		Tools: []types.ToolCall{
			{Name: "Write", Input: map[string]any{"file_path": "/test.go"}},
		},
	}
	msgCh <- types.TranscriptMessage{
		Content: "another message",
	}
	close(msgCh)

	var buf bytes.Buffer
	consumeTranscriptMessages(msgCh, session, extractor, state, true, &buf, 100)

	if session.ID != "s1" {
		t.Errorf("expected session ID 's1', got %q", session.ID)
	}
	if len(state.FilesChanged) != 1 {
		t.Errorf("expected 1 file changed, got %d", len(state.FilesChanged))
	}
}

func TestForgeCoverage_processMarkdown_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "valid.md")
	content := "# Title\n\nSome content here.\n\n## Section 2\n\nMore content about ag-xyz."
	os.WriteFile(path, []byte(content), 0644)

	extractor := parser.NewExtractor()
	session, err := processMarkdown(path, extractor, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if !strings.HasPrefix(session.ID, "md-") {
		t.Errorf("expected ID to start with 'md-', got %q", session.ID)
	}
	if session.TranscriptPath != path {
		t.Errorf("unexpected transcript path: %s", session.TranscriptPath)
	}
}

// ---------------------------------------------------------------------------
// processMarkdown deterministic ID
// ---------------------------------------------------------------------------

func TestForgeCoverage_processMarkdown_DeterministicID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")
	os.WriteFile(path, []byte("# Test\ncontent"), 0644)

	extractor := parser.NewExtractor()
	s1, _ := processMarkdown(path, extractor, true)
	s2, _ := processMarkdown(path, extractor, true)

	if s1.ID != s2.ID {
		t.Errorf("expected deterministic IDs to match, got %q and %q", s1.ID, s2.ID)
	}
}

// ---------------------------------------------------------------------------
// issueIDPattern
// ---------------------------------------------------------------------------

func TestForgeCoverage_issueIDPattern(t *testing.T) {
	tests := []struct {
		input   string
		matches []string
	}{
		{"ag-abc", []string{"ag-abc"}},
		{"ol-0001", []string{"ol-0001"}},
		{"gt-abc-def", []string{"gt-abc-def"}},
		{"no match here", nil},
		{"a-bc", nil},        // prefix too short
		{"abcd-ab", nil},     // prefix too long (4 chars)
		{"ag-ab", nil},       // suffix too short (2 chars)
		{"ag-abcdefgh", nil}, // suffix too long (8 chars)
	}

	for _, tt := range tests {
		matches := issueIDPattern.FindAllString(tt.input, -1)
		if len(matches) != len(tt.matches) {
			t.Errorf("issueIDPattern on %q: got %v, want %v", tt.input, matches, tt.matches)
		}
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestForgeCoverage_Constants(t *testing.T) {
	if SnippetMaxLength != 200 {
		t.Errorf("SnippetMaxLength = %d, want 200", SnippetMaxLength)
	}
	if SummaryMaxLength != 100 {
		t.Errorf("SummaryMaxLength = %d, want 100", SummaryMaxLength)
	}
	if CharsPerToken != 4 {
		t.Errorf("CharsPerToken = %d, want 4", CharsPerToken)
	}
}
