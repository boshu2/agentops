package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

// ===========================================================================
// search.go — classifyResultType
// ===========================================================================

func TestSearchClassifyResultType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"learnings dir", "/project/.agents/ao/learnings/mutex.md", "learning"},
		{"patterns dir", "/project/.agents/ao/patterns/retry.md", "pattern"},
		{"retros dir", "/project/.agents/ao/retro/2026-02-01.md", "retro"},
		{"research dir", "/project/.agents/ao/research/topic.md", "research"},
		{"sessions dir", "/project/.agents/ao/sessions/sess-abc.jsonl", "session"},
		{"decisions dir", "/project/.agents/ao/decisions/adr-001.md", "decision"},
		{"unknown dir", "/project/.agents/ao/misc/other.md", "knowledge"},
		{"empty path", "", "knowledge"},
		{"mixed case learnings", "/FOO/LEARNINGS/bar.md", "learning"},
		{"nested learnings", "/a/b/learnings/c/d.md", "learning"},
		{"root-level file", "file.md", "knowledge"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyResultType(tc.path)
			if got != tc.want {
				t.Errorf("classifyResultType(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// search.go — filterByType
// ===========================================================================

func TestSearchFilterByType(t *testing.T) {
	results := []searchResult{
		{Path: "/a.md", Type: "session"},
		{Path: "/b.md", Type: "learning"},
		{Path: "/c.md", Type: "session"},
		{Path: "/d.md", Type: "pattern"},
		{Path: "/e.md", Type: "learning"},
	}

	tests := []struct {
		name       string
		filterType string
		wantLen    int
	}{
		{"filter session", "session", 2},
		{"filter learning", "learning", 2},
		{"filter pattern", "pattern", 1},
		{"filter retro (none)", "retro", 0},
		{"empty filter returns all", "", 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filterByType(results, tc.filterType)
			if len(got) != tc.wantLen {
				t.Errorf("filterByType(%q) returned %d results, want %d", tc.filterType, len(got), tc.wantLen)
			}
		})
	}

	t.Run("nil results", func(t *testing.T) {
		got := filterByType(nil, "session")
		if len(got) != 0 {
			t.Errorf("filterByType(nil) returned %d, want 0", len(got))
		}
	})
}

// ===========================================================================
// search.go — parseGrepResults
// ===========================================================================

func TestSearchParseGrepResults(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		pattern    string
		query      string
		useRipgrep bool
		wantLen    int
	}{
		{
			name:       "ripgrep output lines",
			output:     "/dir/file1.md\n/dir/file2.md\n",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: true,
			wantLen:    2,
		},
		{
			name:       "empty output",
			output:     "",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: true,
			wantLen:    0,
		},
		{
			name:       "grep output filtered by pattern",
			output:     "/dir/file1.md\n/dir/file2.txt\n/dir/file3.md\n",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: false,
			wantLen:    2,
		},
		{
			name:       "grep output no pattern filter",
			output:     "/dir/file1.md\n/dir/file2.txt\n",
			pattern:    "",
			query:      "test",
			useRipgrep: false,
			wantLen:    2,
		},
		{
			name:       "trailing newlines handled",
			output:     "/dir/file.md\n\n\n",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: true,
			wantLen:    1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseGrepResults([]byte(tc.output), tc.pattern, tc.query, tc.useRipgrep)
			if len(got) != tc.wantLen {
				t.Errorf("parseGrepResults() returned %d results, want %d", len(got), tc.wantLen)
			}
			for _, r := range got {
				if r.Type != "session" {
					t.Errorf("expected type 'session', got %q", r.Type)
				}
			}
		})
	}
}

// ===========================================================================
// search.go — parseJSONLMatch
// ===========================================================================

