package search

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// ContextLineMaxLength is the maximum length for context lines in search results.
	ContextLineMaxLength = 100

	// MaxContextLines is the maximum number of context lines to show per result.
	MaxContextLines = 3
)

// SearchResult represents a single search result with path, score, context, and type.
type SearchResult struct {
	Path    string  `json:"path"`
	Score   float64 `json:"score,omitempty"`
	Context string  `json:"context,omitempty"`
	Type    string  `json:"type,omitempty"`
}

// MaturityWeights maps CASS maturity levels to ranking weights.
var MaturityWeights = map[string]float64{
	"established":  1.5,
	"candidate":    1.2,
	"provisional":  1.0,
	"anti-pattern": 0.3,
}

// NormalizeSearchResults deduplicates results by path (keeping highest score)
// and sorts by descending score, then ascending path.
func NormalizeSearchResults(results []SearchResult, limit int) []SearchResult {
	seen := make(map[string]SearchResult, len(results))
	for _, result := range results {
		existing, ok := seen[result.Path]
		if !ok || result.Score > existing.Score {
			seen[result.Path] = result
		}
	}

	unique := make([]SearchResult, 0, len(seen))
	for _, result := range seen {
		unique = append(unique, result)
	}

	slices.SortFunc(unique, func(a, b SearchResult) int {
		if cmp := cmp.Compare(b.Score, a.Score); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Path, b.Path)
	})

	if limit > 0 && len(unique) > limit {
		return unique[:limit]
	}
	return unique
}

// SearchDataExists checks if any knowledge directories exist under sessionsDir
// or its derived knowledge root.
func SearchDataExists(sessionsDir string) bool {
	if _, err := os.Stat(sessionsDir); err == nil {
		return true
	}

	root := KnowledgeRootFromSessions(sessionsDir)
	for _, name := range []string{"learnings", "patterns", "findings", "research", "compiled"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}

	return false
}

// KnowledgeRootFromSessions derives the knowledge root directory from a sessions path.
// If sessionsDir is under .agents/ao/sessions, returns the .agents directory.
func KnowledgeRootFromSessions(sessionsDir string) string {
	sessionsDir = filepath.Clean(sessionsDir)
	aoRoot := filepath.Dir(sessionsDir)
	if filepath.Base(aoRoot) == "ao" && filepath.Base(filepath.Dir(aoRoot)) == ".agents" {
		return filepath.Dir(aoRoot)
	}
	return filepath.Dir(sessionsDir)
}

// BuildGrepCommand creates the grep/ripgrep command.
// Prefers ripgrep (rg) if available, falls back to grep.
func BuildGrepCommand(query, dir, pattern string) (*exec.Cmd, bool) {
	if _, err := exec.LookPath("rg"); err == nil {
		return exec.Command("rg", "-l", "-i", "--max-count", "1", "--glob", pattern, query, dir), true
	}
	return exec.Command("grep", "-l", "-i", "-r", query, dir), false
}

// ExecuteGrepWithFallback runs the grep command with retry logic.
// If ripgrep glob pattern fails, retries without glob filter (recursive mode).
// The verbosePrintf parameter is used for diagnostic logging; pass nil to suppress.
func ExecuteGrepWithFallback(cmd *exec.Cmd, useRipgrep bool, query, dir string, verbosePrintf func(string, ...any)) ([]byte, error) {
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}

	// Both grep and rg return exit code 1 if no matches - this is normal
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil, nil
	}

	// If ripgrep failed with glob, retry without glob filter (recursive search)
	if useRipgrep {
		if verbosePrintf != nil {
			verbosePrintf("ripgrep glob failed, trying recursive search: %v\n", err)
		}
		fallbackCmd := exec.Command("rg", "-l", "-i", "--max-count", "1", query, dir)
		output, err = fallbackCmd.Output()
		if err != nil {
			var exitErr2 *exec.ExitError
			if errors.As(err, &exitErr2) && exitErr2.ExitCode() == 1 {
				return nil, nil
			}
			return nil, fmt.Errorf("search failed: %w", err)
		}
		return output, nil
	}

	return nil, fmt.Errorf("grep failed: %w", err)
}

// ParseGrepResults converts grep output lines into search results.
func ParseGrepResults(output []byte, pattern, query string, useRipgrep bool) []SearchResult {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	results := make([]SearchResult, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		// Filter by pattern if using grep (which doesn't filter by extension)
		if !useRipgrep && pattern != "" {
			matched, _ := filepath.Match(pattern, filepath.Base(line))
			if !matched {
				continue
			}
		}
		context := GetFileContext(line, query)
		results = append(results, SearchResult{
			Path:    line,
			Context: context,
			Type:    "session",
		})
	}

	return results
}

// GetFileContext reads a file and returns matching lines as context.
func GetFileContext(path, query string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only context extraction, close error non-fatal
	}()

	scanner := bufio.NewScanner(f)
	queryLower := strings.ToLower(query)
	var context []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), queryLower) {
			line = strings.TrimSpace(line)
			if len(line) > ContextLineMaxLength {
				line = line[:ContextLineMaxLength] + "..."
			}
			context = append(context, line)
			if len(context) >= MaxContextLines {
				break
			}
		}
	}

	return strings.Join(context, "\n")
}

