package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/search"
)

func writeFakeCass(t *testing.T, output string, exitCode int) (binDir string, argsPath string) {
	t.Helper()

	binDir = t.TempDir()
	argsPath = filepath.Join(binDir, "cass.args")
	outputPath := filepath.Join(binDir, "cass-output.json")

	if exitCode == 0 {
		if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
			t.Fatalf("write fake cass output: %v", err)
		}
	}

	script := "#!/bin/sh\nset -eu\n"
	script += fmt.Sprintf("printf '%%s\\n' \"$@\" > %q\n", argsPath)
	if exitCode == 0 {
		script += fmt.Sprintf("cat %q\n", outputPath)
	} else {
		script += "echo 'cass failed' >&2\n"
		script += fmt.Sprintf("exit %d\n", exitCode)
	}

	cassPath := filepath.Join(binDir, "cass")
	if err := os.WriteFile(cassPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake cass script: %v", err)
	}

	return binDir, argsPath
}

// chdirTempWorkspace is a thin wrapper around chdirTo (testutil_test.go) that
// discards the returned previous-directory string. Prefer chdirTo directly in
// new code; this wrapper exists only to avoid a mass-rename in existing tests.
func chdirTempWorkspace(t *testing.T, dir string) {
	t.Helper()
	_ = chdirTo(t, dir)
}

func TestClassifyResultType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"learnings path", "/foo/.agents/learnings/L42.md", "learning"},
		{"patterns path", "/foo/.agents/patterns/mutex.md", "pattern"},
		{"retros path", "/foo/.agents/retro/2026-01.md", "retro"},
		{"research path", "/foo/.agents/research/auth.md", "research"},
		{"compiled path", "/foo/.agents/compiled/testing-strategy.md", "compiled"},
		{"windows compiled path", `C:\repo\.agents\compiled\testing-strategy.md`, "compiled"},
		{"sessions path", "/foo/.agents/ao/sessions/s1.md", "session"},
		{"decisions path", "/foo/.agents/decisions/use-go.md", "decision"},
		{"unknown path", "/foo/bar/baz.md", "knowledge"},
		{"case insensitive", "/foo/LEARNINGS/test.md", "learning"},
		{"empty path", "", "knowledge"},
		{"nested learnings", "/a/b/learnings/deep/nested.md", "learning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyResultType(tt.path)
			if got != tt.want {
				t.Errorf("classifyResultType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilterByType(t *testing.T) {
	results := []searchResult{
		{Path: "a.md", Type: "session"},
		{Path: "b.md", Type: "learning"},
		{Path: "c.md", Type: "session"},
		{Path: "d.md", Type: "pattern"},
	}

	tests := []struct {
		name       string
		filterType string
		wantCount  int
	}{
		{"filter sessions", "session", 2},
		{"filter learnings", "learning", 1},
		{"filter patterns", "pattern", 1},
		{"filter nonexistent", "retro", 0},
		{"empty filter returns all", "", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByType(results, tt.filterType)
			if len(got) != tt.wantCount {
				t.Errorf("filterByType(%q) returned %d results, want %d", tt.filterType, len(got), tt.wantCount)
			}
		})
	}
}

func TestNormalizeSearchType(t *testing.T) {
	tests := map[string]string{
		"":          "",
		"sessions":  "session",
		"learnings": "learning",
		"patterns":  "pattern",
		"findings":  "finding",
		"decisions": "decision",
		"retros":    "retro",
		"research":  "research",
		"compiled":  "compiled",
		"synthesis": "compiled",
	}

	for input, want := range tests {
		if got := normalizeSearchType(input); got != want {
			t.Fatalf("normalizeSearchType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOutputSearchResults_JSONEmptyResults(t *testing.T) {
	prevOutput := output
	output = "json"
	t.Cleanup(func() { output = prevOutput })

	out := captureJSONStdout(t, func() {
		if err := outputSearchResults("session", nil); err != nil {
			t.Fatalf("outputSearchResults() error = %v", err)
		}
	})

	out = strings.TrimSpace(out)
	if out != "[]" {
		t.Fatalf("expected output [] for empty JSON results, got %q", out)
	}

	var results []searchResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", out, err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results from outputSearchResults(), got %d", len(results))
	}
}

func TestRunSearch_JSONNoResultsAfterTypeFilter(t *testing.T) {
	tmp := t.TempDir()

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })

	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	sessionPath := filepath.Join(sessionsDir, "session-sample.md")
	if err := os.WriteFile(sessionPath, []byte("Session about mutex patterns"), 0644); err != nil {
		t.Fatal(err)
	}

	prevOutput := output
	prevDryRun := dryRun
	prevSearchType := searchType
	prevSearchLimit := searchLimit
	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	output = "json"
	dryRun = false
	searchType = "decisions"
	searchLimit = 10
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		output = prevOutput
		dryRun = prevDryRun
		searchType = prevSearchType
		searchLimit = prevSearchLimit
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	out := captureJSONStdout(t, func() {
		if err := runSearch(searchCmd, []string{"mutex"}); err != nil {
			t.Fatalf("runSearch() error = %v", err)
		}
	})

	out = strings.TrimSpace(out)
	var results []searchResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("expected JSON array, got %q: %v", out, err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results after type filter, got %d", len(results))
	}
}

