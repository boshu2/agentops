package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/storage"
)

const (
	defaultSearchEvalK       = 5
	defaultSearchEvalBackend = "local-lexical"
)

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

type legacySearchEvalCase struct {
	Query    string   `json:"query"`
	Relevant []string `json:"relevant"`
	Category string   `json:"category,omitempty"`
}

type searchEvalReport struct {
	ID                 string             `json:"id"`
	Backend            string             `json:"backend"`
	ManifestPath       string             `json:"manifest_path"`
	SearchRoot         string             `json:"search_root"`
	Queries            int                `json:"queries"`
	K                  int                `json:"k"`
	Hits               int                `json:"hits"`
	MissingGroundTruth int                `json:"missing_ground_truth"`
	AnyRelevantAtK     float64            `json:"any_relevant_at_k"`
	AvgPrecisionAtK    float64            `json:"avg_precision_at_k"`
	MeanReciprocalRank float64            `json:"mean_reciprocal_rank"`
	Results            []searchEvalResult `json:"results"`
}

type searchEvalComparisonReport struct {
	ID           string             `json:"id"`
	ManifestPath string             `json:"manifest_path"`
	SearchRoot   string             `json:"search_root"`
	Queries      int                `json:"queries"`
	K            int                `json:"k"`
	Backends     []searchEvalReport `json:"backends"`
}

type searchEvalResult struct {
	ID                 string   `json:"id"`
	Backend            string   `json:"backend"`
	Query              string   `json:"query"`
	Intent             string   `json:"intent,omitempty"`
	GroundTruth        []string `json:"ground_truth"`
	MissingGroundTruth []string `json:"missing_ground_truth,omitempty"`
	ResultPaths        []string `json:"result_paths"`
	HitPaths           []string `json:"hit_paths,omitempty"`
	AnyRelevant        bool     `json:"any_relevant"`
	PrecisionAtK       float64  `json:"precision_at_k"`
	FirstRelevantRank  int      `json:"first_relevant_rank,omitempty"`
	ReciprocalRank     float64  `json:"reciprocal_rank"`
}

