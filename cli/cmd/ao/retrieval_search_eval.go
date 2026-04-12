package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/storage"
)

const defaultSearchEvalK = 5

type searchEvalManifest struct {
	ID          string           `json:"id"`
	Description string           `json:"description,omitempty"`
	Queries     []searchEvalCase `json:"queries"`
}

type searchEvalCase struct {
	ID          string   `json:"id"`
	Query       string   `json:"query"`
	Intent      string   `json:"intent,omitempty"`
	GroundTruth []string `json:"ground_truth"`
}

type searchEvalReport struct {
	ID                 string             `json:"id"`
	ManifestPath       string             `json:"manifest_path"`
	SearchRoot         string             `json:"search_root"`
	Queries            int                `json:"queries"`
	K                  int                `json:"k"`
	Hits               int                `json:"hits"`
	MissingGroundTruth int                `json:"missing_ground_truth"`
	AnyRelevantAtK     float64            `json:"any_relevant_at_k"`
	AvgPrecisionAtK    float64            `json:"avg_precision_at_k"`
	Results            []searchEvalResult `json:"results"`
}

type searchEvalResult struct {
	ID                 string   `json:"id"`
	Query              string   `json:"query"`
	Intent             string   `json:"intent,omitempty"`
	GroundTruth        []string `json:"ground_truth"`
	MissingGroundTruth []string `json:"missing_ground_truth,omitempty"`
	ResultPaths        []string `json:"result_paths"`
	HitPaths           []string `json:"hit_paths,omitempty"`
	AnyRelevant        bool     `json:"any_relevant"`
	PrecisionAtK       float64  `json:"precision_at_k"`
}

func runSearchEval(k int, asJSON bool, repoRoot, manifestPath string) error {
	report, err := buildSearchEvalReport(repoRoot, manifestPath, k)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Println("AO Search Retrieval Eval")
	fmt.Println("========================")
	fmt.Printf("Eval set:       %s\n", report.ID)
	fmt.Printf("Manifest:       %s\n", report.ManifestPath)
	fmt.Printf("Search root:    %s\n", report.SearchRoot)
	fmt.Printf("Queries:        %d\n", report.Queries)
	fmt.Printf("K:              %d\n", report.K)
	if report.MissingGroundTruth > 0 {
		fmt.Printf("Missing labels: %d ground-truth path(s)\n", report.MissingGroundTruth)
	}
	fmt.Printf("Any-relevant@%d: %.0f%% (%d/%d)\n", report.K, report.AnyRelevantAtK*100, report.Hits, report.Queries)
	fmt.Printf("Avg precision@%d: %.2f\n", report.K, report.AvgPrecisionAtK)
	fmt.Println()
	fmt.Println("Per-query breakdown:")
	for _, result := range report.Results {
		status := "MISS"
		if result.AnyRelevant {
			status = "HIT"
		}
		fmt.Printf("  %-5s %-4s precision@%d=%.2f  %q\n", result.ID, status, report.K, result.PrecisionAtK, result.Query)
		if len(result.HitPaths) > 0 {
			fmt.Printf("        hits=%v\n", result.HitPaths)
		}
		if len(result.MissingGroundTruth) > 0 {
			fmt.Printf("        missing_ground_truth=%v\n", result.MissingGroundTruth)
		}
		fmt.Printf("        top=%v\n", result.ResultPaths)
	}
	return nil
}

func buildSearchEvalReport(repoRoot, manifestPath string, k int) (searchEvalReport, error) {
	if k <= 0 {
		k = defaultSearchEvalK
	}

	root, err := resolveSearchEvalRoot(repoRoot)
	if err != nil {
		return searchEvalReport{}, err
	}
	manifestFile := resolveSearchEvalManifestPath(root, manifestPath)

	manifest, err := loadSearchEvalManifest(manifestFile)
	if err != nil {
		return searchEvalReport{}, err
	}

	report := searchEvalReport{
		ID:           manifest.ID,
		ManifestPath: manifestFile,
		SearchRoot:   root,
		Queries:      len(manifest.Queries),
		K:            k,
		Results:      make([]searchEvalResult, 0, len(manifest.Queries)),
	}

	sessionsDir := filepath.Join(root, storage.DefaultBaseDir, storage.SessionsDir)
	for _, evalCase := range manifest.Queries {
		result, err := runSearchEvalCase(root, sessionsDir, evalCase, k)
		if err != nil {
			return searchEvalReport{}, err
		}
		report.Results = append(report.Results, result)
		if result.AnyRelevant {
			report.Hits++
		}
		report.MissingGroundTruth += len(result.MissingGroundTruth)
		report.AvgPrecisionAtK += result.PrecisionAtK
	}

	if report.Queries > 0 {
		report.AnyRelevantAtK = float64(report.Hits) / float64(report.Queries)
		report.AvgPrecisionAtK /= float64(report.Queries)
	}

	return report, nil
}

