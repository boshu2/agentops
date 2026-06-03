package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsContainedPath checks if path is contained within baseDir.
func IsContainedPath(baseDir, path string) bool {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	cleanBase := filepath.Clean(absBase)
	cleanPath := filepath.Clean(absPath)
	if !strings.HasSuffix(cleanBase, string(filepath.Separator)) {
		cleanBase += string(filepath.Separator)
	}
	return strings.HasPrefix(cleanPath+string(filepath.Separator), cleanBase) || cleanPath == filepath.Clean(absBase)
}

// IsArtifactFile checks if a filename is a valid artifact file type.
func IsArtifactFile(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".jsonl")
}

// ParseMarkdownField extracts a value for a field from a markdown line.
func ParseMarkdownField(line, field string) (string, bool) {
	prefixes := []string{
		"**" + field + "**:",
		"**" + field + ":**",
		"- **" + field + "**:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

// ArtifactMeta holds parsed metadata from a knowledge artifact.
type ArtifactMeta struct {
	ID            string
	Maturity      string
	Utility       float64
	Confidence    float64
	FeedbackCount int
	SchemaVersion int
	Tempered      bool
}

// ApplyMarkdownLine applies a single parsed markdown line to artifact metadata.
func ApplyMarkdownLine(line string, meta *ArtifactMeta) {
	if val, ok := ParseMarkdownField(line, "ID"); ok {
		meta.ID = val
	}
	if val, ok := ParseMarkdownField(line, "Maturity"); ok {
		meta.Maturity = strings.ToLower(val)
	}
	if val, ok := ParseMarkdownField(line, "Utility"); ok {
		// ag-chvc: assign only on a clean parse; a malformed value keeps the prior
		// default rather than silently corrupting it to zero.
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			meta.Utility = f
		}
	}
	if val, ok := ParseMarkdownField(line, "Confidence"); ok {
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			meta.Confidence = f
		}
	}
	if val, ok := ParseMarkdownField(line, "Schema Version"); ok {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			meta.SchemaVersion = n
		}
	}
	if val, ok := ParseMarkdownField(line, "Status"); ok {
		if strings.ToLower(val) == "tempered" || strings.ToLower(val) == "locked" {
			meta.Tempered = true
		}
	}
}

// ParseMarkdownMeta extracts metadata from markdown content by scanning lines.
func ParseMarkdownMeta(content string) ArtifactMeta {
	var meta ArtifactMeta
	for _, line := range strings.Split(content, "\n") {
		ApplyMarkdownLine(strings.TrimSpace(line), &meta)
	}
	return meta
}

// ValidateArtifactMeta checks whether an artifact meets temper requirements.
// Returns lists of issues (hard failures) and warnings (soft).
func ValidateArtifactMeta(meta ArtifactMeta, minMaturity string, minUtility float64, minFeedback int) (issues, warnings []string) {
	if meta.ID == "" {
		issues = append(issues, "missing ID")
	}
	if meta.SchemaVersion == 0 {
		warnings = append(warnings, "missing schema_version (add 'Schema Version: 1')")
	}

	maturityOrder := map[string]int{
		"provisional": 1,
		"candidate":   2,
		"established": 3,
	}
	if maturityOrder[meta.Maturity] < maturityOrder[minMaturity] {
		issues = append(issues, fmt.Sprintf("maturity %s below minimum %s", meta.Maturity, minMaturity))
	}
	if meta.Utility < minUtility {
		issues = append(issues, fmt.Sprintf("utility %.2f below minimum %.2f", meta.Utility, minUtility))
	}
	if meta.FeedbackCount < minFeedback {
		issues = append(issues, fmt.Sprintf("feedback count %d below minimum %d", meta.FeedbackCount, minFeedback))
	}
	return issues, warnings
}

// ExpandGlobPattern expands a glob pattern, filtering results to baseDir.
func ExpandGlobPattern(baseDir, pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	var files []string
	for _, match := range matches {
		if IsContainedPath(baseDir, match) {
			files = append(files, match)
		}
	}
	return files, nil
}

// ExpandDirectoryRecursive walks a directory recursively collecting artifact files.
func ExpandDirectoryRecursive(baseDir, dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !IsContainedPath(baseDir, path) {
			return nil
		}
		if IsArtifactFile(info.Name()) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ExpandDirectoryFlat collects artifact files from a directory (non-recursive).
func ExpandDirectoryFlat(dir string) []string {
	var files []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && IsArtifactFile(e.Name()) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

// ExpandSinglePattern expands a single file pattern within baseDir.
// When recursive is true, directories are walked recursively.
func ExpandSinglePattern(baseDir, pattern string, recursive bool) ([]string, error) {
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}

	if !IsContainedPath(baseDir, pattern) {
		return nil, fmt.Errorf("path %q is outside allowed directory", pattern)
	}

	info, err := os.Stat(pattern)
	if err == nil && info.IsDir() {
		if recursive {
			return ExpandDirectoryRecursive(baseDir, pattern)
		}
		return ExpandDirectoryFlat(pattern), nil
	}

	matches, err := ExpandGlobPattern(baseDir, pattern)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		if _, err := os.Stat(pattern); err == nil {
			return []string{pattern}, nil
		}
	}

	return matches, nil
}

// ExpandFilePatterns expands glob patterns and handles recursive flag.
func ExpandFilePatterns(baseDir string, patterns []string, recursive bool) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		expanded, err := ExpandSinglePattern(baseDir, pattern, recursive)
		if err != nil {
			return nil, err
		}
		files = append(files, expanded...)
	}
	return files, nil
}
