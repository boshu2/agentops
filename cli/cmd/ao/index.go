package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// defaultIndexSubdirs are the subdirectories under .agents/ that the index
// command rebuilds. Each entry is joined with the resolved agents-dir at
// runtime by buildDefaultIndexDirs() so the index honors AO_AGENTS_DIR /
// AO_HOME (lib/ao-paths.sh + cli/internal/paths from soc-irg1.1).
var defaultIndexSubdirs = []string{
	"learnings",
	"findings",
	"research",
	"plans",
	"retro",
	"patterns",
}

// defaultIndexDirs is the legacy ".agents/<sub>" prefixed list, retained as
// a back-compat alias for tests and any external caller that constructed
// fixture directories from this slice. New code should call
// buildDefaultIndexDirs(cwd) so AO_AGENTS_DIR overrides work end-to-end.
var defaultIndexDirs = func() []string {
	out := make([]string, len(defaultIndexSubdirs))
	for i, sub := range defaultIndexSubdirs {
		out[i] = filepath.Join(".agents", sub)
	}
	return out
}()

// buildDefaultIndexDirs returns the list of full directory paths to index,
// resolved relative to the AgentsDir for the given cwd. The returned paths
// remain relative when no env override is in effect (".agents/learnings" etc.)
// to preserve display strings on stdout/JSON output unchanged.
func buildDefaultIndexDirs(cwd string) []string {
	agentsDir := agentsDirIn(cwd)
	rel, err := filepath.Rel(cwd, agentsDir)
	// When the resolved AgentsDir is outside cwd (or rel resolution fails),
	// fall back to absolute paths so the indexer still finds the directory.
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = agentsDir
	}
	dirs := make([]string, len(defaultIndexSubdirs))
	for i, sub := range defaultIndexSubdirs {
		dirs[i] = filepath.Join(rel, sub)
	}
	return dirs
}

var dateFromFilenameRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)
var dateExtractRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// indexEntry represents one .md file's metadata for INDEX.md generation.
type indexEntry struct {
	Filename string `json:"filename"`
	Date     string `json:"date"`
	Summary  string `json:"summary"`
	Tags     string `json:"tags"`
}

// indexResult represents the result of indexing one directory.
type indexResult struct {
	Dir     string       `json:"dir"`
	Entries []indexEntry `json:"entries"`
	Written bool         `json:"written"`
	Error   string       `json:"error,omitempty"`
}

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Generate INDEX.md manifests for .agents/ directories",
	Long: `Generate INDEX.md manifest files for .agents/ knowledge directories.

Scans each directory for .md files, extracts frontmatter metadata (date,
summary, tags), and produces a markdown table sorted by date (newest first).

Directories indexed:
  .agents/learnings
  .agents/findings
  .agents/research
  .agents/plans
  .agents/retro
  .agents/patterns

Example:
  ao index              # Rebuild all INDEX.md files
  ao index --check      # Verify INDEX.md files are current
  ao index --dir .agents/learnings  # Rebuild one directory`,
	RunE: runIndex,
}

func init() {
	indexCmd.Hidden = true
	indexCmd.GroupID = "knowledge"
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().Bool("check", false, "Verify INDEX.md is current, exit 1 if stale")
	indexCmd.Flags().Bool("json", false, "Machine-readable output")
	indexCmd.Flags().String("dir", "", "Specific directory (default: all 5)")
	indexCmd.Flags().Bool("quiet", false, "Suppress non-error output")
}

// scanAndSortDir scans a directory for index entries and sorts them by date descending.
// Returns nil entries and false if the directory should be skipped.
func scanAndSortDir(cwd, dir string) (string, []indexEntry, bool) {
	fullPath := filepath.Join(cwd, dir)
	info, err := os.Stat(fullPath)
	if err != nil || !info.IsDir() {
		VerbosePrintf("Warning: directory not found: %s\n", fullPath)
		return fullPath, nil, false
	}

	entries, err := scanDirectory(fullPath)
	if err != nil {
		VerbosePrintf("Warning: scanning %s: %v\n", dir, err)
		return fullPath, nil, false
	}

	slices.SortFunc(entries, func(a, b indexEntry) int {
		return cmp.Compare(b.Date, a.Date)
	})
	return fullPath, entries, true
}

// processAllIndexDirs scans, sorts, and processes each directory, returning results and staleness.
func processAllIndexDirs(cwd string, dirs []string, checkMode, quiet bool) ([]indexResult, bool) {
	var results []indexResult
	stale := false
	for _, dir := range dirs {
		fullPath, entries, ok := scanAndSortDir(cwd, dir)
		if !ok {
			continue
		}
		result, dirStale := processIndexDir(checkMode, quiet, dir, fullPath, entries)
		if dirStale {
			stale = true
		}
		results = append(results, result)
	}
	return results, stale
}

