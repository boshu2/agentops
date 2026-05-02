package main

import (
	"path/filepath"
	"testing"
)

func TestSearchEvalBackendAgenticRGPathTokenSearch(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/research/path-token-note.md", "body intentionally omits the filename terms")

	results, err := searchEvalBackendResults(root, "agentic-rg", "path token note", filepath.Join(root, ".agents", "ao", "sessions"), 5)
	if err != nil {
		t.Fatalf("searchEvalBackendResults: %v", err)
	}

	if !searchEvalResultsContain(results, filepath.Join(root, ".agents", "research", "path-token-note.md")) {
		t.Fatalf("results did not include path-token note: %+v", results)
	}
}

func TestSearchEvalBackendWikiLinkExpand(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/wiki/concepts/source.md", "auth graph mentions [[target-note]]")
	writeSearchEvalFixtureFile(t, root, ".agents/wiki/concepts/target-note.md", "linked target details")

	results, err := searchEvalBackendResults(root, "wiki-link-expand", "auth graph", filepath.Join(root, ".agents", "ao", "sessions"), 5)
	if err != nil {
		t.Fatalf("searchEvalBackendResults: %v", err)
	}

	target := filepath.Join(root, ".agents", "wiki", "concepts", "target-note.md")
	if !searchEvalResultsContain(results, target) {
		t.Fatalf("expanded results did not include linked target %s: %+v", target, results)
	}
}

func TestValidateSearchEvalBackendResultsRejectsMissingPath(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, ".agents", "research", "missing.md")

	err := validateSearchEvalBackendResults(root, []searchResult{{Path: missing}}, []string{root})
	if err == nil {
		t.Fatal("validateSearchEvalBackendResults succeeded, want missing path error")
	}
}

func searchEvalResultsContain(results []searchResult, path string) bool {
	want, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, result := range results {
		got := result.Path
		if !filepath.IsAbs(got) {
			got = filepath.Join(".", got)
		}
		abs, err := filepath.Abs(got)
		if err == nil && abs == want {
			return true
		}
	}
	return false
}