func runSearchEval(k int, asJSON bool, repoRoot, manifestPath, backend, compareBackends string) error {
	backends, compare, err := resolveSearchEvalRunBackends(backend, compareBackends)
	if err != nil {
		return err
	}
	if compare {
		report, err := buildSearchEvalComparisonReport(repoRoot, manifestPath, k, backends)
		if err != nil {
			return err
		}
		if asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printSearchEvalComparisonReport(report)
		return nil
	}

	report, err := buildSearchEvalReportForBackend(repoRoot, manifestPath, k, backends[0])
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
	fmt.Printf("Backend:        %s\n", report.Backend)
	fmt.Printf("Manifest:       %s\n", report.ManifestPath)
	fmt.Printf("Search root:    %s\n", report.SearchRoot)
	fmt.Printf("Queries:        %d\n", report.Queries)
	fmt.Printf("K:              %d\n", report.K)
	if report.MissingGroundTruth > 0 {
		fmt.Printf("Missing labels: %d ground-truth path(s)\n", report.MissingGroundTruth)
	}
	fmt.Printf("Any-relevant@%d: %.0f%% (%d/%d)\n", report.K, report.AnyRelevantAtK*100, report.Hits, report.Queries)
	fmt.Printf("Avg precision@%d: %.2f\n", report.K, report.AvgPrecisionAtK)
	fmt.Printf("MRR:            %.2f\n", report.MeanReciprocalRank)
	fmt.Println()
	fmt.Println("Per-query breakdown:")
	for _, result := range report.Results {
		status := "MISS"
		if result.AnyRelevant {
			status = "HIT"
		}
		fmt.Printf("  %-5s %-4s precision@%d=%.2f mrr=%.2f  %q\n", result.ID, status, report.K, result.PrecisionAtK, result.ReciprocalRank, result.Query)
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
	return buildSearchEvalReportForBackend(repoRoot, manifestPath, k, defaultSearchEvalBackend)
}

func buildSearchEvalReportForBackend(repoRoot, manifestPath string, k int, backend string) (searchEvalReport, error) {
	if k <= 0 {
		k = defaultSearchEvalK
	}
	backend, err := normalizeSearchEvalBackend(backend)
	if err != nil {
		return searchEvalReport{}, err
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
		Backend:      backend,
		ManifestPath: manifestFile,
		SearchRoot:   root,
		Queries:      len(manifest.Queries),
		K:            k,
		Results:      make([]searchEvalResult, 0, len(manifest.Queries)),
	}

	sessionsDir := filepath.Join(root, storage.DefaultBaseDir, storage.SessionsDir)
	for _, evalCase := range manifest.Queries {
		result, err := runSearchEvalCase(root, sessionsDir, evalCase, k, backend)
		if err != nil {
			return searchEvalReport{}, err
		}
		report.Results = append(report.Results, result)
		if result.AnyRelevant {
			report.Hits++
		}
		report.MissingGroundTruth += len(result.MissingGroundTruth)
		report.AvgPrecisionAtK += result.PrecisionAtK
		report.MeanReciprocalRank += result.ReciprocalRank
	}

	if report.Queries > 0 {
		report.AnyRelevantAtK = float64(report.Hits) / float64(report.Queries)
		report.AvgPrecisionAtK /= float64(report.Queries)
		report.MeanReciprocalRank /= float64(report.Queries)
	}

	return report, nil
}

func buildSearchEvalComparisonReport(repoRoot, manifestPath string, k int, backends []string) (searchEvalComparisonReport, error) {
	backends, err := normalizeSearchEvalBackends(backends)
	if err != nil {
		return searchEvalComparisonReport{}, err
	}
	reports := make([]searchEvalReport, 0, len(backends))
	for _, backend := range backends {
		report, err := buildSearchEvalReportForBackend(repoRoot, manifestPath, k, backend)
		if err != nil {
			return searchEvalComparisonReport{}, err
		}
		reports = append(reports, report)
	}
	if len(reports) == 0 {
		return searchEvalComparisonReport{}, fmt.Errorf("no search eval backends configured")
	}
	first := reports[0]
	return searchEvalComparisonReport{
		ID:           first.ID,
		ManifestPath: first.ManifestPath,
		SearchRoot:   first.SearchRoot,
		Queries:      first.Queries,
		K:            first.K,
		Backends:     reports,
	}, nil
}

func runSearchEvalCase(repoRoot, sessionsDir string, evalCase searchEvalCase, k int, backend string) (searchEvalResult, error) {
	results, err := searchEvalBackendResults(repoRoot, backend, evalCase.Query, sessionsDir, k)
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
	firstRelevantRank := 0
	for i, path := range topPaths {
		if expected[path] {
			if firstRelevantRank == 0 {
				firstRelevantRank = i + 1
			}
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
	reciprocalRank := 0.0
	if firstRelevantRank > 0 {
		reciprocalRank = 1 / float64(firstRelevantRank)
	}

	return searchEvalResult{
		ID:                 evalCase.ID,
		Backend:            backend,
		Query:              evalCase.Query,
		Intent:             evalCase.Intent,
		GroundTruth:        groundTruth,
		MissingGroundTruth: missingGroundTruth,
		ResultPaths:        topPaths,
		HitPaths:           hitPaths,
		AnyRelevant:        len(hitPaths) > 0,
		PrecisionAtK:       precision,
		FirstRelevantRank:  firstRelevantRank,
		ReciprocalRank:     reciprocalRank,
	}, nil
}

func resolveSearchEvalRunBackends(backend, compareBackends string) ([]string, bool, error) {
	compareBackends = strings.TrimSpace(compareBackends)
	if compareBackends == "" {
		normalized, err := normalizeSearchEvalBackend(backend)
		if err != nil {
			return nil, false, err
		}
		return []string{normalized}, false, nil
	}
	parts := strings.Split(compareBackends, ",")
	backends := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			backends = append(backends, part)
		}
	}
	normalized, err := normalizeSearchEvalBackends(backends)
	if err != nil {
		return nil, true, err
	}
	return normalized, true, nil
}

func normalizeSearchEvalBackends(backends []string) ([]string, error) {
	if len(backends) == 0 {
		backends = []string{defaultSearchEvalBackend}
	}
	seen := make(map[string]struct{}, len(backends))
	normalized := make([]string, 0, len(backends))
	for _, backend := range backends {
		value, err := normalizeSearchEvalBackend(backend)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func normalizeSearchEvalBackend(backend string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "local", "lexical", defaultSearchEvalBackend:
		return defaultSearchEvalBackend, nil
	case searchEvalBackendAOAuto:
		return searchEvalBackendAOAuto, nil
	case searchEvalBackendAgenticRG:
		return searchEvalBackendAgenticRG, nil
	case searchEvalBackendWikiLinkExpand:
		return searchEvalBackendWikiLinkExpand, nil
	default:
		return "", fmt.Errorf("unsupported search eval backend %q: supported backends: %s", backend, supportedSearchEvalBackends())
	}
}

func printSearchEvalComparisonReport(report searchEvalComparisonReport) {
	fmt.Println("AO Search Retrieval Eval Comparison")
	fmt.Println("===================================")
	fmt.Printf("Eval set:       %s\n", report.ID)
	fmt.Printf("Manifest:       %s\n", report.ManifestPath)
	fmt.Printf("Search root:    %s\n", report.SearchRoot)
	fmt.Printf("Queries:        %d\n", report.Queries)
	fmt.Printf("K:              %d\n", report.K)
	fmt.Println()
	fmt.Println("Backends:")
	for _, backend := range report.Backends {
		fmt.Printf("  %-14s any-relevant@%d=%.0f%% hits=%d/%d avg_precision@%d=%.2f mrr=%.2f missing_ground_truth=%d\n",
			backend.Backend,
			backend.K,
			backend.AnyRelevantAtK*100,
			backend.Hits,
			backend.Queries,
			backend.K,
			backend.AvgPrecisionAtK,
			backend.MeanReciprocalRank,
			backend.MissingGroundTruth,
		)
	}
}

func loadSearchEvalManifest(path string) (searchEvalManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return searchEvalManifest{}, fmt.Errorf("read search eval manifest %s: %w", path, err)
	}

	manifest, err := parseSearchEvalManifest(data)
	if err != nil {
		return searchEvalManifest{}, fmt.Errorf("parse search eval manifest %s: %w", path, err)
	}
	return validateSearchEvalManifest(path, manifest)
}

func parseSearchEvalManifest(data []byte) (searchEvalManifest, error) {
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		return parseLegacySearchEvalManifest(data)
	}
	var manifest searchEvalManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return searchEvalManifest{}, err
	}
	return manifest, nil
}