func runIndex(cmd *cobra.Command, args []string) error {
	checkMode, _ := cmd.Flags().GetBool("check")
	localJSONMode, _ := cmd.Flags().GetBool("json")
	jsonMode := localJSONMode || GetOutput() == "json"
	dirFlag, _ := cmd.Flags().GetString("dir")
	quiet, _ := cmd.Flags().GetBool("quiet")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dirs := buildDefaultIndexDirs(cwd)
	if dirFlag != "" {
		dirs = []string{dirFlag}
	}

	results, stale := processAllIndexDirs(cwd, dirs, checkMode, quiet)

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return fmt.Errorf("encode index results: %w", err)
		}
	}

	if checkMode && stale {
		return fmt.Errorf("INDEX.md files are stale")
	}

	return nil
}

// processIndexDir runs check or write mode for a single index directory.
// Returns the indexResult and whether the index is stale.
func processIndexDir(checkMode, quiet bool, dir, fullPath string, entries []indexEntry) (indexResult, bool) {
	if checkMode {
		isStale, msg := checkIndex(fullPath, dir, entries)
		if isStale {
			if !quiet {
				fmt.Fprintf(os.Stderr, "%s\n", msg)
			}
		} else if !quiet {
			fmt.Printf("%s: current (%d entries)\n", dir, len(entries))
		}
		return indexResult{Dir: dir, Entries: entries, Written: false}, isStale
	}
	err := writeIndex(fullPath, dir, entries, GetDryRun())
	written := err == nil && !GetDryRun()
	result := indexResult{Dir: dir, Entries: entries, Written: written}
	if err != nil {
		result.Error = err.Error()
		fmt.Fprintf(os.Stderr, "Error writing INDEX.md for %s: %v\n", dir, err)
	} else if !quiet {
		fmt.Printf("%s: %d entries indexed\n", dir, len(entries))
	}
	return result, false
}

// scanDirectory reads all .md files (excluding INDEX.md) in a directory and
// extracts metadata from each.
func scanDirectory(dirPath string) ([]indexEntry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	entries := make([]indexEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".md") || name == "INDEX.md" {
			continue
		}

		filePath := filepath.Join(dirPath, name)
		entry := extractEntry(filePath, name)
		entries = append(entries, entry)
	}
	return entries, nil
}

// extractEntry reads a single .md file and extracts date, summary, and tags.
func extractEntry(filePath, filename string) indexEntry {
	entry := indexEntry{Filename: filename}

	content, err := os.ReadFile(filePath)
	if err != nil {
		VerbosePrintf("Warning: reading %s: %v\n", filePath, err)
		entry.Date = extractDateFromFilename(filename)
		entry.Summary = summaryFromFilename(filename)
		return entry
	}

	fm := parseFrontmatter(string(content))

	// Date: try frontmatter created_at, then date, then filename
	entry.Date = extractDateField(fm, filename)

	// Summary: frontmatter summary, then H1, then filename
	if s, ok := fm["summary"].(string); ok && s != "" {
		entry.Summary = s
	} else {
		entry.Summary = extractH1(string(content))
		if entry.Summary == "" {
			entry.Summary = summaryFromFilename(filename)
		}
	}

	// Tags
	entry.Tags = extractTagsFromFrontmatter(fm)

	// Clean for table
	entry.Summary = cleanForTable(entry.Summary)
	entry.Tags = cleanForTable(entry.Tags)

	return entry
}

// parseFrontmatter extracts YAML frontmatter between --- delimiters.
func parseFrontmatter(content string) map[string]any {
	result := make(map[string]any)

	scanner := bufio.NewScanner(strings.NewReader(content))
	fmCount := 0
	var fmLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			fmCount++
			if fmCount == 2 {
				break
			}
			continue
		}
		if fmCount == 1 {
			fmLines = append(fmLines, line)
		}
	}

	if len(fmLines) == 0 {
		return result
	}

	fmText := strings.Join(fmLines, "\n")
	_ = yaml.Unmarshal([]byte(fmText), &result)
	return result
}

// extractDateField tries frontmatter fields then filename for a date string.
func extractDateField(fm map[string]any, filename string) string {
	// Try created_at first, then date
	for _, field := range []string{"created_at", "date"} {
		if val, ok := fm[field]; ok {
			dateStr := fmt.Sprintf("%v", val)
			if m := dateExtractRe.FindString(dateStr); m != "" {
				return m
			}
			return dateStr
		}
	}
	return extractDateFromFilename(filename)
}

