// practices: [dora-metrics, wiki-knowledge-surface]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFindingsListJSON_ExcludesRetiredByDefault(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	writeFindingFixture(t, repo, "f-active", "Active finding", "active", 2)
	writeFindingFixture(t, repo, "f-retired", "Retired finding", "retired", 9)

	out, err := executeCommand("findings", "list", "--json")
	if err != nil {
		t.Fatalf("findings list failed: %v\n%s", err, out)
	}

	var findings []knowledgeFinding
	if err := json.Unmarshal([]byte(out), &findings); err != nil {
		t.Fatalf("parse findings list json: %v\n%s", err, out)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 active finding, got %d", len(findings))
	}
	if findings[0].ID != "f-active" {
		t.Fatalf("expected active finding, got %q", findings[0].ID)
	}
}

func TestFindingsRetire_UpdatesFrontMatter(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	path := writeFindingFixture(t, repo, "f-retire", "Retire me", "active", 1)

	out, err := executeCommand("findings", "retire", "f-retire", "--by", "tester", "--json")
	if err != nil {
		t.Fatalf("findings retire failed: %v\n%s", err, out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read retired finding: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "status: retired") {
		t.Fatalf("retired file missing retired status:\n%s", content)
	}
	if !strings.Contains(content, "retired_by: tester") {
		t.Fatalf("retired file missing retired_by marker:\n%s", content)
	}
}

func TestFindingsPullAndExport_JSON(t *testing.T) {
	sourceRepo := t.TempDir()
	destRepo := t.TempDir()
	exportRepo := t.TempDir()

	writeFindingFixture(t, sourceRepo, "f-pull", "Pulled finding", "active", 4)

	t.Chdir(destRepo)
	out, err := executeCommand("findings", "pull", "f-pull", "--from", sourceRepo, "--json")
	if err != nil {
		t.Fatalf("findings pull failed: %v\n%s", err, out)
	}

	localPath := filepath.Join(destRepo, ".agents", SectionFindings, "f-pull.md")
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected pulled finding at %s: %v", localPath, err)
	}

	out, err = executeCommand("findings", "export", "f-pull", "--to", exportRepo, "--json")
	if err != nil {
		t.Fatalf("findings export failed: %v\n%s", err, out)
	}

	exportPath := filepath.Join(exportRepo, ".agents", SectionFindings, "f-pull.md")
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("expected exported finding at %s: %v", exportPath, err)
	}
}

func TestFindingsStats_JSON(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	writeFindingFixture(t, repo, "f-one", "One", "active", 3)
	writeFindingFixture(t, repo, "f-two", "Two", "retired", 5)

	out, err := executeCommand("findings", "stats", "--json")
	if err != nil {
		t.Fatalf("findings stats failed: %v\n%s", err, out)
	}

	var stats findingStats
	if err := json.Unmarshal([]byte(out), &stats); err != nil {
		t.Fatalf("parse findings stats json: %v\n%s", err, out)
	}
	if stats.Total != 2 {
		t.Fatalf("total = %d, want 2", stats.Total)
	}
	if stats.ByStatus["active"] != 1 || stats.ByStatus["retired"] != 1 {
		t.Fatalf("unexpected status counts: %+v", stats.ByStatus)
	}
	if stats.TotalHits != 8 {
		t.Fatalf("total_hits = %d, want 8", stats.TotalHits)
	}
	if len(stats.MostCited) == 0 || stats.MostCited[0].ID != "f-two" {
		t.Fatalf("most cited ordering incorrect: %+v", stats.MostCited)
	}
}

// ---------------------------------------------------------------------------
// buildFindingStats
// ---------------------------------------------------------------------------

func TestBuildFindingStats_EmptyFindings(t *testing.T) {
	stats := buildFindingStats(nil)
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
	if stats.TotalHits != 0 {
		t.Errorf("TotalHits = %d, want 0", stats.TotalHits)
	}
	if len(stats.MostCited) != 0 {
		t.Errorf("MostCited should be empty, got %d", len(stats.MostCited))
	}
}

func TestBuildFindingStats_SortsByHitCountDescending(t *testing.T) {
	findings := []knowledgeFinding{
		{ID: "f-low", Status: "active", Severity: "low", Detectability: "manual", HitCount: 1},
		{ID: "f-high", Status: "active", Severity: "high", Detectability: "mechanical", HitCount: 10},
		{ID: "f-mid", Status: "retired", Severity: "medium", Detectability: "manual", HitCount: 5},
	}
	stats := buildFindingStats(findings)
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.TotalHits != 16 {
		t.Errorf("TotalHits = %d, want 16", stats.TotalHits)
	}
	if stats.ByStatus["active"] != 2 {
		t.Errorf("ByStatus[active] = %d, want 2", stats.ByStatus["active"])
	}
	if stats.ByStatus["retired"] != 1 {
		t.Errorf("ByStatus[retired] = %d, want 1", stats.ByStatus["retired"])
	}
	if stats.BySeverity["high"] != 1 {
		t.Errorf("BySeverity[high] = %d, want 1", stats.BySeverity["high"])
	}
	if stats.ByDetectability["mechanical"] != 1 {
		t.Errorf("ByDetectability[mechanical] = %d, want 1", stats.ByDetectability["mechanical"])
	}
	// Most cited should be ordered by hit_count descending
	if len(stats.MostCited) != 3 {
		t.Fatalf("MostCited len = %d, want 3", len(stats.MostCited))
	}
	if stats.MostCited[0].ID != "f-high" {
		t.Errorf("MostCited[0] = %q, want f-high", stats.MostCited[0].ID)
	}
}

