// practices: [wiki-knowledge-surface, design-by-contract]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatalf("create test file %s: %v", name, err)
	}
}

func TestIndexGenerate(t *testing.T) {
	dir := t.TempDir()

	createTestFile(t, dir, "2026-01-15-first.md", `---
summary: First learning entry
tags: [alpha, beta]
---
# First
Content here.
`)

	createTestFile(t, dir, "2026-02-10-second.md", `---
summary: Second learning entry
tags:
  - gamma
  - delta
---
# Second
More content.
`)

	entries, err := scanDirectory(dir)
	if err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Write INDEX.md
	err = writeIndex(dir, ".agents/learnings", entries, false)
	if err != nil {
		t.Fatalf("writeIndex: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}

	s := string(content)
	today := time.Now().Format("2006-01-02")

	if !strings.Contains(s, "# Index: Learnings") {
		t.Error("missing header")
	}
	if !strings.Contains(s, today) {
		t.Error("missing today's date in header")
	}
	if !strings.Contains(s, "| 2026-01-15-first.md |") {
		t.Error("missing first file entry")
	}
	if !strings.Contains(s, "| 2026-02-10-second.md |") {
		t.Error("missing second file entry")
	}
	if !strings.Contains(s, "| File | Date | Summary | Tags |") {
		t.Error("missing table header")
	}
	if !strings.Contains(s, "2 entries") {
		t.Error("missing entry count")
	}
}

func TestIndexCheck(t *testing.T) {
	dir := t.TempDir()

	createTestFile(t, dir, "2026-01-01-test.md", `---
summary: Test entry
tags: [test]
---
# Test
`)

	entries, err := scanDirectory(dir)
	if err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}

	err = writeIndex(dir, ".agents/learnings", entries, false)
	if err != nil {
		t.Fatalf("writeIndex: %v", err)
	}

	// Check should report current
	isStale, msg := checkIndex(dir, ".agents/learnings", entries)
	if isStale {
		t.Errorf("expected current, got stale: %s", msg)
	}
}

func TestIndexCheckStale(t *testing.T) {
	dir := t.TempDir()

	createTestFile(t, dir, "2026-01-01-test.md", `---
summary: Test entry
tags: [test]
---
# Test
`)

	entries, err := scanDirectory(dir)
	if err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}

	err = writeIndex(dir, ".agents/learnings", entries, false)
	if err != nil {
		t.Fatalf("writeIndex: %v", err)
	}

	// Add a new file after INDEX.md was written
	createTestFile(t, dir, "2026-02-01-new.md", `---
summary: New entry
---
# New
`)

	// Re-scan to get updated entries
	newEntries, err := scanDirectory(dir)
	if err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}

	isStale, msg := checkIndex(dir, ".agents/learnings", newEntries)
	if !isStale {
		t.Error("expected stale, got current")
	}
	if !strings.Contains(msg, "missing") {
		t.Errorf("expected 'missing' in message, got: %s", msg)
	}
}

func TestIndexMalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()

	createTestFile(t, dir, "2026-03-01-broken.md", `---
this is: [not valid: yaml: {{
---
# Broken File
Some content here.
`)

	entries, err := scanDirectory(dir)
	if err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Filename != "2026-03-01-broken.md" {
		t.Errorf("expected filename 2026-03-01-broken.md, got %s", e.Filename)
	}
	// Should fall back to filename date
	if e.Date != "2026-03-01" {
		t.Errorf("expected date 2026-03-01, got %s", e.Date)
	}
	// Summary should fall back to H1
	if e.Summary != "Broken File" {
		t.Errorf("expected summary 'Broken File', got '%s'", e.Summary)
	}
}

func TestIndexBothDateFields(t *testing.T) {
	dir := t.TempDir()

	// File with created_at
	createTestFile(t, dir, "file-with-created-at.md", `---
created_at: 2026-01-20
summary: Has created_at
---
# Created At File
`)

	// File with date
	createTestFile(t, dir, "file-with-date.md", `---
date: 2026-01-25
summary: Has date field
---
# Date File
`)

	// File with both (created_at should win)
	createTestFile(t, dir, "file-with-both.md", `---
created_at: 2026-01-30
date: 2026-01-01
summary: Has both fields
---
# Both Fields
`)

	entries, err := scanDirectory(dir)
	if err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Build a map for easy lookup
	byFile := make(map[string]indexEntry)
	for _, e := range entries {
		byFile[e.Filename] = e
	}

	if e := byFile["file-with-created-at.md"]; e.Date != "2026-01-20" {
		t.Errorf("created_at file: expected date 2026-01-20, got %s", e.Date)
	}
	if e := byFile["file-with-date.md"]; e.Date != "2026-01-25" {
		t.Errorf("date file: expected date 2026-01-25, got %s", e.Date)
	}
	if e := byFile["file-with-both.md"]; e.Date != "2026-01-30" {
		t.Errorf("both fields file: expected created_at=2026-01-30, got %s", e.Date)
	}
}