func TestSearchParseJSONLMatch(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		file    string
		wantOK  bool
		wantCtx string
	}{
		{
			name:    "valid JSON with summary",
			line:    `{"summary":"Fixed auth bug","type":"session"}`,
			file:    "/data/sessions.jsonl",
			wantOK:  true,
			wantCtx: "Fixed auth bug",
		},
		{
			name:   "valid JSON without summary",
			line:   `{"content":"some content"}`,
			file:   "/data/sessions.jsonl",
			wantOK: true,
		},
		{
			name:   "invalid JSON",
			line:   `{broken`,
			file:   "/data/sessions.jsonl",
			wantOK: false,
		},
		{
			name:   "empty object",
			line:   `{}`,
			file:   "/data/sessions.jsonl",
			wantOK: true,
		},
		{
			name:   "long summary truncated",
			line:   `{"summary":"` + longString(200) + `"}`,
			file:   "/data/sessions.jsonl",
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := parseJSONLMatch(tc.line, tc.file)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if result.Path != tc.file {
				t.Errorf("Path = %q, want %q", result.Path, tc.file)
			}
			if result.Type != "session" {
				t.Errorf("Type = %q, want 'session'", result.Type)
			}
			if tc.wantCtx != "" && result.Context != tc.wantCtx {
				t.Errorf("Context = %q, want %q", result.Context, tc.wantCtx)
			}
		})
	}
}

// ===========================================================================
// search.go — calculateCASSScore
// ===========================================================================

func TestSearchCalculateCASSScore(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want float64
	}{
		{
			name: "all defaults",
			data: map[string]any{},
			want: 0.5 * 1.0 * 0.5, // utility=0.5, maturity=1.0, confidence=0.5
		},
		{
			name: "established high utility high confidence",
			data: map[string]any{
				"utility":    0.9,
				"maturity":   "established",
				"confidence": 0.8,
			},
			want: 0.9 * 1.5 * 0.8,
		},
		{
			name: "anti-pattern low score",
			data: map[string]any{
				"utility":    0.5,
				"maturity":   "anti-pattern",
				"confidence": 0.5,
			},
			want: 0.5 * 0.3 * 0.5,
		},
		{
			name: "candidate maturity",
			data: map[string]any{
				"utility":    0.7,
				"maturity":   "candidate",
				"confidence": 0.6,
			},
			want: 0.7 * 1.2 * 0.6,
		},
		{
			name: "zero utility uses default",
			data: map[string]any{
				"utility":    0.0,
				"maturity":   "provisional",
				"confidence": 0.4,
			},
			want: 0.5 * 1.0 * 0.4, // utility 0.0 fails >0 check, falls to default 0.5
		},
		{
			name: "zero confidence uses default",
			data: map[string]any{
				"utility":    0.8,
				"confidence": 0.0,
			},
			want: 0.8 * 1.0 * 0.5, // confidence 0.0 fails >0 check, falls to default 0.5
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateCASSScore(tc.data)
			diff := got - tc.want
			if diff > 0.001 || diff < -0.001 {
				t.Errorf("calculateCASSScore() = %f, want %f", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// search.go — extractLearningContext
// ===========================================================================

func TestSearchExtractLearningContext(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "has summary",
			data: map[string]any{"summary": "Auth bug fix", "content": "Detailed content"},
			want: "Auth bug fix",
		},
		{
			name: "no summary, has content",
			data: map[string]any{"content": "Fallback content"},
			want: "Fallback content",
		},
		{
			name: "neither summary nor content",
			data: map[string]any{"other": "value"},
			want: "",
		},
		{
			name: "empty map",
			data: map[string]any{},
			want: "",
		},
		{
			name: "summary is not a string",
			data: map[string]any{"summary": 42, "content": "Fallback"},
			want: "Fallback",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractLearningContext(tc.data)
			if got != tc.want {
				t.Errorf("extractLearningContext() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// search.go — parseLearningMatch
// ===========================================================================

func TestSearchParseLearningMatch(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		file       string
		wantOK     bool
		wantType   string
		wantScore  bool // whether score should be > 0
		wantCtxPfx string
	}{
		{
			name:       "valid with maturity and summary",
			line:       `{"maturity":"established","summary":"Use mutex for shared state","utility":0.9,"confidence":0.8}`,
			file:       "/learnings/mutex.jsonl",
			wantOK:     true,
			wantType:   "learning",
			wantScore:  true,
			wantCtxPfx: "[established]",
		},
		{
			name:       "valid with default maturity",
			line:       `{"summary":"Simple learning"}`,
			file:       "/learnings/simple.jsonl",
			wantOK:     true,
			wantType:   "learning",
			wantCtxPfx: "[provisional]",
		},
		{
			name:   "invalid JSON",
			line:   `not json`,
			file:   "/learnings/bad.jsonl",
			wantOK: false,
		},
		{
			name:       "valid with content instead of summary",
			line:       `{"content":"Content fallback","maturity":"candidate"}`,
			file:       "/learnings/content.jsonl",
			wantOK:     true,
			wantType:   "learning",
			wantCtxPfx: "[candidate]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, ok := parseLearningMatch(tc.line, tc.file)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if result.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", result.Type, tc.wantType)
			}
			if result.Path != tc.file {
				t.Errorf("Path = %q, want %q", result.Path, tc.file)
			}
			if tc.wantScore && result.Score <= 0 {
				t.Errorf("Score = %f, expected > 0", result.Score)
			}
			if tc.wantCtxPfx != "" {
				if len(result.Context) < len(tc.wantCtxPfx) || result.Context[:len(tc.wantCtxPfx)] != tc.wantCtxPfx {
					t.Errorf("Context = %q, want prefix %q", result.Context, tc.wantCtxPfx)
				}
			}
		})
	}
}

// ===========================================================================
// search.go — getFileContext (uses temp files)
// ===========================================================================

func TestSearchGetFileContext(t *testing.T) {
	t.Run("finds matching lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.md")
		content := "# Title\nThis has the KEYWORD in it.\nAnother line.\nAlso keyword here.\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := getFileContext(path, "keyword")
		if got == "" {
			t.Error("expected non-empty context")
		}
		// Should contain both matching lines (case-insensitive)
		lines := testCountContextLines(got)
		if lines < 2 {
			t.Errorf("expected at least 2 lines in context, got %d: %q", lines, got)
		}
	})

	t.Run("returns empty for no matches", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.md")
		if err := os.WriteFile(path, []byte("nothing relevant here"), 0644); err != nil {
			t.Fatal(err)
		}

		got := getFileContext(path, "nonexistent")
		if got != "" {
			t.Errorf("expected empty context, got %q", got)
		}
	})

	t.Run("returns empty for nonexistent file", func(t *testing.T) {
		got := getFileContext("/nonexistent/file.md", "query")
		if got != "" {
			t.Errorf("expected empty context for missing file, got %q", got)
		}
	})

	t.Run("truncates long lines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "long.md")
		longLine := make([]byte, 200)
		for i := range longLine {
			longLine[i] = 'x'
		}
		// Insert "keyword" in the middle
		copy(longLine[50:], []byte("keyword"))
		if err := os.WriteFile(path, longLine, 0644); err != nil {
			t.Fatal(err)
		}

		got := getFileContext(path, "keyword")
		if got == "" {
			t.Fatal("expected non-empty context")
		}
		// Should be truncated with "..."
		if len(got) > search.ContextLineMaxLength+10 { // some slack for "..."
			t.Errorf("context too long: %d chars", len(got))
		}
	})

	t.Run("respects search.MaxContextLines", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "many.md")
		content := ""
		for i := 0; i < 10; i++ {
			content += "this line has keyword\n"
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := getFileContext(path, "keyword")
		lines := testCountContextLines(got)
		if lines > search.MaxContextLines {
			t.Errorf("context has %d lines, should be at most %d", lines, search.MaxContextLines)
		}
	})
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func testCountContextLines(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}

