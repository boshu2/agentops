// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"strings"
	"testing"
)

// FuzzParseContextFromFrontmatter fuzzes the context declaration parser
// that handles both string form (context: fork) and object form.
func FuzzParseContextFromFrontmatter(f *testing.F) {
	// Seed corpus: string form
	f.Add([]byte("context: fork\n"))
	f.Add([]byte("context: isolated\n"))
	f.Add([]byte("context: inherit\n"))

	// Seed corpus: object form
	f.Add([]byte("context:\n  window: fork\n"))
	f.Add([]byte("context:\n  window: inherit\n  intel_scope: full\n"))
	f.Add([]byte("context:\n  window: isolated\n  sections:\n    include:\n      - learnings\n"))
	f.Add([]byte("context:\n  window: fork\n  sections:\n    exclude:\n      - sessions\n  intent:\n    mode: questions\n"))

	// Seed corpus: edge cases
	f.Add([]byte(""))
	f.Add([]byte("no_context_field: true\n"))
	f.Add([]byte("context: invalid_window\n"))
	f.Add([]byte("context:\n"))
	f.Add([]byte("not yaml at all: [[["))
	f.Add([]byte("context: 123\n"))
	f.Add([]byte("context:\n  window: fork\n  intel_scope: invalid\n"))
	f.Add([]byte("---\ncontext: fork\n---\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic — errors are acceptable
		_, _ = parseContextFromFrontmatter(data)
	})
}

func TestFuzzParseContext_SeedCorrectness(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		window string
		hasErr bool
	}{
		{"string form fork", []byte("context: fork\n"), "fork", false},
		{"string form isolated", []byte("context: isolated\n"), "isolated", false},
		{"object form with intel_scope", []byte("context:\n  window: inherit\n  intel_scope: full\n"), "inherit", false},
		{"empty input returns nil decl", []byte(""), "", false},
		{"no context field returns nil decl", []byte("no_context_field: true\n"), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decl, err := parseContextFromFrontmatter(tt.input)
			if tt.hasErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.hasErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tt.window != "" {
				if decl == nil {
					t.Fatal("expected non-nil decl with window")
				}
				if decl.Window != tt.window {
					t.Fatalf("expected window=%q, got %q", tt.window, decl.Window)
				}
			}
			if err == nil && tt.window == "" && decl != nil && decl.Window != "" {
				t.Fatalf("expected nil decl or empty window, got window=%q", decl.Window)
			}
		})
	}
}

// FuzzExtractFrontmatter fuzzes the frontmatter extraction from markdown content.
func FuzzExtractFrontmatter(f *testing.F) {
	// Seed corpus with realistic markdown frontmatter
	f.Add("---\ncontext: fork\n---\n# Skill Title\n")
	f.Add("---\ntitle: test\ncontext:\n  window: inherit\n---\n")
	f.Add("")
	f.Add("no frontmatter here")
	f.Add("---\n---\n")
	f.Add("---\nunclosed frontmatter\n")
	f.Add("---\nkey: value\n---\n---\nmore: stuff\n---\n")
	f.Add("   ---\nkey: value\n---\n")

	f.Fuzz(func(t *testing.T, data string) {
		// Must never panic — errors are acceptable
		_, _ = extractFrontmatter(data)
	})
}

func TestFuzzExtractFrontmatter_SeedCorrectness(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		hasErr   bool
	}{
		{"valid frontmatter", "---\ncontext: fork\n---\n# Title\n", "context: fork", false},
		{"empty frontmatter", "---\n---\n", "", false},
		{"no frontmatter", "no frontmatter here", "", false},
		{"empty string", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := extractFrontmatter(tt.input)
			if tt.hasErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.hasErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.contains != "" && err == nil {
				if !strings.Contains(fm, tt.contains) {
					t.Fatalf("expected frontmatter containing %q, got %q", tt.contains, fm)
				}
			}
		})
	}
}
