package formatter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
)

func TestNewMarkdownFormatter(t *testing.T) {
	mf := NewMarkdownFormatter()
	if mf == nil {
		t.Fatal("NewMarkdownFormatter returned nil")
	}
	// VaultPath and UseWikiLinks depend on environment, so just check they're set consistently
	if mf.VaultPath != "" && !mf.UseWikiLinks {
		t.Error("UseWikiLinks should be true when VaultPath is set")
	}
	if mf.VaultPath == "" && mf.UseWikiLinks {
		t.Error("UseWikiLinks should be false when VaultPath is empty")
	}
}

func TestMarkdownFormatter_Extension(t *testing.T) {
	mf := NewMarkdownFormatter()
	ext := mf.Extension()
	if ext != ".md" {
		t.Errorf("Extension() = %q, want .md", ext)
	}
}

func TestMarkdownFormatter_Format_FullSession(t *testing.T) {
	mf := &MarkdownFormatter{
		VaultPath:    "",
		UseWikiLinks: false, // Force standard markdown links
	}

	session := &storage.Session{
		ID:      "test-session-001",
		Date:    time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC),
		Summary: "Test session with all fields",
		Decisions: []string{
			"Use Go for implementation",
			"Follow TDD approach",
		},
		Knowledge: []string{
			"Learned about context.WithCancel",
			"mutex patterns are important",
		},
		FilesChanged: []string{
			"cmd/main.go",
			"internal/handler.go",
		},
		Issues: []string{"ol-001", "ol-002"},
		ToolCalls: map[string]int{
			"Read":  10,
			"Write": 5,
		},
		Tokens: storage.TokenUsage{
			Input:  1000,
			Output: 500,
			Total:  1500,
		},
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Check frontmatter
	if !strings.Contains(output, "---") {
		t.Error("Output should contain YAML frontmatter delimiters")
	}
	if !strings.Contains(output, "session_id: test-session-001") {
		t.Error("Output should contain session_id in frontmatter")
	}
	if !strings.Contains(output, "date: 2026-01-25") {
		t.Error("Output should contain date in frontmatter")
	}

	// Check heading
	if !strings.Contains(output, "# Test session with all fields") {
		t.Error("Output should contain summary as H1 heading")
	}

	// Check sections
	if !strings.Contains(output, "## Decisions") {
		t.Error("Output should contain Decisions section")
	}
	if !strings.Contains(output, "## Knowledge") {
		t.Error("Output should contain Knowledge section")
	}
	if !strings.Contains(output, "## Files Changed") {
		t.Error("Output should contain Files Changed section")
	}
	if !strings.Contains(output, "## Issues") {
		t.Error("Output should contain Issues section")
	}
	if !strings.Contains(output, "## Tool Usage") {
		t.Error("Output should contain Tool Usage section")
	}
	if !strings.Contains(output, "## Tokens") {
		t.Error("Output should contain Tokens section")
	}

	// Check content items
	if !strings.Contains(output, "- Use Go for implementation") {
		t.Error("Output should contain decision items")
	}
	if !strings.Contains(output, "- Learned about context.WithCancel") {
		t.Error("Output should contain knowledge items")
	}
}