func TestBuildFindingStats_MoreThanFiveCapsToFive(t *testing.T) {
	findings := make([]knowledgeFinding, 8)
	for i := range findings {
		findings[i] = knowledgeFinding{
			ID:       "f-" + strconv.Itoa(i),
			Status:   "active",
			HitCount: 8 - i,
		}
	}
	stats := buildFindingStats(findings)
	if len(stats.MostCited) != 5 {
		t.Errorf("MostCited len = %d, want 5 (capped)", len(stats.MostCited))
	}
}

func TestBuildFindingStats_EmptyFieldsFallbackToUnknown(t *testing.T) {
	findings := []knowledgeFinding{
		{ID: "f-blank", Status: "", Severity: "", Detectability: ""},
	}
	stats := buildFindingStats(findings)
	if stats.ByStatus["unknown"] != 1 {
		t.Errorf("ByStatus[unknown] = %d, want 1", stats.ByStatus["unknown"])
	}
	if stats.BySeverity["unknown"] != 1 {
		t.Errorf("BySeverity[unknown] = %d, want 1", stats.BySeverity["unknown"])
	}
}

// ---------------------------------------------------------------------------
// selectFindingFiles
// ---------------------------------------------------------------------------

func TestSelectFindingFiles_AllFlag(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".agents", SectionFindings)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"f-one", "f-two"} {
		writeFindingFixture(t, repo, name, "Title", "active", 1)
	}
	files, err := selectFindingFiles(dir, nil, true)
	if err != nil {
		t.Fatalf("selectFindingFiles(all=true): %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestSelectFindingFiles_NotFound(t *testing.T) {
	repo := t.TempDir()
	writeFindingFixture(t, repo, "f-exists", "Title", "active", 1)
	dir := filepath.Join(repo, ".agents", SectionFindings)
	_, err := selectFindingFiles(dir, []string{"nonexistent-id"}, false)
	if err == nil {
		t.Error("expected error for non-existent finding ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// normalizeStatKey
// ---------------------------------------------------------------------------

func TestNormalizeStatKey(t *testing.T) {
	tests := []struct {
		value    string
		fallback string
		want     string
	}{
		{"active", "unknown", "active"},
		{"  ", "unknown", "unknown"},
		{"", "default", "default"},
		{"  high  ", "unknown", "high"},
	}
	for _, tt := range tests {
		got := normalizeStatKey(tt.value, tt.fallback)
		if got != tt.want {
			t.Errorf("normalizeStatKey(%q, %q) = %q, want %q", tt.value, tt.fallback, got, tt.want)
		}
	}
}

func writeFindingFixture(t *testing.T, repoRoot, id, title, status string, hits int) string {
	t.Helper()
	dir := filepath.Join(repoRoot, ".agents", SectionFindings)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir findings dir: %v", err)
	}
	path := filepath.Join(dir, id+".md")
	content := `---
id: ` + id + `
title: ` + title + `
source_skill: vibe
severity: high
detectability: mechanical
status: ` + status + `
compiler_targets: [inject, lookup]
scope_tags: [cli, validation]
applicable_when: [plan-shape, validation-gap]
applicable_languages: [go]
hit_count: ` + strconv.Itoa(hits) + `
last_cited: 2026-03-10T01:00:00Z
---

# ` + title + `

Summary text for ` + id + `.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write finding fixture: %v", err)
	}
	return path
}

func TestUpdateFindingFrontMatter_FileNotFound(t *testing.T) {
	err := updateFindingFrontMatter("/nonexistent/path/finding.md", map[string]string{"status": "retired"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "read finding") {
		t.Errorf("expected 'read finding' in error, got %q", err.Error())
	}
}

func TestUpdateFindingFrontMatter_NoFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "finding.md")
	// File with no frontmatter delimiters.
	if err := os.WriteFile(path, []byte("# Title\n\nBody text.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := updateFindingFrontMatter(path, map[string]string{"status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	// Should have wrapped content with new frontmatter containing the key.
	if !strings.Contains(content, "status: active") {
		t.Errorf("expected 'status: active' in output, got:\n%s", content)
	}
	if !strings.Contains(content, "---") {
		t.Error("expected frontmatter delimiters in output")
	}
}

func TestUpdateFindingFrontMatter_UpdateExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "finding.md")
	original := "---\nstatus: active\nseverity: high\n---\n\n# Title\n\nBody.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	err := updateFindingFrontMatter(path, map[string]string{"status": "retired"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "status: retired") {
		t.Errorf("expected 'status: retired', got:\n%s", content)
	}
	// Original key should be replaced, not duplicated.
	if strings.Count(content, "status:") != 1 {
		t.Errorf("expected exactly one 'status:' line, got:\n%s", content)
	}
	if !strings.Contains(content, "severity: high") {
		t.Error("expected other frontmatter keys to be preserved")
	}
	if !strings.Contains(content, "Body.") {
		t.Error("expected body text to be preserved")
	}
}

func TestPrintStringCountMap(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	values := map[string]int{
		"beta":  3,
		"alpha": 5,
		"gamma": 1,
	}
	printStringCountMap(values)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// Should be sorted alphabetically
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "alpha: 5") {
		t.Errorf("first line = %q, want alpha: 5", lines[0])
	}
	if !strings.Contains(lines[1], "beta: 3") {
		t.Errorf("second line = %q, want beta: 3", lines[1])
	}
	if !strings.Contains(lines[2], "gamma: 1") {
		t.Errorf("third line = %q, want gamma: 1", lines[2])
	}
}

func TestPrintStringCountMap_Empty(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStringCountMap(map[string]int{})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	if out != "" {
		t.Errorf("empty map should produce no output, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// printFindingTransferResult
// ---------------------------------------------------------------------------

func TestPrintFindingTransferResult_Human_Empty(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	out, err := captureStdout(t, func() error {
		return printFindingTransferResult("exported", nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No findings exported.") {
		t.Errorf("missing empty message, got: %q", out)
	}
}

func TestPrintFindingTransferResult_Human_WithPaths(t *testing.T) {
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	out, err := captureStdout(t, func() error {
		return printFindingTransferResult("imported", []string{"/tmp/a.json", "/tmp/b.json"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Imported 2 finding(s):") {
		t.Errorf("missing count message, got: %q", out)
	}
	if !strings.Contains(out, "/tmp/a.json") {
		t.Errorf("missing first path, got: %q", out)
	}
}

func TestPrintFindingTransferResult_JSON(t *testing.T) {
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	out, err := captureStdout(t, func() error {
		return printFindingTransferResult("exported", []string{"/tmp/f1.json"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["action"] != "exported" {
		t.Errorf("action = %v, want %q", parsed["action"], "exported")
	}
}

// ---------------------------------------------------------------------------
// runFindingsStats (functional test with fixture dir)
// ---------------------------------------------------------------------------

func TestRunFindingsStats_Human(t *testing.T) {
	tmp := chdirTemp(t)
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	findingsDir := filepath.Join(tmp, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// collectFindingsFromDir reads .md files with YAML frontmatter
	findingMD := `---
id: F-TEST-001
title: Test finding
status: active
severity: high
hit_count: 3
---
Test finding body.
`
	if err := os.WriteFile(filepath.Join(findingsDir, "F-TEST-001.md"), []byte(findingMD), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return runFindingsStats(nil, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Total findings:") {
		t.Errorf("missing total count, got: %q", out)
	}
	if !strings.Contains(out, "Total hits:") {
		t.Errorf("missing total hits, got: %q", out)
	}
	if !strings.Contains(out, "By status:") {
		t.Errorf("missing by status, got: %q", out)
	}
}

func TestRunFindingsStats_JSON(t *testing.T) {
	tmp := chdirTemp(t)
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	findingsDir := filepath.Join(tmp, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return runFindingsStats(nil, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed findingStats
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if parsed.Total != 0 {
		t.Errorf("Total = %d, want 0 for empty dir", parsed.Total)
	}
}

// ---------------------------------------------------------------------------
// buildFindingStats
// ---------------------------------------------------------------------------

func TestBuildFindingStats(t *testing.T) {
	findings := []knowledgeFinding{
		{ID: "F-1", Status: "active", Severity: "high", HitCount: 5, Detectability: "automated"},
		{ID: "F-2", Status: "active", Severity: "medium", HitCount: 2, Detectability: "manual"},
		{ID: "F-3", Status: "retired", Severity: "high", HitCount: 0, Detectability: "automated"},
	}

	stats := buildFindingStats(findings)
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.TotalHits != 7 {
		t.Errorf("TotalHits = %d, want 7", stats.TotalHits)
	}
	if stats.ByStatus["active"] != 2 {
		t.Errorf("ByStatus[active] = %d, want 2", stats.ByStatus["active"])
	}
	if stats.BySeverity["high"] != 2 {
		t.Errorf("BySeverity[high] = %d, want 2", stats.BySeverity["high"])
	}
	if stats.ByDetectability["automated"] != 2 {
		t.Errorf("ByDetectability[automated] = %d, want 2", stats.ByDetectability["automated"])
	}
}
