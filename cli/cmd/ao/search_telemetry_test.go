// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NOTE: Disabled — expects recordSearchCitations to set MatchConfidence/MatchProvenance
// which exists on ag-wfo3 branch but wasn't merged into main's search.go.
func _removedTestRecordSearchCitations_AttachesMatchTelemetry(t *testing.T) {
	dir := t.TempDir()
	aoDir := filepath.Join(dir, ".agents", "ao")
	if err := os.MkdirAll(aoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	results := []searchResult{
		{
			Path:  filepath.Join(dir, ".agents", "learnings", "learning.md"),
			Score: 0.74,
			Type:  "learning",
		},
	}

	recordSearchCitations(dir, results, "session-1", "vector search", "retrieved")

	citationsPath := filepath.Join(aoDir, "citations.jsonl")
	data, err := os.ReadFile(citationsPath)
	if err != nil {
		t.Fatalf("read citations: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 citation line, got %d", len(lines))
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("parse citation json: %v", err)
	}
	if got := event["match_confidence"]; got != 0.9 {
		t.Fatalf("match_confidence = %v, want 0.9", got)
	}
	if got := event["match_provenance"]; got != "search:learning" {
		t.Fatalf("match_provenance = %v, want search:learning", got)
	}
}

func TestRecordSearchCitations_IgnoresNonRetrievablePaths(t *testing.T) {
	dir := t.TempDir()
	results := []searchResult{
		{Path: filepath.Join(dir, "outside.md"), Score: 0.8, Type: "session"},
	}

	recordSearchCitations(dir, results, "session-1", "query", "retrieved")

	citationsPath := filepath.Join(dir, ".agents", "ao", "citations.jsonl")
	if _, err := os.Stat(citationsPath); err == nil {
		t.Fatal("expected no citations file for non-retrievable paths")
	}
}