func TestMarkdownFormatter_Format_MinimalSession(t *testing.T) {
	mf := &MarkdownFormatter{UseWikiLinks: false}

	session := &storage.Session{
		ID:      "minimal-session",
		Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Summary: "Minimal session",
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Should have frontmatter and heading
	if !strings.Contains(output, "session_id: minimal-session") {
		t.Error("Output should contain session_id")
	}
	if !strings.Contains(output, "# Minimal session") {
		t.Error("Output should contain summary heading")
	}

	// Should NOT have empty sections
	if strings.Contains(output, "## Decisions") {
		t.Error("Output should not contain empty Decisions section")
	}
	if strings.Contains(output, "## Knowledge") {
		t.Error("Output should not contain empty Knowledge section")
	}
}

func TestMarkdownFormatter_Format_WikiLinks(t *testing.T) {
	mf := &MarkdownFormatter{
		VaultPath:    "/fake/vault",
		UseWikiLinks: true,
	}

	session := &storage.Session{
		ID:           "wiki-test",
		Date:         time.Now(),
		Summary:      "Wiki links test",
		FilesChanged: []string{"src/main.go"},
		Issues:       []string{"ol-123"},
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Files should use wiki link format
	if !strings.Contains(output, "[[src/main.go]]") {
		t.Errorf("Files should use wiki link format, got:\n%s", output)
	}

	// Issues should use wiki link format with display text
	if !strings.Contains(output, "[[issues/ol-123|ol-123]]") {
		t.Errorf("Issues should use wiki link format, got:\n%s", output)
	}
}

func TestMarkdownFormatter_Format_StandardLinks(t *testing.T) {
	mf := &MarkdownFormatter{
		VaultPath:    "",
		UseWikiLinks: false,
	}

	session := &storage.Session{
		ID:           "standard-test",
		Date:         time.Now(),
		Summary:      "Standard links test",
		FilesChanged: []string{"src/main.go"},
		Issues:       []string{"ol-456"},
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// Files should use code format (no links)
	if !strings.Contains(output, "`src/main.go`") {
		t.Errorf("Files should use code format, got:\n%s", output)
	}

	// Issues should use code format
	if !strings.Contains(output, "`ol-456`") {
		t.Errorf("Issues should use code format, got:\n%s", output)
	}
}

func TestMarkdownFormatter_extractTags(t *testing.T) {
	mf := &MarkdownFormatter{}

	session := &storage.Session{
		ID:   "tag-test",
		Date: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
	}

	tags := mf.extractTags(session)

	// Should contain base tags
	foundAgentops := false
	foundSession := false
	foundDateTag := false

	for _, tag := range tags {
		if tag == "agentops" {
			foundAgentops = true
		}
		if tag == "session" {
			foundSession = true
		}
		if tag == "2026-03" {
			foundDateTag = true
		}
	}

	if !foundAgentops {
		t.Error("Tags should contain 'agentops'")
	}
	if !foundSession {
		t.Error("Tags should contain 'session'")
	}
	if !foundDateTag {
		t.Error("Tags should contain date tag '2026-03'")
	}
}

func TestMarkdownFormatter_templateFuncs(t *testing.T) {
	t.Run("link with wiki links", func(t *testing.T) {
		mf := &MarkdownFormatter{UseWikiLinks: true}
		funcs := mf.templateFuncs()
		linkFn := funcs["link"].(func(string, string) string)

		result := linkFn("Display Text", "path/to/file")
		expected := "[[path/to/file|Display Text]]"
		if result != expected {
			t.Errorf("link() = %q, want %q", result, expected)
		}
	})

	t.Run("link without wiki links", func(t *testing.T) {
		mf := &MarkdownFormatter{UseWikiLinks: false}
		funcs := mf.templateFuncs()
		linkFn := funcs["link"].(func(string, string) string)

		result := linkFn("Display Text", "path/to/file")
		expected := "[Display Text](path/to/file)"
		if result != expected {
			t.Errorf("link() = %q, want %q", result, expected)
		}
	})

	t.Run("fileLink with wiki links", func(t *testing.T) {
		mf := &MarkdownFormatter{UseWikiLinks: true}
		funcs := mf.templateFuncs()
		fileLinkFn := funcs["fileLink"].(func(string) string)

		result := fileLinkFn("src/main.go")
		expected := "[[src/main.go]]"
		if result != expected {
			t.Errorf("fileLink() = %q, want %q", result, expected)
		}
	})

	t.Run("fileLink without wiki links", func(t *testing.T) {
		mf := &MarkdownFormatter{UseWikiLinks: false}
		funcs := mf.templateFuncs()
		fileLinkFn := funcs["fileLink"].(func(string) string)

		result := fileLinkFn("src/main.go")
		expected := "`src/main.go`"
		if result != expected {
			t.Errorf("fileLink() = %q, want %q", result, expected)
		}
	})

	t.Run("issueLink with wiki links", func(t *testing.T) {
		mf := &MarkdownFormatter{UseWikiLinks: true}
		funcs := mf.templateFuncs()
		issueLinkFn := funcs["issueLink"].(func(string) string)

		result := issueLinkFn("ol-123")
		expected := "[[issues/ol-123|ol-123]]"
		if result != expected {
			t.Errorf("issueLink() = %q, want %q", result, expected)
		}
	})

	t.Run("hasContent", func(t *testing.T) {
		mf := &MarkdownFormatter{}
		funcs := mf.templateFuncs()
		hasContentFn := funcs["hasContent"].(func([]string) bool)

		if !hasContentFn([]string{"item"}) {
			t.Error("hasContent should return true for non-empty slice")
		}
		if hasContentFn([]string{}) {
			t.Error("hasContent should return false for empty slice")
		}
		if hasContentFn(nil) {
			t.Error("hasContent should return false for nil slice")
		}
	})

	t.Run("hasToolCalls", func(t *testing.T) {
		mf := &MarkdownFormatter{}
		funcs := mf.templateFuncs()
		hasToolCallsFn := funcs["hasToolCalls"].(func(map[string]int) bool)

		if !hasToolCallsFn(map[string]int{"Read": 1}) {
			t.Error("hasToolCalls should return true for non-empty map")
		}
		if hasToolCallsFn(map[string]int{}) {
			t.Error("hasToolCalls should return false for empty map")
		}
		if hasToolCallsFn(nil) {
			t.Error("hasToolCalls should return false for nil map")
		}
	})
}

func TestMarkdownFormatter_Format_SpecialCharacters(t *testing.T) {
	mf := &MarkdownFormatter{UseWikiLinks: false}

	session := &storage.Session{
		ID:      "special-chars",
		Date:    time.Now(),
		Summary: `Test with "quotes" and <html> & unicode: 日本語`,
		Knowledge: []string{
			"Code: `func() { return }`",
			"Markdown: **bold** and *italic*",
		},
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	// The summary should be in the output (may be escaped in frontmatter)
	if !strings.Contains(output, "日本語") {
		t.Error("Output should preserve unicode characters")
	}
}

func TestMarkdownFormatter_Format_TokensEstimated(t *testing.T) {
	mf := &MarkdownFormatter{UseWikiLinks: false}

	session := &storage.Session{
		ID:      "tokens-test",
		Date:    time.Now(),
		Summary: "Tokens test",
		Tokens: storage.TokenUsage{
			Input:     1000,
			Output:    500,
			Total:     1500,
			Estimated: true,
		},
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "(estimated)") {
		t.Error("Output should indicate estimated tokens")
	}
	if !strings.Contains(output, "~1500") {
		t.Error("Output should prefix estimated total with ~")
	}
}

func TestMarkdownFormatter_Format_TemplateParseError(t *testing.T) {
	// Inject an invalid template to exercise the parse-error return path.
	orig := markdownTmplSource
	markdownTmplSource = "{{ .Unclosed"
	t.Cleanup(func() { markdownTmplSource = orig })

	mf := &MarkdownFormatter{UseWikiLinks: false}
	session := &storage.Session{
		ID:      "parse-err",
		Date:    time.Now(),
		Summary: "trigger parse error",
	}

	var buf bytes.Buffer
	err := mf.Format(&buf, session)
	if err == nil {
		t.Fatal("expected error from invalid template, got nil")
	}
	expectedMsg := "parse template:"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), expectedMsg)
	}
}
