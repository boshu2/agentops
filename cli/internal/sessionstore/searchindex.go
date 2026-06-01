package sessionstore

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boshu2/agentops/cli/internal/domain"
)

const (
	// SearchIndexFileName is the file name of the search index.
	SearchIndexFileName = "search-index.jsonl"
	// SearchIndexDir is the directory for index files relative to the workspace.
	SearchIndexDir = ".agents/ao/index"
)

// SearchIndexEntry represents a single entry in the artifact search index.
type SearchIndexEntry struct {
	Path       string    `json:"path"`
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Keywords   []string  `json:"keywords,omitempty"`
	Category   string    `json:"category,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Utility    float64   `json:"utility,omitempty"`
	Maturity   string    `json:"maturity,omitempty"`
	IndexedAt  time.Time `json:"indexed_at"`
	ModifiedAt time.Time `json:"modified_at"`
}

// SearchResult represents a single match in the search index.
type SearchResult struct {
	Entry   SearchIndexEntry `json:"entry"`
	Score   float64          `json:"score"`
	Snippet string           `json:"snippet,omitempty"`
}

// SearchIndexStats holds aggregate stats about the index.
type SearchIndexStats struct {
	TotalEntries int            `json:"total_entries"`
	ByType       map[string]int `json:"by_type"`
	MeanUtility  float64        `json:"mean_utility"`
	OldestEntry  time.Time      `json:"oldest_entry"`
	NewestEntry  time.Time      `json:"newest_entry"`
	IndexPath    string         `json:"index_path"`
}

// ArtifactSubdirs lists the subdirectories under .agents/ that contain indexable artifacts.
var ArtifactSubdirs = []string{"learnings", "patterns", "research", "retros", "candidates"}

// WalkIndexableFiles returns all .md and .jsonl files under dir.
func WalkIndexableFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ArtifactTypeFromPath determines the artifact type based on the file path.
func ArtifactTypeFromPath(path string) string {
	pathTypeMap := []struct {
		segment    string
		resultType string
	}{
		{"/learnings/", "learning"},
		{"/patterns/", "pattern"},
		{"/research/", "research"},
		{"/retro/", "retro"},
		{"/candidates/", "candidate"},
	}
	for _, m := range pathTypeMap {
		if strings.Contains(path, m.segment) {
			return m.resultType
		}
	}
	return "unknown"
}

// AppendCategoryKeywords appends category and tag values as lowercase keywords.
func AppendCategoryKeywords(keywords []string, category string, tags []string) []string {
	if category != "" {
		keywords = append(keywords, strings.ToLower(category))
	}
	for _, t := range tags {
		tt := strings.TrimSpace(t)
		if tt != "" {
			keywords = append(keywords, strings.ToLower(tt))
		}
	}
	return keywords
}

// ExtractTitle gets the title from markdown content.
func ExtractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "---") {
			if len(line) > 80 {
				return line[:77] + "..."
			}
			return line
		}
	}
	return "Untitled"
}

// ExtractKeywords extracts keywords from content.
func ExtractKeywords(content string) []string {
	keywords := make(map[string]bool)
	patterns := []string{
		"pattern:", "solution:", "learning:", "decision:",
		"fix:", "issue:", "error:", "warning:",
		"config:", "setup:", "install:", "deploy:",
	}
	lowerContent := strings.ToLower(content)
	for _, pattern := range patterns {
		if strings.Contains(lowerContent, pattern) {
			keywords[strings.TrimSuffix(pattern, ":")] = true
		}
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "**Tags**:") || strings.HasPrefix(line, "**Keywords**:") {
			tags := strings.TrimPrefix(strings.TrimPrefix(line, "**Tags**:"), "**Keywords**:")
			for _, tag := range strings.Split(tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					keywords[tag] = true
				}
			}
		}
	}
	result := make([]string, 0, len(keywords))
	for kw := range keywords {
		result = append(result, kw)
	}
	return result
}

// ExtractCategoryAndTags derives category/tags from frontmatter or markdown metadata.
func ExtractCategoryAndTags(content string) (category string, tags []string) {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		category, tags = ExtractFrontmatterMeta(lines[1:])
	}
	mdCategory, mdTags := ExtractMarkdownMeta(lines)
	if category == "" {
		category = mdCategory
	}
	tags = append(tags, mdTags...)
	return category, tags
}

// ExtractFrontmatterMeta parses category/tags from YAML frontmatter lines.
func ExtractFrontmatterMeta(lines []string) (category string, tags []string) {
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "category:") {
			category = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "category:")), "\"'")
		}
		if strings.HasPrefix(line, "tags:") {
			tags = ParseBracketedList(strings.TrimSpace(strings.TrimPrefix(line, "tags:")))
		}
	}
	return category, tags
}

// ExtractMarkdownMeta parses category/tags from bold-key markdown lines.
func ExtractMarkdownMeta(lines []string) (category string, tags []string) {
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if category == "" && strings.HasPrefix(line, "**Category**:") {
			category = strings.TrimSpace(strings.TrimPrefix(line, "**Category**:"))
		}
		if strings.HasPrefix(line, "**Tags**:") {
			tags = append(tags, SplitCSV(strings.TrimSpace(strings.TrimPrefix(line, "**Tags**:")))...)
		}
	}
	return category, tags
}

// ParseBracketedList parses "[a, b, c]" into a trimmed string slice.
func ParseBracketedList(s string) []string {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	var out []string
	for _, t := range strings.Split(inner, ",") {
		tt := strings.TrimSpace(strings.Trim(t, "\"'"))
		if tt != "" {
			out = append(out, tt)
		}
	}
	return out
}

// SplitCSV splits a comma-separated string and returns non-empty trimmed values.
func SplitCSV(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		tt := strings.TrimSpace(t)
		if tt != "" {
			out = append(out, tt)
		}
	}
	return out
}

// ParseMemRLMetadata extracts utility and maturity from content.
func ParseMemRLMetadata(content string) (utility float64, maturity string) {
	utility = domain.InitialUtility
	maturity = "provisional"
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "**Utility**:") || strings.HasPrefix(line, "- **Utility**:") {
			utilStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "**Utility**:"), "- **Utility**:"))
			//nolint:errcheck
			fmt.Sscanf(utilStr, "%f", &utility) // #nosec G104
		}
		if strings.HasPrefix(line, "**Maturity**:") || strings.HasPrefix(line, "- **Maturity**:") {
			maturity = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "**Maturity**:"), "- **Maturity**:"))
		}
	}
	return utility, maturity
}

// CreateSearchSnippet creates a context snippet around query matches.
func CreateSearchSnippet(content, query string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	idx := strings.Index(lowerContent, lowerQuery)
	if idx == -1 {
		terms := strings.Fields(lowerQuery)
		if len(terms) > 0 {
			idx = strings.Index(lowerContent, terms[0])
		}
	}

	if idx == -1 {
		runes := []rune(content)
		if len(runes) > maxLen {
			if maxLen <= 3 {
				return string(runes[:maxLen])
			}
			return string(runes[:maxLen-3]) + "..."
		}
		return content
	}

	start := idx - 50
	if start < 0 {
		start = 0
	}
	for start > 0 && !utf8.RuneStart(content[start]) {
		start--
	}
	end := idx + maxLen
	if end > len(content) {
		end = len(content)
	}
	for end < len(content) && !utf8.RuneStart(content[end]) {
		end++
	}

	snippet := content[start:end]
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.TrimSpace(snippet)

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet += "..."
	}

	return snippet
}

// ComputeSearchScore calculates relevance score for a query.
func ComputeSearchScore(entry SearchIndexEntry, queryTerms []string) float64 {
	var score float64
	lowerContent := strings.ToLower(entry.Content)
	lowerTitle := strings.ToLower(entry.Title)

	for _, term := range queryTerms {
		if strings.Contains(lowerTitle, term) {
			score += 3.0
		}
		if strings.Contains(lowerContent, term) {
			score += 1.0
		}
		for _, kw := range entry.Keywords {
			if strings.Contains(strings.ToLower(kw), term) {
				score += 2.0
				break
			}
		}
	}

	lambda := domain.DefaultLambda
	if entry.Utility > 0 {
		score = (1-lambda)*score + lambda*entry.Utility*score
	}
	return score
}

// CreateSearchIndexEntry creates an index entry from a file.
func CreateSearchIndexEntry(path string, categorize bool) (*SearchIndexEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(content)
	keywords := ExtractKeywords(text)

	var category string
	var tags []string
	if categorize {
		category, tags = ExtractCategoryAndTags(text)
		keywords = AppendCategoryKeywords(keywords, category, tags)
	}

	utility, maturity := ParseMemRLMetadata(text)

	return &SearchIndexEntry{
		Path:       path,
		ID:         filepath.Base(path),
		Type:       ArtifactTypeFromPath(path),
		Title:      ExtractTitle(text),
		Content:    text,
		Keywords:   keywords,
		Category:   category,
		Tags:       tags,
		Utility:    utility,
		Maturity:   maturity,
		IndexedAt:  time.Now(),
		ModifiedAt: info.ModTime(),
	}, nil
}

// AppendToSearchIndex adds an entry to the index file.
func AppendToSearchIndex(baseDir string, entry *SearchIndexEntry) error {
	indexDir := filepath.Join(baseDir, SearchIndexDir)
	if err := os.MkdirAll(indexDir, 0750); err != nil {
		return err
	}
	indexPath := filepath.Join(indexDir, SearchIndexFileName)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck
	_, err = f.Write(append(data, '\n'))
	return err
}

// SearchIndex searches the index for matching entries.
func SearchIndex(baseDir, query string, limit int) ([]SearchResult, error) {
	indexPath := filepath.Join(baseDir, SearchIndexDir, SearchIndexFileName)
	f, err := os.Open(indexPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found - run 'ao store rebuild' first")
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck

	queryTerms := strings.Fields(strings.ToLower(query))
	var results []SearchResult

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry SearchIndexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		score := ComputeSearchScore(entry, queryTerms)
		if score > 0 {
			snippet := CreateSearchSnippet(entry.Content, query, 150)
			results = append(results, SearchResult{
				Entry:   entry,
				Score:   score,
				Snippet: snippet,
			})
		}
	}

	slices.SortFunc(results, func(a, b SearchResult) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		return cmp.Compare(b.Entry.Utility, a.Entry.Utility)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, scanner.Err()
}

// AccumulateEntryStats updates running stats counters from a single index entry.
func AccumulateEntryStats(stats *SearchIndexStats, entry SearchIndexEntry, totalUtility *float64, utilityCount *int) {
	stats.TotalEntries++
	stats.ByType[entry.Type]++

	if entry.Utility > 0 {
		*totalUtility += entry.Utility
		*utilityCount++
	}
	if stats.OldestEntry.IsZero() || entry.IndexedAt.Before(stats.OldestEntry) {
		stats.OldestEntry = entry.IndexedAt
	}
	if entry.IndexedAt.After(stats.NewestEntry) {
		stats.NewestEntry = entry.IndexedAt
	}
}

// ComputeSearchIndexStats calculates index statistics.
func ComputeSearchIndexStats(baseDir string) (*SearchIndexStats, error) {
	indexPath := filepath.Join(baseDir, SearchIndexDir, SearchIndexFileName)
	stats := &SearchIndexStats{
		ByType:    make(map[string]int),
		IndexPath: indexPath,
	}
	f, err := os.Open(indexPath)
	if os.IsNotExist(err) {
		return stats, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck

	var totalUtility float64
	var utilityCount int

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var entry SearchIndexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		AccumulateEntryStats(stats, entry, &totalUtility, &utilityCount)
	}

	if utilityCount > 0 {
		stats.MeanUtility = totalUtility / float64(utilityCount)
	}
	return stats, scanner.Err()
}
