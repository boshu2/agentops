package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCurate_Catalog_Learning catalogs a learning artifact with correct
// structure and verifies it gets written to .agents/learnings/.
func TestCurate_Catalog_Learning(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create a test artifact file with YAML frontmatter
	artifactContent := `---
type: learning
date: 2026-02-24
---
Agents forget context between sessions. Use .agents/ for persistence.
`
	inputPath := filepath.Join(tmpDir, "test-learning.md")
	if err := os.WriteFile(inputPath, []byte(artifactContent), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// Run catalog
	if err := runCurateCatalog(nil, []string{inputPath}); err != nil {
		t.Fatalf("runCurateCatalog: %v", err)
	}

	// Verify artifact was written to .agents/learnings/
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		t.Fatalf("reading learnings dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 artifact in learnings dir, got %d", len(entries))
	}

	// Read and validate the artifact
	data, err := os.ReadFile(filepath.Join(learningsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("reading artifact: %v", err)
	}

	var artifact curateArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}

	if artifact.Type != "learning" {
		t.Errorf("expected type 'learning', got %q", artifact.Type)
	}
	if artifact.Date != "2026-02-24" {
		t.Errorf("expected date '2026-02-24', got %q", artifact.Date)
	}
	if artifact.SchemaVersion != 1 {
		t.Errorf("expected schema_version 1, got %d", artifact.SchemaVersion)
	}
	if artifact.CuratedAt == "" {
		t.Error("expected curated_at to be set")
	}
	if artifact.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if artifact.Content == "" {
		t.Error("expected non-empty content")
	}
}

// TestCurate_Catalog_InvalidType rejects artifacts with unknown type.
func TestCurate_Catalog_InvalidType(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create artifact with invalid type
	artifactContent := `---
type: opinion
date: 2026-02-24
---
This is not a valid type.
`
	inputPath := filepath.Join(tmpDir, "bad-type.md")
	if err := os.WriteFile(inputPath, []byte(artifactContent), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	err = runCurateCatalog(nil, []string{inputPath})
	if err == nil {
		t.Fatal("expected error for unknown artifact type, got nil")
	}

	expectedMsg := `unknown artifact type "opinion": must be one of learning, decision, failure, pattern`
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}

	// Verify nothing was written
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if _, statErr := os.Stat(learningsDir); !os.IsNotExist(statErr) {
		t.Error("learnings dir should not exist after rejected artifact")
	}
}