func parseLegacySearchEvalManifest(data []byte) (searchEvalManifest, error) {
	var legacy []legacySearchEvalCase
	if err := json.Unmarshal(data, &legacy); err != nil {
		return searchEvalManifest{}, err
	}
	manifest := searchEvalManifest{
		ID:          "legacy-search-eval",
		Description: "Generated from legacy retrieval eval query array",
		Queries:     make([]searchEvalCase, 0, len(legacy)),
	}
	for i, evalCase := range legacy {
		if len(evalCase.Relevant) == 0 {
			continue
		}
		manifest.Queries = append(manifest.Queries, searchEvalCase{
			ID:          fmt.Sprintf("q%d", i+1),
			Query:       strings.TrimSpace(evalCase.Query),
			Intent:      strings.TrimSpace(evalCase.Category),
			GroundTruth: append([]string(nil), evalCase.Relevant...),
		})
	}
	return manifest, nil
}

func validateSearchEvalManifest(path string, manifest searchEvalManifest) (searchEvalManifest, error) {
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
	normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	if shouldPrefixAgentsKnowledgePath(normalized) {
		return ".agents/" + normalized
	}
	return normalized
}

func shouldPrefixAgentsKnowledgePath(path string) bool {
	if path == "" || path == "." || strings.HasPrefix(path, "/") || strings.HasPrefix(path, ".agents/") {
		return false
	}
	top, _, _ := strings.Cut(path, "/")
	switch top {
	case "learnings", "patterns", "findings", "research", "compiled", "plans", "brainstorm", "council", "design", "wiki":
		return true
	default:
		return false
	}
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
