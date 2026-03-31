package main

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// enrichPatternFreshness sets age, freshness, and default utility on a pattern
// based on the file's modification time.
func enrichPatternFreshness(p *pattern, file string, now time.Time) {
	info, statErr := os.Stat(file)
	if statErr != nil {
		VerbosePrintf("Warning: stat %s: %v\n", file, statErr)
	}
	if info != nil {
		ageHours := now.Sub(info.ModTime()).Hours()
		p.AgeWeeks = ageHours / (24 * 7)
		p.FreshnessScore = freshnessScore(p.AgeWeeks)
	} else {
		p.FreshnessScore = 0.5
	}
	if p.Utility == 0 {
		p.Utility = types.InitialUtility
	}
}

// patternMatchesQuery returns true if the pattern name or description contains
// the query (case-insensitive). An empty query matches everything.
func patternMatchesQuery(p pattern, queryLower string) bool {
	if queryLower == "" {
		return true
	}
	content := strings.ToLower(p.Name + " " + p.Description)
	return strings.Contains(content, queryLower)
}

// collectPatterns finds patterns from .agents/patterns/ and optionally ~/.agents/patterns/.
// Global patterns receive a post-scoring weight penalty (globalWeight, default 0.8).
func collectPatterns(cwd, query string, limit int, globalDir string, globalWeight float64) ([]pattern, error) {
	patternsDir := filepath.Join(cwd, ".agents", SectionPatterns)
	if _, err := os.Stat(patternsDir); os.IsNotExist(err) {
		patternsDir = findAgentsSubdir(cwd, SectionPatterns)
	}

	queryLower := strings.ToLower(query)
	now := time.Now()

	local, err := collectPatternsFromDir(patternsDir, queryLower, now, false)
	if err != nil {
		return nil, err
	}

	localPaths := buildLocalPathSet(patternsDir)
	global, err := collectGlobalPatterns(globalDir, localPaths, queryLower, now)
	if err != nil {
		return nil, err
	}

	patterns := append(local, global...)
	if len(patterns) == 0 {
		return nil, nil
	}

	scoreAndWeighPatterns(patterns, globalWeight)

	slices.SortFunc(patterns, func(a, b pattern) int {
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
	if len(patterns) > limit {
		patterns = patterns[:limit]
	}

	return patterns, nil
}

func collectPatternsFromDir(dir, queryLower string, now time.Time, isGlobal bool) ([]pattern, error) {
	if dir == "" {
		return nil, nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	var result []pattern
	for _, file := range files {
		p, err := parsePatternFile(file)
		if err != nil {
			continue
		}
		enrichPatternFreshness(&p, file, now)
		if !patternMatchesQuery(p, queryLower) {
			continue
		}
		p.Global = isGlobal
		result = append(result, p)
	}
	return result, nil
}

func buildLocalPathSet(patternsDir string) map[string]bool {
	localPaths := make(map[string]bool)
	if patternsDir == "" {
		return localPaths
	}
	localFiles, _ := filepath.Glob(filepath.Join(patternsDir, "*.md"))
	for _, f := range localFiles {
		if abs, err := filepath.Abs(f); err == nil {
			localPaths[abs] = true
		}
	}
	return localPaths
}

func collectGlobalPatterns(globalDir string, localPaths map[string]bool, queryLower string, now time.Time) ([]pattern, error) {
	if globalDir == "" {
		return nil, nil
	}
	globalFiles := walkKnowledgeFiles(globalDir, ".md")
	var result []pattern
	for _, file := range globalFiles {
		if abs, err := filepath.Abs(file); err == nil && localPaths[abs] {
			continue
		}
		p, err := parsePatternFile(file)
		if err != nil {
			continue
		}
		enrichPatternFreshness(&p, file, now)
		if !patternMatchesQuery(p, queryLower) {
			continue
		}
		p.Global = true
		result = append(result, p)
	}
	return result, nil
}

func scoreAndWeighPatterns(patterns []pattern, globalWeight float64) {
	items := make([]scorable, len(patterns))
	for i := range patterns {
		items[i] = &patterns[i]
	}
	applyCompositeScoringTo(items, types.DefaultLambda)

	if globalWeight > 0 && globalWeight < 1.0 {
		for i := range patterns {
			if patterns[i].Global {
				patterns[i].CompositeScore *= globalWeight
			}
		}
	}
}

// parsePatternFile extracts pattern info from a markdown file
func parsePatternFile(path string) (pattern, error) {
	p := pattern{
		Name:     strings.TrimSuffix(filepath.Base(path), ".md"),
		FilePath: path,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}

	lines := strings.Split(string(content), "\n")
	contentStart, utility := parseFrontmatterBlock(lines)
	if utility > 0 {
		p.Utility = utility
	}
	name, description := extractPatternNameAndDescription(lines, contentStart)
	if name != "" {
		p.Name = name
	}
	if description != "" {
		p.Description = description
	}

	return p, nil
}

// parseFrontmatterBlock scans YAML frontmatter and returns content start index and utility value.
// Delegates to parseFrontMatter (inject_learnings.go) for the actual parsing.
func parseFrontmatterBlock(lines []string) (contentStart int, utility float64) {
	fm, start := parseFrontMatter(lines)
	if fm.HasUtility {
		utility = fm.Utility
	}
	return start, utility
}

// assembleDescriptionFrom builds a description by joining the line at index i
// with up to one following continuation line.
func assembleDescriptionFrom(lines []string, i int) string {
	desc := strings.TrimSpace(lines[i])
	for j := i + 1; j < len(lines) && j < i+2; j++ {
		nextLine := strings.TrimSpace(lines[j])
		if nextLine == "" || strings.HasPrefix(nextLine, "#") {
			break
		}
		desc += " " + nextLine
	}
	return truncateText(desc, 150)
}

// isContentLine returns true if the trimmed line is a non-empty body line
// (not a heading, frontmatter delimiter, or inline metadata).
func isContentLine(line string) bool {
	return line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "---") && !isInlineMetadata(line)
}

// extractPatternNameAndDescription scans content lines for title and description.
func extractPatternNameAndDescription(lines []string, contentStart int) (name, description string) {
	for i := contentStart; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			name = strings.TrimPrefix(line, "# ")
			continue
		}
		if description == "" && isContentLine(line) {
			description = assembleDescriptionFrom(lines, i)
			break
		}
	}
	return name, description
}
