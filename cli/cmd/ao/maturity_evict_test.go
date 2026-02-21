package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestLearningJSONL creates a JSONL learning file with the given first-line JSON data.
func createTestLearningJSONL(t *testing.T, dir, name string, data map[string]interface{}) {
	t.Helper()
	firstLine, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal test data for %s: %v", name, err)
	}
	content := string(firstLine) + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
}

// createTestCitation appends a citation entry to a citations.jsonl file.
func createTestCitation(t *testing.T, citationsPath, artifactPath string, citedAt time.Time) {
	t.Helper()
	entry := map[string]interface{}{
		"artifact_path": artifactPath,
		"cited_at":      citedAt.Format(time.RFC3339),
	}
	line, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal citation: %v", err)
	}
	f, err := os.OpenFile(citationsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open citations file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatalf("write citation: %v", err)
	}
}

func TestEvictIdentifiesCandidate(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a learning that meets ALL 4 eviction criteria:
	// utility < 0.3, confidence < 0.2, not established, no recent citation
	createTestLearningJSONL(t, learningsDir, "stale.jsonl", map[string]interface{}{
		"id":         "L-stale",
		"utility":    0.1,
		"confidence": 0.1,
		"maturity":   "provisional",
	})

	// Empty citations file (no citations at all)
	citationsPath := filepath.Join(aoDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	maturityEvict = true
	maturityArchive = false

	err := runMaturityEvict(nil)
	if err != nil {
		t.Fatalf("runMaturityEvict failed: %v", err)
	}

	// Verify the file is still in learnings (dry-run by default, no --archive)
	if _, err := os.Stat(filepath.Join(learningsDir, "stale.jsonl")); os.IsNotExist(err) {
		t.Error("stale.jsonl should still exist (no --archive flag)")
	}
}

func TestEvictSkipsEstablished(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Low scores but established maturity -> should NOT be evicted
	createTestLearningJSONL(t, learningsDir, "established.jsonl", map[string]interface{}{
		"id":         "L-established",
		"utility":    0.1,
		"confidence": 0.05,
		"maturity":   "established",
	})

	citationsPath := filepath.Join(aoDir, "citations.jsonl")
	if err := os.WriteFile(citationsPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	maturityEvict = true
	maturityArchive = true
	dryRun = false

	err := runMaturityEvict(nil)
	if err != nil {
		t.Fatalf("runMaturityEvict failed: %v", err)
	}

	// Established learning should NOT be archived
	if _, err := os.Stat(filepath.Join(learningsDir, "established.jsonl")); os.IsNotExist(err) {
		t.Error("established.jsonl should NOT be evicted")
	}
}

func TestEvictSkipsRecentlyCited(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	learningFile := "recent-cite.jsonl"

	// Low scores but recently cited -> should NOT be evicted
	createTestLearningJSONL(t, learningsDir, learningFile, map[string]interface{}{
		"id":         "L-recent",
		"utility":    0.1,
		"confidence": 0.05,
		"maturity":   "provisional",
	})

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Resolve the learning path AFTER chdir so it matches what os.Getwd() returns
	// (macOS /var -> /private/var symlink resolution)
	resolvedCwd, _ := os.Getwd()
	learningPath := filepath.Join(resolvedCwd, ".agents", "learnings", learningFile)

	// Cite it 30 days ago (within 90-day window)
	citationsPath := filepath.Join(aoDir, "citations.jsonl")
	createTestCitation(t, citationsPath, learningPath, time.Now().AddDate(0, 0, -30))

	maturityEvict = true
	maturityArchive = true
	dryRun = false

	err := runMaturityEvict(nil)
	if err != nil {
		t.Fatalf("runMaturityEvict failed: %v", err)
	}

	// Recently cited learning should NOT be archived
	if _, err := os.Stat(learningPath); os.IsNotExist(err) {
		t.Error("recent-cite.jsonl should NOT be evicted (cited 30 days ago)")
	}
}

func TestEvictArchivesCandidate(t *testing.T) {
	tmp := t.TempDir()
	learningsDir := filepath.Join(tmp, ".agents", "learnings")
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Meets all criteria: low utility, low confidence, provisional, cited 120 days ago
	learningFile := "evictable.jsonl"
	createTestLearningJSONL(t, learningsDir, learningFile, map[string]interface{}{
		"id":         "L-evictable",
		"utility":    0.15,
		"confidence": 0.1,
		"maturity":   "provisional",
	})

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Resolve path AFTER chdir (macOS /var -> /private/var symlink)
	resolvedCwd, _ := os.Getwd()
	learningPath := filepath.Join(resolvedCwd, ".agents", "learnings", learningFile)

	// Cite it 120 days ago (outside 90-day window)
	citationsPath := filepath.Join(aoDir, "citations.jsonl")
	createTestCitation(t, citationsPath, learningPath, time.Now().AddDate(0, 0, -120))

	maturityEvict = true
	maturityArchive = true
	dryRun = false

	err := runMaturityEvict(nil)
	if err != nil {
		t.Fatalf("runMaturityEvict with archive failed: %v", err)
	}

	// Verify file was moved to archive (use resolved paths for macOS symlinks)
	archiveDir := filepath.Join(resolvedCwd, ".agents", "archive", "learnings")
	if _, err := os.Stat(filepath.Join(archiveDir, learningFile)); os.IsNotExist(err) {
		t.Errorf("expected %s to be archived", learningFile)
	}
	if _, err := os.Stat(learningPath); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed from learnings dir", learningFile)
	}
}

func TestBuildCitationMap(t *testing.T) {
	tmp := t.TempDir()
	aoDir := filepath.Join(tmp, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	citationsPath := filepath.Join(aoDir, "citations.jsonl")

	t1 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)

	// Write entries: two for path-a (t1, t2), one for path-b (t3)
	createTestCitation(t, citationsPath, "/path/to/a.jsonl", t1)
	createTestCitation(t, citationsPath, "/path/to/a.jsonl", t2)
	createTestCitation(t, citationsPath, "/path/to/b.jsonl", t3)

	m := buildCitationMap(tmp)

	if len(m) != 2 {
		t.Fatalf("expected 2 entries in citation map, got %d", len(m))
	}

	// path-a should have the LATEST citation (t2)
	if got := m[canonicalArtifactPath(tmp, "/path/to/a.jsonl")]; !got.Equal(t2) {
		t.Errorf("expected latest citation for path-a to be %v, got %v", t2, got)
	}

	// path-b should have t3
	if got := m[canonicalArtifactPath(tmp, "/path/to/b.jsonl")]; !got.Equal(t3) {
		t.Errorf("expected citation for path-b to be %v, got %v", t3, got)
	}
}

func TestBuildCitationMapMissingFile(t *testing.T) {
	m := buildCitationMap(t.TempDir())
	if len(m) != 0 {
		t.Errorf("expected empty map for missing file, got %d entries", len(m))
	}
}