func runSearchEvalCase(repoRoot, sessionsDir string, evalCase searchEvalCase, k int) (searchEvalResult, error) {
	results, err := searchRepoLocalKnowledge(evalCase.Query, sessionsDir, k)
	if err != nil {
		return searchEvalResult{}, fmt.Errorf("search eval case %s: %w", evalCase.ID, err)
	}

	topPaths := make([]string, 0, len(results))
	for _, result := range results {
		topPaths = append(topPaths, normalizeSearchEvalResultPath(repoRoot, result.Path))
	}

	groundTruth := normalizedSearchEvalExpectedPaths(evalCase.GroundTruth)
	missingGroundTruth := missingSearchEvalGroundTruth(repoRoot, groundTruth)

	expected := make(map[string]bool, len(groundTruth))
	for _, path := range groundTruth {
		expected[path] = true
	}

	hitPaths := make([]string, 0)
	for _, path := range topPaths {
		if expected[path] {
			hitPaths = append(hitPaths, path)
		}
	}

	denominator := len(evalCase.GroundTruth)
	if denominator > k {
		denominator = k
	}
	precision := 0.0
	if denominator > 0 {
		precision = float64(len(hitPaths)) / float64(denominator)
	}

	return searchEvalResult{
		ID:                 evalCase.ID,
		Query:              evalCase.Query,
		Intent:             evalCase.Intent,
		GroundTruth:        groundTruth,
		MissingGroundTruth: missingGroundTruth,
		ResultPaths:        topPaths,
		HitPaths:           hitPaths,
		AnyRelevant:        len(hitPaths) > 0,
		PrecisionAtK:       precision,
	}, nil
}

func loadSearchEvalManifest(path string) (searchEvalManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return searchEvalManifest{}, fmt.Errorf("read search eval manifest %s: %w", path, err)
	}

	var manifest searchEvalManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return searchEvalManifest{}, fmt.Errorf("parse search eval manifest %s: %w", path, err)
	}
	if strings.TrimSpace(manifest.ID) == "" {
		return searchEvalManifest{}, fmt.Errorf("search eval manifest %s missing id", path)
	}
	if len(manifest.Queries) == 0 {
		return searchEvalManifest{}, fmt.Errorf("search eval manifest %s has no queries", path)
	}
	for i, evalCase := range manifest.Queries {
		if strings.TrimSpace(evalCase.ID) == "" {
			return searchEvalManifest{}, fmt.Errorf("search eval manifest %s query %d missing id", path, i)
		}
		if strings.TrimSpace(evalCase.Query) == "" {
			return searchEvalManifest{}, fmt.Errorf("search eval manifest %s query %s missing query text", path, evalCase.ID)
		}
		if len(evalCase.GroundTruth) == 0 {
			return searchEvalManifest{}, fmt.Errorf("search eval manifest %s query %s missing ground_truth", path, evalCase.ID)
		}
	}
	return manifest, nil
}

func resolveSearchEvalRoot(repoRoot string) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		repoRoot = "."
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve search root %s: %w", repoRoot, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("search root %s: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("search root %s is not a directory", root)
	}
	return filepath.Clean(root), nil
}

func resolveSearchEvalManifestPath(repoRoot, manifestPath string) string {
	if filepath.IsAbs(manifestPath) {
		return filepath.Clean(manifestPath)
	}
	return filepath.Clean(filepath.Join(repoRoot, manifestPath))
}

func normalizedSearchEvalExpectedPaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		normalized = append(normalized, normalizeSearchEvalExpectedPath(path))
	}
	return normalized
}

func missingSearchEvalGroundTruth(repoRoot string, paths []string) []string {
	missing := make([]string, 0)
	for _, path := range paths {
		candidate := filepath.FromSlash(path)
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(repoRoot, candidate)
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			missing = append(missing, path)
		}
	}
	return missing
}

func normalizeSearchEvalExpectedPath(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
}

func normalizeSearchEvalResultPath(repoRoot, path string) string {
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(repoRoot, path); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
			return filepath.ToSlash(rel)
		}
		return filepath.ToSlash(filepath.Clean(path))
	}
	return normalizeSearchEvalExpectedPath(path)
}
