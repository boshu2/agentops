// practices: [pragmatic-programmer, design-patterns]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

// ---------------------------------------------------------------------------
// 1. temper.go — parseMarkdownMetadata
// ---------------------------------------------------------------------------

func TestHelper3_parseMarkdownMetadata(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantID   string
		wantMat  types.Maturity
		wantUtil float64
		wantConf float64
		wantSV   int
		wantTemp bool
	}{
		{
			name: "all fields present",
			content: `# Test Artifact
**ID**: ART-001
**Maturity**: candidate
**Utility**: 0.75
**Confidence**: 0.88
**Schema Version**: 3
**Status**: tempered
`,
			wantID:   "ART-001",
			wantMat:  types.MaturityCandidate,
			wantUtil: 0.75,
			wantConf: 0.88,
			wantSV:   3,
			wantTemp: true,
		},
		{
			name:    "empty content",
			content: "",
			wantID:  "",
		},
		{
			name:    "only ID",
			content: "**ID**: X-42\nSome body text.",
			wantID:  "X-42",
		},
		{
			name:     "locked status sets tempered",
			content:  "**Status**: locked",
			wantTemp: true,
		},
		{
			name:     "non-tempered status",
			content:  "**Status**: pending",
			wantTemp: false,
		},
		{
			name: "bullet-list prefix format",
			content: `- **ID**: B-99
- **Maturity**: established
`,
			wantID:  "B-99",
			wantMat: types.MaturityEstablished,
		},
		{
			name:    "colon in field prefix format",
			content: "**ID:** COLON-1",
			wantID:  "COLON-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var meta artifactMetadata
			parseMarkdownMetadata(tt.content, &meta)

			if meta.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", meta.ID, tt.wantID)
			}
			if tt.wantMat != "" && meta.Maturity != tt.wantMat {
				t.Errorf("Maturity = %q, want %q", meta.Maturity, tt.wantMat)
			}
			if tt.wantUtil > 0 && (meta.Utility < tt.wantUtil-0.01 || meta.Utility > tt.wantUtil+0.01) {
				t.Errorf("Utility = %f, want ~%f", meta.Utility, tt.wantUtil)
			}
			if tt.wantConf > 0 && (meta.Confidence < tt.wantConf-0.01 || meta.Confidence > tt.wantConf+0.01) {
				t.Errorf("Confidence = %f, want ~%f", meta.Confidence, tt.wantConf)
			}
			if meta.SchemaVersion != tt.wantSV {
				t.Errorf("SchemaVersion = %d, want %d", meta.SchemaVersion, tt.wantSV)
			}
			if meta.Tempered != tt.wantTemp {
				t.Errorf("Tempered = %v, want %v", meta.Tempered, tt.wantTemp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 1b. temper.go — parseMarkdownField
// ---------------------------------------------------------------------------

func TestHelper3_parseMarkdownField(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		field   string
		wantVal string
		wantOK  bool
	}{
		{
			name:    "standard bold field",
			line:    "**ID**: abc123",
			field:   "ID",
			wantVal: "abc123",
			wantOK:  true,
		},
		{
			name:    "colon inside bold",
			line:    "**ID:** xyz",
			field:   "ID",
			wantVal: "xyz",
			wantOK:  true,
		},
		{
			name:    "bullet-list prefix",
			line:    "- **Status**: active",
			field:   "Status",
			wantVal: "active",
			wantOK:  true,
		},
		{
			name:   "no match",
			line:   "Just a normal line",
			field:  "ID",
			wantOK: false,
		},
		{
			name:   "partial field name mismatch",
			line:   "**Identity**: foo",
			field:  "ID",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := parseMarkdownField(tt.line, tt.field)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && val != tt.wantVal {
				t.Errorf("val = %q, want %q", val, tt.wantVal)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 1c. temper.go — isArtifactFile, isContainedPath
// ---------------------------------------------------------------------------

func TestHelper3_isArtifactFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"foo.md", true},
		{"data.jsonl", true},
		{"script.go", false},
		{"readme.txt", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isArtifactFile(tt.name); got != tt.want {
				t.Errorf("isArtifactFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestHelper3_isContainedPath(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want bool
	}{
		{"same dir", "/a/b", "/a/b", true},
		{"child", "/a/b", "/a/b/c", true},
		{"sibling", "/a/b", "/a/c", false},
		{"parent", "/a/b", "/a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isContainedPath(tt.base, tt.path); got != tt.want {
				t.Errorf("isContainedPath(%q, %q) = %v, want %v", tt.base, tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. store.go — accumulateEntryStats, artifactTypeFromPath, extractTitle,
//    extractKeywords, computeSearchScore, createSearchSnippet,
//    parseBracketedList, splitCSV, parseMemRLMetadata,
//    appendCategoryKeywords, extractCategoryAndTags
// ---------------------------------------------------------------------------

func TestHelper3_accumulateEntryStats(t *testing.T) {
	stats := &IndexStats{
		ByType: make(map[string]int),
	}
	var totalUtility float64
	var utilityCount int
	now := time.Now()

	e1 := IndexEntry{Type: "learning", Utility: 0.8, IndexedAt: now.Add(-2 * time.Hour)}
	e2 := IndexEntry{Type: "pattern", Utility: 0.6, IndexedAt: now.Add(-1 * time.Hour)}
	e3 := IndexEntry{Type: "learning", Utility: 0, IndexedAt: now}

	accumulateEntryStats(stats, e1, &totalUtility, &utilityCount)
	accumulateEntryStats(stats, e2, &totalUtility, &utilityCount)
	accumulateEntryStats(stats, e3, &totalUtility, &utilityCount)

	if stats.TotalEntries != 3 {
		t.Errorf("TotalEntries = %d, want 3", stats.TotalEntries)
	}
	if stats.ByType["learning"] != 2 {
		t.Errorf("ByType[learning] = %d, want 2", stats.ByType["learning"])
	}
	if stats.ByType["pattern"] != 1 {
		t.Errorf("ByType[pattern] = %d, want 1", stats.ByType["pattern"])
	}
	if utilityCount != 2 {
		t.Errorf("utilityCount = %d, want 2", utilityCount)
	}
	if totalUtility < 1.39 || totalUtility > 1.41 {
		t.Errorf("totalUtility = %f, want ~1.4", totalUtility)
	}
	// Oldest = e1
	if !stats.OldestEntry.Equal(e1.IndexedAt) {
		t.Errorf("OldestEntry = %v, want %v", stats.OldestEntry, e1.IndexedAt)
	}
	// Newest = e3
	if !stats.NewestEntry.Equal(e3.IndexedAt) {
		t.Errorf("NewestEntry = %v, want %v", stats.NewestEntry, e3.IndexedAt)
	}
}

func TestHelper3_artifactTypeFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/foo/learnings/bar.md", "learning"},
		{"/foo/patterns/x.md", "pattern"},
		{"/foo/research/r.md", "research"},
		{"/foo/retro/r.md", "retro"},
		{"/foo/candidates/c.jsonl", "candidate"},
		{"/foo/other/o.md", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := artifactTypeFromPath(tt.path); got != tt.want {
				t.Errorf("artifactTypeFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestHelper3_extractTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"heading", "# My Title\n\nContent", "My Title"},
		{"no heading uses first line", "First line\nSecond line", "First line"},
		{"skips frontmatter delimiters", "---\n---\nContent here", "Content here"},
		{"empty content", "", "Untitled"},
		{"long first line gets truncated", strings.Repeat("x", 100), strings.Repeat("x", 77) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.content)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHelper3_extractKeywords(t *testing.T) {
	content := `**Tags**: go, testing, ci
**Keywords**: deployment, config

This is about a pattern: we found a solution: to fix: errors.
`
	kw := extractKeywords(content)
	kwMap := make(map[string]bool)
	for _, k := range kw {
		kwMap[k] = true
	}

	// From metadata lines
	if !kwMap["go"] {
		t.Error("expected keyword 'go'")
	}
	if !kwMap["testing"] {
		t.Error("expected keyword 'testing'")
	}
	// From content patterns
	if !kwMap["pattern"] {
		t.Error("expected keyword 'pattern'")
	}
	if !kwMap["solution"] {
		t.Error("expected keyword 'solution'")
	}
	if !kwMap["fix"] {
		t.Error("expected keyword 'fix'")
	}
}

func TestHelper3_computeSearchScore(t *testing.T) {
	entry := IndexEntry{
		Title:    "Mutex Pattern",
		Content:  "This is about mutex locking in Go",
		Keywords: []string{"mutex", "concurrency"},
		Utility:  0.8,
	}

	// Single term matching title, content, and keyword
	score := computeSearchScore(entry, []string{"mutex"})
	if score <= 0 {
		t.Errorf("expected positive score for matching query, got %f", score)
	}

	// Non-matching
	noScore := computeSearchScore(entry, []string{"database"})
	if noScore != 0 {
		t.Errorf("expected zero score for non-matching query, got %f", noScore)
	}

	// Title match should score higher than content-only match
	titleMatch := computeSearchScore(entry, []string{"mutex"})
	contentOnly := computeSearchScore(entry, []string{"locking"})
	if titleMatch <= contentOnly {
		t.Errorf("title match score (%f) should be > content-only score (%f)", titleMatch, contentOnly)
	}
}

func TestHelper3_createSearchSnippet(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog. This is a longer piece of text that continues on."

	// Exact match found
	snippet := createSearchSnippet(content, "fox", 50)
	if !strings.Contains(strings.ToLower(snippet), "fox") {
		t.Errorf("snippet should contain 'fox', got %q", snippet)
	}

	// No match returns start of content
	snippet2 := createSearchSnippet(content, "zzzznotfound", 30)
	if len(snippet2) == 0 {
		t.Error("expected non-empty snippet for no-match")
	}

	// Short content returned as-is
	snippet3 := createSearchSnippet("short", "short", 100)
	if snippet3 != "short" {
		t.Errorf("expected 'short', got %q", snippet3)
	}
}

func TestHelper3_parseBracketedList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"bracketed", "[a, b, c]", []string{"a", "b", "c"}},
		{"quoted elements no space", `['go','testing']`, []string{"go", "testing"}},
		{"not bracketed", "a, b, c", nil},
		{"empty brackets", "[]", nil},
		{"single element", "[solo]", []string{"solo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBracketedList(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHelper3_splitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"  ", nil},
		{"single", []string{"single"}},
		{", ,x", []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHelper3_parseMemRLMetadata(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantUtil float64
		wantMat  string
	}{
		{
			name:     "standard format",
			content:  "**Utility**: 0.72\n**Maturity**: candidate",
			wantUtil: 0.72,
			wantMat:  "candidate",
		},
		{
			name:     "bullet list format",
			content:  "- **Utility**: 0.95\n- **Maturity**: established",
			wantUtil: 0.95,
			wantMat:  "established",
		},
		{
			name:     "defaults on empty",
			content:  "No metadata here",
			wantUtil: types.InitialUtility,
			wantMat:  "provisional",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, m := parseMemRLMetadata(tt.content)
			if u < tt.wantUtil-0.01 || u > tt.wantUtil+0.01 {
				t.Errorf("utility = %f, want ~%f", u, tt.wantUtil)
			}
			if m != tt.wantMat {
				t.Errorf("maturity = %q, want %q", m, tt.wantMat)
			}
		})
	}
}

func TestHelper3_appendCategoryKeywords(t *testing.T) {
	base := []string{"existing"}
	result := appendCategoryKeywords(base, "Security", []string{"TLS", "", "Auth"})
	// Should have: existing, security, tls, auth (not empty string)
	if len(result) != 4 {
		t.Fatalf("len = %d, want 4; got %v", len(result), result)
	}
	if result[1] != "security" {
		t.Errorf("category not lowered: %q", result[1])
	}
}

func TestHelper3_extractCategoryAndTags(t *testing.T) {
	t.Run("frontmatter", func(t *testing.T) {
		content := "---\ncategory: infra\ntags: [go, cli]\n---\nContent"
		cat, tags := extractCategoryAndTags(content)
		if cat != "infra" {
			t.Errorf("category = %q, want 'infra'", cat)
		}
		if len(tags) < 2 {
			t.Fatalf("expected >= 2 tags, got %v", tags)
		}
	})

	t.Run("markdown metadata", func(t *testing.T) {
		content := "# Title\n**Category**: operations\n**Tags**: deploy, helm"
		cat, tags := extractCategoryAndTags(content)
		if cat != "operations" {
			t.Errorf("category = %q, want 'operations'", cat)
		}
		if len(tags) < 2 {
			t.Fatalf("expected >= 2 tags, got %v", tags)
		}
	})

	t.Run("no metadata", func(t *testing.T) {
		cat, tags := extractCategoryAndTags("Just plain text")
		if cat != "" {
			t.Errorf("category = %q, want empty", cat)
		}
		if len(tags) != 0 {
			t.Errorf("tags = %v, want empty", tags)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. inject_sessions.go — parseSessionFile (with temp files)
// ---------------------------------------------------------------------------

func TestHelper3_parseSessionFile(t *testing.T) {
	t.Run("jsonl session file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.jsonl")
		data := `{"summary":"Fixed the bug in auth module"}`
		if err := os.WriteFile(path, []byte(data+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		s, err := parseSessionFile(path)
		if err != nil {
			t.Fatalf("parseSessionFile: %v", err)
		}
		if s.Summary == "" {
			t.Error("expected non-empty summary")
		}
		if !strings.Contains(s.Summary, "Fixed the bug") {
			t.Errorf("summary = %q, expected to contain 'Fixed the bug'", s.Summary)
		}
		if s.Date == "" {
			t.Error("expected non-empty date")
		}
	})

	t.Run("markdown session file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "session.md")
		content := "# Session Notes\n\nWorked on refactoring the CLI.\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		s, err := parseSessionFile(path)
		if err != nil {
			t.Fatalf("parseSessionFile: %v", err)
		}
		if !strings.Contains(s.Summary, "Worked on refactoring") {
			t.Errorf("summary = %q, expected content paragraph", s.Summary)
		}
	})

	t.Run("empty jsonl file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.jsonl")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		s, err := parseSessionFile(path)
		if err != nil {
			t.Fatalf("parseSessionFile: %v", err)
		}
		if s.Summary != "" {
			t.Errorf("expected empty summary, got %q", s.Summary)
		}
	})

	t.Run("nonexistent file errors", func(t *testing.T) {
		_, err := parseSessionFile("/nonexistent/path/file.jsonl")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

// ---------------------------------------------------------------------------
// 4. inject_patterns.go — extractPatternNameAndDescription,
//    parseFrontmatterBlock, assembleDescriptionFrom, isContentLine,
//    patternMatchesQuery
// ---------------------------------------------------------------------------

func TestHelper3_extractPatternNameAndDescription(t *testing.T) {
	tests := []struct {
		name         string
		lines        []string
		contentStart int
		wantName     string
		wantDesc     string
	}{
		{
			name:         "heading and paragraph",
			lines:        []string{"# Error Handling", "", "Retry transient failures with backoff.", "More details here."},
			contentStart: 0,
			wantName:     "Error Handling",
			wantDesc:     "Retry transient failures with backoff. More details here.",
		},
		{
			name:         "no heading",
			lines:        []string{"This is a description without heading."},
			contentStart: 0,
			wantName:     "",
			wantDesc:     "This is a description without heading.",
		},
		{
			name:         "skip frontmatter",
			lines:        []string{"---", "utility: 0.5", "---", "# Pattern", "", "Desc line."},
			contentStart: 3,
			wantName:     "Pattern",
			wantDesc:     "Desc line.",
		},
		{
			name:         "empty lines only",
			lines:        []string{"", "", ""},
			contentStart: 0,
			wantName:     "",
			wantDesc:     "",
		},
		{
			name:         "heading only no desc",
			lines:        []string{"# Solo Heading"},
			contentStart: 0,
			wantName:     "Solo Heading",
			wantDesc:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc := extractPatternNameAndDescription(tt.lines, tt.contentStart)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if tt.wantDesc != "" && !strings.HasPrefix(desc, tt.wantDesc[:min(len(tt.wantDesc), 30)]) {
				t.Errorf("desc = %q, want prefix %q", desc, tt.wantDesc[:min(len(tt.wantDesc), 30)])
			}
			if tt.wantDesc == "" && desc != "" {
				t.Errorf("desc = %q, want empty", desc)
			}
		})
	}
}

func TestHelper3_parseFrontmatterBlock(t *testing.T) {
	tests := []struct {
		name        string
		lines       []string
		wantStart   int
		wantUtility float64
	}{
		{
			name:        "valid frontmatter",
			lines:       []string{"---", "utility: 0.85", "---", "# Content"},
			wantStart:   3,
			wantUtility: 0.85,
		},
		{
			name:      "no frontmatter",
			lines:     []string{"# Just Content"},
			wantStart: 0,
		},
		{
			name:      "empty lines",
			lines:     []string{},
			wantStart: 0,
		},
		{
			name:        "frontmatter without utility",
			lines:       []string{"---", "title: test", "---", "body"},
			wantStart:   3,
			wantUtility: 0,
		},
		{
			name:        "unclosed frontmatter",
			lines:       []string{"---", "utility: 0.5", "content"},
			wantStart:   0,
			wantUtility: 0.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, utility := parseFrontmatterBlock(tt.lines)
			if start != tt.wantStart {
				t.Errorf("contentStart = %d, want %d", start, tt.wantStart)
			}
			if utility < tt.wantUtility-0.01 || utility > tt.wantUtility+0.01 {
				t.Errorf("utility = %f, want ~%f", utility, tt.wantUtility)
			}
		})
	}
}

func TestHelper3_isContentLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"hello", true},
		{"", false},
		{"# Heading", false},
		{"---", false},
		{"Some paragraph text", true},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isContentLine(tt.line); got != tt.want {
				t.Errorf("isContentLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestHelper3_patternMatchesQuery(t *testing.T) {
	p := pattern{Name: "Error Handling", Description: "retry on transient failures"}

	if !patternMatchesQuery(p, "") {
		t.Error("empty query should match everything")
	}
	if !patternMatchesQuery(p, "error") {
		t.Error("should match 'error' in name")
	}
	if !patternMatchesQuery(p, "retry") {
		t.Error("should match 'retry' in description")
	}
	if patternMatchesQuery(p, "database") {
		t.Error("should not match 'database'")
	}
}

func TestHelper3_assembleDescriptionFrom(t *testing.T) {
	lines := []string{"First line.", "Second line.", "# Heading"}
	desc := assembleDescriptionFrom(lines, 0)
	if !strings.Contains(desc, "First line.") {
		t.Error("expected first line in description")
	}
	if !strings.Contains(desc, "Second line.") {
		t.Error("expected continuation line")
	}

	// Should stop at heading
	lines2 := []string{"Start.", "# Stop here"}
	desc2 := assembleDescriptionFrom(lines2, 0)
	if strings.Contains(desc2, "Stop") {
		t.Error("should not include heading in description")
	}
}

// ---------------------------------------------------------------------------
// 5. ratchet_validate.go — formatValidationResult, formatValidationStatus,
//    formatLenientInfo, formatStringList
// ---------------------------------------------------------------------------

func TestHelper3_formatValidationResult(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		var buf bytes.Buffer
		result := &ratchet.ValidationResult{
			Valid: true,
		}
		allValid := true
		formatValidationResult(&buf, "test.md", result, &allValid)
		output := buf.String()

		if !strings.Contains(output, "test.md") {
			t.Error("output should contain filename")
		}
		if !strings.Contains(output, "VALID") {
			t.Error("output should contain VALID")
		}
		if !allValid {
			t.Error("allValid should still be true")
		}
	})

	t.Run("invalid result with issues", func(t *testing.T) {
		var buf bytes.Buffer
		result := &ratchet.ValidationResult{
			Valid:    false,
			Issues:   []string{"missing required section", "bad format"},
			Warnings: []string{"deprecated field"},
		}
		allValid := true
		formatValidationResult(&buf, "broken.md", result, &allValid)
		output := buf.String()

		if !strings.Contains(output, "INVALID") {
			t.Error("output should contain INVALID")
		}
		if allValid {
			t.Error("allValid should be false after invalid result")
		}
		if !strings.Contains(output, "missing required section") {
			t.Error("output should contain issue text")
		}
		if !strings.Contains(output, "deprecated field") {
			t.Error("output should contain warning text")
		}
	})

	t.Run("with tier", func(t *testing.T) {
		var buf bytes.Buffer
		tier := ratchet.TierLearning
		result := &ratchet.ValidationResult{
			Valid: true,
			Tier:  &tier,
		}
		allValid := true
		formatValidationResult(&buf, "tiered.md", result, &allValid)
		output := buf.String()

		if !strings.Contains(output, "Tier:") {
			t.Error("output should contain Tier info")
		}
	})

	t.Run("lenient mode", func(t *testing.T) {
		var buf bytes.Buffer
		expiry := "2026-06-01"
		result := &ratchet.ValidationResult{
			Valid:               true,
			Lenient:             true,
			LenientExpiryDate:   &expiry,
			LenientExpiringSoon: true,
		}
		allValid := true
		formatValidationResult(&buf, "legacy.md", result, &allValid)
		output := buf.String()

		if !strings.Contains(output, "LENIENT") {
			t.Error("output should contain LENIENT mode indicator")
		}
		if !strings.Contains(output, "2026-06-01") {
			t.Error("output should contain expiry date")
		}
		if !strings.Contains(output, "Expiring soon") {
			t.Error("output should contain expiring soon warning")
		}
	})
}

func TestHelper3_formatStringList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		var buf bytes.Buffer
		formatStringList(&buf, "Issues", nil)
		if buf.Len() != 0 {
			t.Error("empty list should produce no output")
		}
	})

	t.Run("non-empty list", func(t *testing.T) {
		var buf bytes.Buffer
		formatStringList(&buf, "Warnings", []string{"warn1", "warn2"})
		output := buf.String()
		if !strings.Contains(output, "Warnings:") {
			t.Error("should contain label")
		}
		if !strings.Contains(output, "warn1") || !strings.Contains(output, "warn2") {
			t.Error("should contain all items")
		}
	})
}

// ---------------------------------------------------------------------------
// 6. plans.go — filterPlans, resolvePlanName, buildBeadsIDIndex,
//    syncEpicStatus, countUnlinkedEntries, buildBeadsStatusIndex,
//    detectOrphanedEntries, computePlanChecksum
// ---------------------------------------------------------------------------

func TestHelper3_filterPlans(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "plan-a", ProjectPath: "/proj/alpha", Status: types.PlanStatusActive},
		{PlanName: "plan-b", ProjectPath: "/proj/beta", Status: types.PlanStatusCompleted},
		{PlanName: "plan-c", ProjectPath: "/proj/alpha", Status: types.PlanStatusCompleted},
	}

	t.Run("no filter", func(t *testing.T) {
		got := filterPlans(entries, "", "")
		if len(got) != 3 {
			t.Errorf("len = %d, want 3", len(got))
		}
	})

	t.Run("filter by project", func(t *testing.T) {
		got := filterPlans(entries, "alpha", "")
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		got := filterPlans(entries, "", "completed")
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("filter by both", func(t *testing.T) {
		got := filterPlans(entries, "alpha", "completed")
		if len(got) != 1 {
			t.Errorf("len = %d, want 1", len(got))
		}
		if got[0].PlanName != "plan-c" {
			t.Errorf("PlanName = %q, want 'plan-c'", got[0].PlanName)
		}
	})
}

func TestHelper3_resolvePlanName(t *testing.T) {
	if got := resolvePlanName("explicit", "/some/path/file.md"); got != "explicit" {
		t.Errorf("resolvePlanName = %q, want 'explicit'", got)
	}
	if got := resolvePlanName("", "/plans/peaceful-stirring-tome.md"); got != "peaceful-stirring-tome" {
		t.Errorf("resolvePlanName = %q, want 'peaceful-stirring-tome'", got)
	}
}

func TestHelper3_buildBeadsIDIndex(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "a", BeadsID: "ol-001"},
		{PlanName: "b", BeadsID: ""},
		{PlanName: "c", BeadsID: "ol-003"},
	}
	idx := buildBeadsIDIndex(entries)
	if len(idx) != 2 {
		t.Fatalf("len = %d, want 2", len(idx))
	}
	if idx["ol-001"] != 0 {
		t.Errorf("ol-001 -> %d, want 0", idx["ol-001"])
	}
	if idx["ol-003"] != 2 {
		t.Errorf("ol-003 -> %d, want 2", idx["ol-003"])
	}
}

func TestHelper3_syncEpicStatus(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "plan", Status: types.PlanStatusActive},
	}

	// Active -> closed => should change
	changed := syncEpicStatus(entries, 0, "closed")
	if !changed {
		t.Error("expected change from active -> completed")
	}
	if entries[0].Status != types.PlanStatusCompleted {
		t.Errorf("status = %q, want 'completed'", entries[0].Status)
	}

	// Already completed, closed again => no change
	changed2 := syncEpicStatus(entries, 0, "closed")
	if changed2 {
		t.Error("expected no change when already completed")
	}

	// Completed -> open => should change back
	changed3 := syncEpicStatus(entries, 0, "open")
	if !changed3 {
		t.Error("expected change from completed -> active")
	}
	if entries[0].Status != types.PlanStatusActive {
		t.Errorf("status = %q, want 'active'", entries[0].Status)
	}
}

func TestHelper3_countUnlinkedEntries(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "a", BeadsID: "ol-1"},
		{PlanName: "b", BeadsID: ""},
		{PlanName: "c", BeadsID: ""},
	}
	if got := countUnlinkedEntries(entries); got != 2 {
		t.Errorf("countUnlinkedEntries = %d, want 2", got)
	}
}

func TestHelper3_buildBeadsStatusIndex(t *testing.T) {
	epics := []beadsEpic{
		{ID: "e1", Status: "open"},
		{ID: "e2", Status: "closed"},
	}
	idx := buildBeadsStatusIndex(epics)
	if idx["e1"] != "open" {
		t.Errorf("e1 = %q, want 'open'", idx["e1"])
	}
	if idx["e2"] != "closed" {
		t.Errorf("e2 = %q, want 'closed'", idx["e2"])
	}
}

func TestHelper3_detectOrphanedEntries(t *testing.T) {
	entries := []types.PlanManifestEntry{
		{PlanName: "linked", BeadsID: "x"},
		{PlanName: "orphan1", BeadsID: ""},
		{PlanName: "orphan2", BeadsID: ""},
	}
	drifts := detectOrphanedEntries(entries)
	if len(drifts) != 2 {
		t.Fatalf("len = %d, want 2", len(drifts))
	}
	for _, d := range drifts {
		if d.Type != "orphaned" {
			t.Errorf("Type = %q, want 'orphaned'", d.Type)
		}
	}
}

func TestHelper3_computePlanChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(path, []byte("# Test Plan\n\nContent here."), 0644); err != nil {
		t.Fatal(err)
	}
	checksum, err := computePlanChecksum(path)
	if err != nil {
		t.Fatalf("computePlanChecksum: %v", err)
	}
	if len(checksum) != 16 { // 8 bytes hex = 16 chars
		t.Errorf("checksum len = %d, want 16", len(checksum))
	}

	// Same content gives same checksum
	checksum2, _ := computePlanChecksum(path)
	if checksum != checksum2 {
		t.Error("same content should produce same checksum")
	}

	// Different content gives different checksum
	path2 := filepath.Join(dir, "plan2.md")
	if err := os.WriteFile(path2, []byte("Different content"), 0644); err != nil {
		t.Fatal(err)
	}
	checksum3, _ := computePlanChecksum(path2)
	if checksum == checksum3 {
		t.Error("different content should produce different checksum")
	}
}

// ---------------------------------------------------------------------------
// 7. status.go — truncateStatus, firstLine, formatDurationBrief
// ---------------------------------------------------------------------------

func TestHelper3_truncateStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "12345", 5, "12345"},
		{"truncated", "this is a long string that needs truncation", 20, "this is a long st..."},
		{"multiline takes first", "line1\nline2\nline3", 50, "line1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStatus(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStatus(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestHelper3_firstLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"single line", "single line"},
		{"first\nsecond", "first"},
		{"", ""},
		{"\nstarts with newline", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := firstLine(tt.input); got != tt.want {
				t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHelper3_formatDurationBrief(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"sub-minute", 30 * time.Second, "<1m"},
		{"minutes", 45 * time.Minute, "45m"},
		{"hours", 5 * time.Hour, "5h"},
		{"days", 3 * 24 * time.Hour, "3d"},
		{"weeks", 60 * 24 * time.Hour, "8w"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDurationBrief(tt.d); got != tt.want {
				t.Errorf("formatDurationBrief(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 8. rpi_cleanup.go — findStaleRunsWithMinAge using t.TempDir()
// ---------------------------------------------------------------------------

func TestHelper3_findStaleRunsWithMinAge(t *testing.T) {
	tmpDir := t.TempDir()
	runsDir := filepath.Join(tmpDir, ".agents", "rpi", "runs")

	// Create a stale run (worktree missing, non-terminal)
	staleRunDir := filepath.Join(runsDir, "stale-1")
	if err := os.MkdirAll(staleRunDir, 0755); err != nil {
		t.Fatal(err)
	}
	staleState := map[string]any{
		"schema_version": 1,
		"run_id":         "stale-1",
		"goal":           "test stale run",
		"phase":          2,
		"worktree_path":  "/nonexistent/worktree",
		"started_at":     time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
	}
	staleData, _ := json.Marshal(staleState)
	if err := os.WriteFile(filepath.Join(staleRunDir, phasedStateFile), staleData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a completed run (terminal, should be excluded)
	completedRunDir := filepath.Join(runsDir, "completed-1")
	if err := os.MkdirAll(completedRunDir, 0755); err != nil {
		t.Fatal(err)
	}
	completedState := map[string]any{
		"schema_version":  1,
		"run_id":          "completed-1",
		"goal":            "test completed run",
		"phase":           3,
		"started_at":      time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
		"terminal_status": "completed",
	}
	completedData, _ := json.Marshal(completedState)
	if err := os.WriteFile(filepath.Join(completedRunDir, phasedStateFile), completedData, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("no min age finds stale", func(t *testing.T) {
		stale := findStaleRunsWithMinAge(tmpDir, 0, time.Now())
		found := false
		for _, sr := range stale {
			if sr.RunID == "stale-1" {
				found = true
			}
			if sr.RunID == "completed-1" {
				t.Error("completed runs should not appear as stale")
			}
		}
		if !found {
			t.Error("expected to find stale-1 in results")
		}
	})

	t.Run("min age filters recent", func(t *testing.T) {
		// Use minAge of 10 hours - the stale run is only 2h old
		stale := findStaleRunsWithMinAge(tmpDir, 10*time.Hour, time.Now())
		for _, sr := range stale {
			if sr.RunID == "stale-1" {
				t.Error("stale-1 is only 2h old, should be filtered with 10h minAge")
			}
		}
	})

	t.Run("empty runs dir", func(t *testing.T) {
		emptyDir := t.TempDir()
		stale := findStaleRunsWithMinAge(emptyDir, 0, time.Now())
		if len(stale) != 0 {
			t.Errorf("expected 0 stale runs for empty dir, got %d", len(stale))
		}
	})
}

// ---------------------------------------------------------------------------
// 9. pool_migrate_legacy.go — nextLegacyDestination
// ---------------------------------------------------------------------------

func TestHelper3_nextLegacyDestination(t *testing.T) {
	t.Run("no collision", func(t *testing.T) {
		dir := t.TempDir()
		got, err := nextLegacyDestination(dir, "capture.md")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "capture.md" {
			t.Errorf("base = %q, want 'capture.md'", filepath.Base(got))
		}
	})

	t.Run("collision increments suffix", func(t *testing.T) {
		dir := t.TempDir()
		// Create the existing file to cause collision
		existing := filepath.Join(dir, "capture.md")
		if err := os.WriteFile(existing, []byte("exists"), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := nextLegacyDestination(dir, "capture.md")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "capture-migrated-1.md" {
			t.Errorf("base = %q, want 'capture-migrated-1.md'", filepath.Base(got))
		}
	})

	t.Run("multiple collisions", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "data.md"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data-migrated-1.md"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := nextLegacyDestination(dir, "data.md")
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(got) != "data-migrated-2.md" {
			t.Errorf("base = %q, want 'data-migrated-2.md'", filepath.Base(got))
		}
	})
}

// ---------------------------------------------------------------------------
// 10. session_close.go — computeVelocityDelta, classifyFlywheelStatus,
//     shortenPath
// ---------------------------------------------------------------------------

func TestHelper3_computeVelocityDelta(t *testing.T) {
	tests := []struct {
		name string
		pre  *types.FlywheelMetrics
		post *types.FlywheelMetrics
		want float64
	}{
		{
			name: "both present",
			pre:  &types.FlywheelMetrics{Velocity: 0.1},
			post: &types.FlywheelMetrics{Velocity: 0.3},
			want: 0.2,
		},
		{
			name: "pre nil",
			pre:  nil,
			post: &types.FlywheelMetrics{Velocity: 0.5},
			want: 0.0,
		},
		{
			name: "post nil",
			pre:  &types.FlywheelMetrics{Velocity: 0.5},
			post: nil,
			want: 0.0,
		},
		{
			name: "both nil",
			pre:  nil,
			post: nil,
			want: 0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeVelocityDelta(tt.pre, tt.post)
			if got < tt.want-0.001 || got > tt.want+0.001 {
				t.Errorf("computeVelocityDelta() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestHelper3_classifyFlywheelStatus(t *testing.T) {
	tests := []struct {
		name string
		post *types.FlywheelMetrics
		want string
	}{
		{"nil returns unknown", nil, "unknown"},
		{"above escape velocity", &types.FlywheelMetrics{AboveEscapeVelocity: true}, "compounding"},
		{"near escape", &types.FlywheelMetrics{Velocity: -0.01}, "accumulating"},
		{"decaying", &types.FlywheelMetrics{Velocity: -0.1}, "decaying"},
		{"exactly threshold", &types.FlywheelMetrics{Velocity: -0.05}, "decaying"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyFlywheelStatus(tt.post)
			if got != tt.want {
				t.Errorf("classifyFlywheelStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 11. task_sync.go — statusToMaturity, extractContentBlocks,
//     filterProcessableTasks, filterTasksBySession,
//     computeTaskDistributions, assignMaturityAndUtility,
//     processTranscriptLine, updateTask, parseTaskCreate
// ---------------------------------------------------------------------------

func TestHelper3_statusToMaturity(t *testing.T) {
	tests := []struct {
		status string
		want   types.Maturity
	}{
		{"completed", types.MaturityEstablished},
		{"in_progress", types.MaturityCandidate},
		{"pending", types.MaturityProvisional},
		{"unknown", types.MaturityProvisional},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusToMaturity(tt.status)
			if got != tt.want {
				t.Errorf("statusToMaturity(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestHelper3_extractContentBlocks(t *testing.T) {
	t.Run("has tool_use blocks", func(t *testing.T) {
		data := map[string]any{
			"message": map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
					map[string]any{"type": "tool_use", "name": "TaskCreate", "input": map[string]any{}},
					map[string]any{"type": "tool_use", "name": "TaskUpdate", "input": map[string]any{}},
				},
			},
		}
		blocks := extractContentBlocks(data)
		if len(blocks) != 2 {
			t.Fatalf("expected 2 tool_use blocks, got %d", len(blocks))
		}
	})

	t.Run("no message", func(t *testing.T) {
		data := map[string]any{"other": "value"}
		blocks := extractContentBlocks(data)
		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks for no message, got %d", len(blocks))
		}
	})

	t.Run("no content", func(t *testing.T) {
		data := map[string]any{"message": map[string]any{}}
		blocks := extractContentBlocks(data)
		if len(blocks) != 0 {
			t.Errorf("expected 0 blocks for no content, got %d", len(blocks))
		}
	})
}

func TestHelper3_filterProcessableTasks(t *testing.T) {
	tasks := []TaskEvent{
		{TaskID: "t1", Status: "completed", LearningID: "L-1", SessionID: "s1"},
		{TaskID: "t2", Status: "completed", LearningID: "", SessionID: "s1"},
		{TaskID: "t3", Status: "pending", LearningID: "L-3", SessionID: "s2"},
		{TaskID: "t4", Status: "completed", LearningID: "L-4", SessionID: "s2"},
	}

	t.Run("no session filter", func(t *testing.T) {
		got := filterProcessableTasks(tasks, "")
		if len(got) != 2 {
			t.Errorf("expected 2 processable tasks, got %d", len(got))
		}
	})

	t.Run("with session filter", func(t *testing.T) {
		got := filterProcessableTasks(tasks, "s1")
		if len(got) != 1 {
			t.Errorf("expected 1 processable task for s1, got %d", len(got))
		}
		if got[0].TaskID != "t1" {
			t.Errorf("expected t1, got %s", got[0].TaskID)
		}
	})
}

func TestHelper3_filterTasksBySession(t *testing.T) {
	tasks := []TaskEvent{
		{TaskID: "t1", SessionID: "s1"},
		{TaskID: "t2", SessionID: "s2"},
		{TaskID: "t3", SessionID: "s1"},
	}

	t.Run("empty filter returns all", func(t *testing.T) {
		got := filterTasksBySession(tasks, "")
		if len(got) != 3 {
			t.Errorf("expected 3, got %d", len(got))
		}
	})

	t.Run("specific session", func(t *testing.T) {
		got := filterTasksBySession(tasks, "s1")
		if len(got) != 2 {
			t.Errorf("expected 2 for s1, got %d", len(got))
		}
	})
}

func TestHelper3_computeTaskDistributions(t *testing.T) {
	tasks := []TaskEvent{
		{Status: "pending", Maturity: types.MaturityProvisional, LearningID: ""},
		{Status: "completed", Maturity: types.MaturityEstablished, LearningID: "L-1"},
		{Status: "completed", Maturity: types.MaturityEstablished, LearningID: "L-2"},
		{Status: "in_progress", Maturity: types.MaturityCandidate, LearningID: ""},
	}

	statusCounts, maturityCounts, withLearnings := computeTaskDistributions(tasks)

	if statusCounts["pending"] != 1 {
		t.Errorf("pending = %d, want 1", statusCounts["pending"])
	}
	if statusCounts["completed"] != 2 {
		t.Errorf("completed = %d, want 2", statusCounts["completed"])
	}
	if maturityCounts[types.MaturityEstablished] != 2 {
		t.Errorf("established = %d, want 2", maturityCounts[types.MaturityEstablished])
	}
	if withLearnings != 2 {
		t.Errorf("withLearnings = %d, want 2", withLearnings)
	}
}

func TestHelper3_assignMaturityAndUtility(t *testing.T) {
	tasks := []TaskEvent{
		{Status: "completed", Utility: 0},
		{Status: "pending", Utility: 0.9},
	}

	assignMaturityAndUtility(tasks)

	if tasks[0].Maturity != types.MaturityEstablished {
		t.Errorf("tasks[0].Maturity = %q, want established", tasks[0].Maturity)
	}
	if tasks[0].Utility != types.InitialUtility {
		t.Errorf("tasks[0].Utility = %f, want %f", tasks[0].Utility, types.InitialUtility)
	}
	// Second task already had utility
	if tasks[1].Utility != 0.9 {
		t.Errorf("tasks[1].Utility = %f, want 0.9", tasks[1].Utility)
	}
}

func TestHelper3_processTranscriptLine(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)

	// Line with sessionId and TaskCreate
	line := `{"sessionId":"sess-abc","message":{"content":[{"type":"tool_use","name":"TaskCreate","input":{"subject":"Fix bug"}}]}}`
	sid := processTranscriptLine(line, "", "", taskMap)

	if sid != "sess-abc" {
		t.Errorf("sessionID = %q, want 'sess-abc'", sid)
	}
	if len(taskMap) != 1 {
		t.Fatalf("expected 1 task in map, got %d", len(taskMap))
	}
	for _, task := range taskMap {
		if task.Subject != "Fix bug" {
			t.Errorf("Subject = %q, want 'Fix bug'", task.Subject)
		}
		if task.SessionID != "sess-abc" {
			t.Errorf("SessionID = %q, want 'sess-abc'", task.SessionID)
		}
	}

	t.Run("filtered out by session", func(t *testing.T) {
		taskMap2 := make(map[string]*TaskEvent)
		processTranscriptLine(line, "other-session", "", taskMap2)
		if len(taskMap2) != 0 {
			t.Errorf("expected 0 tasks when filtered, got %d", len(taskMap2))
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		taskMap3 := make(map[string]*TaskEvent)
		sid2 := processTranscriptLine("not json", "", "prev-sid", taskMap3)
		if sid2 != "prev-sid" {
			t.Errorf("expected prev-sid preserved, got %q", sid2)
		}
	})
}

func TestHelper3_updateTask(t *testing.T) {
	task := &TaskEvent{
		TaskID:  "t1",
		Subject: "original",
		Status:  "pending",
	}

	input := map[string]any{
		"status":      "completed",
		"subject":     "updated subject",
		"description": "new desc",
		"owner":       "agent-1",
	}

	updateTask(task, input)

	if task.Status != "completed" {
		t.Errorf("Status = %q, want 'completed'", task.Status)
	}
	if task.Subject != "updated subject" {
		t.Errorf("Subject = %q, want 'updated subject'", task.Subject)
	}
	if task.Description != "new desc" {
		t.Errorf("Description = %q, want 'new desc'", task.Description)
	}
	if task.Owner != "agent-1" {
		t.Errorf("Owner = %q, want 'agent-1'", task.Owner)
	}
	if task.Maturity != types.MaturityEstablished {
		t.Errorf("Maturity = %q, want 'established'", task.Maturity)
	}
	if task.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set for completed status")
	}
}

func TestHelper3_parseTaskCreate(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		input := map[string]any{
			"subject":     "Implement feature X",
			"description": "Build the thing",
			"activeForm":  "checkbox",
		}
		task := parseTaskCreate(input, "sess-1")
		if task == nil {
			t.Fatal("expected non-nil task")
		}
		if task.Subject != "Implement feature X" {
			t.Errorf("Subject = %q", task.Subject)
		}
		if task.Description != "Build the thing" {
			t.Errorf("Description = %q", task.Description)
		}
		if task.SessionID != "sess-1" {
			t.Errorf("SessionID = %q", task.SessionID)
		}
		if task.Metadata["active_form"] != "checkbox" {
			t.Errorf("Metadata[active_form] = %v", task.Metadata["active_form"])
		}
		if task.Status != "pending" {
			t.Errorf("Status = %q, want 'pending'", task.Status)
		}
	})

	t.Run("empty subject returns nil", func(t *testing.T) {
		input := map[string]any{"description": "no subject"}
		task := parseTaskCreate(input, "sess-1")
		if task != nil {
			t.Error("expected nil for empty subject")
		}
	})
}

// ---------------------------------------------------------------------------
// 11b. task_sync.go — applyToolBlock
// ---------------------------------------------------------------------------

func TestHelper3_applyToolBlock(t *testing.T) {
	taskMap := make(map[string]*TaskEvent)

	// TaskCreate
	createBlock := map[string]any{
		"name": "TaskCreate",
		"input": map[string]any{
			"subject": "New task",
		},
	}
	applyToolBlock(createBlock, "sess-1", taskMap)
	if len(taskMap) != 1 {
		t.Fatalf("expected 1 task after TaskCreate, got %d", len(taskMap))
	}

	// Get the task ID
	var taskID string
	for id := range taskMap {
		taskID = id
	}

	// TaskUpdate
	updateBlock := map[string]any{
		"name": "TaskUpdate",
		"input": map[string]any{
			"taskId": taskID,
			"status": "in_progress",
		},
	}
	applyToolBlock(updateBlock, "sess-1", taskMap)
	if taskMap[taskID].Status != "in_progress" {
		t.Errorf("Status = %q, want 'in_progress'", taskMap[taskID].Status)
	}

	// Unknown tool name - should be a no-op
	unknownBlock := map[string]any{
		"name":  "UnknownTool",
		"input": map[string]any{},
	}
	applyToolBlock(unknownBlock, "sess-1", taskMap)
	if len(taskMap) != 1 {
		t.Errorf("expected no change after unknown tool, got %d tasks", len(taskMap))
	}
}
