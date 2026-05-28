// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	searchEvalBackendAOAuto         = "ao-auto"
	searchEvalBackendAgenticRG      = "agentic-rg"
	searchEvalBackendWikiLinkExpand = "wiki-link-expand"
	searchEvalBackendRerankLlamaCPP = "rerank-llamacpp"
	searchEvalRerankEndpointEnv     = "AGENTOPS_RETRIEVAL_RERANK_ENDPOINT"
)

func supportedSearchEvalBackends() string {
	return strings.Join([]string{
		defaultSearchEvalBackend,
		searchEvalBackendAOAuto,
		searchEvalBackendAgenticRG,
		searchEvalBackendWikiLinkExpand,
		searchEvalBackendRerankLlamaCPP,
	}, ", ")
}

func searchEvalBackendResults(repoRoot, backend, query, sessionsDir string, limit int) ([]searchResult, error) {
	backend, err := normalizeSearchEvalBackend(backend)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultSearchEvalK
	}

	var results []searchResult
	switch backend {
	case defaultSearchEvalBackend:
		results, err = searchRepoLocalKnowledge(query, sessionsDir, limit)
	case searchEvalBackendAOAuto:
		results, err = searchAgenticRGBackend(query, sessionsDir, limit)
	case searchEvalBackendAgenticRG:
		results, err = searchAgenticRGBackend(query, sessionsDir, limit)
	case searchEvalBackendWikiLinkExpand:
		results, err = searchWikiLinkExpandBackend(repoRoot, query, sessionsDir, limit)
	case searchEvalBackendRerankLlamaCPP:
		results, err = searchRerankLlamaCPPBackend(repoRoot, query, sessionsDir, limit)
	default:
		err = fmt.Errorf("unsupported search eval backend %q", backend)
	}
	if err != nil {
		return nil, err
	}
	if err := validateSearchEvalBackendResults(repoRoot, results, searchEvalAllowedRoots(repoRoot)); err != nil {
		return nil, err
	}
	if backend == searchEvalBackendRerankLlamaCPP {
		return preserveSearchEvalResultOrder(results, limit), nil
	}
	return rankUniqueSearchResults(results, limit), nil
}

func preserveSearchEvalResultOrder(results []searchResult, limit int) []searchResult {
	seen := make(map[string]struct{}, len(results))
	ordered := make([]searchResult, 0, len(results))
	for _, result := range results {
		if _, ok := seen[result.Path]; ok {
			continue
		}
		seen[result.Path] = struct{}{}
		ordered = append(ordered, result)
		if limit > 0 && len(ordered) >= limit {
			break
		}
	}
	return ordered
}

func searchAgenticRGBackend(query, sessionsDir string, limit int) ([]searchResult, error) {
	knowledgeRoot := knowledgeRootFromSessions(sessionsDir)
	results := make([]searchResult, 0)
	for _, surface := range searchEvalKnowledgeSurfaces() {
		results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, surface.subdir, surface.resultType, surface.label, limit)
	}
	results = appendSessionSearchResults(results, query, sessionsDir, limit)
	return rankUniqueSearchResults(results, limit), nil
}

func searchWikiLinkExpandBackend(repoRoot, query, sessionsDir string, limit int) ([]searchResult, error) {
	results, err := searchAgenticRGBackend(query, sessionsDir, limit)
	if err != nil {
		return nil, err
	}
	expanded := append([]searchResult(nil), results...)
	for _, result := range results {
		expanded = append(expanded, expandWikiLinkedResults(repoRoot, result)...)
	}
	return rankUniqueSearchResults(expanded, limit), nil
}

func searchRerankLlamaCPPBackend(repoRoot, query, sessionsDir string, limit int) ([]searchResult, error) {
	results, err := searchAgenticRGBackend(query, sessionsDir, limit)
	if err != nil {
		return nil, err
	}
	if err := validateSearchEvalBackendResults(repoRoot, results, searchEvalAllowedRoots(repoRoot)); err != nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(os.Getenv(searchEvalRerankEndpointEnv))
	if endpoint == "" {
		return results, nil
	}
	return rerankSearchEvalResults(repoRoot, endpoint, query, results, limit)
}

