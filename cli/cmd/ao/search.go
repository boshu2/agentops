package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/storage"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/boshu2/agentops/cli/pkg/vault"
)

var (
	searchLimit    int
	searchType     string
	searchCiteType string
	searchSession  string
	searchUseSC    bool
	searchUseCASS  bool
	searchUseLocal bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search workspace session history and repo-local knowledge",
	Long: `Search workspace session history and repo-local AgentOps knowledge.

By default, ao search brokers across two backends:
  1. upstream cass search --workspace <cwd> for session history when cass is available
  2. repo-local AgentOps artifacts such as .agents/ao/sessions/, learnings,
     patterns, findings, research, compiled synthesis, and configured Dream
     curator vault/wiki/sources pages when present

Use --cass to require upstream cass only.
Use --local to force repo-local AgentOps search only.
Use --use-sc to try Smart Connections semantic search first when Obsidian is
available. If Smart Connections is unavailable or fails, ao search falls back
to the selected non-Smart-Connections backend chain.

Use ao lookup when you specifically want curated learnings, patterns, and
findings by relevance.`,
	Example: `  ao search "mutex pattern"
  ao search "authentication" --limit 20
  ao search "database migration" --type decisions
  ao search "config" --use-sc
  ao search "auth" --cass
  ao search "auth" --local`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.GroupID = "knowledge"
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "Maximum results to return")
	searchCmd.Flags().StringVar(&searchType, "type", "", "Filter by type: session(s), learning(s), pattern(s), finding(s), research, compiled, vault-source(s), decision(s), knowledge")
	searchCmd.Flags().StringVar(&searchCiteType, "cite", "", "Optional citation type to record for matching repo-local artifacts: retrieved, reference, applied")
	searchCmd.Flags().StringVar(&searchSession, "session", "", "Session ID for citation tracking (defaults to the active runtime session)")
	searchCmd.Flags().BoolVar(&searchUseSC, "use-sc", false, "Try Smart Connections semantic search first (requires Obsidian)")
	searchCmd.Flags().BoolVar(&searchUseCASS, "cass", false, "Require upstream cass session-history search")
	searchCmd.Flags().BoolVar(&searchUseLocal, "local", false, "Force repo-local AgentOps search only")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	if GetDryRun() {
		fmt.Printf("[dry-run] Would search for: %s\n", query)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	baseDir := filepath.Join(cwd, storage.DefaultBaseDir)
	sessionsDir := filepath.Join(baseDir, storage.SessionsDir)

	if searchUseCASS && searchUseLocal {
		return fmt.Errorf("--cass and --local are mutually exclusive")
	}

	if searchUseLocal && !searchDataExists(sessionsDir) {
		if GetOutput() == "json" {
			return outputSearchResults(query, []searchResult{})
		}
		fmt.Println("No repo-local AgentOps search data found.")
		fmt.Println("Run 'ao init' and forge transcripts or knowledge into this repo first.")
		return nil
	}

	results, err := selectAndSearch(query, sessionsDir, searchLimit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Filter by type if specified
	if searchType != "" {
		results = filterByType(results, searchType)
	}

	if len(results) == 0 {
		if GetOutput() == "json" {
			return outputSearchResults(query, []searchResult{})
		}
		fmt.Printf("No results found for: %s\n", query)
		return nil
	}

	// Limit results
	if len(results) > searchLimit {
		results = results[:searchLimit]
	}

	if citationType := canonicalCitationType(searchCiteType); citationType != "" {
		recordSearchCitations(cwd, results, resolveSessionID(searchSession), query, citationType)
	}

	return outputSearchResults(query, results)
}

func outputSearchResults(query string, results []searchResult) error {
	if GetOutput() == "json" {
		if results == nil {
			results = []searchResult{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	displaySearchResults(query, results)
	return nil
}

func recordSearchCitations(cwd string, results []searchResult, sessionID, query, citationType string) {
	for _, result := range results {
		if !isRetrievableArtifactPath(cwd, result.Path) {
			continue
		}
		event := types.CitationEvent{
			ArtifactPath:    canonicalArtifactPath(cwd, result.Path),
			SessionID:       sessionID,
			CitedAt:         time.Now(),
			CitationType:    citationType,
			Query:           query,
			MetricNamespace: defaultCitationMetricNamespace(),
		}
		if err := ratchet.RecordCitation(cwd, event); err != nil {
			VerbosePrintf("Warning: record citation for %s: %v\n", result.Path, err)
		}
	}
}

// selectAndSearch chooses the search backend and executes the search.
// Default: upstream cass plus repo-local AgentOps artifacts. Optional:
// Smart Connections with --use-sc flag.
func selectAndSearch(query, sessionsDir string, limit int) ([]searchResult, error) {
	if searchUseCASS && searchUseLocal {
		return nil, fmt.Errorf("--cass and --local are mutually exclusive")
	}

	if searchUseSC {
		vaultPath := vault.DetectVault("")
		if vaultPath != "" && vault.HasSmartConnections(vaultPath) {
			VerbosePrintf("Using Smart Connections for semantic search...\n")
			results, err := searchSmartConnections(query, sessionsDir, limit)
			if err != nil {
				VerbosePrintf("Smart Connections failed, falling back to CASS: %v\n", err)
				return searchCASS(query, sessionsDir, limit)
			}
			return results, nil
		}
		VerbosePrintf("Smart Connections not available, using configured search backends...\n")
	}

	if searchUseLocal {
		VerbosePrintf("Using repo-local AgentOps search only...\n")
		return searchRepoLocalKnowledge(query, sessionsDir, limit)
	}

	if searchUseCASS {
		VerbosePrintf("Using upstream cass search...\n")
		return searchUpstreamCASS(query, limit)
	}

	VerbosePrintf("Using upstream cass plus repo-local AgentOps search...\n")
	return searchAuto(query, sessionsDir, limit)
}

func searchAuto(query, sessionsDir string, limit int) ([]searchResult, error) {
	results := make([]searchResult, 0)
	hasLocalData := searchDataExists(sessionsDir)
	var cassErr error

	cassResults, err := searchUpstreamCASS(query, limit)
	if err != nil {
		cassErr = err
		VerbosePrintf("cass search unavailable, falling back to repo-local AgentOps data: %v\n", err)
	} else {
		results = append(results, cassResults...)
	}

	if hasLocalData {
		localResults, err := searchRepoLocalKnowledge(query, sessionsDir, limit)
		if err != nil {
			if cassErr != nil {
				return nil, fmt.Errorf("cass search failed (%v) and repo-local search failed: %w", cassErr, err)
			}
			return nil, err
		}
		results = append(results, localResults...)
	}

	if len(results) == 0 && !hasLocalData {
		if errors.Is(cassErr, exec.ErrNotFound) {
			return []searchResult{}, nil
		}
		if cassErr != nil {
			return nil, cassErr
		}
	}

	return normalizeSearchResults(results, limit), nil
}

type cassSearchResponse struct {
	Hits []cassSearchHit `json:"hits"`
}

type cassSearchHit struct {
	SourcePath string  `json:"source_path"`
	Score      float64 `json:"score"`
	Snippet    string  `json:"snippet"`
	Content    string  `json:"content"`
}

func searchUpstreamCASS(query string, limit int) ([]searchResult, error) {
	if _, err := exec.LookPath("cass"); err != nil {
		return nil, fmt.Errorf("cass not found on PATH: %w", err)
	}

	workspace, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	args := []string{"search", "--json", "--workspace", workspace}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	args = append(args, query)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cass", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cass search failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var response cassSearchResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse cass search response: %w", err)
	}

	results := make([]searchResult, 0, len(response.Hits))
	for _, hit := range response.Hits {
		context := hit.Snippet
		if context == "" {
			context = hit.Content
		}
		results = append(results, searchResult{
			Path:    hit.SourcePath,
			Score:   hit.Score,
			Context: truncateContext(strings.TrimSpace(context)),
			Type:    "session",
		})
	}

	return normalizeSearchResults(results, limit), nil
}

func normalizeSearchResults(results []searchResult, limit int) []searchResult {
	return search.NormalizeSearchResults(results, limit)
}

func searchDataExists(sessionsDir string) bool {
	return search.SearchDataExists(sessionsDir) || len(configuredDreamVaultSourceRoots()) > 0
}

func knowledgeRootFromSessions(sessionsDir string) string {
	return search.KnowledgeRootFromSessions(sessionsDir)
}

// displaySearchResults formats and prints search results to stdout.
func displaySearchResults(query string, results []searchResult) {
	fmt.Printf("Found %d result(s) for: %s\n\n", len(results), query)

	for i, r := range results {
		fmt.Printf("%d. %s\n", i+1, r.Path)
		if r.Context != "" {
			lines := strings.Split(r.Context, "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Printf("   %s\n", line)
				}
			}
		}
		fmt.Println()
	}
}

// searchResult is a local alias for search.SearchResult.
type searchResult = search.SearchResult

// searchFiles performs grep-based search on markdown and JSONL files.
func searchFiles(query string, dir string, limit int) ([]searchResult, error) {
	var results []searchResult

	// Search markdown files
	mdResults, err := grepFiles(query, dir, "*.md", limit)
	if err != nil {
		return nil, err
	}
	results = append(results, mdResults...)

	// Search JSONL files
	jsonlResults, err := searchJSONL(query, dir, limit)
	if err != nil {
		return nil, err
	}
	results = append(results, jsonlResults...)

	// Dedupe by path
	seen := make(map[string]bool)
	unique := make([]searchResult, 0)
	for _, r := range results {
		if !seen[r.Path] {
			seen[r.Path] = true
			unique = append(unique, r)
		}
	}

	// Enforce combined result limit after deduplication
	if limit > 0 && len(unique) > limit {
		unique = unique[:limit]
	}

	return unique, nil
}

// grepFiles uses grep to search files.
func grepFiles(query, dir, pattern string, limit int) ([]searchResult, error) {
	cmd, useRipgrep := buildGrepCommand(query, dir, pattern)

	output, err := executeGrepWithFallback(cmd, useRipgrep, query, dir)
	if err != nil {
		return nil, err
	}
	if output == nil {
		return nil, nil
	}

	return parseGrepResults(output, pattern, query, useRipgrep), nil
}

// buildGrepCommand creates the grep/ripgrep command.
func buildGrepCommand(query, dir, pattern string) (*exec.Cmd, bool) {
	return search.BuildGrepCommand(query, dir, pattern)
}

// executeGrepWithFallback runs the grep command with retry logic.
func executeGrepWithFallback(cmd *exec.Cmd, useRipgrep bool, query, dir string) ([]byte, error) {
	return search.ExecuteGrepWithFallback(cmd, useRipgrep, query, dir, VerbosePrintf)
}

// parseGrepResults converts grep output lines into search results.
func parseGrepResults(output []byte, pattern, query string, useRipgrep bool) []searchResult {
	return search.ParseGrepResults(output, pattern, query, useRipgrep)
}

// getFileContext gets context around a match in a file.
func getFileContext(path, query string) string {
	return search.GetFileContext(path, query)
}

// searchJSONL searches JSONL files using jq-like parsing.
func searchJSONL(query string, dir string, limit int) ([]searchResult, error) {
	return search.SearchJSONL(query, dir, limit)
}

func parseJSONLMatch(line, file string) (searchResult, bool) {
	return search.ParseJSONLMatch(line, file)
}

// searchSmartConnections uses Smart Connections HTTP API for semantic search.
// Smart Connections exposes an HTTP API at localhost:37042 when Obsidian is running.
// Falls back to file-based search if not available.
func searchSmartConnections(query, dir string, limit int) ([]searchResult, error) {
	// Smart Connections HTTP API endpoint
	const scAPIBase = "http://localhost:37042"

	// Try to connect to Smart Connections API
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Build search request
	searchURL := fmt.Sprintf("%s/search?query=%s&limit=%d",
		scAPIBase, url.QueryEscape(query), limit)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build search request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		// API not available - fall back to file search
		VerbosePrintf("Smart Connections API not available: %v\n", err)
		return nil, fmt.Errorf("smart connections not running: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck // HTTP response body close best-effort
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("smart connections API error: %s", resp.Status)
	}

	// Parse response
	var scResponse struct {
		Results []struct {
			Path    string  `json:"path"`
			Score   float64 `json:"score"`
			Content string  `json:"content,omitempty"`
			Title   string  `json:"title,omitempty"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scResponse); err != nil {
		return nil, fmt.Errorf("parse Smart Connections response: %w", err)
	}

	// Convert to searchResult format
	results := make([]searchResult, 0, len(scResponse.Results))
	for _, r := range scResponse.Results {
		context := r.Content
		if context == "" && r.Title != "" {
			context = r.Title
		}
		if len(context) > search.ContextLineMaxLength {
			context = context[:search.ContextLineMaxLength] + "..."
		}

		results = append(results, searchResult{
			Path:    r.Path,
			Score:   r.Score,
			Context: context,
			Type:    classifyResultType(r.Path),
		})
	}

	return results, nil
}

// classifyResultType determines the knowledge type based on file path.
func classifyResultType(path string) string {
	return search.ClassifyResultType(path)
}

// searchRepoLocalKnowledge performs AO's repo-local search over forged sessions
// and adjacent .agents knowledge surfaces with maturity weighting for learnings.
// This searches learnings and patterns with awareness of:
// 1. Session context (what was the session about)
// 2. Maturity level (provisional vs established)
// 3. Confidence decay (older untested learnings rank lower)
func searchRepoLocalKnowledge(query, dir string, limit int) ([]searchResult, error) {
	var results []searchResult
	knowledgeRoot := knowledgeRootFromSessions(dir)

	results = appendLearningSearchResults(results, query, knowledgeRoot, limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "patterns", "pattern", "patterns", limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "findings", "finding", "findings", limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "research", "research", "research", limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "compiled", "compiled", "compiled", limit)
	results = appendDreamVaultSourceSearch(results, query, limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "plans", "plan", "plans", limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "brainstorm", "brainstorm", "brainstorm", limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "council", "council", "council", limit)
	results = appendKnowledgeMarkdownSearch(results, query, knowledgeRoot, "design", "design", "design", limit)
	results = appendSessionSearchResults(results, query, dir, limit)

	return rankUniqueSearchResults(results, limit), nil
}

func appendLearningSearchResults(results []searchResult, query, knowledgeRoot string, limit int) []searchResult {
	learningsDir := filepath.Join(knowledgeRoot, "learnings")
	if _, err := os.Stat(learningsDir); err != nil {
		return results
	}
	learningResults, err := searchLearningsWithMaturity(query, learningsDir, limit)
	if err != nil {
		VerbosePrintf("CASS learnings search error: %v\n", err)
	}
	learningResults = append(learningResults, searchMarkdownFilesByTokens(query, learningsDir, "learning", limit)...)
	return append(results, learningResults...)
}

func appendKnowledgeMarkdownSearch(results []searchResult, query, knowledgeRoot, subdir, resultType, label string, limit int) []searchResult {
	dir := filepath.Join(knowledgeRoot, subdir)
	if _, err := os.Stat(dir); err != nil {
		return results
	}
	found, err := grepFiles(query, dir, "*.md", limit)
	if err != nil {
		VerbosePrintf("CASS %s search error: %v\n", label, err)
	}
	for i := range found {
		found[i].Type = resultType
	}
	found = append(found, searchMarkdownFilesByTokens(query, dir, resultType, limit)...)
	return append(results, found...)
}

func appendDreamVaultSourceSearch(results []searchResult, query string, limit int) []searchResult {
	for _, dir := range configuredDreamVaultSourceRoots() {
		found, err := grepFiles(query, dir, "*.md", limit)
		if err != nil {
			VerbosePrintf("Dream vault source search error: %v\n", err)
		}
		for i := range found {
			found[i].Type = "vault-source"
		}
		found = append(found, searchMarkdownFilesByTokens(query, dir, "vault-source", limit)...)
		results = append(results, found...)
	}
	return results
}

func configuredDreamVaultSourceRoots() []string {
	resolved := config.Resolve("", "", false)
	vaultDir, _ := resolved.DreamCuratorVaultDir.Value.(string)
	vaultDir = expandConfiguredSearchPath(vaultDir)
	if vaultDir == "" {
		return nil
	}
	sourceDir := filepath.Join(vaultDir, "wiki", "sources")
	if info, err := os.Stat(sourceDir); err == nil && info.IsDir() {
		return []string{sourceDir}
	}
	return nil
}

func expandConfiguredSearchPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func appendSessionSearchResults(results []searchResult, query, dir string, limit int) []searchResult {
	if _, err := os.Stat(dir); err != nil {
		return results
	}
	sessionResults, err := searchFiles(query, dir, limit)
	if err != nil {
		VerbosePrintf("CASS sessions search error: %v\n", err)
		return results
	}
	return append(results, sessionResults...)
}

func searchMarkdownFilesByTokens(query, dir, resultType string, limit int) []searchResult {
	tokens := searchFallbackTokens(query)
	if len(tokens) == 0 {
		return nil
	}
	minMatches := searchFallbackMinMatches(len(tokens))

	results := make([]searchResult, 0)
	root, err := os.OpenRoot(dir)
	if err != nil {
		VerbosePrintf("token fallback search root error for %s: %v\n", dir, err)
		return nil
	}
	defer root.Close()

	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
			return nil
		}
		data, err := root.ReadFile(relPath)
		if err != nil {
			return nil
		}
		content := strings.ToLower(string(data))
		searchable := content + " " + strings.ToLower(filepath.Base(path))
		matches := 0
		for _, token := range tokens {
			if strings.Contains(searchable, token) {
				matches++
			}
		}
		if matches < minMatches {
			return nil
		}
		results = append(results, searchResult{
			Path:    path,
			Score:   float64(matches) / float64(len(tokens)),
			Context: searchFallbackContext(string(data), tokens),
			Type:    resultType,
		})
		return nil
	}); err != nil {
		VerbosePrintf("token fallback search error for %s: %v\n", dir, err)
		return nil
	}

	slices.SortFunc(results, func(a, b searchResult) int {
		if cmp := cmp.Compare(b.Score, a.Score); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Path, b.Path)
	})
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func searchFallbackTokens(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	seen := make(map[string]bool, len(fields))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) < 2 || seen[field] {
			continue
		}
		seen[field] = true
		tokens = append(tokens, field)
	}
	return tokens
}

func searchFallbackMinMatches(tokenCount int) int {
	if tokenCount <= 1 {
		return 1
	}
	minMatches := (tokenCount + 1) / 2
	if minMatches < 2 {
		return 2
	}
	return minMatches
}

func searchFallbackContext(content string, tokens []string) string {
	context := make([]string, 0, search.MaxContextLines)
	for _, line := range strings.Split(content, "\n") {
		lineLower := strings.ToLower(line)
		for _, token := range tokens {
			if !strings.Contains(lineLower, token) {
				continue
			}
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > search.ContextLineMaxLength {
				trimmed = trimmed[:search.ContextLineMaxLength] + "..."
			}
			context = append(context, trimmed)
			break
		}
		if len(context) >= search.MaxContextLines {
			break
		}
	}
	return strings.Join(context, "\n")
}

func rankUniqueSearchResults(results []searchResult, limit int) []searchResult {
	slices.SortFunc(results, func(a, b searchResult) int {
		if cmp := cmp.Compare(b.Score, a.Score); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Path, b.Path)
	})

	seen := make(map[string]bool, len(results))
	unique := make([]searchResult, 0, len(results))
	for _, result := range results {
		if seen[result.Path] {
			continue
		}
		seen[result.Path] = true
		unique = append(unique, result)
	}

	// Limit results
	if limit > 0 && len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}

func searchCASS(query, dir string, limit int) ([]searchResult, error) {
	return searchRepoLocalKnowledge(query, dir, limit)
}

// searchLearningsWithMaturity searches learnings and weights by maturity and confidence.
func searchLearningsWithMaturity(query, dir string, limit int) ([]searchResult, error) {
	return search.SearchLearningsWithMaturity(query, dir, limit)
}

func truncateContext(s string) string {
	return search.TruncateContext(s)
}

func parseLearningMatch(line, file string) (searchResult, bool) {
	return search.ParseLearningMatch(line, file)
}

func extractLearningContext(data map[string]any) string {
	return search.ExtractLearningContext(data)
}

func calculateCASSScore(data map[string]any) float64 {
	return search.CalculateCASSScore(data)
}

var maturityWeights = search.MaturityWeights

func maturityToWeight(data map[string]any) float64 {
	return search.MaturityToWeight(data)
}

// filterByType filters results by knowledge type.
func filterByType(results []searchResult, filterType string) []searchResult {
	return search.FilterByType(results, filterType)
}

func normalizeSearchType(filterType string) string {
	return search.NormalizeSearchType(filterType)
}