// TestCurate_Verify_AllPass returns verified=true when all gates pass.
// This test mocks the gate checks by creating a baseline with all passing gates
// and verifying that no regressions are detected.
func TestCurate_Verify_AllPass(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create a minimal GOALS.yaml with a trivially passing check
	goalsContent := `version: 3
goals:
  - id: always-pass
    description: A gate that always passes
    check: "true"
    weight: 1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "GOALS.yaml"), []byte(goalsContent), 0o644); err != nil {
		t.Fatalf("write GOALS.yaml: %v", err)
	}

	// Create a baseline snapshot where everything passes
	baselineDir := filepath.Join(tmpDir, ".agents", "ao", "baselines")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		t.Fatalf("mkdir baselines: %v", err)
	}

	baselineSnap := map[string]interface{}{
		"timestamp": "2026-02-23T10:00:00Z",
		"git_sha":   "abc123",
		"goals": []map[string]interface{}{
			{"goal_id": "always-pass", "result": "pass", "duration_s": 0.1, "weight": 1},
		},
		"summary": map[string]interface{}{
			"total": 1, "passing": 1, "failing": 0, "skipped": 0, "score": 100.0,
		},
	}
	baselineData, _ := json.MarshalIndent(baselineSnap, "", "  ")
	if err := os.WriteFile(filepath.Join(baselineDir, "2026-02-23T10-00-00.000.json"), baselineData, 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	// Force JSON output so we can parse the result
	origOutput := output
	output = "json"
	t.Cleanup(func() { output = origOutput })

	// Capture stdout
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runCurateVerify(nil, nil)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runCurateVerify: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	r.Close()

	var result curateVerifyResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %s)", err, string(buf[:n]))
	}

	if !result.Verified {
		t.Error("expected verified=true")
	}
	if result.GatesPassed != 1 {
		t.Errorf("expected 1 gate passed, got %d", result.GatesPassed)
	}
	if result.GatesFailed != 0 {
		t.Errorf("expected 0 gates failed, got %d", result.GatesFailed)
	}
	if len(result.Regressions) != 0 {
		t.Errorf("expected 0 regressions, got %v", result.Regressions)
	}
}

// TestCurate_Verify_Regression detects regression when a gate that previously
// passed now fails.
func TestCurate_Verify_Regression(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Create a GOALS.yaml with a gate that fails
	goalsContent := `version: 3
goals:
  - id: now-fails
    description: A gate that now fails
    check: "false"
    weight: 1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "GOALS.yaml"), []byte(goalsContent), 0o644); err != nil {
		t.Fatalf("write GOALS.yaml: %v", err)
	}

	// Create a baseline where it previously passed
	baselineDir := filepath.Join(tmpDir, ".agents", "ao", "baselines")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		t.Fatalf("mkdir baselines: %v", err)
	}

	baselineSnap := map[string]interface{}{
		"timestamp": "2026-02-23T10:00:00Z",
		"git_sha":   "abc123",
		"goals": []map[string]interface{}{
			{"goal_id": "now-fails", "result": "pass", "duration_s": 0.1, "weight": 1},
		},
		"summary": map[string]interface{}{
			"total": 1, "passing": 1, "failing": 0, "skipped": 0, "score": 100.0,
		},
	}
	baselineData, _ := json.MarshalIndent(baselineSnap, "", "  ")
	if err := os.WriteFile(filepath.Join(baselineDir, "2026-02-23T10-00-00.000.json"), baselineData, 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	// Force JSON output
	origOutput := output
	output = "json"
	t.Cleanup(func() { output = origOutput })

	// Capture stdout
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runCurateVerify(nil, nil)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runCurateVerify: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	r.Close()

	var result curateVerifyResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %s)", err, string(buf[:n]))
	}

	if result.Verified {
		t.Error("expected verified=false due to regression")
	}
	if result.GatesFailed != 1 {
		t.Errorf("expected 1 gate failed, got %d", result.GatesFailed)
	}
	if len(result.Regressions) != 1 {
		t.Fatalf("expected 1 regression, got %d", len(result.Regressions))
	}
	if result.Regressions[0] != "now-fails" {
		t.Errorf("expected regression on 'now-fails', got %q", result.Regressions[0])
	}
}

// TestCurate_Status_EmptyRepo returns zero counts gracefully when no artifacts exist.
func TestCurate_Status_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Force JSON output
	origOutput := output
	output = "json"
	t.Cleanup(func() { output = origOutput })

	// Capture stdout
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runCurateStatus(nil, nil)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runCurateStatus: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	r.Close()

	var result curateStatusResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %s)", err, string(buf[:n]))
	}

	if result.Total != 0 {
		t.Errorf("expected total=0, got %d", result.Total)
	}
	if result.Learnings != 0 {
		t.Errorf("expected learnings=0, got %d", result.Learnings)
	}
	if result.Decisions != 0 {
		t.Errorf("expected decisions=0, got %d", result.Decisions)
	}
	if result.Failures != 0 {
		t.Errorf("expected failures=0, got %d", result.Failures)
	}
	if result.Patterns != 0 {
		t.Errorf("expected patterns=0, got %d", result.Patterns)
	}
	if result.LastCatalogAt != "" {
		t.Errorf("expected empty last_catalog_at, got %q", result.LastCatalogAt)
	}
	if result.LastVerifyAt != "" {
		t.Errorf("expected empty last_verify_at, got %q", result.LastVerifyAt)
	}
	if result.PendingVerify != 0 {
		t.Errorf("expected pending_verify=0, got %d", result.PendingVerify)
	}
}

