// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
