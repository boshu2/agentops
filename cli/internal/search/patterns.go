package search

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// EnrichPatternFreshness sets age, freshness, and default utility on a pattern.
func EnrichPatternFreshness(p *Pattern, file string, now time.Time) {
	info, _ := os.Stat(file)
	if info != nil {
		ageHours := now.Sub(info.ModTime()).Hours()
		p.AgeWeeks = ageHours / (24 * 7)
		p.FreshnessScore = FreshnessScore(p.AgeWeeks)
	} else {
		p.FreshnessScore = 0.5
	}
	if p.Utility == 0 {
		p.Utility = types.InitialUtility
	}
}

// PatternMatchesQuery returns true if the pattern matches the query (case-insensitive).
func PatternMatchesQuery(p Pattern, queryLower string) bool {
	if queryLower == "" {
		return true
	}
	content := strings.ToLower(p.Name + " " + p.Description)
	return strings.Contains(content, queryLower)
}

// ParsePatternFile extracts pattern info from a markdown file.
func ParsePatternFile(path string) (Pattern, error) {
	p := Pattern{
		Name:     strings.TrimSuffix(filepath.Base(path), ".md"),
		FilePath: path,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}

	lines := strings.Split(string(content), "\n")
	contentStart, utility := ParseFrontmatterBlock(lines)
	if utility > 0 {
		p.Utility = utility
	}
	name, description := ExtractPatternNameAndDescription(lines, contentStart)
	if name != "" {
		p.Name = name
	}
	if description != "" {
		p.Description = description
	}

	return p, nil
}

// ParseFrontmatterBlock scans YAML frontmatter and returns the content start
// index and the utility value.
//
// Block-boundary detection is delegated to wiki.FrontmatterCodec; the utility
// field is read via the codec's field extractor and parsed with the same
// strconv semantics (positive float only) the legacy struct parser used.
func ParseFrontmatterBlock(lines []string) (contentStart int, utility float64) {
	codec := wiki.NewFrontmatterCodec()
	doc := codec.DecodeLines(lines)
	extracted := codec.ExtractStringFields(lines, "utility")
	if raw := strings.TrimSpace(extracted["utility"]); raw != "" {
		if u, err := strconv.ParseFloat(raw, 64); err == nil && u > 0 {
			utility = u
		}
	}
	return doc.ContentStart, utility
}

// AssembleDescriptionFrom builds a description by joining the line at index i
// with up to one following continuation line.
func AssembleDescriptionFrom(lines []string, i int) string {
	desc := strings.TrimSpace(lines[i])
	for j := i + 1; j < len(lines) && j < i+2; j++ {
		nextLine := strings.TrimSpace(lines[j])
		if nextLine == "" || strings.HasPrefix(nextLine, "#") {
			break
		}
		desc += " " + nextLine
	}
	return TruncateText(desc, 150)
}

// IsContentLine returns true if the trimmed line is a non-empty body line.
func IsContentLine(line string) bool {
	return line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "---") && !IsInlineMetadata(line)
}

// ExtractPatternNameAndDescription scans content lines for title and description.
func ExtractPatternNameAndDescription(lines []string, contentStart int) (name, description string) {
	for i := contentStart; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			name = strings.TrimPrefix(line, "# ")
			continue
		}
		if description == "" && IsContentLine(line) {
			description = AssembleDescriptionFrom(lines, i)
			break
		}
	}
	return name, description
}