func TestCurateVerify_GoalsMDFallback(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	goalsMD := `# Goals

## Mission
Curate verify fallback test

## Gates
| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| always-pass | ` + "`true`" + ` | 1 | always passes |
`
	if err := os.WriteFile(filepath.Join(tmpDir, "GOALS.md"), []byte(goalsMD), 0o644); err != nil {
		t.Fatalf("write GOALS.md: %v", err)
	}

	origOutput := output
	output = "json"
	t.Cleanup(func() { output = origOutput })

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = runCurateVerify(nil, nil)
	w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("runCurateVerify: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	r.Close()

	var result curateVerifyResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %s)", err, string(buf[:n]))
	}
	if !result.Verified {
		t.Fatalf("expected verified=true, got false (%+v)", result)
	}
	if result.GatesPassed < 1 {
		t.Fatalf("expected at least one gate passed, got %d", result.GatesPassed)
	}
}

func TestCurateParseFrontmatter_YAMLMultiline(t *testing.T) {
	input := `---
type: learning
date: 2026-02-24
content: |
  line one
  line two
tags:
  - constraint
  - parser
---
Body fallback content
`

	fm, body := curateParseFrontmatter(input)

	if got := curateFrontmatterString(fm, "type"); got != "learning" {
		t.Fatalf("type = %q, want learning", got)
	}
	if got := curateFrontmatterString(fm, "date"); got != "2026-02-24" {
		t.Fatalf("date = %q, want 2026-02-24", got)
	}
	if got := curateFrontmatterString(fm, "content"); !strings.Contains(got, "line one") || !strings.Contains(got, "line two") {
		t.Fatalf("expected multiline content in frontmatter, got %q", got)
	}
	if body != "Body fallback content" {
		t.Fatalf("body = %q, want %q", body, "Body fallback content")
	}
}

func TestCuratePipeline_CatalogAssembleVerify(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	goalsMD := `# Goals

## Mission
Pipeline integration

## Gates
| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| always-pass | ` + "`true`" + ` | 1 | always passes |
`
	if err := os.WriteFile(filepath.Join(tmpDir, "GOALS.md"), []byte(goalsMD), 0o644); err != nil {
		t.Fatalf("write GOALS.md: %v", err)
	}

	artifact := `---
type: learning
date: 2026-02-24
---
Context assembly should read curated JSON artifacts.
`
	artifactPath := filepath.Join(tmpDir, "pipeline-learning.md")
	if err := os.WriteFile(artifactPath, []byte(artifact), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := runCurateCatalog(nil, []string{artifactPath}); err != nil {
		t.Fatalf("runCurateCatalog: %v", err)
	}

	sections := assembleSections(tmpDir, "Read curated artifacts", defaultAssembleMaxChars)
	if len(sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(sections))
	}
	if !strings.Contains(sections[2].Content, "Context assembly should read curated JSON artifacts.") {
		t.Fatalf("INTEL section did not include curated learning content: %s", sections[2].Content)
	}

	origOutput := output
	output = "json"
	t.Cleanup(func() { output = origOutput })

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = runCurateVerify(nil, nil)
	w.Close()
	os.Stdout = origStdout
	if err != nil {
		t.Fatalf("runCurateVerify: %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	r.Close()

	var result curateVerifyResult
	if err := json.Unmarshal(buf[:n], &result); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %s)", err, string(buf[:n]))
	}
	if !result.Verified {
		t.Fatalf("expected verified=true in pipeline path, got false (%+v)", result)
	}
}