// extractDateFromFilename extracts YYYY-MM-DD from a filename prefix.
func extractDateFromFilename(filename string) string {
	if m := dateFromFilenameRe.FindStringSubmatch(filename); len(m) > 1 {
		return m[1]
	}
	return "unknown"
}

// summaryFromFilename derives a summary from the filename.
func summaryFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	// Remove date prefix if present (YYYY-MM-DD-)
	if len(name) > 11 && dateFromFilenameRe.MatchString(name) && name[10] == '-' {
		rest := name[11:] // len("YYYY-MM-DD-") == 11
		if len(rest) > 0 {
			name = rest
		}
	}
	return name
}

// extractH1 returns the text of the first H1 heading in the content.
func extractH1(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// extractTagsFromFrontmatter extracts tags as a space-separated string.
func extractTagsFromFrontmatter(fm map[string]any) string {
	val, ok := fm["tags"]
	if !ok {
		return ""
	}

	switch v := val.(type) {
	case []any:
		var tags []string
		for _, t := range v {
			tags = append(tags, fmt.Sprintf("%v", t))
		}
		return strings.Join(tags, " ")
	case string:
		// Handle inline "[tag1, tag2]" format
		s := strings.TrimSpace(v)
		s = strings.Trim(s, "[]")
		parts := strings.Split(s, ",")
		var tags []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				tags = append(tags, p)
			}
		}
		return strings.Join(tags, " ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// cleanForTable escapes pipe characters and normalizes whitespace.
func cleanForTable(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	// Normalize multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// titleCase capitalizes the first letter of a string.
func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// writeIndex generates INDEX.md for a single directory.
func writeIndex(dirPath, relDir string, entries []indexEntry, dryRun bool) error {
	dirName := titleCase(filepath.Base(relDir))
	today := time.Now().Format("2006-01-02")

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Index: %s\n\n", dirName)
	fmt.Fprintf(&sb, "> Last rebuilt: %s | %d entries\n\n", today, len(entries))
	sb.WriteString("## Entries\n\n")
	sb.WriteString("| File | Date | Summary | Tags |\n")
	sb.WriteString("|------|------|---------|------|\n")

	for _, e := range entries {
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", e.Filename, e.Date, e.Summary, e.Tags)
	}

	if dryRun {
		VerbosePrintf("Would write INDEX.md to %s\n", filepath.Join(dirPath, "INDEX.md"))
		return nil
	}

	indexPath := filepath.Join(dirPath, "INDEX.md")
	return os.WriteFile(indexPath, []byte(sb.String()), 0600)
}

// diffFileSets returns slices of files missing from existing and extra files not in expected.
func diffFileSets(expected, existing map[string]bool) (missing, extra []string) {
	for f := range expected {
		if !existing[f] {
			missing = append(missing, f)
		}
	}
	for f := range existing {
		if !expected[f] {
			extra = append(extra, f)
		}
	}
	return missing, extra
}

// buildExpectedFileSet creates a set of expected filenames from index entries.
func buildExpectedFileSet(entries []indexEntry) map[string]bool {
	expected := make(map[string]bool, len(entries))
	for _, e := range entries {
		expected[e.Filename] = true
	}
	return expected
}

// checkIndex verifies INDEX.md is current by comparing file lists.
func checkIndex(dirPath, relDir string, entries []indexEntry) (bool, string) {
	indexPath := filepath.Join(dirPath, "INDEX.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return true, fmt.Sprintf("STALE %s: INDEX.md missing", relDir)
	}

	existingFiles := parseIndexTableRows(content)
	expectedFiles := buildExpectedFileSet(entries)
	missing, extra := diffFileSets(expectedFiles, existingFiles)

	if len(missing) == 0 && len(extra) == 0 && len(existingFiles) == len(expectedFiles) {
		return false, ""
	}
	return true, buildIndexDiffMessage(relDir, missing, extra)
}

// parseIndexTableRows extracts filenames from an INDEX.md markdown table.
func parseIndexTableRows(content []byte) map[string]bool {
	existing := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	headerPassed := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "|------") || strings.HasPrefix(line, "|---") {
			headerPassed = true
			continue
		}
		if !headerPassed {
			continue
		}
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) >= 3 {
			fname := strings.TrimSpace(parts[1])
			if fname != "" && fname != "File" {
				existing[fname] = true
			}
		}
	}
	return existing
}

// buildIndexDiffMessage formats a STALE message listing missing and extra files.
func buildIndexDiffMessage(relDir string, missing, extra []string) string {
	var msg strings.Builder
	fmt.Fprintf(&msg, "STALE %s:", relDir)
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(&msg, " missing=[%s]", strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		fmt.Fprintf(&msg, " extra=[%s]", strings.Join(extra, ", "))
	}
	return msg.String()
}
