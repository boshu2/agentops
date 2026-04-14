package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSessionFile(t *testing.T) {
	tmpDir := t.TempDir()

	for _, tc := range parseSessionFileCases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, tmpDir)
		})
	}
}

type parseSessionFileCase struct {
	name string
	run  func(t *testing.T, tmpDir string)
}

func parseSessionFileCases() []parseSessionFileCase {
	return []parseSessionFileCase{
		{name: "JSONL session", run: assertParseSessionJSONL},
		{name: "markdown session", run: assertParseSessionMarkdown},
		{name: "markdown with YAML frontmatter", run: assertParseSessionMarkdownFrontmatter},
		{name: "empty markdown", run: assertParseSessionEmptyMarkdown},
		{name: "nonexistent file", run: assertParseSessionNonexistentFile},
		{name: "invalid JSONL", run: assertParseSessionInvalidJSONL},
		{name: "long summary truncated", run: assertParseSessionLongSummaryTruncated},
	}
}

func assertParseSessionJSONL(t *testing.T, tmpDir string) {
	t.Helper()

	data := map[string]any{
		"summary": "Worked on authentication module",
	}
	line, _ := json.Marshal(data)
	path := writeParseSessionFixture(t, tmpDir, "session1.jsonl", line)

	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Summary != "Worked on authentication module" {
		t.Errorf("Summary = %q, want %q", s.Summary, "Worked on authentication module")
	}
	if s.Date == "" {
		t.Error("expected non-empty Date")
	}
}

func assertParseSessionMarkdown(t *testing.T, tmpDir string) {
	t.Helper()

	content := `# Session Summary

Implemented new database migration system.
`
	path := writeParseSessionFixture(t, tmpDir, "session2.md", []byte(content))

	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Summary != "Implemented new database migration system." {
		t.Errorf("Summary = %q, want %q", s.Summary, "Implemented new database migration system.")
	}
}

func assertParseSessionMarkdownFrontmatter(t *testing.T, tmpDir string) {
	t.Helper()

	content := "---\nutility: 0.50\nlast_reward: 0.35\nreward_count: 1\n---\n\n# Session\n\nActual session content here.\n"
	path := writeParseSessionFixture(t, tmpDir, "frontmatter.md", []byte(content))

	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Summary != "Actual session content here." {
		t.Errorf("Summary = %q, want %q (should skip frontmatter)", s.Summary, "Actual session content here.")
	}
}

func assertParseSessionEmptyMarkdown(t *testing.T, tmpDir string) {
	t.Helper()

	path := writeParseSessionFixture(t, tmpDir, "empty.md", []byte("# Title\n---\n"))

	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Summary != "" {
		t.Errorf("Summary = %q, want empty (only headings and separators)", s.Summary)
	}
}

func assertParseSessionNonexistentFile(t *testing.T, tmpDir string) {
	t.Helper()

	_, err := parseSessionFile(filepath.Join(tmpDir, "nope.jsonl"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func assertParseSessionInvalidJSONL(t *testing.T, tmpDir string) {
	t.Helper()

	path := writeParseSessionFixture(t, tmpDir, "bad.jsonl", []byte("not json"))

	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid JSON should result in empty summary
	if s.Summary != "" {
		t.Errorf("Summary = %q, want empty for invalid JSON", s.Summary)
	}
}

func assertParseSessionLongSummaryTruncated(t *testing.T, tmpDir string) {
	t.Helper()

	longSummary := make([]byte, 200)
	for i := range longSummary {
		longSummary[i] = 'a'
	}
	data := map[string]any{
		"summary": string(longSummary),
	}
	line, _ := json.Marshal(data)
	path := writeParseSessionFixture(t, tmpDir, "long.jsonl", line)

	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Summary) > 153 { // 150 + "..."
		t.Errorf("Summary length = %d, want at most 153 (truncated)", len(s.Summary))
	}
}

func writeParseSessionFixture(t *testing.T, tmpDir, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCollectSessionFiles_DeduplicatesPairs(t *testing.T) {
	sessionsDir := t.TempDir()

	// Create a .jsonl + .md pair for the same stem
	jsonlData, _ := json.Marshal(map[string]any{"summary": "paired session"})
	if err := os.WriteFile(filepath.Join(sessionsDir, "session-abc.jsonl"), jsonlData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "session-abc.md"), []byte("# Summary\n\nPaired session md"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a .md-only file (no matching .jsonl)
	if err := os.WriteFile(filepath.Join(sessionsDir, "session-only-md.md"), []byte("# Summary\n\nMd only"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a .jsonl-only file (no matching .md)
	jsonlOnly, _ := json.Marshal(map[string]any{"summary": "jsonl only"})
	if err := os.WriteFile(filepath.Join(sessionsDir, "session-only-jsonl.jsonl"), jsonlOnly, 0644); err != nil {
		t.Fatal(err)
	}

	files, err := collectSessionFiles(sessionsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 3 files: session-abc.jsonl (not .md), session-only-md.md, session-only-jsonl.jsonl
	if len(files) != 3 {
		t.Errorf("got %d files, want 3 (deduplicated pair + 2 singles)", len(files))
		for _, f := range files {
			t.Logf("  %s", filepath.Base(f))
		}
	}

	// Verify the paired .md was excluded
	for _, f := range files {
		if filepath.Base(f) == "session-abc.md" {
			t.Error("session-abc.md should be excluded (paired with .jsonl)")
		}
	}
}

func TestCollectRecentSessions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sessions directory
	sessionsDir := filepath.Join(tmpDir, ".agents", "ao", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	data1 := map[string]any{"summary": "Auth work"}
	line1, err := json.Marshal(data1)
	if err != nil {
		t.Fatalf("marshal data1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "s1.jsonl"), line1, 0644); err != nil {
		t.Fatal(err)
	}

	data2 := map[string]any{"summary": "Database migration"}
	line2, err := json.Marshal(data2)
	if err != nil {
		t.Fatalf("marshal data2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "s2.jsonl"), line2, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sessionsDir, "s3.md"), []byte("# Summary\n\nWorked on testing"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("collects all sessions", func(t *testing.T) {
		got, err := collectRecentSessions(tmpDir, "", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("got %d sessions, want 3", len(got))
		}
	})

	t.Run("filters by query", func(t *testing.T) {
		got, err := collectRecentSessions(tmpDir, "auth", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d sessions for 'auth', want 1", len(got))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		got, err := collectRecentSessions(tmpDir, "", 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) > 2 {
			t.Errorf("got %d sessions, want at most 2", len(got))
		}
	})

	t.Run("no sessions directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		got, err := collectRecentSessions(emptyDir, "", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestParseMarkdownSessionSummary_FileNotFound(t *testing.T) {
	_, err := parseMarkdownSessionSummary("/nonexistent/path/session.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseMarkdownSessionSummary_WithFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.md")
	content := "---\ndate: 2026-03-10\nid: ag-test\n---\n\n# Session Summary\n\nWorked on auth module refactoring.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := parseMarkdownSessionSummary(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Worked on auth module refactoring." {
		t.Errorf("expected body text, got %q", summary)
	}
}