type searchEvalRerankCandidate struct {
	Path    string  `json:"path"`
	Score   float64 `json:"score"`
	Context string  `json:"context,omitempty"`
	Type    string  `json:"type,omitempty"`
}

type searchEvalRerankRequest struct {
	Query      string                      `json:"query"`
	Candidates []searchEvalRerankCandidate `json:"candidates"`
}

type searchEvalRerankResponse struct {
	RankedPaths []string                 `json:"ranked_paths,omitempty"`
	Results     []searchEvalRerankResult `json:"results,omitempty"`
}

type searchEvalRerankResult struct {
	Path  string  `json:"path"`
	Score float64 `json:"score,omitempty"`
}

func rerankSearchEvalResults(repoRoot, endpoint, query string, results []searchResult, limit int) ([]searchResult, error) {
	reqPayload := searchEvalRerankRequest{
		Query:      query,
		Candidates: make([]searchEvalRerankCandidate, 0, len(results)),
	}
	candidates := make(map[string]searchResult, len(results))
	for _, result := range results {
		path, err := normalizeSearchEvalRerankPath(repoRoot, result.Path)
		if err != nil {
			return nil, err
		}
		result.Path = path
		candidates[path] = result
		reqPayload.Candidates = append(reqPayload.Candidates, searchEvalRerankCandidate{
			Path:    path,
			Score:   result.Score,
			Context: result.Context,
			Type:    result.Type,
		})
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("rerank request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rerank request returned HTTP %d", resp.StatusCode)
	}

	var rerankResp searchEvalRerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return nil, fmt.Errorf("decode rerank response: %w", err)
	}
	return applySearchEvalRerankResponse(repoRoot, candidates, results, rerankResp, limit)
}

func applySearchEvalRerankResponse(repoRoot string, candidates map[string]searchResult, original []searchResult, response searchEvalRerankResponse, limit int) ([]searchResult, error) {
	ordered, scores := searchEvalRerankOrder(response)
	if len(ordered) == 0 {
		return original, nil
	}
	used := make(map[string]struct{}, len(ordered))
	reranked := make([]searchResult, 0, len(original))
	for _, rawPath := range ordered {
		path, err := normalizeSearchEvalRerankPath(repoRoot, rawPath)
		if err != nil {
			return nil, err
		}
		result, ok := candidates[path]
		if !ok {
			return nil, fmt.Errorf("rerank response introduced unknown path %s", path)
		}
		if score, ok := scores[rawPath]; ok {
			result.Score = score
		} else if score, ok := scores[path]; ok {
			result.Score = score
		}
		result.Type = searchEvalBackendRerankLlamaCPP
		reranked = append(reranked, result)
		used[path] = struct{}{}
	}
	for _, result := range original {
		if _, ok := used[result.Path]; ok {
			continue
		}
		reranked = append(reranked, result)
	}
	if limit > 0 && len(reranked) > limit {
		return reranked[:limit], nil
	}
	return reranked, nil
}

func searchEvalRerankOrder(response searchEvalRerankResponse) ([]string, map[string]float64) {
	if len(response.RankedPaths) > 0 {
		return response.RankedPaths, nil
	}
	ordered := make([]string, 0, len(response.Results))
	scores := make(map[string]float64, len(response.Results))
	for _, result := range response.Results {
		if strings.TrimSpace(result.Path) == "" {
			continue
		}
		ordered = append(ordered, result.Path)
		scores[result.Path] = result.Score
	}
	return ordered, scores
}

func normalizeSearchEvalRerankPath(repoRoot, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("rerank path is empty")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve rerank path %s: %w", path, err)
	}
	return filepath.Clean(abs), nil
}

type searchEvalKnowledgeSurface struct {
	subdir     string
	resultType string
	label      string
}

func searchEvalKnowledgeSurfaces() []searchEvalKnowledgeSurface {
	return []searchEvalKnowledgeSurface{
		{"learnings", "learning", "learnings"},
		{"patterns", "pattern", "patterns"},
		{"findings", "finding", "findings"},
		{"research", "research", "research"},
		{"compiled", "compiled", "compiled"},
		{"plans", "plan", "plans"},
		{"brainstorm", "brainstorm", "brainstorm"},
		{"council", "council", "council"},
		{"design", "design", "design"},
		{"wiki/sources", "wiki-source", "wiki-sources"},
		{"wiki/synthesis", "wiki-synthesis", "wiki-synthesis"},
		{"wiki/concepts", "wiki-concept", "wiki-concepts"},
	}
}

