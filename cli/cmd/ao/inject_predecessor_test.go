// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePredecessorFile_StructuredHandoff(t *testing.T) {
	tmp := t.TempDir()
	handoff := filepath.Join(tmp, "20260224T103000Z-auth-bug.md")
	content := `# Handoff: Auth Token Refresh Bug

## Working On
ag-7abc — Fix auth token refresh returning empty body

## Accomplishments
- Found root cause: token TTL not checked before refresh
- Identified the failing path in auth.go line 145

## Blockers
- Refresh endpoint returns 200 but empty body on expired tokens
- No test coverage for the refresh path

## Next Steps
- Add TTL validation before calling refresh endpoint
- Write test for expired token scenario
`
	if err := os.WriteFile(handoff, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := parsePredecessorFile(handoff)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.WorkingOn == "" {
		t.Error("expected WorkingOn to be populated")
	}
	if ctx.Progress == "" {
		t.Error("expected Progress to be populated")
	}
	if ctx.Blocker == "" {
		t.Error("expected Blocker to be populated")
	}
	if ctx.NextStep == "" {
		t.Error("expected NextStep to be populated")
	}
	if ctx.RawSummary != "" {
		t.Error("expected RawSummary to be empty (structured headers found)")
	}
}

func TestParsePredecessorFile_AutoHandoff(t *testing.T) {
	tmp := t.TempDir()
	handoff := filepath.Join(tmp, "stop-1708793400.md")
	content := `# Session Handoff

Working on the authentication module. Found that token refresh
has a race condition when multiple requests hit simultaneously.

The mutex in middleware/auth.go needs to be upgraded to RWMutex
for the token cache access pattern.
`
	if err := os.WriteFile(handoff, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := parsePredecessorFile(handoff)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	// Auto-handoff has a top-level header but no structured ## sections
	// Should extract raw paragraphs as fallback
	if ctx.RawSummary == "" && ctx.Progress == "" {
		t.Error("expected either Progress or RawSummary to be populated")
	}
}

func TestParsePredecessorFile_ProseOnly(t *testing.T) {
	tmp := t.TempDir()
	handoff := filepath.Join(tmp, "notes.md")
	content := `Made progress on the database migration. Schema v3 is deployed
to staging but needs the foreign key constraint on users.email.

The rollback script works but is slow on tables > 1M rows.

Need to add an index before running in production.
`
	if err := os.WriteFile(handoff, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := parsePredecessorFile(handoff)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.RawSummary == "" {
		t.Error("expected RawSummary for prose-only file")
	}
}

func TestParsePredecessorFile_MissingFile(t *testing.T) {
	ctx := parsePredecessorFile("/nonexistent/path/handoff.md")
	if ctx != nil {
		t.Error("expected nil for missing file")
	}
}

func TestParsePredecessorFile_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	handoff := filepath.Join(tmp, "empty.md")
	if err := os.WriteFile(handoff, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := parsePredecessorFile(handoff)
	if ctx != nil {
		t.Error("expected nil for empty file")
	}
}

func TestDeriveTopicFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"20260224T103000Z-auth-bug.md", "auth bug"},
		{"stop-1708793400.md", "1708793400"},
		{"auto-1708793400.md", "1708793400"},
		{"simple-topic.md", "simple topic"},
	}
	for _, tt := range tests {
		got := deriveTopicFromPath(tt.path)
		if got != tt.want {
			t.Errorf("deriveTopicFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractSections(t *testing.T) {
	content := `# Main Title

## Progress
Did things.

## Blockers
Stuck on X.

## Next Steps
Do Y next.
`
	sections := extractSections(content)
	if len(sections) < 3 {
		t.Errorf("expected at least 3 sections, got %d", len(sections))
	}
	if _, ok := sections["progress"]; !ok {
		t.Error("expected 'progress' section")
	}
	if _, ok := sections["blockers"]; !ok {
		t.Error("expected 'blockers' section")
	}
}

func TestExtractFirstParagraphs(t *testing.T) {
	content := `First paragraph about something.

Second paragraph with more detail.

Third paragraph concludes.

Fourth paragraph should be excluded.
`
	result := extractFirstParagraphs(content, 3)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if len(result) > 500 {
		t.Errorf("result too long: %d chars", len(result))
	}
}

func TestExtractFirstParagraphs_SkipsFrontMatterDelimiters(t *testing.T) {
	// The function skips lines that are exactly "---" (frontmatter delimiters)
	// and lines starting with "#" (headers). Content between delimiters is NOT
	// skipped — that's handled by the caller's parser.
	content := "---\n---\n\n# Heading\n\nFirst real paragraph.\n\nSecond paragraph.\n"
	result := extractFirstParagraphs(content, 2)
	// Each content line appends a trailing space; paragraphs separated by \n.
	// strings.TrimSpace only trims the outer boundaries.
	want := "First real paragraph. \nSecond paragraph."
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}

// ---------------------------------------------------------------------------
// truncatePredecessor
// ---------------------------------------------------------------------------

func TestTruncatePredecessor_UnderBudget(t *testing.T) {
	ctx := &predecessorContext{
		WorkingOn: "small task",
		Progress:  "50% done",
		Blocker:   "none",
		NextStep:  "finish it",
	}
	truncatePredecessor(ctx)
	if ctx.WorkingOn != "small task" {
		t.Errorf("WorkingOn modified: %q", ctx.WorkingOn)
	}
}

func TestTruncatePredecessor_OverBudget(t *testing.T) {
	longText := strings.Repeat("x", 300)
	ctx := &predecessorContext{
		WorkingOn:  longText,
		Progress:   longText,
		Blocker:    longText,
		NextStep:   longText,
		RawSummary: longText,
	}
	truncatePredecessor(ctx)

	if len(ctx.WorkingOn) > 103 {
		t.Errorf("WorkingOn not truncated: len=%d", len(ctx.WorkingOn))
	}
	if len(ctx.Progress) > 253 {
		t.Errorf("Progress not truncated: len=%d", len(ctx.Progress))
	}
	if len(ctx.Blocker) > 203 {
		t.Errorf("Blocker not truncated: len=%d", len(ctx.Blocker))
	}
	if len(ctx.NextStep) > 153 {
		t.Errorf("NextStep not truncated: len=%d", len(ctx.NextStep))
	}
	if len(ctx.RawSummary) > 303 {
		t.Errorf("RawSummary not truncated: len=%d", len(ctx.RawSummary))
	}
}
