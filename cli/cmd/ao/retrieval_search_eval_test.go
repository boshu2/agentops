// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildSearchEvalReport_DefaultBackendAndMRR(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/learnings/backend.md", "backend aware reciprocal rank search eval")

	manifestPath := filepath.Join(root, "eval.json")
	writeSearchEvalManifest(t, manifestPath, searchEvalManifest{
		ID: "backend-mrr",
		Queries: []searchEvalCase{
			{
				ID:          "q01",
				Query:       "backend aware reciprocal rank",
				GroundTruth: []string{".agents/learnings/backend.md"},
			},
		},
	})

	report, err := buildSearchEvalReport(root, manifestPath, 5)
	if err != nil {
		t.Fatalf("buildSearchEvalReport: %v", err)
	}

	if report.Backend != defaultSearchEvalBackend {
		t.Fatalf("backend = %q, want %q", report.Backend, defaultSearchEvalBackend)
	}
	if report.MeanReciprocalRank != 1 {
		t.Fatalf("MRR = %.2f, want 1.00; report=%+v", report.MeanReciprocalRank, report)
	}
	result := report.Results[0]
	if result.Backend != defaultSearchEvalBackend {
		t.Fatalf("result backend = %q, want %q", result.Backend, defaultSearchEvalBackend)
	}
	if result.FirstRelevantRank != 1 || result.ReciprocalRank != 1 {
		t.Fatalf("rank metrics = rank %d rr %.2f, want rank 1 rr 1.00", result.FirstRelevantRank, result.ReciprocalRank)
	}
}

func TestBuildSearchEvalComparisonReport_JSONIncludesBackendMetrics(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/learnings/backend.md", "backend comparison deterministic json")

	manifestPath := filepath.Join(root, "eval.json")
	writeSearchEvalManifest(t, manifestPath, searchEvalManifest{
		ID: "backend-comparison",
		Queries: []searchEvalCase{
			{
				ID:          "q01",
				Query:       "backend comparison deterministic",
				GroundTruth: []string{".agents/learnings/backend.md"},
			},
		},
	})

	report, err := buildSearchEvalComparisonReport(root, manifestPath, 5, []string{defaultSearchEvalBackend})
	if err != nil {
		t.Fatalf("buildSearchEvalComparisonReport: %v", err)
	}
	if len(report.Backends) != 1 {
		t.Fatalf("backends = %d, want 1", len(report.Backends))
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal comparison: %v", err)
	}
	jsonText := string(data)
	for _, want := range []string{`"backends"`, `"backend":"local-lexical"`, `"mean_reciprocal_rank":1`} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("comparison JSON missing %s: %s", want, jsonText)
		}
	}
}

func TestResolveSearchEvalRunBackends_ComparisonDeduplicatesAliases(t *testing.T) {
	backends, compare, err := resolveSearchEvalRunBackends(defaultSearchEvalBackend, "local-lexical, local, lexical")
	if err != nil {
		t.Fatalf("resolveSearchEvalRunBackends: %v", err)
	}
	if !compare {
		t.Fatal("compare = false, want true")
	}
	if len(backends) != 1 || backends[0] != defaultSearchEvalBackend {
		t.Fatalf("backends = %v, want [%s]", backends, defaultSearchEvalBackend)
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

func TestLoadSearchEvalManifest_AcceptsLegacyArray(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "legacy.json")
	legacy := `[
  {"query":"parallel session hazards","relevant":["learnings/parallel.md"],"category":"process"},
  {"query":"empty labels are ignored","relevant":[],"category":"tooling"}
]`
	if err := os.WriteFile(manifestPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}

	manifest, err := loadSearchEvalManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadSearchEvalManifest: %v", err)
	}

	if manifest.ID != "legacy-search-eval" {
		t.Fatalf("manifest ID = %q, want legacy-search-eval", manifest.ID)
	}
	if len(manifest.Queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(manifest.Queries))
	}
	got := manifest.Queries[0]
	if got.ID != "q1" || got.Query != "parallel session hazards" || got.Intent != "process" {
		t.Fatalf("legacy query normalization = %+v", got)
	}
	if len(got.GroundTruth) != 1 || got.GroundTruth[0] != "learnings/parallel.md" {
		t.Fatalf("ground truth = %v, want legacy relevant path", got.GroundTruth)
	}
}

func TestBuildSearchEvalReport_NormalizesLegacyAgentsGroundTruth(t *testing.T) {
	root := t.TempDir()
	writeSearchEvalFixtureFile(t, root, ".agents/learnings/parallel.md", "parallel session hazards file changes corruption")

	manifestPath := filepath.Join(root, "eval.json")
	writeSearchEvalManifest(t, manifestPath, searchEvalManifest{
		ID: "legacy-paths",
		Queries: []searchEvalCase{
			{
				ID:          "q01",
				Query:       "parallel session hazards",
				GroundTruth: []string{"learnings/parallel.md"},
			},
		},
	})

	report, err := buildSearchEvalReport(root, manifestPath, 5)
	if err != nil {
		t.Fatalf("buildSearchEvalReport: %v", err)
	}

	if report.MissingGroundTruth != 0 {
		t.Fatalf("missing ground truth = %d, want 0; results=%+v", report.MissingGroundTruth, report.Results)
	}
	if report.Hits != 1 {
		t.Fatalf("hits = %d, want 1; results=%+v", report.Hits, report.Results)
	}
	if got := report.Results[0].GroundTruth; len(got) != 1 || got[0] != ".agents/learnings/parallel.md" {
		t.Fatalf("normalized ground truth = %v, want .agents/learnings/parallel.md", got)
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
