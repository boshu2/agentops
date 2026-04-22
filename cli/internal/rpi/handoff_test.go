package rpi

import (
	"strings"
	"testing"
)

func TestUniqueStringsPreserveOrder(t *testing.T) {
	in := []string{"  b ", "a", "b", "", "a", "  c"}
	got := UniqueStringsPreserveOrder(in)
	want := []string{"b", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStripMarkdownFrontmatter(t *testing.T) {
	with := "---\nkey: value\n---\nbody content"
	if got := StripMarkdownFrontmatter(with); got != "body content" {
		t.Errorf("got %q", got)
	}
	without := "body only\nmore"
	if got := StripMarkdownFrontmatter(without); got != without {
		t.Error("no frontmatter should be returned as-is")
	}
	unterminated := "---\nkey: value\n(never closed)"
	if got := StripMarkdownFrontmatter(unterminated); got != unterminated {
		t.Error("unterminated fm should return original")
	}
}

func TestExtractFindingIDs(t *testing.T) {
	text := "found f-2026-04-02-1 and also f-2026-04-02-2, plus f-2026-04-02-1 again. Not a match: f-202-04-02-1"
	got := ExtractFindingIDs(text)
	want := []string{"f-2026-04-02-1", "f-2026-04-02-2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractBulletItemsAfterMarker(t *testing.T) {
	text := `Some text.
Marker:
- item one
- item two
- item one

## Next heading
- ignored`
	got := ExtractBulletItemsAfterMarker(text, "Marker:")
	want := []string{"item one", "item two"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractMarkdownListItemsUnderHeading(t *testing.T) {
	text := `# Title

## Findings

- first
* second
- first

## Other
- not this`
	got := ExtractMarkdownListItemsUnderHeading(text, "## Findings")
	want := []string{"first", "second"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	// Short ASCII
	if got := TruncateRunes("hello", 10); got != "hello" {
		t.Errorf("short: got %q", got)
	}
	// Long ASCII gets truncated
	if got := TruncateRunes("hello world", 5); got != "hello..." {
		t.Errorf("long: got %q", got)
	}
	// Multi-byte
	s := "αβγδε" // 5 runes, 10 bytes
	if got := TruncateRunes(s, 3); got != "αβγ..." {
		t.Errorf("multi-byte: got %q", got)
	}
	// Multi-byte fits -> no change
	if got := TruncateRunes(s, 10); got != s {
		t.Errorf("fits: got %q", got)
	}
}

func TestFormatVerdicts(t *testing.T) {
	if got := FormatVerdicts(nil); got != "" {
		t.Errorf("nil: got %q", got)
	}
	if got := FormatVerdicts(map[string]string{}); got != "" {
		t.Errorf("empty: got %q", got)
	}
	got := FormatVerdicts(map[string]string{"code-review": "PASS", "pre-mortem": "WARN"})
	// Keys are sorted alphabetically
	if !strings.Contains(got, "code-review PASS") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "pre-mortem WARN") {
		t.Errorf("got %q", got)
	}
	// code-review should come before pre-mortem
	if idx1, idx2 := strings.Index(got, "code-review"), strings.Index(got, "pre-mortem"); idx1 > idx2 {
		t.Errorf("keys not sorted: %q", got)
	}
}

func TestRenderHandoffField(t *testing.T) {
	if got := RenderHandoffField("Label", ""); got != "" {
		t.Errorf("empty string: got %q", got)
	}
	if got := RenderHandoffField("Label", "value"); got != "Label: value\n" {
		t.Errorf("got %q", got)
	}
	if got := RenderHandoffField("Items", []string{}); got != "" {
		t.Errorf("empty list: got %q", got)
	}
	if got := RenderHandoffField("Items", []string{"a", "b"}); got != "Items: a, b\n" {
		t.Errorf("got %q", got)
	}
	if got := RenderHandoffField("Bad", 42); got != "" {
		t.Errorf("unsupported type: got %q", got)
	}
}

func TestFieldAllowed(t *testing.T) {
	// Empty list = allow all (backward compat)
	if !FieldAllowed(nil, "any") {
		t.Error("empty list should allow all")
	}
	if !FieldAllowed([]string{}, "any") {
		t.Error("empty slice should allow all")
	}
	// Listed field allowed
	if !FieldAllowed([]string{"findings", "verdicts"}, "findings") {
		t.Error("listed field should be allowed")
	}
	// Unlisted field not allowed
	if FieldAllowed([]string{"findings"}, "other") {
		t.Error("unlisted field should not be allowed")
	}
}

func TestResolveNarrativeCap(t *testing.T) {
	// Explicit positive cap wins
	if got := ResolveNarrativeCap(500, nil); got != 500 {
		t.Errorf("got %d", got)
	}
	// Zero with empty fields -> default 1000
	if got := ResolveNarrativeCap(0, nil); got != 1000 {
		t.Errorf("got %d", got)
	}
	// Zero with non-empty fields -> 0 (omit)
	if got := ResolveNarrativeCap(0, []string{"findings"}); got != 0 {
		t.Errorf("got %d", got)
	}
}

func TestRenderDegradationWarnings(t *testing.T) {
	var sb strings.Builder
	RenderDegradationWarnings(&sb, []int{2, 3})
	out := sb.String()
	if !strings.Contains(out, "Phase 1") {
		t.Errorf("should mention phase-1, got %q", out)
	}
	if !strings.Contains(out, "Phase 2") {
		t.Errorf("should mention phase-2, got %q", out)
	}
}

func TestCompiledChecklistSummaryFromContent(t *testing.T) {
	body := `---
id: x
---
# Heading
Prevent this known failure mode from recurring.

- Do the thing
- Source: somewhere
- Do the other thing
- Yet another item
- This should be dropped`

	got := CompiledChecklistSummaryFromContent("f-2026-04-22-1", body)
	if !strings.HasPrefix(got, "f-2026-04-22-1 —") {
		t.Errorf("should prefix with id, got %q", got)
	}
	if strings.Contains(got, "Source:") {
		t.Errorf("Source line should be filtered, got %q", got)
	}
	if !strings.Contains(got, "Do the thing") {
		t.Errorf("missing first item: %q", got)
	}
}

func TestCompiledChecklistSummaryFromContent_EmptyReturnsID(t *testing.T) {
	if got := CompiledChecklistSummaryFromContent("x-1", ""); got != "x-1" {
		t.Errorf("got %q, want x-1", got)
	}
}
