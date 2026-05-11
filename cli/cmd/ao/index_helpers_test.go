// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"testing"
)

// Tests for pure helper functions in index.go

func TestExtractDateFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{name: "date prefix extracted", filename: "2026-01-15-my-learning.md", want: "2026-01-15"},
		{name: "no date prefix returns unknown", filename: "my-learning.md", want: "unknown"},
		{name: "partial date returns unknown", filename: "2026-01.md", want: "unknown"},
		{name: "empty filename returns unknown", filename: "", want: "unknown"},
		{name: "date in middle not matched", filename: "prefix-2026-01-15-suffix.md", want: "unknown"},
		{name: "date at start with extra suffix", filename: "2026-12-31-year-end.md", want: "2026-12-31"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDateFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("extractDateFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestExtractTagsFromFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		fm   map[string]any
		want string
	}{
		{
			name: "missing tags returns empty",
			fm:   map[string]any{"title": "test"},
			want: "",
		},
		{
			name: "slice of tags joined with space",
			fm:   map[string]any{"tags": []any{"go", "testing", "coverage"}},
			want: "go testing coverage",
		},
		{
			name: "string tags in bracket format",
			fm:   map[string]any{"tags": "[go, testing]"},
			want: "go testing",
		},
		{
			name: "plain string tag",
			fm:   map[string]any{"tags": "single-tag"},
			want: "single-tag",
		},
		{
			name: "empty slice returns empty",
			fm:   map[string]any{"tags": []any{}},
			want: "",
		},
		{
			name: "numeric tag uses Sprintf",
			fm:   map[string]any{"tags": 42},
			want: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTagsFromFrontmatter(tt.fm)
			if got != tt.want {
				t.Errorf("extractTagsFromFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanForTable(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "newlines replaced with spaces", input: "hello\nworld", want: "hello world"},
		{name: "pipe characters escaped", input: "a|b|c", want: `a\|b\|c`},
		{name: "multiple spaces normalized", input: "hello   world", want: "hello world"},
		{name: "leading/trailing whitespace trimmed", input: "  hello  ", want: "hello"},
		{name: "combined: newline+pipe", input: "a\n|b", want: `a \|b`},
		{name: "empty string stays empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanForTable(tt.input)
			if got != tt.want {
				t.Errorf("cleanForTable(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string unchanged", input: "", want: ""},
		{name: "lowercase word capitalized", input: "hello", want: "Hello"},
		{name: "already capitalized unchanged", input: "Hello", want: "Hello"},
		{name: "single char capitalized", input: "a", want: "A"},
		{name: "all caps unchanged", input: "HELLO", want: "HELLO"},
		{name: "sentence only first char changed", input: "hello world", want: "Hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := titleCase(tt.input)
			if got != tt.want {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractH1(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "extracts first H1", content: "# My Title\n\nsome content", want: "My Title"},
		{name: "no H1 returns empty", content: "## H2 heading\nsome content", want: ""},
		{name: "empty content returns empty", content: "", want: ""},
		{name: "H1 after other content", content: "intro\n# Title Here\n", want: "Title Here"},
		{name: "hash without space not matched", content: "#NoSpace", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractH1(tt.content)
			if got != tt.want {
				t.Errorf("extractH1(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestSummaryFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "date prefix stripped",
			filename: "2026-01-15-my-learning.md",
			want:     "my-learning",
		},
		{
			name:     "no date prefix keeps name",
			filename: "my-learning.md",
			want:     "my-learning",
		},
		{
			name:     "md extension stripped",
			filename: "just-filename.md",
			want:     "just-filename",
		},
		{
			name:     "date-only file falls back to trimmed name",
			filename: "2026-01-15.md",
			want:     "2026-01-15",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summaryFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("summaryFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
