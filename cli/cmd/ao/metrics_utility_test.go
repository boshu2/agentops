// practices: [dora-metrics, sre]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseUtilityFromMarkdownFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "learning.md")
	content := `---
utility: 0.73
---
# Learning
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if got := parseUtilityFromFile(path); got != 0.73 {
		t.Fatalf("parseUtilityFromFile() = %f, want 0.73", got)
	}
}

func TestComputeUtilityMetricsIncludesMarkdownAndPatterns(t *testing.T) {
	baseDir := t.TempDir()
	learningsDir := filepath.Join(baseDir, ".agents", "learnings")
	patternsDir := filepath.Join(baseDir, ".agents", "patterns")
	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(patternsDir, 0755); err != nil {
		t.Fatal(err)
	}

	jsonl := map[string]any{"id": "L1", "utility": 0.8}
	line, err := json.Marshal(jsonl)
	if err != nil {
		t.Fatalf("marshal jsonl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "l1.jsonl"), line, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(learningsDir, "l2.md"), []byte("---\nutility: 0.6\n---\n# L2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patternsDir, "p1.md"), []byte("---\nutility: 0.9\n---\n# P1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stats := computeUtilityMetrics(baseDir)
	if stats.highCount != 2 {
		t.Fatalf("highCount = %d, want 2", stats.highCount)
	}
	if stats.lowCount != 0 {
		t.Fatalf("lowCount = %d, want 0", stats.lowCount)
	}
	if stats.mean < 0.75 || stats.mean > 0.78 {
		t.Fatalf("mean = %f, want approximately 0.766", stats.mean)
	}
}
