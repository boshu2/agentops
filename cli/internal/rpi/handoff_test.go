// practices: [design-by-contract, ai-assisted-dev]
package rpi

import (
	"reflect"
	"testing"
)

func TestUniqueStringsPreserveOrder(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"dedup preserves first-seen order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"drops empty and whitespace-only", []string{"a", "", "  ", "a", "b"}, []string{"a", "b"}},
		{"trims before comparing", []string{" a ", "a", "a "}, []string{"a"}},
		{"empty input", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UniqueStringsPreserveOrder(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UniqueStringsPreserveOrder(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestStripMarkdownFrontmatter(t *testing.T) {
	if got := StripMarkdownFrontmatter("---\ntitle: x\n---\nbody"); got != "body" {
		t.Errorf("frontmatter not stripped: %q", got)
	}
	if got := StripMarkdownFrontmatter("no frontmatter\nbody"); got != "no frontmatter\nbody" {
		t.Errorf("content without frontmatter changed: %q", got)
	}
	// An unterminated frontmatter block is returned unchanged (no closing ---).
	if got := StripMarkdownFrontmatter("---\ntitle: x\nbody"); got != "---\ntitle: x\nbody" {
		t.Errorf("unterminated frontmatter should be unchanged: %q", got)
	}
}

func TestCompiledChecklistSummaryFromContent(t *testing.T) {
	// Frontmatter stripped, headings/Source lines skipped, first 3 bullets joined.
	body := "---\nx: 1\n---\n# Heading\n- first\n- Source: ignore\n- second\n- third\n- fourth"
	got := CompiledChecklistSummaryFromContent("ID1", body)
	want := "ID1 — first | second | third"
	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	// No items -> returns the id unchanged.
	if got := CompiledChecklistSummaryFromContent("ID2", "# only a heading"); got != "ID2" {
		t.Errorf("empty checklist = %q, want ID2", got)
	}
}