func TestProcessAllIndexDirs_WithFiles(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-test.md"), []byte("---\nsummary: test\n---\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write mode (not check, not quiet)
	results, stale := processAllIndexDirs(tmp, []string{".agents/learnings"}, false, false)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if stale {
		t.Error("expected not stale in write mode")
	}
	if len(results) > 0 && !results[0].Written {
		t.Error("expected Written=true in write mode")
	}
}

func TestProcessAllIndexDirs_CheckMode(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-test.md"), []byte("---\nsummary: test\n---\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// First, write the index
	processAllIndexDirs(tmp, []string{".agents/learnings"}, false, true)

	// Now check mode — should be current
	results, stale := processAllIndexDirs(tmp, []string{".agents/learnings"}, true, false)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if stale {
		t.Error("expected not stale when index was just written")
	}
}

func TestProcessAllIndexDirs_CheckModeStale(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-test.md"), []byte("---\nsummary: test\n---\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write the index first
	processAllIndexDirs(tmp, []string{".agents/learnings"}, false, true)

	// Add a new file to make it stale
	if err := os.WriteFile(filepath.Join(dir, "2026-02-01-new.md"), []byte("---\nsummary: new\n---\n# New\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Check mode should detect staleness
	_, stale := processAllIndexDirs(tmp, []string{".agents/learnings"}, true, false)
	if !stale {
		t.Error("expected stale when new file was added after index")
	}
}

// ---------------------------------------------------------------------------
// processIndexDir (0%)
// ---------------------------------------------------------------------------

func TestProcessIndexDir_WriteMode(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	entries := []indexEntry{
		{Filename: "test.md", Date: "2026-01-01", Summary: "Test", Tags: "alpha"},
	}

	origDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = origDryRun })

	result, isStale := processIndexDir(false, false, ".agents/learnings", dir, entries)
	if isStale {
		t.Error("expected not stale in write mode")
	}
	if !result.Written {
		t.Error("expected Written=true")
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestProcessIndexDir_CheckMode_Current(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	entries := []indexEntry{
		{Filename: "test.md", Date: "2026-01-01", Summary: "Test", Tags: "alpha"},
	}

	// Write the index first
	if err := writeIndex(dir, ".agents/learnings", entries, false); err != nil {
		t.Fatal(err)
	}

	result, isStale := processIndexDir(true, false, ".agents/learnings", dir, entries)
	if isStale {
		t.Error("expected current index not stale")
	}
	if result.Written {
		t.Error("expected Written=false in check mode")
	}
}

func TestProcessIndexDir_CheckMode_Stale(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write index with just one entry
	initialEntries := []indexEntry{
		{Filename: "test.md", Date: "2026-01-01", Summary: "Test"},
	}
	if err := writeIndex(dir, ".agents/learnings", initialEntries, false); err != nil {
		t.Fatal(err)
	}

	// Check with more entries (stale)
	newEntries := []indexEntry{
		{Filename: "test.md", Date: "2026-01-01", Summary: "Test"},
		{Filename: "new.md", Date: "2026-02-01", Summary: "New"},
	}

	_, isStale := processIndexDir(true, false, ".agents/learnings", dir, newEntries)
	if !isStale {
		t.Error("expected stale when entries don't match")
	}
}

func TestProcessIndexDir_Quiet(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	entries := []indexEntry{
		{Filename: "test.md", Date: "2026-01-01", Summary: "Test"},
	}

	origDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = origDryRun })

	// quiet mode should not panic
	result, _ := processIndexDir(false, true, ".agents/learnings", dir, entries)
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

// ---------------------------------------------------------------------------
// runIndex (0%) — exercise through cobra
// ---------------------------------------------------------------------------

func TestRunIndex_WriteMode(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-test.md"), []byte("---\nsummary: test\n---\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDryRun := dryRun
	origOutput := output
	dryRun = false
	output = "table"
	t.Cleanup(func() {
		dryRun = origDryRun
		output = origOutput
	})

	// Reset flags on the cobra command
	_ = indexCmd.Flags().Set("check", "false")
	_ = indexCmd.Flags().Set("json", "false")
	_ = indexCmd.Flags().Set("dir", "")
	_ = indexCmd.Flags().Set("quiet", "true")

	err := runIndex(indexCmd, nil)
	if err != nil {
		t.Fatalf("runIndex() error = %v", err)
	}
}

func TestRunIndex_CheckMode_Stale(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-test.md"), []byte("---\nsummary: test\n---\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Don't write INDEX.md → stale

	origOutput := output
	output = "table"
	t.Cleanup(func() { output = origOutput })

	_ = indexCmd.Flags().Set("check", "true")
	_ = indexCmd.Flags().Set("json", "false")
	_ = indexCmd.Flags().Set("dir", ".agents/learnings")
	_ = indexCmd.Flags().Set("quiet", "false")

	err := runIndex(indexCmd, nil)
	if err == nil {
		t.Error("expected error for stale INDEX.md")
	}
	if err != nil && !strings.Contains(err.Error(), "stale") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunIndex_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-test.md"), []byte("---\nsummary: test\n---\n# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDryRun := dryRun
	origOutput := output
	dryRun = false
	output = "json"
	t.Cleanup(func() {
		dryRun = origDryRun
		output = origOutput
	})

	_ = indexCmd.Flags().Set("check", "false")
	_ = indexCmd.Flags().Set("json", "true")
	_ = indexCmd.Flags().Set("dir", ".agents/learnings")
	_ = indexCmd.Flags().Set("quiet", "true")

	// Capture stdout
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runIndex(indexCmd, nil)

	_ = w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runIndex() error = %v", err)
	}

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "learnings") {
		t.Errorf("expected 'learnings' in JSON output, got: %s", out[:min(len(out), 200)])
	}
}