func TestCountArtifactsSince(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, "learnings")
	patternsDir := filepath.Join(tmp, "patterns")
	os.MkdirAll(learningsDir, 0o755)
	os.MkdirAll(patternsDir, 0o755)

	now := time.Now()
	recent := now.Add(-1 * time.Hour).Format(time.RFC3339)
	old := now.Add(-48 * time.Hour).Format(time.RFC3339)
	since := now.Add(-24 * time.Hour)

	// Recent artifact in learnings (should count)
	a1, _ := json.Marshal(curateArtifact{ID: "l1", CuratedAt: recent})
	os.WriteFile(filepath.Join(learningsDir, "l1.json"), a1, 0o644)

	// Old artifact in learnings (should not count)
	a2, _ := json.Marshal(curateArtifact{ID: "l2", CuratedAt: old})
	os.WriteFile(filepath.Join(learningsDir, "l2.json"), a2, 0o644)

	// Recent artifact in patterns (should count)
	a3, _ := json.Marshal(curateArtifact{ID: "p1", CuratedAt: recent})
	os.WriteFile(filepath.Join(patternsDir, "p1.json"), a3, 0o644)

	// Non-json file (should be skipped)
	os.WriteFile(filepath.Join(learningsDir, "readme.md"), []byte("# Learnings"), 0o644)

	count := countArtifactsSince(learningsDir, patternsDir, since)
	if count != 2 {
		t.Errorf("count = %d, want 2 (1 recent learning + 1 recent pattern)", count)
	}
}

func TestCountArtifactsSince_EmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	count := countArtifactsSince(filepath.Join(tmp, "missing1"), filepath.Join(tmp, "missing2"), time.Now())
	if count != 0 {
		t.Errorf("count = %d, want 0 for missing dirs", count)
	}
}

func TestCountArtifactsSince_MalformedJSON(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "learnings")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0o644)

	count := countArtifactsSince(dir, filepath.Join(tmp, "empty"), time.Now().Add(-24*time.Hour))
	if count != 0 {
		t.Errorf("count = %d, want 0 for malformed JSON", count)
	}
}

// ---------------------------------------------------------------------------
// countArtifactsInDir
// ---------------------------------------------------------------------------

func TestCountArtifactsInDir_Empty(t *testing.T) {
	dir := t.TempDir()
	counts, latest := countArtifactsInDir(dir)
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty for empty dir", counts)
	}
	if !latest.IsZero() {
		t.Errorf("latest = %v, want zero for empty dir", latest)
	}
}

func TestCountArtifactsInDir_NonexistentDir(t *testing.T) {
	counts, latest := countArtifactsInDir("/nonexistent-dir-xyz")
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty for nonexistent dir", counts)
	}
	if !latest.IsZero() {
		t.Errorf("latest = %v, want zero for nonexistent dir", latest)
	}
}

