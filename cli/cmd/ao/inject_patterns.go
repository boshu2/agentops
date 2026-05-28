// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/types"
)

// Thin wrappers — canonical definitions in internal/search/patterns.go.
func enrichPatternFreshness(p *pattern, file string, now time.Time) {
	search.EnrichPatternFreshness(p, file, now)
}
func patternMatchesQuery(p pattern, queryLower string) bool {
	return search.PatternMatchesQuery(p, queryLower)
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

func parsePatternFile(path string) (pattern, error) { return search.ParsePatternFile(path) }
func parseFrontmatterBlock(lines []string) (int, float64) {
	return search.ParseFrontmatterBlock(lines)
}
func assembleDescriptionFrom(lines []string, i int) string {
	return search.AssembleDescriptionFrom(lines, i)
}
func isContentLine(line string) bool { return search.IsContentLine(line) }
func extractPatternNameAndDescription(lines []string, contentStart int) (string, string) {
	return search.ExtractPatternNameAndDescription(lines, contentStart)
}
