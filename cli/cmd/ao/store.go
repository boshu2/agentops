// practices: [wiki-knowledge-surface, event-sourcing-cqrs]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/storage"
)

var (
	storeLimit      int
	storeCategorize bool
)

const (
	IndexFileName = storage.SearchIndexFileName
	IndexDir      = storage.SearchIndexDir
)

type (
	IndexEntry   = storage.SearchIndexEntry
	SearchResult = storage.SearchResult
)

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "STORE phase - index for retrieval",
	Long: `The STORE phase indexes artifacts for retrieval and search.

In the metallurgical metaphor:
  FORGE  → Extract raw knowledge from transcripts
  TEMPER → Validate, harden, and lock for storage
  STORE  → Index for retrieval and search

The store command manages the knowledge index that enables fast
semantic search across all artifacts.

Commands:
  index    Add files to the search index
  search   Query the index
  rebuild  Rebuild index from .agents/`,
}

func init() {
	storeCmd.Hidden = true
	storeCmd.GroupID = "knowledge"
	rootCmd.AddCommand(storeCmd)

	// index subcommand
	indexCmd := &cobra.Command{
		Use:   "index <files...>",
		Short: "Add files to search index",
		Long: `Add artifacts to the search index.

Indexes:
  - Full text content
  - Extracted keywords
  - MemRL utility scores
  - CASS maturity levels

Examples:
  ao store index .agents/learnings/*.md
  ao store index .agents/patterns/error-handling.md
  ao store index --rebuild .agents/`,
		Args: cobra.MinimumNArgs(1),
		RunE: runStoreIndex,
	}
	indexCmd.Flags().BoolVar(&storeCategorize, "categorize", false, "Extract and store category/tags for retrieval")
	storeCmd.AddCommand(indexCmd)

	// search subcommand
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the index",
		Long: `Search for artifacts matching a query.

Returns results ranked by relevance with snippets.

Examples:
  ao store search "mutex pattern"
  ao store search "error handling" --limit 5
  ao store search "authentication" --json`,
		Args: cobra.ExactArgs(1),
		RunE: runStoreSearch,
	}
	searchCmd.Flags().IntVar(&storeLimit, "limit", 10, "Maximum results to return")
	storeCmd.AddCommand(searchCmd)

	// rebuild subcommand
	rebuildCmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild search index",
		Long: `Rebuild the search index from scratch.

Scans all .agents/ directories and re-indexes:
  - learnings/
  - patterns/
  - research/
  - retros/

Examples:
  ao store rebuild
  ao store rebuild --verbose`,
		RunE: runStoreRebuild,
	}
	rebuildCmd.Flags().BoolVar(&storeCategorize, "categorize", false, "Extract and store category/tags for retrieval")
	storeCmd.AddCommand(rebuildCmd)

	// stats subcommand
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show index statistics",
		Long: `Display statistics about the search index.

Shows:
  - Total indexed entries
  - Breakdown by type
  - Index freshness
  - Coverage metrics

Examples:
  ao store stats
  ao store stats --json`,
		RunE: runStoreStats,
	}
	storeCmd.AddCommand(statsCmd)
}

func runStoreIndex(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Expand file patterns
	files, err := expandFilePatterns(cwd, args)
	if err != nil {
		return fmt.Errorf("expand patterns: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found matching patterns")
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would index %d file(s)\n", len(files))
		for _, f := range files {
			fmt.Printf("  - %s\n", f)
		}
		return nil
	}

	indexed := 0
	for _, path := range files {
		entry, err := createIndexEntry(path, storeCategorize)
		if err != nil {
			VerbosePrintf("Warning: skip %s: %v\n", filepath.Base(path), err)
			continue
		}

		if err := appendToIndex(cwd, entry); err != nil {
			VerbosePrintf("Warning: index %s: %v\n", filepath.Base(path), err)
			continue
		}

		indexed++
		VerbosePrintf("Indexed: %s\n", filepath.Base(path))
	}

	fmt.Printf("Indexed %d artifact(s)\n", indexed)
	return nil
}

func runStoreSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	results, err := searchIndex(cwd, query, storeLimit)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(results)

	default:
		printSearchResults(query, results)
	}

	return nil
}

func runStoreRebuild(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if GetDryRun() {
		fmt.Println("[dry-run] Would rebuild search index")
		return nil
	}

	// Remove existing index
	indexPath := filepath.Join(cwd, IndexDir, IndexFileName)
	if err := os.Remove(indexPath); err != nil && !os.IsNotExist(err) {
		VerbosePrintf("Warning: remove old index: %v\n", err)
	}

	files := collectArtifactFiles(cwd)
	indexed := indexFiles(cwd, files, storeCategorize)

	fmt.Printf("Rebuilt index: %d artifacts\n", indexed)
	return nil
}

var artifactSubdirs = storage.ArtifactSubdirs

// collectArtifactFiles walks all artifact subdirectories and returns indexable file paths.
func collectArtifactFiles(cwd string) []string {
	var files []string
	for _, sub := range artifactSubdirs {
		dir := filepath.Join(cwd, ".agents", sub)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		found, err := walkIndexableFiles(dir)
		if err != nil {
			VerbosePrintf("Warning: scan %s: %v\n", dir, err)
		}
		files = append(files, found...)
	}
	return files
}