// ===========================================================================
// worktree.go — runIDFromWorktreePath
// ===========================================================================

func TestWorktreeRunIDFromWorktreePath(t *testing.T) {
	tests := []struct {
		name         string
		repoRoot     string
		worktreePath string
		want         string
	}{
		{
			name:         "standard pattern",
			repoRoot:     "/home/user/myrepo",
			worktreePath: "/home/user/myrepo-rpi-abc123",
			want:         "abc123",
		},
		{
			name:         "hyphenated run ID",
			repoRoot:     "/project/cli",
			worktreePath: "/project/cli-rpi-ag-m0r",
			want:         "ag-m0r",
		},
		{
			name:         "no match — different prefix",
			repoRoot:     "/project/cli",
			worktreePath: "/project/other-rpi-abc",
			want:         "",
		},
		{
			name:         "no match — missing rpi prefix",
			repoRoot:     "/project/cli",
			worktreePath: "/project/cli-worktree-abc",
			want:         "",
		},
		{
			name:         "empty run ID after prefix",
			repoRoot:     "/project/cli",
			worktreePath: "/project/cli-rpi-",
			want:         "",
		},
		{
			name:         "nested path",
			repoRoot:     "/a/b/c",
			worktreePath: "/somewhere/else/c-rpi-run42",
			want:         "run42",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runIDFromWorktreePath(tc.repoRoot, tc.worktreePath)
			if got != tc.want {
				t.Errorf("runIDFromWorktreePath(%q, %q) = %q, want %q",
					tc.repoRoot, tc.worktreePath, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// worktree.go — parseRPITmuxSessionRunID
// ===========================================================================

func TestWorktreeParseRPITmuxSessionRunID(t *testing.T) {
	tests := []struct {
		name    string
		session string
		wantID  string
		wantOK  bool
	}{
		{
			name:    "valid p1 session",
			session: "ao-rpi-abc123-p1",
			wantID:  "abc123",
			wantOK:  true,
		},
		{
			name:    "valid p2 session",
			session: "ao-rpi-run-xyz-p2",
			wantID:  "run-xyz",
			wantOK:  true,
		},
		{
			name:    "valid p3 session",
			session: "ao-rpi-test-p3",
			wantID:  "test",
			wantOK:  true,
		},
		{
			name:    "invalid — no phase suffix",
			session: "ao-rpi-abc123",
			wantOK:  false,
		},
		{
			name:    "invalid — wrong prefix",
			session: "other-rpi-abc-p1",
			wantOK:  false,
		},
		{
			name:    "invalid — p4 not allowed",
			session: "ao-rpi-abc-p4",
			wantOK:  false,
		},
		{
			name:    "empty session name",
			session: "",
			wantOK:  false,
		},
		{
			name:    "just prefix no runID",
			session: "ao-rpi--p1",
			wantOK:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := parseRPITmuxSessionRunID(tc.session)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (id=%q)", ok, tc.wantOK, id)
			}
			if ok && id != tc.wantID {
				t.Errorf("id = %q, want %q", id, tc.wantID)
			}
		})
	}
}

// ===========================================================================
// worktree.go — parseTmuxSessionListOutput
// ===========================================================================

func TestWorktreeParseTmuxSessionListOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantLen int
		wantIDs []string
	}{
		{
			name:    "empty output",
			output:  "",
			wantLen: 0,
		},
		{
			name:    "whitespace only",
			output:  "   \n  \n",
			wantLen: 0,
		},
		{
			name:    "valid RPI sessions",
			output:  "ao-rpi-abc-p1\t1700000000\nao-rpi-abc-p2\t1700000000\n",
			wantLen: 2,
			wantIDs: []string{"abc", "abc"},
		},
		{
			name:    "mixed RPI and non-RPI sessions",
			output:  "ao-rpi-run1-p1\t1700000000\nmy-session\t1700000000\n",
			wantLen: 1,
			wantIDs: []string{"run1"},
		},
		{
			name:    "invalid epoch skipped",
			output:  "ao-rpi-abc-p1\tnot_a_number\n",
			wantLen: 0,
		},
		{
			name:    "wrong field count skipped",
			output:  "ao-rpi-abc-p1\t1700000000\textra\n",
			wantLen: 0,
		},
		{
			name:    "single field line skipped",
			output:  "ao-rpi-abc-p1\n",
			wantLen: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTmuxSessionListOutput(tc.output)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d; sessions: %+v", len(got), tc.wantLen, got)
			}
			for i, wantID := range tc.wantIDs {
				if got[i].RunID != wantID {
					t.Errorf("[%d] RunID = %q, want %q", i, got[i].RunID, wantID)
				}
			}
		})
	}
}