func expandWikiLinkedResults(repoRoot string, result searchResult) []searchResult {
	path := result.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	links := extractSearchEvalWikiLinks(string(data))
	if len(links) == 0 {
		return nil
	}
	expanded := make([]searchResult, 0, len(links))
	for _, link := range links {
		target, ok := resolveSearchEvalWikiLink(repoRoot, path, link)
		if !ok {
			continue
		}
		expanded = append(expanded, searchResult{
			Path:    target,
			Score:   result.Score * 0.95,
			Context: "wiki link from " + normalizeSearchEvalResultPath(repoRoot, path),
			Type:    searchEvalBackendWikiLinkExpand,
		})
	}
	return expanded
}

func extractSearchEvalWikiLinks(content string) []string {
	var links []string
	for {
		start := strings.Index(content, "[[")
		if start < 0 {
			return links
		}
		content = content[start+2:]
		end := strings.Index(content, "]]")
		if end < 0 {
			return links
		}
		target := normalizeSearchEvalWikiLink(content[:end])
		if target != "" {
			links = append(links, target)
		}
		content = content[end+2:]
	}
}

func normalizeSearchEvalWikiLink(link string) string {
	link = strings.TrimSpace(link)
	if before, _, ok := strings.Cut(link, "|"); ok {
		link = strings.TrimSpace(before)
	}
	if before, _, ok := strings.Cut(link, "#"); ok {
		link = strings.TrimSpace(before)
	}
	return link
}

func resolveSearchEvalWikiLink(repoRoot, sourcePath, link string) (string, bool) {
	if link == "" || filepath.IsAbs(link) {
		return "", false
	}
	target := filepath.FromSlash(link)
	if filepath.Ext(target) == "" {
		target += ".md"
	}

	candidates := []string{filepath.Join(filepath.Dir(sourcePath), target)}
	if strings.HasPrefix(filepath.ToSlash(target), ".agents/") {
		candidates = append(candidates, filepath.Join(repoRoot, target))
	} else if strings.Contains(target, string(filepath.Separator)) {
		candidates = append(candidates, filepath.Join(repoRoot, target))
	} else {
		wikiRoot := filepath.Join(repoRoot, ".agents", "wiki")
		candidates = append(candidates,
			filepath.Join(wikiRoot, "concepts", target),
			filepath.Join(wikiRoot, "synthesis", target),
			filepath.Join(wikiRoot, "sources", target),
		)
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return filepath.Clean(candidate), true
		}
	}
	return "", false
}

func searchEvalAllowedRoots(repoRoot string) []string {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		root = repoRoot
	}
	roots := []string{filepath.Clean(root)}
	for _, configured := range configuredDreamVaultSourceRoots() {
		abs, err := filepath.Abs(configured)
		if err != nil {
			continue
		}
		roots = append(roots, filepath.Clean(abs))
	}
	return roots
}

func validateSearchEvalBackendResults(repoRoot string, results []searchResult, allowedRoots []string) error {
	if len(allowedRoots) == 0 {
		allowedRoots = searchEvalAllowedRoots(repoRoot)
	}
	for i := range results {
		path := strings.TrimSpace(results[i].Path)
		if path == "" {
			return fmt.Errorf("search backend returned empty result path")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(repoRoot, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve backend result path %s: %w", results[i].Path, err)
		}
		abs = filepath.Clean(abs)
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("search backend result path %s: %w", abs, err)
		}
		if info.IsDir() {
			return fmt.Errorf("search backend result path %s is a directory", abs)
		}
		if !pathUnderAnyRoot(abs, allowedRoots) {
			return fmt.Errorf("search backend result path %s is outside allowed roots", abs)
		}
		results[i].Path = abs
	}
	return nil
}

func pathUnderAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if pathWithinRoot(path, root) {
			return true
		}
	}
	return false
}

func pathWithinRoot(path, root string) bool {
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
