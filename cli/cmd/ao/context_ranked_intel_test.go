// practices: [ai-assisted-dev, distributed-tracing]
package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// findingBullets
// ---------------------------------------------------------------------------

func TestFindingBullets_Empty(t *testing.T) {
	got := findingBullets(nil)
	if len(got) != 0 {
		t.Errorf("findingBullets(nil) = %v, want empty", got)
	}
}

func TestFindingBullets_WithSeverity(t *testing.T) {
	findings := []knowledgeFinding{
		{ID: "F-001", Title: "Auth bypass", Summary: "Token validation skipped", Severity: "high"},
		{ID: "F-002", Title: "Slow query", Summary: "N+1 detected in handler", Severity: "medium"},
	}

	got := findingBullets(findings)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// First bullet should contain uppercase severity prefix
	if got[0] == "" {
		t.Error("first bullet is empty")
	}
	if got[0][:6] != "[HIGH]" {
		t.Errorf("first bullet = %q, want [HIGH] prefix", got[0])
	}
	if got[1][:8] != "[MEDIUM]" {
		t.Errorf("second bullet = %q, want [MEDIUM] prefix", got[1])
	}
}

func TestFindingBullets_NoSeverity(t *testing.T) {
	findings := []knowledgeFinding{
		{ID: "F-003", Title: "Minor note", Summary: "Just a note"},
	}

	got := findingBullets(findings)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	// Should not have a severity prefix
	if len(got[0]) > 0 && got[0][0] == '[' {
		t.Errorf("bullet = %q, should not have severity prefix for empty severity", got[0])
	}
}

func TestFindingBullets_FallbackToID(t *testing.T) {
	findings := []knowledgeFinding{
		{ID: "F-004"},
	}

	got := findingBullets(findings)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0] == "" {
		t.Error("bullet should not be empty; should use ID as fallback")
	}
}

// ---------------------------------------------------------------------------
// joinBullet
// ---------------------------------------------------------------------------

func TestJoinBullet(t *testing.T) {
	tests := []struct {
		title, details, want string
	}{
		{"Title", "Details here", "Title - Details here"},
		{"Title", "", "Title"},
		{"", "Details", "Details"},
		{"", "", ""},
	}
	for _, tt := range tests {
		got := joinBullet(tt.title, tt.details)
		if got != tt.want {
			t.Errorf("joinBullet(%q, %q) = %q, want %q", tt.title, tt.details, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// firstNonEmpty
// ---------------------------------------------------------------------------

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		values []string
		want   string
	}{
		{[]string{"", "second"}, "second"},
		{[]string{"first", "second"}, "first"},
		{[]string{"", "", ""}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		got := firstNonEmpty(tt.values...)
		if got != tt.want {
			t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// compactText
// ---------------------------------------------------------------------------

func TestCompactText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello world"},
		{"  extra   spaces  ", "extra spaces"},
		{"line1\nline2\nline3", "line1 line2 line3"},
		{"", ""},
	}
	for _, tt := range tests {
		got := compactText(tt.input)
		if got != tt.want {
			t.Errorf("compactText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// patternBullets
// ---------------------------------------------------------------------------

func TestPatternBullets(t *testing.T) {
	items := []pattern{
		{Name: "Auth retry", Description: "Retries on 401"},
		{Name: "", FilePath: "/tmp/.agents/patterns/fallback.md", Description: "Fallback name"},
	}
	got := patternBullets(items)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] == "" {
		t.Error("first bullet is empty")
	}
	if got[1] == "" {
		t.Error("second bullet is empty")
	}
}

func TestPatternBullets_Empty(t *testing.T) {
	got := patternBullets(nil)
	if len(got) != 0 {
		t.Errorf("patternBullets(nil) = %v, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// nextWorkBullets
// ---------------------------------------------------------------------------

func TestNextWorkBullets(t *testing.T) {
	items := []nextWorkItem{
		{Title: "Fix auth", Description: "Token expired", Severity: "high"},
		{Title: "", Type: "refactor", Evidence: "CC=25", Severity: ""},
	}
	got := nextWorkBullets(items)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] == "" {
		t.Error("first bullet is empty")
	}
}

func TestNextWorkBullets_Empty(t *testing.T) {
	got := nextWorkBullets(nil)
	if len(got) != 0 {
		t.Errorf("nextWorkBullets(nil) = %v, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// sessionBullets
// ---------------------------------------------------------------------------

func TestSessionBullets(t *testing.T) {
	items := []session{
		{Date: "2026-04-01", Summary: "Worked on auth"},
		{Date: "2026-04-02", Summary: "  Multiline\nsummary  "},
	}
	got := sessionBullets(items)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestSessionBullets_Empty(t *testing.T) {
	got := sessionBullets(nil)
	if len(got) != 0 {
		t.Errorf("sessionBullets(nil) = %v, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// researchBullets
// ---------------------------------------------------------------------------

func TestResearchBullets(t *testing.T) {
	items := []codexArtifactRef{
		{Title: "Helm analysis", Path: "/tmp/research/helm.md", ModifiedAt: "2026-04-01"},
		{Title: "", Path: "/tmp/research/untitled.md"},
	}
	got := researchBullets(items)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestResearchBullets_Empty(t *testing.T) {
	got := researchBullets(nil)
	if len(got) != 0 {
		t.Errorf("researchBullets(nil) = %v, want empty", got)
	}
}
