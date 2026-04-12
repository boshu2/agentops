package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSearchEvalReport_FixtureAnyRelevantAtFive(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/patterns/topological-wave-decomposition.md", "topological wave decomposition dependency ordering parallel")
	writeSearchEvalFixtureFile(t, root, ".agents/research/ao-session-mining.md", "ao session mining research inverted index existing")

	manifestPath := filepath.Join(root, ".agents", "rpi", "ao-sessions-eval-queries-v1.json")
	writeSearchEvalManifest(t, manifestPath, searchEvalManifest{
		ID: "fixture-search-eval",
		Queries: []searchEvalCase{
			{
				ID:          "q01",
				Query:       "topological wave decomposition dependency ordering parallel",
				GroundTruth: []string{".agents/patterns/topological-wave-decomposition.md"},
			},
			{
				ID:          "q02",
				Query:       "missing content phrase",
				GroundTruth: []string{".agents/research/missing.md"},
			},
		},
	})

	report, err := buildSearchEvalReport(root, ".agents/rpi/ao-sessions-eval-queries-v1.json", 0)
	if err != nil {
		t.Fatalf("buildSearchEvalReport: %v", err)
	}

	if report.Queries != 2 {
		t.Fatalf("queries = %d, want 2", report.Queries)
	}
	if report.K != 5 {
		t.Fatalf("K = %d, want 5", report.K)
	}
	if report.Hits != 1 {
		t.Fatalf("hits = %d, want 1; report=%+v", report.Hits, report)
	}
	if report.MissingGroundTruth != 1 {
		t.Fatalf("missing ground truth = %d, want 1", report.MissingGroundTruth)
	}
	if report.AnyRelevantAtK != 0.5 {
		t.Fatalf("any relevant = %.2f, want 0.50", report.AnyRelevantAtK)
	}
	if got := report.Results[0].HitPaths; len(got) != 1 || got[0] != ".agents/patterns/topological-wave-decomposition.md" {
		t.Fatalf("q01 hit paths = %v, want topological-wave-decomposition", got)
	}
	if report.Results[1].AnyRelevant {
		t.Fatalf("q02 AnyRelevant = true, want false")
	}
}

func TestLoadSearchEvalManifest_RequiresGroundTruth(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "eval.json")
	writeSearchEvalManifest(t, manifestPath, searchEvalManifest{
		ID: "invalid",
		Queries: []searchEvalCase{
			{ID: "q01", Query: "query without labels"},
		},
	})

	if _, err := loadSearchEvalManifest(manifestPath); err == nil {
		t.Fatal("loadSearchEvalManifest succeeded, want missing ground_truth error")
	}
}

func TestNormalizeSearchEvalResultPath_RelativeToRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".agents", "research", "note.md")

	got := normalizeSearchEvalResultPath(root, path)
	if got != ".agents/research/note.md" {
		t.Fatalf("normalizeSearchEvalResultPath() = %q, want .agents/research/note.md", got)
	}
}

func writeSearchEvalFixtureFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func writeSearchEvalManifest(t *testing.T, path string, manifest searchEvalManifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