// SearchJSONL searches JSONL files in a directory for a query string.
func SearchJSONL(query string, dir string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(strings.ToLower(line), queryLower) {
				continue
			}
			if r, ok := ParseJSONLMatch(line, file); ok {
				results = append(results, r)
				break
			}
		}
		_ = f.Close() //nolint:errcheck // read-only search, close error non-fatal

		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// ParseJSONLMatch parses a single JSONL line into a search result.
func ParseJSONLMatch(line, file string) (SearchResult, bool) {
	var data map[string]any
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return SearchResult{}, false
	}
	context := ""
	if summary, ok := data["summary"].(string); ok {
		context = summary
		if len(context) > ContextLineMaxLength {
			context = context[:ContextLineMaxLength] + "..."
		}
	}
	return SearchResult{
		Path:    file,
		Context: context,
		Type:    "session",
	}, true
}

// SearchLearningsWithMaturity searches learnings and weights by maturity and confidence.
func SearchLearningsWithMaturity(query, dir string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	mdFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	files = append(files, mdFiles...)

	queryLower := strings.ToLower(query)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(strings.ToLower(line), queryLower) {
				continue
			}
			if r, ok := ParseLearningMatch(line, file); ok {
				results = append(results, r)
				break
			}
		}
		_ = f.Close() //nolint:errcheck // read-only search, close error non-fatal
	}

	return results, nil
}

// TruncateContext truncates a search context string to ContextLineMaxLength.
func TruncateContext(s string) string {
	runes := []rune(s)
	if len(runes) > ContextLineMaxLength {
		return string(runes[:ContextLineMaxLength]) + "..."
	}
	return s
}

// ParseLearningMatch parses a JSONL line into a learning search result with CASS score.
func ParseLearningMatch(line, file string) (SearchResult, bool) {
	var data map[string]any
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return SearchResult{}, false
	}

	score := CalculateCASSScore(data)
	context := ExtractLearningContext(data)

	maturityStr := "provisional"
	if m, ok := data["maturity"].(string); ok {
		maturityStr = m
	}

	return SearchResult{
		Path:    file,
		Score:   score,
		Context: fmt.Sprintf("[%s] %s", maturityStr, context),
		Type:    "learning",
	}, true
}

// ExtractLearningContext extracts a context snippet from learning data.
func ExtractLearningContext(data map[string]any) string {
	if summary, ok := data["summary"].(string); ok {
		return TruncateContext(summary)
	}
	if content, ok := data["content"].(string); ok {
		return TruncateContext(content)
	}
	return ""
}

// CalculateCASSScore computes a maturity-weighted score for CASS ranking.
// Score = utility * maturityWeight * confidenceWeight
func CalculateCASSScore(data map[string]any) float64 {
	utility := 0.5
	if u, ok := data["utility"].(float64); ok && u > 0 {
		utility = u
	}

	maturityWeight := MaturityToWeight(data)

	confidenceWeight := 0.5
	if c, ok := data["confidence"].(float64); ok && c > 0 {
		confidenceWeight = c
	}

	return utility * maturityWeight * confidenceWeight
}

// MaturityToWeight maps a maturity field in data to its ranking weight.
func MaturityToWeight(data map[string]any) float64 {
	maturity, ok := data["maturity"].(string)
	if !ok {
		return 1.0
	}
	if w, found := MaturityWeights[maturity]; found {
		return w
	}
	return 1.0
}

// FilterByType filters results by knowledge type.
func FilterByType(results []SearchResult, filterType string) []SearchResult {
	normalizedType := NormalizeSearchType(filterType)
	var filtered []SearchResult
	for _, r := range results {
		if r.Type == normalizedType || normalizedType == "" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// NormalizeSearchType normalizes a user-provided search type string.
func NormalizeSearchType(filterType string) string {
	switch strings.ToLower(strings.TrimSpace(filterType)) {
	case "", "knowledge":
		return strings.ToLower(strings.TrimSpace(filterType))
	case "sessions":
		return "session"
	case "learnings":
		return "learning"
	case "patterns":
		return "pattern"
	case "findings":
		return "finding"
	case "compiled", "synthesis", "syntheses":
		return "compiled"
	case "decisions":
		return "decision"
	case "retros":
		return "retro"
	default:
		return strings.ToLower(strings.TrimSpace(filterType))
	}
}

// ClassifyResultType determines the knowledge type based on file path.
func ClassifyResultType(path string) string {
	pathLower := strings.ReplaceAll(strings.ToLower(path), "\\", "/")

	if strings.Contains(pathLower, "/learnings/") {
		return "learning"
	}
	if strings.Contains(pathLower, "/findings/") {
		return "finding"
	}
	if strings.Contains(pathLower, "/patterns/") {
		return "pattern"
	}
	if strings.Contains(pathLower, "/retro/") {
		return "retro"
	}
	if strings.Contains(pathLower, "/research/") {
		return "research"
	}
	if strings.Contains(pathLower, "/compiled/") {
		return "compiled"
	}
	if strings.Contains(pathLower, "/sessions/") {
		return "session"
	}
	if strings.Contains(pathLower, "/decisions/") {
		return "decision"
	}

	return "knowledge"
}