func TestRunSearch_JSONNoDataDirReturnsEmptyArray(t *testing.T) {
	tmp := t.TempDir()

	chdirTempWorkspace(t, tmp)
	t.Setenv("PATH", t.TempDir())

	prevOutput := output
	prevDryRun := dryRun
	prevSearchType := searchType
	prevSearchLimit := searchLimit
	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	output = "json"
	dryRun = false
	searchType = ""
	searchLimit = 10
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		output = prevOutput
		dryRun = prevDryRun
		searchType = prevSearchType
		searchLimit = prevSearchLimit
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	out := captureJSONStdout(t, func() {
		if err := runSearch(searchCmd, []string{"mutex"}); err != nil {
			t.Fatalf("runSearch() error = %v", err)
		}
	})

	out = strings.TrimSpace(out)
	var results []searchResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("expected JSON array, got %q: %v", out, err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for missing data dir, got %d", len(results))
	}
}

func TestRunSearch_AutoUsesCassWithoutRepoLocalData(t *testing.T) {
	tmp := t.TempDir()
	chdirTempWorkspace(t, tmp)

	cassJSON := `{
  "query": "Shield AI recruiter",
  "limit": 10,
  "offset": 0,
  "count": 1,
  "total_matches": 1,
  "hits": [
    {
      "source_path": "/tmp/cass-session.jsonl",
      "score": 12.3,
      "snippet": "Known Shield AI recruiter history",
      "content": "Known Shield AI recruiter history",
      "workspace": "` + tmp + `",
      "line_number": 1
    }
  ]
}`
	binDir, argsPath := writeFakeCass(t, cassJSON, 0)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevOutput := output
	prevDryRun := dryRun
	prevSearchType := searchType
	prevSearchLimit := searchLimit
	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	output = "json"
	dryRun = false
	searchType = ""
	searchLimit = 10
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		output = prevOutput
		dryRun = prevDryRun
		searchType = prevSearchType
		searchLimit = prevSearchLimit
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	out := captureJSONStdout(t, func() {
		if err := runSearch(searchCmd, []string{"Shield AI recruiter"}); err != nil {
			t.Fatalf("runSearch() error = %v", err)
		}
	})

	var results []searchResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &results); err != nil {
		t.Fatalf("parse runSearch JSON output: %v\noutput=%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 cass-backed result, got %d: %+v", len(results), results)
	}
	if results[0].Path != "/tmp/cass-session.jsonl" {
		t.Fatalf("result path = %q, want /tmp/cass-session.jsonl", results[0].Path)
	}
	if results[0].Type != "session" {
		t.Fatalf("result type = %q, want session", results[0].Type)
	}
	if !strings.Contains(results[0].Context, "Shield AI recruiter") {
		t.Fatalf("result context = %q, want cass snippet", results[0].Context)
	}

	argsRaw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read fake cass args: %v", err)
	}
	argsText := string(argsRaw)
	for _, want := range []string{"search", "--json", "--workspace", tmp, "--limit", "10", "Shield AI recruiter"} {
		if !strings.Contains(argsText, want) {
			t.Fatalf("cass args %q did not include %q", argsText, want)
		}
	}
}

