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

func collectFindings(cwd, query string, limit int, globalDir string, globalWeight float64) ([]knowledgeFinding, error) {
	return collectFindingsWithOptions(cwd, query, limit, globalDir, globalWeight, false)
}

func collectFindingsWithOptions(cwd, query string, limit int, globalDir string, globalWeight float64, includeInactive bool) ([]knowledgeFinding, error) {
	queryLower := strings.ToLower(query)
	now := time.Now()

	findingsDir := resolveFindingsDir(cwd)
	local, err := collectFindingsFromDir(findingsDir, queryLower, now, false, includeInactive)
	if err != nil {
		return nil, err
	}

	localPaths := indexFindingPaths(findingsDir)
	findings := append([]knowledgeFinding{}, local...)
	if globalDir != "" {
		findings = append(findings, collectGlobalFindings(globalDir, localPaths, queryLower, now, includeInactive)...)
	}

	if len(findings) == 0 {
		return nil, nil
	}

	items := make([]scorable, len(findings))
	for i := range findings {
		items[i] = &findings[i]
	}
	applyCompositeScoringTo(items, types.DefaultLambda)

	applyGlobalFindingWeight(findings, globalWeight)

	slices.SortFunc(findings, func(a, b knowledgeFinding) int {
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
	if len(findings) > limit {
		findings = findings[:limit]
	}
	return findings, nil
}

func resolveFindingsDir(cwd string) string {
	findingsDir := filepath.Join(cwd, ".agents", SectionFindings)
	if _, err := os.Stat(findingsDir); os.IsNotExist(err) {
		return findAgentsSubdir(cwd, SectionFindings)
	}
	return findingsDir
}

func indexFindingPaths(dir string) map[string]bool {
	localPaths := make(map[string]bool)
	localFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	for _, f := range localFiles {
		if abs, err := filepath.Abs(f); err == nil {
			localPaths[abs] = true
		}
	}
	return localPaths
}

func collectGlobalFindings(globalDir string, localPaths map[string]bool, queryLower string, now time.Time, includeInactive bool) []knowledgeFinding {
	var findings []knowledgeFinding
	for _, file := range walkKnowledgeFiles(globalDir, ".md") {
		if abs, err := filepath.Abs(file); err == nil && localPaths[abs] {
			continue
		}
		f, err := parseFindingFile(file)
		if err != nil {
			continue
		}
		applyFindingFreshness(&f, file, now)
		if !includeInactive && !findingStatusActiveForRetrieval(f.Status) {
			continue
		}
		if !findingMatchesQuery(f, queryLower) {
			continue
		}
		f.Global = true
		findings = append(findings, f)
	}
	return findings
}

func applyGlobalFindingWeight(findings []knowledgeFinding, globalWeight float64) {
	if globalWeight <= 0 || globalWeight >= 1.0 {
		return
	}
	for i := range findings {
		if findings[i].Global {
			findings[i].CompositeScore *= globalWeight
		}
	}
}

func collectFindingsFromDir(dir, queryLower string, now time.Time, isGlobal, includeInactive bool) ([]knowledgeFinding, error) {
	if dir == "" {
		return nil, nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	var result []knowledgeFinding
	for _, file := range files {
		f, err := parseFindingFile(file)
		if err != nil {
			continue
		}
		applyFindingFreshness(&f, file, now)
		if !includeInactive && !findingStatusActiveForRetrieval(f.Status) {
			continue
		}
		if !findingMatchesQuery(f, queryLower) {
			continue
		}
		f.Global = isGlobal
		result = append(result, f)
	}
	return result, nil
}

// Thin wrappers — canonical definitions in internal/search/findings.go.
func applyFindingFreshness(f *knowledgeFinding, file string, now time.Time) {
	search.ApplyFindingFreshness(f, file, now)
}
func findingMatchesQuery(f knowledgeFinding, queryLower string) bool {
	return search.FindingMatchesQuery(f, queryLower)
}
func parseFindingFile(path string) (knowledgeFinding, error) { return search.ParseFindingFile(path) }
func parseFindingFrontmatterFields(f *knowledgeFinding, lines []string, contentStart int) {
	search.ParseFindingFrontmatterFields(f, lines, contentStart)
}
func applyFindingField(f *knowledgeFinding, line string) { search.ApplyFindingField(f, line) }
func parseFindingTitle(f *knowledgeFinding, lines []string, contentStart int, path string) {
	search.ParseFindingTitle(f, lines, contentStart, path)
}
func trimField(line string) string                  { return search.TrimField(line) }
func parseListField(raw string) []string            { return search.ParseListField(raw) }
func parseIntField(raw string) int                  { return search.ParseIntField(raw) }
func parseFindingTime(raw string) (time.Time, bool) { return search.ParseFindingTime(raw) }
func findingStatusActiveForRetrieval(status string) bool {
	return search.FindingStatusActiveForRetrieval(status)
}
