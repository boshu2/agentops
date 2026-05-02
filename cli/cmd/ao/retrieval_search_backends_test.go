package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestSearchEvalBackendRerankLlamaCPPSkipsWhenEndpointUnset(t *testing.T) {
	t.Setenv(searchEvalRerankEndpointEnv, "")
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/research/rerank-a.md", "rerank target alpha")

	results, err := searchEvalBackendResults(root, searchEvalBackendRerankLlamaCPP, "rerank target", filepath.Join(root, ".agents", "ao", "sessions"), 5)
	if err != nil {
		t.Fatalf("searchEvalBackendResults: %v", err)
	}
	if !searchEvalResultsContain(results, filepath.Join(root, ".agents", "research", "rerank-a.md")) {
		t.Fatalf("unset endpoint fallback did not return base result: %+v", results)
	}
}

func TestSearchEvalBackendRerankLlamaCPPReordersWithFakeEndpoint(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, ".agents", "research", "rerank-a.md")
	second := filepath.Join(root, ".agents", "research", "rerank-b.md")
	writeSearchEvalFixtureFile(t, root, ".agents/research/rerank-a.md", "rerank target alpha")
	writeSearchEvalFixtureFile(t, root, ".agents/research/rerank-b.md", "rerank target beta")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ranked_paths":[%q,%q]}`, second, first)
	}))
	defer server.Close()
	t.Setenv(searchEvalRerankEndpointEnv, server.URL)

	results, err := searchEvalBackendResults(root, searchEvalBackendRerankLlamaCPP, "rerank target", filepath.Join(root, ".agents", "ao", "sessions"), 5)
	if err != nil {
		t.Fatalf("searchEvalBackendResults: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("results = %d, want at least 2: %+v", len(results), results)
	}
	if results[0].Path != second {
		t.Fatalf("first reranked path = %q, want %q; results=%+v", results[0].Path, second, results)
	}
}

func TestSearchEvalBackendRerankLlamaCPPRejectsUnknownPath(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/research/rerank-a.md", "rerank target alpha")
	unknown := filepath.Join(root, ".agents", "research", "unknown.md")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ranked_paths":[%q]}`, unknown)
	}))
	defer server.Close()
	t.Setenv(searchEvalRerankEndpointEnv, server.URL)

	_, err := searchEvalBackendResults(root, searchEvalBackendRerankLlamaCPP, "rerank target", filepath.Join(root, ".agents", "ao", "sessions"), 5)
	if err == nil {
		t.Fatal("searchEvalBackendResults succeeded, want unknown rerank path error")
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