func TestCalculateCASSScore(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantMin float64
		wantMax float64
	}{
		{
			name:    "all defaults",
			data:    map[string]any{},
			wantMin: 0.124, // 0.5 * 1.0 * 0.5 = 0.25
			wantMax: 0.251,
		},
		{
			name: "established maturity",
			data: map[string]any{
				"maturity":   "established",
				"utility":    0.8,
				"confidence": 0.9,
			},
			wantMin: 1.07, // 0.8 * 1.5 * 0.9 = 1.08
			wantMax: 1.09,
		},
		{
			name: "anti-pattern low weight",
			data: map[string]any{
				"maturity":   "anti-pattern",
				"utility":    0.5,
				"confidence": 0.5,
			},
			wantMin: 0.074, // 0.5 * 0.3 * 0.5 = 0.075
			wantMax: 0.076,
		},
		{
			name: "candidate maturity",
			data: map[string]any{
				"maturity":   "candidate",
				"utility":    1.0,
				"confidence": 1.0,
			},
			wantMin: 1.19, // 1.0 * 1.2 * 1.0 = 1.2
			wantMax: 1.21,
		},
		{
			name: "provisional maturity explicit",
			data: map[string]any{
				"maturity":   "provisional",
				"utility":    0.5,
				"confidence": 0.5,
			},
			wantMin: 0.249, // 0.5 * 1.0 * 0.5 = 0.25
			wantMax: 0.251,
		},
		{
			name: "zero utility uses default",
			data: map[string]any{
				"utility": 0.0,
			},
			wantMin: 0.249, // default 0.5 * 1.0 * 0.5 = 0.25
			wantMax: 0.251,
		},
		{
			name: "negative utility uses default",
			data: map[string]any{
				"utility": -1.0,
			},
			wantMin: 0.249,
			wantMax: 0.251,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCASSScore(tt.data)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateCASSScore() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGetFileContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file with matching content
	content := `Line one
This line contains the QUERY term
Another unrelated line
Also has query in it
Third match with query here
Fourth query should be excluded (max 3)
`
	path := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("finds matching lines", func(t *testing.T) {
		ctx := getFileContext(path, "query")
		if ctx == "" {
			t.Error("expected non-empty context")
		}
		// Should contain up to search.MaxContextLines matches
		lines := splitNonEmpty(ctx)
		if len(lines) > search.MaxContextLines {
			t.Errorf("got %d context lines, want at most %d", len(lines), search.MaxContextLines)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		ctx := getFileContext(path, "QUERY")
		if ctx == "" {
			t.Error("expected case-insensitive match")
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		ctx := getFileContext(path, "nonexistent_xyz_999")
		if ctx != "" {
			t.Errorf("expected empty context, got %q", ctx)
		}
	})

	t.Run("nonexistent file returns empty", func(t *testing.T) {
		ctx := getFileContext(filepath.Join(tmpDir, "nope.md"), "query")
		if ctx != "" {
			t.Errorf("expected empty context for missing file, got %q", ctx)
		}
	})

	// Test line truncation
	t.Run("long lines are truncated", func(t *testing.T) {
		longLine := "query " + string(make([]byte, search.ContextLineMaxLength+50))
		longPath := filepath.Join(tmpDir, "long.md")
		if err := os.WriteFile(longPath, []byte(longLine), 0644); err != nil {
			t.Fatal(err)
		}
		ctx := getFileContext(longPath, "query")
		// Each line should be at most search.ContextLineMaxLength + "..."
		for _, line := range splitNonEmpty(ctx) {
			if len(line) > search.ContextLineMaxLength+3 {
				t.Errorf("line length %d exceeds max %d+3", len(line), search.ContextLineMaxLength)
			}
		}
	})
}

// splitNonEmpty splits a string by newlines and removes empty strings.
func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range splitLines(s) {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func TestSearchJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create JSONL files
	data1 := map[string]any{
		"id":      "L1",
		"summary": "Authentication patterns for Go services",
		"content": "Use middleware for auth",
	}
	line1, _ := json.Marshal(data1)
	if err := os.WriteFile(filepath.Join(tmpDir, "auth.jsonl"), line1, 0644); err != nil {
		t.Fatal(err)
	}

	data2 := map[string]any{
		"id":      "L2",
		"summary": "Database connection pooling",
		"content": "Pool connections for efficiency",
	}
	line2, _ := json.Marshal(data2)
	if err := os.WriteFile(filepath.Join(tmpDir, "db.jsonl"), line2, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("finds matching JSONL", func(t *testing.T) {
		results, err := searchJSONL("auth", tmpDir, 10)
		if err != nil {
			t.Fatalf("searchJSONL() error = %v", err)
		}
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if len(results) > 0 && results[0].Context == "" {
			t.Error("expected non-empty context from summary field")
		}
	})

	t.Run("no match", func(t *testing.T) {
		results, err := searchJSONL("kubernetes", tmpDir, 10)
		if err != nil {
			t.Fatalf("searchJSONL() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		// Both files contain common words
		results, err := searchJSONL("for", tmpDir, 1)
		if err != nil {
			t.Fatalf("searchJSONL() error = %v", err)
		}
		if len(results) > 1 {
			t.Errorf("got %d results, want at most 1", len(results))
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		results, err := searchJSONL("test", emptyDir, 10)
		if err != nil {
			t.Fatalf("searchJSONL() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results from empty dir, want 0", len(results))
		}
	})
}

func TestSearchFilesWithFixtures(t *testing.T) {
	tmp := t.TempDir()

	// Create a sessions directory with sample session files
	sessDir := filepath.Join(tmp, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a session file with searchable content
	sessionContent := `# Session: abc-123

## Summary
Implemented mutex pattern for concurrent access to shared state.

## Learnings
- Use sync.RWMutex for read-heavy workloads
- Always defer Unlock() calls
`
	if err := os.WriteFile(filepath.Join(sessDir, "session-abc-123.md"), []byte(sessionContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write another session
	session2 := `# Session: def-456

## Summary
Database migration patterns for PostgreSQL.
`
	if err := os.WriteFile(filepath.Join(sessDir, "session-def-456.md"), []byte(session2), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("finds matching content", func(t *testing.T) {
		results, err := searchFiles("mutex", sessDir, 10)
		if err != nil {
			t.Fatalf("searchFiles() error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected at least 1 result for 'mutex'")
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		results, err := searchFiles("kubernetes_xyz_nonexistent", sessDir, 10)
		if err != nil {
			t.Fatalf("searchFiles() error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("returns results from both sources", func(t *testing.T) {
		results, err := searchFiles("Session", sessDir, 10)
		if err != nil {
			t.Fatalf("searchFiles() error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected results for 'Session'")
		}
	})
}

func TestSearchFilesNoData(t *testing.T) {
	tmp := t.TempDir()
	// Use an empty (but existing) directory — grep returns error for nonexistent dirs
	emptyDir := filepath.Join(tmp, "sessions")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	results, err := searchFiles("test", emptyDir, 10)
	if err != nil {
		t.Fatalf("searchFiles() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty dir, got %d", len(results))
	}
}

func TestParseGrepResults(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		pattern    string
		query      string
		useRipgrep bool
		wantCount  int
	}{
		{
			name:       "ripgrep output (no filtering needed)",
			output:     "/tmp/test/a.md\n/tmp/test/b.md\n",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: true,
			wantCount:  2,
		},
		{
			name:       "grep output filtered by pattern",
			output:     "/tmp/test/a.md\n/tmp/test/b.txt\n/tmp/test/c.md\n",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: false,
			wantCount:  2,
		},
		{
			name:       "empty output",
			output:     "",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: true,
			wantCount:  0,
		},
		{
			name:       "only newlines",
			output:     "\n\n\n",
			pattern:    "*.md",
			query:      "test",
			useRipgrep: true,
			wantCount:  0,
		},
		{
			name:       "grep no pattern filter",
			output:     "/tmp/test/a.md\n",
			pattern:    "",
			query:      "test",
			useRipgrep: false,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGrepResults([]byte(tt.output), tt.pattern, tt.query, tt.useRipgrep)
			if len(got) != tt.wantCount {
				t.Errorf("parseGrepResults() returned %d results, want %d", len(got), tt.wantCount)
			}
			// All results should have Type = "session"
			for _, r := range got {
				if r.Type != "session" {
					t.Errorf("result Type = %q, want %q", r.Type, "session")
				}
			}
		})
	}
}

func TestSearchCASS_ResolvesAgentsRoot(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	researchDir := filepath.Join(tmp, ".agents", "research")

	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0755); err != nil {
		t.Fatal(err)
	}

	researchPath := filepath.Join(researchDir, "shield-ai.md")
	if err := os.WriteFile(researchPath, []byte("Shield AI recruiter reached out about a role."), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchCASS("Shield AI recruiter", sessionsDir, 10)
	if err != nil {
		t.Fatalf("searchCASS() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Path != researchPath {
		t.Fatalf("result path = %q, want %q", results[0].Path, researchPath)
	}
	if results[0].Type != "research" {
		t.Fatalf("result type = %q, want research", results[0].Type)
	}
}

func TestSelectAndSearch_DefaultsToCASS(t *testing.T) {
	tmp := t.TempDir()
	chdirTempWorkspace(t, tmp)
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	researchDir := filepath.Join(tmp, ".agents", "research")

	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0755); err != nil {
		t.Fatal(err)
	}

	researchPath := filepath.Join(researchDir, "history.md")
	if err := os.WriteFile(researchPath, []byte("Known Shield AI recruiter chat history."), 0644); err != nil {
		t.Fatal(err)
	}

	cassJSON := `{
  "query": "Shield AI recruiter",
  "limit": 10,
  "offset": 0,
  "count": 2,
  "total_matches": 2,
  "hits": [
    {
      "source_path": "/tmp/cass-session.jsonl",
      "score": 19.4,
      "snippet": "Known Shield AI recruiter chat history.",
      "content": "Known Shield AI recruiter chat history.",
      "workspace": "` + tmp + `",
      "line_number": 1
    },
    {
      "source_path": "/tmp/cass-session.jsonl",
      "score": 18.1,
      "snippet": "Duplicate hit from same transcript should collapse.",
      "content": "Duplicate hit from same transcript should collapse.",
      "workspace": "` + tmp + `",
      "line_number": 2
    }
  ]
}`
	binDir, _ := writeFakeCass(t, cassJSON, 0)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	results, err := selectAndSearch("Shield AI recruiter", sessionsDir, 10)
	if err != nil {
		t.Fatalf("selectAndSearch() error = %v", err)
	}

	seenPaths := make(map[string]int)
	for _, result := range results {
		seenPaths[result.Path]++
	}
	if seenPaths["/tmp/cass-session.jsonl"] != 1 {
		t.Fatalf("expected deduped cass session result, got counts=%v", seenPaths)
	}
	if seenPaths[researchPath] != 1 {
		t.Fatalf("expected local research result, got counts=%v", seenPaths)
	}
}

func TestSelectAndSearch_CassFlagRequiresWorkingCass(t *testing.T) {
	tmp := t.TempDir()
	chdirTempWorkspace(t, tmp)

	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "shield-ai.md"), []byte("Known Shield AI recruiter chat history."), 0644); err != nil {
		t.Fatal(err)
	}

	binDir, _ := writeFakeCass(t, "", 42)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = true
	searchUseLocal = false
	t.Cleanup(func() {
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	_, err := selectAndSearch("Shield AI recruiter", sessionsDir, 10)
	if err == nil {
		t.Fatal("expected selectAndSearch() to fail when --cass backend fails")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cass") {
		t.Fatalf("expected cass error, got %v", err)
	}
}

func TestSelectAndSearch_AutoReturnsEmptyWhenCassUnavailableAndNoLocalData(t *testing.T) {
	tmp := t.TempDir()
	chdirTempWorkspace(t, tmp)
	t.Setenv("PATH", t.TempDir())

	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")

	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	results, err := selectAndSearch("Shield AI recruiter", sessionsDir, 10)
	if err != nil {
		t.Fatalf("selectAndSearch() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results when no backends are available, got %d: %+v", len(results), results)
	}
}

func TestSelectAndSearch_AutoFallsBackToLocalWhenCassFails(t *testing.T) {
	tmp := t.TempDir()
	chdirTempWorkspace(t, tmp)

	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0755); err != nil {
		t.Fatal(err)
	}
	researchPath := filepath.Join(researchDir, "shield-ai.md")
	if err := os.WriteFile(researchPath, []byte("Known Shield AI recruiter chat history."), 0644); err != nil {
		t.Fatal(err)
	}

	binDir, _ := writeFakeCass(t, "", 17)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	results, err := selectAndSearch("Shield AI recruiter", sessionsDir, 10)
	if err != nil {
		t.Fatalf("selectAndSearch() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 local fallback result, got %d: %+v", len(results), results)
	}
	if results[0].Path != researchPath {
		t.Fatalf("result path = %q, want %q", results[0].Path, researchPath)
	}
}

func TestSelectAndSearch_LocalFlagSkipsCass(t *testing.T) {
	tmp := t.TempDir()
	chdirTempWorkspace(t, tmp)

	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	researchDir := filepath.Join(tmp, ".agents", "research")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0755); err != nil {
		t.Fatal(err)
	}
	researchPath := filepath.Join(researchDir, "shield-ai.md")
	if err := os.WriteFile(researchPath, []byte("Known Shield AI recruiter chat history."), 0644); err != nil {
		t.Fatal(err)
	}

	binDir, _ := writeFakeCass(t, "", 23)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevSearchUseSC := searchUseSC
	prevSearchUseCASS := searchUseCASS
	prevSearchUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = true
	t.Cleanup(func() {
		searchUseSC = prevSearchUseSC
		searchUseCASS = prevSearchUseCASS
		searchUseLocal = prevSearchUseLocal
	})

	results, err := selectAndSearch("Shield AI recruiter", sessionsDir, 10)
	if err != nil {
		t.Fatalf("selectAndSearch() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 local result, got %d: %+v", len(results), results)
	}
	if results[0].Path != researchPath {
		t.Fatalf("result path = %q, want %q", results[0].Path, researchPath)
	}
}

func TestSearchFilesCombinedLimitEnforcement(t *testing.T) {
	tmp := t.TempDir()
	sessDir := filepath.Join(tmp, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create multiple markdown files
	for i := 1; i <= 5; i++ {
		content := "test content with searchable term\n"
		path := filepath.Join(sessDir, "session-"+string(rune('a'+i-1))+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create multiple JSONL files
	for i := 1; i <= 5; i++ {
		data := map[string]any{
			"id":      "L" + string(rune('0'+i)),
			"summary": "searchable term in JSONL content",
		}
		line, _ := json.Marshal(data)
		path := filepath.Join(sessDir, "learning-"+string(rune('a'+i-1))+".jsonl")
		if err := os.WriteFile(path, line, 0644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("enforces combined limit after dedup", func(t *testing.T) {
		limit := 3
		results, err := searchFiles("searchable", sessDir, limit)
		if err != nil {
			t.Fatalf("searchFiles() error: %v", err)
		}

		// Total files: 5 md + 5 jsonl = 10 unique results
		// After dedup, should be exactly the limit (3)
		if len(results) > limit {
			t.Errorf("searchFiles() returned %d results, want at most %d", len(results), limit)
		}
	})

	t.Run("no limit enforcement when limit is 0", func(t *testing.T) {
		results, err := searchFiles("searchable", sessDir, 0)
		if err != nil {
			t.Fatalf("searchFiles() error: %v", err)
		}

		// Should return all unique results without limit
		if len(results) == 0 {
			t.Error("expected results when limit=0, got none")
		}
	})

	t.Run("limit larger than results", func(t *testing.T) {
		limit := 100
		results, err := searchFiles("searchable", sessDir, limit)
		if err != nil {
			t.Fatalf("searchFiles() error: %v", err)
		}

		// Should return all available results (10)
		if len(results) > limit {
			t.Errorf("searchFiles() returned %d results, want at most %d", len(results), limit)
		}
	})
}

// ---------------------------------------------------------------------------
// displaySearchResults (0%)
// ---------------------------------------------------------------------------

func TestDisplaySearchResults_Basic(t *testing.T) {
	results := []searchResult{
		{Path: "/path/to/file1.md", Context: "line one\nline two", Type: "session"},
		{Path: "/path/to/file2.md", Context: "", Type: "learning"},
	}

	// Just ensure it doesn't panic. It writes to stdout.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	displaySearchResults("test query", results)

	_ = w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "2 result(s)") {
		t.Errorf("expected '2 result(s)' in output, got: %s", out)
	}
	if !strings.Contains(out, "test query") {
		t.Errorf("expected 'test query' in output, got: %s", out)
	}
	if !strings.Contains(out, "file1.md") {
		t.Errorf("expected 'file1.md' in output, got: %s", out)
	}
}

func TestDisplaySearchResults_Empty(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	displaySearchResults("empty", []searchResult{})

	_ = w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "0 result(s)") {
		t.Errorf("expected '0 result(s)' in output, got: %s", out)
	}
}

func TestDisplaySearchResults_WithContext(t *testing.T) {
	results := []searchResult{
		{Path: "/path/to/file.md", Context: "context line 1\ncontext line 2\n", Type: "session"},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	displaySearchResults("query", results)

	_ = w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "context line 1") {
		t.Errorf("expected context line in output, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// outputSearchResults — text mode (non-JSON)
// ---------------------------------------------------------------------------

func TestOutputSearchResults_TextMode(t *testing.T) {
	origOutput := output
	output = "table"
	t.Cleanup(func() { output = origOutput })

	results := []searchResult{
		{Path: "/test/file.md", Context: "some context", Type: "session"},
	}

	// Capture stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = outputSearchResults("query", results)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("outputSearchResults() error = %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if !strings.Contains(out, "1 result(s)") {
		t.Errorf("expected '1 result(s)' in text output, got: %s", out)
	}
}

func TestOutputSearchResults_JSONMode(t *testing.T) {
	origOutput := output
	output = "json"
	t.Cleanup(func() { output = origOutput })

	results := []searchResult{
		{Path: "/test/file.md", Context: "ctx", Type: "session"},
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = outputSearchResults("query", results)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("outputSearchResults() error = %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := strings.TrimSpace(string(buf[:n]))

	var parsed []searchResult
	if jsonErr := json.Unmarshal([]byte(out), &parsed); jsonErr != nil {
		t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", jsonErr, out)
	}
	if len(parsed) != 1 {
		t.Errorf("expected 1 result, got %d", len(parsed))
	}
}

// ---------------------------------------------------------------------------
// searchCASS (0%)
// ---------------------------------------------------------------------------

func TestSearchCASS(t *testing.T) {
	tmp := t.TempDir()

	// Create sessions dir
	sessDir := filepath.Join(tmp, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessDir, "session-1.md"), []byte("mutex pattern for concurrency"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create learnings dir with JSONL
	learningsDir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	learning := map[string]any{
		"id":         "L1",
		"summary":    "mutex pattern for safe access",
		"maturity":   "established",
		"utility":    0.8,
		"confidence": 0.9,
	}
	line, _ := json.Marshal(learning)
	if err := os.WriteFile(filepath.Join(learningsDir, "L1.jsonl"), line, 0644); err != nil {
		t.Fatal(err)
	}

	// Create patterns dir
	patternsDir := filepath.Join(tmp, "patterns")
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "mutex.md"), []byte("# Mutex Pattern\n\nUse mutex for concurrency."), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchCASS("mutex", sessDir, 10)
	if err != nil {
		t.Fatalf("searchCASS() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result from searchCASS")
	}

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted by score: [%d]=%f > [%d]=%f", i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestSearchCASS_NoLearnings(t *testing.T) {
	tmp := t.TempDir()
	sessDir := filepath.Join(tmp, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessDir, "s1.md"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchCASS("hello", sessDir, 10)
	if err != nil {
		t.Fatalf("searchCASS() error = %v", err)
	}
	// Should still work even without learnings/patterns dirs
	if results == nil {
		t.Error("expected non-nil results")
	}
}

func TestSearchCASS_LimitEnforced(t *testing.T) {
	tmp := t.TempDir()
	sessDir := filepath.Join(tmp, "sessions")
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create many files
	for i := 0; i < 10; i++ {
		name := filepath.Join(sessDir, "session-"+string(rune('a'+i))+".md")
		if err := os.WriteFile(name, []byte("searchable content here"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := searchCASS("searchable", sessDir, 3)
	if err != nil {
		t.Fatalf("searchCASS() error = %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// searchLearningsWithMaturity (0%)
// ---------------------------------------------------------------------------

func TestSearchLearningsWithMaturity(t *testing.T) {
	tmp := t.TempDir()

	// Create JSONL learning
	learning := map[string]any{
		"id":         "L1",
		"summary":    "authentication pattern for services",
		"maturity":   "established",
		"utility":    0.9,
		"confidence": 0.8,
	}
	line, _ := json.Marshal(learning)
	if err := os.WriteFile(filepath.Join(tmp, "auth.jsonl"), line, 0644); err != nil {
		t.Fatal(err)
	}

	// Create MD learning
	if err := os.WriteFile(filepath.Join(tmp, "auth-notes.md"), []byte("authentication notes for services"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchLearningsWithMaturity("authentication", tmp, 10)
	if err != nil {
		t.Fatalf("searchLearningsWithMaturity() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestSearchLearningsWithMaturity_NoMatch(t *testing.T) {
	tmp := t.TempDir()
	learning := map[string]any{"id": "L1", "summary": "unrelated content"}
	line, _ := json.Marshal(learning)
	if err := os.WriteFile(filepath.Join(tmp, "other.jsonl"), line, 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchLearningsWithMaturity("nonexistent_xyz", tmp, 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// truncateContext
// ---------------------------------------------------------------------------

func TestTruncateContext_Short(t *testing.T) {
	got := truncateContext("short")
	if got != "short" {
		t.Errorf("expected 'short', got %q", got)
	}
}

func TestTruncateContext_Long(t *testing.T) {
	long := strings.Repeat("x", search.ContextLineMaxLength+50)
	got := truncateContext(long)
	if len(got) != search.ContextLineMaxLength+3 { // +3 for "..."
		t.Errorf("expected length %d, got %d", search.ContextLineMaxLength+3, len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected '...' suffix")
	}
}

// ---------------------------------------------------------------------------
// parseLearningMatch
// ---------------------------------------------------------------------------

func TestParseLearningMatch_Valid(t *testing.T) {
	data := map[string]any{
		"summary":  "test learning",
		"maturity": "candidate",
		"utility":  0.7,
	}
	line, _ := json.Marshal(data)

	result, ok := parseLearningMatch(string(line), "/path/to/file.jsonl")
	if !ok {
		t.Error("expected ok=true")
	}
	if result.Type != "learning" {
		t.Errorf("type = %q, want learning", result.Type)
	}
	if !strings.Contains(result.Context, "[candidate]") {
		t.Errorf("expected '[candidate]' in context, got: %s", result.Context)
	}
}

func TestParseLearningMatch_InvalidJSON(t *testing.T) {
	_, ok := parseLearningMatch("not json", "/path/file.jsonl")
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// extractLearningContext
// ---------------------------------------------------------------------------

func TestExtractLearningContext_Summary(t *testing.T) {
	data := map[string]any{"summary": "test summary"}
	got := extractLearningContext(data)
	if got != "test summary" {
		t.Errorf("expected 'test summary', got %q", got)
	}
}

func TestExtractLearningContext_Content(t *testing.T) {
	data := map[string]any{"content": "test content"}
	got := extractLearningContext(data)
	if got != "test content" {
		t.Errorf("expected 'test content', got %q", got)
	}
}

func TestExtractLearningContext_Neither(t *testing.T) {
	data := map[string]any{"id": "L1"}
	got := extractLearningContext(data)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// maturityToWeight
// ---------------------------------------------------------------------------

func TestMaturityToWeight(t *testing.T) {
	tests := []struct {
		data map[string]any
		want float64
	}{
		{map[string]any{"maturity": "established"}, 1.5},
		{map[string]any{"maturity": "candidate"}, 1.2},
		{map[string]any{"maturity": "provisional"}, 1.0},
		{map[string]any{"maturity": "anti-pattern"}, 0.3},
		{map[string]any{"maturity": "unknown"}, 1.0},
		{map[string]any{}, 1.0},
	}
	for _, tt := range tests {
		got := maturityToWeight(tt.data)
		if got != tt.want {
			t.Errorf("maturityToWeight(%v) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// parseJSONLMatch
// ---------------------------------------------------------------------------

func TestParseJSONLMatch_WithSummary(t *testing.T) {
	data := map[string]any{"summary": "test summary", "id": "L1"}
	line, _ := json.Marshal(data)
	result, ok := parseJSONLMatch(string(line), "/path/file.jsonl")
	if !ok {
		t.Error("expected ok=true")
	}
	if result.Context != "test summary" {
		t.Errorf("expected 'test summary', got %q", result.Context)
	}
}

func TestParseJSONLMatch_LongSummary(t *testing.T) {
	long := strings.Repeat("x", search.ContextLineMaxLength+50)
	data := map[string]any{"summary": long}
	line, _ := json.Marshal(data)
	result, ok := parseJSONLMatch(string(line), "/path/file.jsonl")
	if !ok {
		t.Error("expected ok=true")
	}
	if len(result.Context) > search.ContextLineMaxLength+3 {
		t.Errorf("expected truncated context, got length %d", len(result.Context))
	}
}

func TestParseJSONLMatch_NoSummary(t *testing.T) {
	data := map[string]any{"id": "L1", "content": "some content"}
	line, _ := json.Marshal(data)
	result, ok := parseJSONLMatch(string(line), "/path/file.jsonl")
	if !ok {
		t.Error("expected ok=true")
	}
	if result.Context != "" {
		t.Errorf("expected empty context when no summary, got %q", result.Context)
	}
}

// ---------------------------------------------------------------------------
// selectAndSearch — file-based default path
// ---------------------------------------------------------------------------

func TestSelectAndSearch_FileBased(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "test.md"), []byte("searchable content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Reset search flags
	origUseSC := searchUseSC
	origUseCASS := searchUseCASS
	origUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		searchUseSC = origUseSC
		searchUseCASS = origUseCASS
		searchUseLocal = origUseLocal
	})

	results, err := selectAndSearch("searchable", tmp, 10)
	if err != nil {
		t.Fatalf("selectAndSearch() error = %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestSelectAndSearch_IncludesResearchDir(t *testing.T) {
	// Create a directory structure: parent/sessions/ and parent/research/
	parent := t.TempDir()
	sessionsDir := filepath.Join(parent, "sessions")
	researchDir := filepath.Join(parent, "research")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(researchDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a research file with unique content
	researchContent := "flywheel knowledge compounding research findings"
	if err := os.WriteFile(filepath.Join(researchDir, "2026-03-20-flywheel.md"), []byte(researchContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Reset search flags to use file-based (non-CASS) path
	origUseSC := searchUseSC
	origUseCASS := searchUseCASS
	origUseLocal := searchUseLocal
	searchUseSC = false
	searchUseCASS = false
	searchUseLocal = false
	t.Cleanup(func() {
		searchUseSC = origUseSC
		searchUseCASS = origUseCASS
		searchUseLocal = origUseLocal
	})

	results, err := selectAndSearch("flywheel", sessionsDir, 10)
	if err != nil {
		t.Fatalf("selectAndSearch() error = %v", err)
	}

	// Should find the research file
	foundResearch := false
	for _, r := range results {
		if r.Type == "research" {
			foundResearch = true
			break
		}
	}
	if !foundResearch {
		t.Error("expected research file in search results, but none found")
	}
}

func TestSearchRepoLocalKnowledgeIncludesCompiledDir(t *testing.T) {
	tmp := t.TempDir()
	sessionsDir := filepath.Join(tmp, ".agents", "ao", "sessions")
	compiledDir := filepath.Join(tmp, ".agents", "compiled")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatal(err)
	}

	compiledPath := filepath.Join(compiledDir, "testing-strategy.md")
	if err := os.WriteFile(compiledPath, []byte("Compiled synthesis about deterministic testing strategy."), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := searchRepoLocalKnowledge("deterministic testing", sessionsDir, 10)
	if err != nil {
		t.Fatalf("searchRepoLocalKnowledge() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Path != compiledPath {
		t.Fatalf("result path = %q, want %q", results[0].Path, compiledPath)
	}
	if results[0].Type != "compiled" {
		t.Fatalf("result type = %q, want compiled", results[0].Type)
	}
}