func TestRunIndex_SingleDir(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	dir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-retry.md"), []byte("---\nsummary: retry pattern\n---\n# Retry\n"), 0644); err != nil {
		t.Fatal(err)
	}

	origDryRun := dryRun
	origOutput := output
	dryRun = false
	output = "table"
	t.Cleanup(func() {
		dryRun = origDryRun
		output = origOutput
	})

	_ = indexCmd.Flags().Set("check", "false")
	_ = indexCmd.Flags().Set("json", "false")
	_ = indexCmd.Flags().Set("dir", ".agents/patterns")
	_ = indexCmd.Flags().Set("quiet", "true")

	err := runIndex(indexCmd, nil)
	if err != nil {
		t.Fatalf("runIndex() error = %v", err)
	}

	// Verify INDEX.md was written
	content, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "retry") {
		t.Error("expected INDEX.md to contain 'retry'")
	}
}

func TestScanAndSortDir_WithFiles(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01-a.md"), []byte("---\nsummary: A\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-02-01-b.md"), []byte("---\nsummary: B\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, entries, ok := scanAndSortDir(tmp, ".agents/learnings")
	if !ok {
		t.Error("expected ok=true")
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	// Should be sorted by date descending (2026-02-01 first)
	if len(entries) >= 2 && entries[0].Date < entries[1].Date {
		t.Errorf("expected descending date sort, got %s before %s", entries[0].Date, entries[1].Date)
	}
}

// ---------------------------------------------------------------------------
// writeIndex — dry-run
// ---------------------------------------------------------------------------

func TestWriteIndex_DryRun(t *testing.T) {
	tmp := t.TempDir()
	entries := []indexEntry{
		{Filename: "test.md", Date: "2026-01-01", Summary: "Test"},
	}

	err := writeIndex(tmp, ".agents/learnings", entries, true)
	if err != nil {
		t.Fatalf("writeIndex dry-run error = %v", err)
	}

	// Verify INDEX.md was NOT written
	_, err = os.Stat(filepath.Join(tmp, "INDEX.md"))
	if err == nil {
		t.Error("expected INDEX.md not written in dry-run mode")
	}
}

func TestParseIndexTableRows_ValidTable(t *testing.T) {
	table := `# Index: Learnings

| File | Date | Summary | Tags |
|------|------|---------|------|
| test.md | 2026-01-01 | Test | alpha |
| foo.md | 2026-02-01 | Foo | beta |
`
	result := parseIndexTableRows([]byte(table))
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d", len(result))
	}
	if !result["test.md"] {
		t.Error("expected test.md in results")
	}
	if !result["foo.md"] {
		t.Error("expected foo.md in results")
	}
}

// ---------------------------------------------------------------------------
// buildIndexDiffMessage
// ---------------------------------------------------------------------------

func TestBuildIndexDiffMessage_MissingAndExtra(t *testing.T) {
	msg := buildIndexDiffMessage(".agents/learnings", []string{"new.md"}, []string{"old.md"})
	if !strings.Contains(msg, "missing=[new.md]") {
		t.Errorf("expected 'missing=[new.md]' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "extra=[old.md]") {
		t.Errorf("expected 'extra=[old.md]' in message, got: %s", msg)
	}
}
