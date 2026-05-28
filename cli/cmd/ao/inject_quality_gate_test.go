// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeLearningFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanLearningQuality_WithSourceFiles(t *testing.T) {
	dir := t.TempDir()

	fixture := `---
utility: 0.8
source_bead: test-123
source_phase: validate
---
# Test Learning
Some content here.
`
	writeLearningFixture(t, dir, "learn-1.md", fixture)
	writeLearningFixture(t, dir, "learn-2.md", fixture)
	writeLearningFixture(t, dir, "learn-3.md", fixture)

	report, err := scanLearningQuality(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalLearnings != 3 {
		t.Errorf("TotalLearnings = %d, want 3", report.TotalLearnings)
	}
	if report.WithSource != 3 {
		t.Errorf("WithSource = %d, want 3", report.WithSource)
	}
	if report.WithoutSource != 0 {
		t.Errorf("WithoutSource = %d, want 0", report.WithoutSource)
	}
	if report.Score < 0.99 {
		t.Errorf("Score = %f, want close to 1.0", report.Score)
	}
}

func TestScanLearningQuality_WithoutSourceFiles(t *testing.T) {
	dir := t.TempDir()

	fixture := `---
utility: 0.5
---
# Unsourced Learning
No provenance.
`
	// Write files and backdate them so they're stale (>90 days)
	for _, name := range []string{"learn-1.md", "learn-2.md", "learn-3.md"} {
		path := writeLearningFixture(t, dir, name, fixture)
		staleTime := time.Now().Add(-120 * 24 * time.Hour)
		if err := os.Chtimes(path, staleTime, staleTime); err != nil {
			t.Fatal(err)
		}
	}

	report, err := scanLearningQuality(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalLearnings != 3 {
		t.Errorf("TotalLearnings = %d, want 3", report.TotalLearnings)
	}
	if report.WithSource != 0 {
		t.Errorf("WithSource = %d, want 0", report.WithSource)
	}
	if report.WithoutSource != 3 {
		t.Errorf("WithoutSource = %d, want 3", report.WithoutSource)
	}
	if report.StaleCount != 3 {
		t.Errorf("StaleCount = %d, want 3", report.StaleCount)
	}
	// sourceRatio=0 so Score should be 0
	if report.Score != 0 {
		t.Errorf("Score = %f, want 0", report.Score)
	}
	if len(report.FlaggedPaths) != 3 {
		t.Errorf("FlaggedPaths len = %d, want 3", len(report.FlaggedPaths))
	}
}

func TestScanLearningQuality_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	report, err := scanLearningQuality(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalLearnings != 0 {
		t.Errorf("TotalLearnings = %d, want 0", report.TotalLearnings)
	}
	if report.Score != 0 {
		t.Errorf("Score = %f, want 0", report.Score)
	}
	if report.FlaggedPaths == nil {
		t.Error("FlaggedPaths should be non-nil empty slice")
	}
}

func TestAssessLearningFile_HasSourceBead(t *testing.T) {
	dir := t.TempDir()
	content := `---
utility: 0.8
source_bead: test-123
source_phase: validate
---
# Test Learning
Some content here.
`
	path := writeLearningFixture(t, dir, "sourced.md", content)

	hasSource, isStale, err := assessLearningFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasSource {
		t.Error("hasSource = false, want true")
	}
	// File was just created, should not be stale
	if isStale {
		t.Error("isStale = true, want false for fresh file")
	}
}

func TestAssessLearningFile_NoSource(t *testing.T) {
	dir := t.TempDir()
	content := `---
utility: 0.5
---
# No Source Learning
Missing provenance.
`
	path := writeLearningFixture(t, dir, "unsourced.md", content)

	hasSource, isStale, err := assessLearningFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasSource {
		t.Error("hasSource = true, want false")
	}
	// File was just created, should not be stale
	if isStale {
		t.Error("isStale = true, want false for fresh file")
	}
}
