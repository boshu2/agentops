package main

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

func collectFindings(cwd, query string, limit int, globalDir string, globalWeight float64) ([]knowledgeFinding, error) {
	return collectFindingsWithOptions(cwd, query, limit, globalDir, globalWeight, false)
}

func collectFindingsWithOptions(cwd, query string, limit int, globalDir string, globalWeight float64, includeInactive bool) ([]knowledgeFinding, error) {
	findingsDir := filepath.Join(cwd, ".agents", SectionFindings)
	if _, err := os.Stat(findingsDir); os.IsNotExist(err) {
		findingsDir = findAgentsSubdir(cwd, SectionFindings)
	}

	queryLower := strings.ToLower(query)
	now := time.Now()

	local, err := collectFindingsFromDir(findingsDir, queryLower, now, false, includeInactive)
	if err != nil {
		return nil, err
	}

	localPaths := make(map[string]bool)
	localFiles, _ := filepath.Glob(filepath.Join(findingsDir, "*.md"))
	for _, f := range localFiles {
		if abs, err := filepath.Abs(f); err == nil {
			localPaths[abs] = true
		}
	}

	findings := append([]knowledgeFinding{}, local...)
	if globalDir != "" {
		globalFiles := walkKnowledgeFiles(globalDir, ".md")
		for _, file := range globalFiles {
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
	}

	if len(findings) == 0 {
		return nil, nil
	}

	items := make([]scorable, len(findings))
	for i := range findings {
		items[i] = &findings[i]
	}
	applyCompositeScoringTo(items, types.DefaultLambda)

	if globalWeight > 0 && globalWeight < 1.0 {
		for i := range findings {
			if findings[i].Global {
				findings[i].CompositeScore *= globalWeight
			}
		}
	}

	slices.SortFunc(findings, func(a, b knowledgeFinding) int {
		return cmp.Compare(b.CompositeScore, a.CompositeScore)
	})
	if len(findings) > limit {
		findings = findings[:limit]
	}
	return findings, nil
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

func applyFindingFreshness(f *knowledgeFinding, file string, now time.Time) {
	info, err := os.Stat(file)
	if err != nil || info == nil {
		f.FreshnessScore = 0.5
		if f.Utility == 0 {
			f.Utility = types.InitialUtility
		}
		return
	}
	anchorTime := info.ModTime()
	if citedAt, ok := parseFindingTime(f.LastCited); ok && citedAt.After(anchorTime) {
		anchorTime = citedAt
	}
	f.AgeWeeks = now.Sub(anchorTime).Hours() / (24 * 7)
	f.FreshnessScore = freshnessScore(f.AgeWeeks)
	if f.Utility == 0 {
		f.Utility = types.InitialUtility
	}
}

func findingMatchesQuery(f knowledgeFinding, queryLower string) bool {
	if queryLower == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		f.ID,
		f.Title,
		f.Summary,
		f.SourceSkill,
		f.Severity,
		f.Detectability,
		f.Status,
		strings.Join(f.ScopeTags, " "),
		strings.Join(f.ApplicableWhen, " "),
		strings.Join(f.ApplicableLanguages, " "),
		strings.Join(f.CompilerTargets, " "),
	}, " "))
	return strings.Contains(haystack, queryLower)
}

func parseFindingFile(path string) (knowledgeFinding, error) {
	f := knowledgeFinding{
		ID:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Source: path,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	lines := strings.Split(string(content), "\n")
	fm, contentStart := parseFrontMatter(lines)
	if fm.HasUtility {
		f.Utility = fm.Utility
	}
	parseFindingFrontmatterFields(&f, lines, contentStart)
	parseFindingTitle(&f, lines, contentStart, path)
	f.Summary = extractSummary(lines, contentStart)
	return f, nil
}

// parseFindingFrontmatterFields populates finding fields from YAML frontmatter lines.
func parseFindingFrontmatterFields(f *knowledgeFinding, lines []string, contentStart int) {
	for i := 1; i < len(lines) && i < contentStart; i++ {
		line := strings.TrimSpace(lines[i])
		applyFindingField(f, line)
	}
}

// applyFindingField sets a single frontmatter field on the finding.
func applyFindingField(f *knowledgeFinding, line string) {
	switch {
	case strings.HasPrefix(line, "id:"):
		f.ID = trimField(line)
	case strings.HasPrefix(line, "title:"):
		f.Title = trimField(line)
	case strings.HasPrefix(line, "source_skill:"), strings.HasPrefix(line, "source-skill:"):
		f.SourceSkill = trimField(line)
	case strings.HasPrefix(line, "severity:"):
		f.Severity = trimField(line)
	case strings.HasPrefix(line, "detectability:"):
		f.Detectability = trimField(line)
	case strings.HasPrefix(line, "status:"):
		f.Status = trimField(line)
	case strings.HasPrefix(line, "compiler_targets:"), strings.HasPrefix(line, "compiler-targets:"):
		f.CompilerTargets = parseListField(trimField(line))
	case strings.HasPrefix(line, "scope_tags:"), strings.HasPrefix(line, "scope-tags:"):
		f.ScopeTags = parseListField(trimField(line))
	case strings.HasPrefix(line, "applicable_when:"), strings.HasPrefix(line, "applicable-when:"):
		f.ApplicableWhen = parseListField(trimField(line))
	case strings.HasPrefix(line, "applicable_languages:"), strings.HasPrefix(line, "applicable-languages:"):
		f.ApplicableLanguages = parseListField(trimField(line))
	case strings.HasPrefix(line, "hit_count:"), strings.HasPrefix(line, "hit-count:"):
		f.HitCount = parseIntField(trimField(line))
	case strings.HasPrefix(line, "last_cited:"), strings.HasPrefix(line, "last-cited:"):
		f.LastCited = trimField(line)
	case strings.HasPrefix(line, "retired_by:"), strings.HasPrefix(line, "retired-by:"):
		f.RetiredBy = trimField(line)
	}
}

// parseFindingTitle extracts the title from content body or falls back to filename.
func parseFindingTitle(f *knowledgeFinding, lines []string, contentStart int, path string) {
	if f.Title == "" {
		for i := contentStart; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(line, "# ") {
				f.Title = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}
	if f.Title == "" {
		f.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
}

func trimField(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
}

func parseListField(raw string) []string {
	raw = strings.TrimSpace(strings.Trim(raw, "[]"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Trim(strings.TrimSpace(part), "\"'")
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseIntField(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}

func parseFindingTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func findingStatusActiveForRetrieval(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "retired", "superseded":
		return false
	default:
		return true
	}
}
