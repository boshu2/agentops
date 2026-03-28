package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
)

func TestWritePendingLearnings_WritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "test-session-abc123",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Always check error returns from file operations",
			"Use table-driven tests for multi-case functions",
			"Gini coefficient measures inequality in distributions",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings failed: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 files written, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 files in pending dir, got %d", len(entries))
	}

	// Verify first file content
	data, err := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# Learning:") {
		t.Error("expected '# Learning:' heading in output")
	}
	if !strings.Contains(content, "**ID**:") {
		t.Error("expected '**ID**:' metadata in output")
	}
	if !strings.Contains(content, "**Category**:") {
		t.Error("expected '**Category**:' metadata in output")
	}
	if !strings.Contains(content, "**Confidence**: medium") {
		t.Error("expected '**Confidence**: medium' in output")
	}
}

func TestWritePendingLearnings_IncludesDecisions(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "test-decisions-def456",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Decisions: []string{
			"We decided to use auto-promote instead of relay",
			"Selected Go over Python for CLI performance",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings failed: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 files, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)
	if !strings.Contains(content, "**Category**: decision") {
		t.Errorf("expected category 'decision' for decisions, got: %s", content)
	}
}

func TestWritePendingLearnings_EmptySession(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "empty-session",
		Date: time.Now(),
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 files for empty session, got %d", n)
	}
}

func TestWritePendingLearnings_NilSession(t *testing.T) {
	n, err := writePendingLearnings(nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 for nil session, got %d", n)
	}
}

func TestWritePendingLearnings_FrontmatterFormat(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:        "frontmatter-test-789",
		Date:      time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{"Test frontmatter is correct"},
	}

	writePendingLearnings(session, dir)

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("expected YAML frontmatter start")
	}
	if !strings.Contains(content, "date: 2026-03-20") {
		t.Error("expected date in frontmatter")
	}
	if !strings.Contains(content, "type: learning") {
		t.Error("expected type in frontmatter")
	}
	if !strings.Contains(content, "source: frontmatter-test-789") {
		t.Error("expected source session ID in frontmatter")
	}
}

func TestWritePendingLearnings_PoolIngestCompatible(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "compat-test-abc",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Pool ingest reads # Learning: headings with **ID**, **Category**, **Confidence** fields",
		},
	}

	writePendingLearnings(session, dir)

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	// Verify parseLearningBlocks can parse this
	blocks := parseLearningBlocks(content)
	if len(blocks) != 1 {
		t.Fatalf("expected parseLearningBlocks to find 1 block, got %d", len(blocks))
	}
	if blocks[0].ID == "" {
		t.Error("expected parsed block to have an ID")
	}
	if blocks[0].Category == "" {
		t.Error("expected parsed block to have a Category")
	}
	if blocks[0].Confidence == "" {
		t.Error("expected parsed block to have a Confidence")
	}
}

func TestWritePendingLearnings_ResearchProvenance(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "provenance-test-123",
		Date: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Based on .agents/research/2026-03-22-flywheel-gap.md we found that escape velocity alone is insufficient for compounding claims",
		},
	}

	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 file written, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	if !strings.Contains(content, "research_sources:") {
		t.Error("expected research_sources: in frontmatter when knowledge references research files")
	}
	if !strings.Contains(content, ".agents/research/2026-03-22-flywheel-gap.md") {
		t.Error("expected exact research file path in research_sources frontmatter")
	}
}

func TestWritePendingLearnings_NoResearchProvenance(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "no-provenance-456",
		Date: time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Generic knowledge without research references",
		},
	}

	writePendingLearnings(session, dir)

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, _ := os.ReadDir(pendingDir)
	data, _ := os.ReadFile(filepath.Join(pendingDir, entries[0].Name()))
	content := string(data)

	if strings.Contains(content, "research_sources:") {
		t.Error("expected NO research_sources in frontmatter when knowledge has no research references")
	}
}

func TestInferCategory(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"We decided to use Go", "decision"},
		{"Selected approach A over B", "decision"},
		{"The test failed because of a race condition", "failure"},
		{"Fixed the bug by adding a mutex", "solution"},
		{"Always run tests before committing", "learning"},
	}

	for _, tt := range tests {
		got := inferCategory(tt.text)
		if got != tt.expected {
			t.Errorf("inferCategory(%q) = %q, want %q", tt.text, got, tt.expected)
		}
	}
}

func TestPendingTitle(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Simple title", "Simple title"},
		{"First line\nSecond line", "First line"},
		{"# Markdown heading", "Markdown heading"},
		{"", "Extracted knowledge"},
		{strings.Repeat("x", 100), strings.Repeat("x", 77) + "..."},
	}

	for _, tt := range tests {
		got := pendingTitle(tt.text)
		if got != tt.expected {
			t.Errorf("pendingTitle(%q) = %q, want %q", tt.text, got, tt.expected)
		}
	}
}