// ===========================================================================
// worktree.go — shouldCleanupRPITmuxSession
// ===========================================================================

func TestWorktreeShouldCleanupRPITmuxSession(t *testing.T) {
	now := time.Now()
	staleAfter := 1 * time.Hour

	tests := []struct {
		name             string
		runID            string
		createdAt        time.Time
		activeRuns       map[string]bool
		liveWorktreeRuns map[string]bool
		want             bool
	}{
		{
			name:             "stale orphan should be cleaned",
			runID:            "old-run",
			createdAt:        now.Add(-2 * time.Hour),
			activeRuns:       map[string]bool{},
			liveWorktreeRuns: map[string]bool{},
			want:             true,
		},
		{
			name:             "empty runID never cleaned",
			runID:            "",
			createdAt:        now.Add(-2 * time.Hour),
			activeRuns:       map[string]bool{},
			liveWorktreeRuns: map[string]bool{},
			want:             false,
		},
		{
			name:             "active run not cleaned",
			runID:            "active-run",
			createdAt:        now.Add(-2 * time.Hour),
			activeRuns:       map[string]bool{"active-run": true},
			liveWorktreeRuns: map[string]bool{},
			want:             false,
		},
		{
			name:             "live worktree run not cleaned",
			runID:            "live-run",
			createdAt:        now.Add(-2 * time.Hour),
			activeRuns:       map[string]bool{},
			liveWorktreeRuns: map[string]bool{"live-run": true},
			want:             false,
		},
		{
			name:             "too recent not cleaned",
			runID:            "recent-run",
			createdAt:        now.Add(-30 * time.Minute),
			activeRuns:       map[string]bool{},
			liveWorktreeRuns: map[string]bool{},
			want:             false,
		},
		{
			name:             "exactly at stale boundary is cleaned",
			runID:            "boundary-run",
			createdAt:        now.Add(-staleAfter),
			activeRuns:       map[string]bool{},
			liveWorktreeRuns: map[string]bool{},
			want:             true, // Sub == staleAfter, and staleAfter < staleAfter is false, so session IS eligible
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldCleanupRPITmuxSession(
				tc.runID, tc.createdAt, now, staleAfter,
				tc.activeRuns, tc.liveWorktreeRuns,
			)
			if got != tc.want {
				t.Errorf("shouldCleanupRPITmuxSession() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ===========================================================================
// worktree.go — worktreeReferenceTime (uses temp dirs)
// ===========================================================================

func TestWorktreeWorktreeReferenceTime(t *testing.T) {
	t.Run("returns epoch for nonexistent path", func(t *testing.T) {
		got := worktreeReferenceTime("/nonexistent/path/xyz")
		if !got.Equal(time.Unix(0, 0)) {
			t.Errorf("expected epoch time, got %v", got)
		}
	})

	t.Run("returns dir mtime for bare worktree", func(t *testing.T) {
		dir := t.TempDir()
		got := worktreeReferenceTime(dir)
		if got.IsZero() || got.Equal(time.Unix(0, 0)) {
			t.Errorf("expected non-zero time for existing dir, got %v", got)
		}
	})

	t.Run("prefers newer rpi state file", func(t *testing.T) {
		dir := t.TempDir()
		rpiDir := filepath.Join(dir, ".agents", "rpi")
		if err := os.MkdirAll(rpiDir, 0755); err != nil {
			t.Fatal(err)
		}

		stateFile := filepath.Join(rpiDir, "phased-state.json")
		if err := os.WriteFile(stateFile, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}

		// Touch the state file to be newer
		futureTime := time.Now().Add(1 * time.Hour)
		if err := os.Chtimes(stateFile, futureTime, futureTime); err != nil {
			t.Fatal(err)
		}

		got := worktreeReferenceTime(dir)
		// Should pick up the state file's time, not the dir's
		if got.Before(futureTime.Add(-time.Second)) {
			t.Errorf("expected reference time near %v, got %v", futureTime, got)
		}
	})
}

// ===========================================================================
// search.go — searchJSONL (uses temp dir for real file I/O)
// ===========================================================================

func TestSearchSearchJSONL(t *testing.T) {
	dir := t.TempDir()

	// Create a JSONL file with some matching content
	f1 := filepath.Join(dir, "sessions.jsonl")
	content := `{"summary":"Fixed mutex bug in auth module"}
{"summary":"Added retry logic for HTTP calls"}
{"summary":"Refactored database connection pool"}
`
	if err := os.WriteFile(f1, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another JSONL file
	f2 := filepath.Join(dir, "learnings.jsonl")
	content2 := `{"summary":"Always use context for cancellation"}
`
	if err := os.WriteFile(f2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("finds matching entries", func(t *testing.T) {
		results, err := searchJSONL("mutex", dir, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Path != f1 {
			t.Errorf("Path = %q, want %q", results[0].Path, f1)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := searchJSONL("a", dir, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) > 1 {
			t.Errorf("expected at most 1 result, got %d", len(results))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		results, err := searchJSONL("zzzznoexist", dir, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		results, err := searchJSONL("query", emptyDir, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}