func walkIndexableFiles(dir string) ([]string, error) {
	return storage.WalkIndexableFiles(dir)
}

// indexFiles creates index entries for each path and appends them to the index.
// Returns the count of successfully indexed files.
func indexFiles(cwd string, files []string, categorize bool) int {
	indexed := 0
	for _, path := range files {
		entry, err := createIndexEntry(path, categorize)
		if err != nil {
			VerbosePrintf("Warning: skip %s: %v\n", filepath.Base(path), err)
			continue
		}
		if err := appendToIndex(cwd, entry); err != nil {
			VerbosePrintf("Warning: index %s: %v\n", filepath.Base(path), err)
			continue
		}
		indexed++
	}
	return indexed
}

func runStoreStats(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	stats, err := computeIndexStats(cwd)
	if err != nil {
		return fmt.Errorf("compute stats: %w", err)
	}

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(stats)

	default:
		printIndexStats(stats)
	}

	return nil
}

type IndexStats = storage.SearchIndexStats

func artifactTypeFromPath(path string) string {
	return storage.ArtifactTypeFromPath(path)
}

func appendCategoryKeywords(keywords []string, category string, tags []string) []string {
	return storage.AppendCategoryKeywords(keywords, category, tags)
}

func createIndexEntry(path string, categorize bool) (*IndexEntry, error) {
	return storage.CreateSearchIndexEntry(path, categorize)
}

func appendToIndex(baseDir string, entry *IndexEntry) error {
	return storage.AppendToSearchIndex(baseDir, entry)
}

func searchIndex(baseDir, query string, limit int) ([]SearchResult, error) {
	return storage.SearchIndex(baseDir, query, limit)
}

func computeSearchScore(entry IndexEntry, queryTerms []string) float64 {
	return storage.ComputeSearchScore(entry, queryTerms)
}

func extractTitle(content string) string {
	return storage.ExtractTitle(content)
}

func extractKeywords(content string) []string {
	return storage.ExtractKeywords(content)
}

func extractCategoryAndTags(content string) (category string, tags []string) {
	return storage.ExtractCategoryAndTags(content)
}

func extractFrontmatterMeta(lines []string) (category string, tags []string) {
	return storage.ExtractFrontmatterMeta(lines)
}

func extractMarkdownMeta(lines []string) (category string, tags []string) {
	return storage.ExtractMarkdownMeta(lines)
}

func parseBracketedList(s string) []string {
	return storage.ParseBracketedList(s)
}

func splitCSV(s string) []string {
	return storage.SplitCSV(s)
}

func parseMemRLMetadata(content string) (utility float64, maturity string) {
	return storage.ParseMemRLMetadata(content)
}

func createSearchSnippet(content, query string, maxLen int) string {
	return storage.CreateSearchSnippet(content, query, maxLen)
}

func accumulateEntryStats(stats *IndexStats, entry IndexEntry, totalUtility *float64, utilityCount *int) {
	storage.AccumulateEntryStats(stats, entry, totalUtility, utilityCount)
}

func computeIndexStats(baseDir string) (*IndexStats, error) {
	return storage.ComputeSearchIndexStats(baseDir)
}

// printSearchResults prints search results in table format.
func printSearchResults(query string, results []SearchResult) {
	fmt.Println()
	fmt.Printf("Search Results for: %s\n", query)
	fmt.Println("======================")
	fmt.Println()

	if len(results) == 0 {
		fmt.Println("No results found")
		return
	}

	for i, r := range results {
		fmt.Printf("%d. %s [%s]\n", i+1, r.Entry.Title, r.Entry.Type)
		fmt.Printf("   Score: %.2f | Utility: %.2f\n", r.Score, r.Entry.Utility)
		fmt.Printf("   Path: %s\n", r.Entry.Path)
		if r.Snippet != "" {
			fmt.Printf("   %s\n", r.Snippet)
		}
		fmt.Println()
	}
}

// printIndexStats prints index statistics.
func printIndexStats(stats *IndexStats) {
	fmt.Println()
	fmt.Println("Search Index Statistics")
	fmt.Println("=======================")
	fmt.Println()

	fmt.Printf("Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("Mean utility:  %.2f\n", stats.MeanUtility)
	fmt.Printf("Index path:    %s\n", stats.IndexPath)
	fmt.Println()

	if len(stats.ByType) > 0 {
		fmt.Println("By Type:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for t, count := range stats.ByType {
			//nolint:errcheck // CLI tabwriter output to stdout
			fmt.Fprintf(w, "  %s:\t%d\n", t, count)
		}
		_ = w.Flush()
	}

	if !stats.OldestEntry.IsZero() {
		fmt.Println()
		fmt.Printf("Oldest indexed: %s\n", stats.OldestEntry.Format("2006-01-02 15:04"))
		fmt.Printf("Newest indexed: %s\n", stats.NewestEntry.Format("2006-01-02 15:04"))
	}
}