func TestCountArtifactsInDir_ValidArtifacts(t *testing.T) {
	dir := t.TempDir()

	artifacts := []curateArtifact{
		{ID: "1", Type: "learning", CuratedAt: "2026-04-01T10:00:00Z"},
		{ID: "2", Type: "learning", CuratedAt: "2026-04-02T10:00:00Z"},
		{ID: "3", Type: "pattern", CuratedAt: "2026-04-03T10:00:00Z"},
	}

	for _, a := range artifacts {
		data, _ := json.Marshal(a)
		if err := os.WriteFile(filepath.Join(dir, a.ID+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also add a non-JSON file to verify it's skipped
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("skip me"), 0644)

	counts, latest := countArtifactsInDir(dir)
	if counts["learning"] != 2 {
		t.Errorf("learning count = %d, want 2", counts["learning"])
	}
	if counts["pattern"] != 1 {
		t.Errorf("pattern count = %d, want 1", counts["pattern"])
	}

	expectedLatest, _ := time.Parse(time.RFC3339, "2026-04-03T10:00:00Z")
	if !latest.Equal(expectedLatest) {
		t.Errorf("latest = %v, want %v", latest, expectedLatest)
	}
}

// ---------------------------------------------------------------------------
// runCurateStatus
// ---------------------------------------------------------------------------

func TestRunCurateStatus_Human(t *testing.T) {
	tmp := chdirTemp(t)
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	// Setup minimal .agents structure with some artifacts
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	patternsDir := filepath.Join(tmp, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	l1, _ := json.Marshal(curateArtifact{ID: "l1", Type: "learning", CuratedAt: "2026-04-01T10:00:00Z"})
	l2, _ := json.Marshal(curateArtifact{ID: "l2", Type: "decision", CuratedAt: "2026-04-02T10:00:00Z"})
	p1, _ := json.Marshal(curateArtifact{ID: "p1", Type: "pattern", CuratedAt: "2026-04-03T10:00:00Z"})

	os.WriteFile(filepath.Join(learningsDir, "l1.json"), l1, 0644)
	os.WriteFile(filepath.Join(learningsDir, "l2.json"), l2, 0644)
	os.WriteFile(filepath.Join(patternsDir, "p1.json"), p1, 0644)

	out, err := captureStdout(t, func() error {
		return runCurateStatus(nil, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Learnings:") {
		t.Errorf("missing learnings label, got: %q", out)
	}
	if !strings.Contains(out, "Patterns:") {
		t.Errorf("missing patterns label, got: %q", out)
	}
	if !strings.Contains(out, "Total:") {
		t.Errorf("missing total label, got: %q", out)
	}
}

func TestRunCurateStatus_JSON(t *testing.T) {
	tmp := chdirTemp(t)
	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	os.MkdirAll(filepath.Join(tmp, ".agents", "learnings"), 0755)
	os.MkdirAll(filepath.Join(tmp, ".agents", "patterns"), 0755)

	out, err := captureStdout(t, func() error {
		return runCurateStatus(nil, nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed curateStatusResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if parsed.Total != 0 {
		t.Errorf("Total = %d, want 0 for empty dirs", parsed.Total)
	}
}

func TestCurateStatusLastVerifyAt_LatestJSONOnly(t *testing.T) {
	dir := t.TempDir()
	older := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 4, 2, 9, 30, 0, 0, time.UTC)

	oldPath := filepath.Join(dir, "old.json")
	newPath := filepath.Join(dir, "new.json")
	ignoredPath := filepath.Join(dir, "notes.txt")
	for _, path := range []string{oldPath, newPath, ignoredPath} {
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := os.Chtimes(oldPath, older, older); err != nil {
		t.Fatalf("chtimes old.json: %v", err)
	}
	if err := os.Chtimes(newPath, newer, newer); err != nil {
		t.Fatalf("chtimes new.json: %v", err)
	}
	if err := os.Chtimes(ignoredPath, newer.Add(time.Hour), newer.Add(time.Hour)); err != nil {
		t.Fatalf("chtimes notes.txt: %v", err)
	}

	got := curateStatusLastVerifyAt(dir)
	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("parse last verify timestamp %q: %v", got, err)
	}
	if !parsed.Equal(newer) {
		t.Fatalf("curateStatusLastVerifyAt() = %v, want %v", parsed, newer)
	}
}

// ---------------------------------------------------------------------------
// detectVerifyRegressions
// ---------------------------------------------------------------------------

func TestDetectVerifyRegressions_NoBaseline(t *testing.T) {
	tmp := t.TempDir()
	// No baseline dir exists
	regressions, err := detectVerifyRegressions(filepath.Join(tmp, "nonexistent"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(regressions) != 0 {
		t.Errorf("regressions = %v, want empty (no baseline)", regressions)
	}
}

func TestCountArtifactsInDir_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0644)
	os.WriteFile(filepath.Join(dir, "empty.json"), []byte(""), 0644)

	counts, latest := countArtifactsInDir(dir)
	if len(counts) != 0 {
		t.Errorf("counts = %v, want empty for malformed JSON", counts)
	}
	if !latest.IsZero() {
		t.Errorf("latest = %v, want zero for malformed JSON", latest)
	}
}
