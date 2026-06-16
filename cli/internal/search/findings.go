package search

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// FindingMatchRatio returns the fraction of salient query tokens (0.0–1.0) present
// in the finding's haystack; 1.0 for an empty query. This is the relevance signal:
// FindingMatchesQuery thresholds it at >0, and callers fold it into MatchConfidence
// so a partial (one-token) match ranks below a full multi-token match (mirrors the
// learnings matcher). Without it, any-token matching + freshness/utility-only ranking
// lets a fresh one-token false positive evict a stale but relevant finding (age-r3w).
func FindingMatchRatio(f KnowledgeFinding, queryLower string) float64 {
	if queryLower == "" {
		return 1.0
	}
	haystack := strings.ToLower(strings.Join([]string{
		f.ID, f.Title, f.Summary, f.SourceSkill, f.Severity,
		f.Detectability, f.Status,
		strings.Join(f.ScopeTags, " "),
		strings.Join(f.ApplicableWhen, " "),
		strings.Join(f.ApplicableLanguages, " "),
		strings.Join(f.CompilerTargets, " "),
	}, " "))
	return MatchRatio(QueryTokens(queryLower), haystack, "", "")
}

// FindingMatchesQuery returns true if at least one salient query token appears in
// the finding's haystack (token-based; replaces the prior strings.Contains whole-query
// substring test that left 0/58 gold findings unmatchable by natural-language queries —
// age-r3w). Ranking relevance is carried by FindingMatchRatio → MatchConfidence.
func FindingMatchesQuery(f KnowledgeFinding, queryLower string) bool {
	return FindingMatchRatio(f, queryLower) > 0
}

// ParseFindingFile extracts finding info from a markdown file.
func ParseFindingFile(path string) (KnowledgeFinding, error) {
	f := KnowledgeFinding{
		ID:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Source: path,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	lines := strings.Split(string(content), "\n")
	fm, contentStart := ParseFrontMatter(lines)
	if fm.HasUtility {
		f.Utility = fm.Utility
	}
	ParseFindingFrontmatterFields(&f, lines, contentStart)
	ParseFindingTitle(&f, lines, contentStart, path)
	f.Summary = ExtractSummary(lines, contentStart)
	return f, nil
}

// ParseFindingFrontmatterFields populates finding fields from YAML frontmatter lines.
func ParseFindingFrontmatterFields(f *KnowledgeFinding, lines []string, contentStart int) {
	for i := 1; i < len(lines) && i < contentStart; i++ {
		line := strings.TrimSpace(lines[i])
		ApplyFindingField(f, line)
	}
}

// ApplyFindingField sets a single frontmatter field on the finding.
func ApplyFindingField(f *KnowledgeFinding, line string) {
	switch {
	case strings.HasPrefix(line, "id:"):
		f.ID = TrimField(line)
	case strings.HasPrefix(line, "title:"):
		f.Title = TrimField(line)
	case strings.HasPrefix(line, "source_skill:"), strings.HasPrefix(line, "source-skill:"):
		f.SourceSkill = TrimField(line)
	case strings.HasPrefix(line, "severity:"):
		f.Severity = TrimField(line)
	case strings.HasPrefix(line, "detectability:"):
		f.Detectability = TrimField(line)
	case strings.HasPrefix(line, "status:"):
		f.Status = TrimField(line)
	case strings.HasPrefix(line, "compiler_targets:"), strings.HasPrefix(line, "compiler-targets:"):
		f.CompilerTargets = ParseListField(TrimField(line))
	case strings.HasPrefix(line, "scope_tags:"), strings.HasPrefix(line, "scope-tags:"):
		f.ScopeTags = ParseListField(TrimField(line))
	case strings.HasPrefix(line, "applicable_when:"), strings.HasPrefix(line, "applicable-when:"):
		f.ApplicableWhen = ParseListField(TrimField(line))
	case strings.HasPrefix(line, "applicable_languages:"), strings.HasPrefix(line, "applicable-languages:"):
		f.ApplicableLanguages = ParseListField(TrimField(line))
	case strings.HasPrefix(line, "hit_count:"), strings.HasPrefix(line, "hit-count:"):
		f.HitCount = ParseIntField(TrimField(line))
	case strings.HasPrefix(line, "last_cited:"), strings.HasPrefix(line, "last-cited:"):
		f.LastCited = TrimField(line)
	case strings.HasPrefix(line, "retired_by:"), strings.HasPrefix(line, "retired-by:"):
		f.RetiredBy = TrimField(line)
	}
}

// ParseFindingTitle extracts the title from content body or falls back to filename.
func ParseFindingTitle(f *KnowledgeFinding, lines []string, contentStart int, path string) {
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

// ApplyFindingFreshness sets freshness and default utility on a finding.
func ApplyFindingFreshness(f *KnowledgeFinding, file string, now time.Time) {
	info, err := os.Stat(file)
	if err != nil || info == nil {
		f.FreshnessScore = 0.5
		if f.Utility == 0 {
			f.Utility = types.InitialUtility
		}
		return
	}
	anchorTime := info.ModTime()
	if citedAt, ok := ParseFindingTime(f.LastCited); ok && citedAt.After(anchorTime) {
		anchorTime = citedAt
	}
	f.AgeWeeks = now.Sub(anchorTime).Hours() / (24 * 7)
	f.FreshnessScore = FreshnessScore(f.AgeWeeks)
	if f.Utility == 0 {
		f.Utility = types.InitialUtility
	}
}

// FindingStatusActiveForRetrieval returns false for retired/superseded findings.
func FindingStatusActiveForRetrieval(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "retired", "superseded":
		return false
	default:
		return true
	}
}

// ParseFindingTime parses a time string in RFC3339 format.
func ParseFindingTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

// TrimField extracts the value after the first colon.
func TrimField(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
}

// ParseListField splits a comma-separated list field.
func ParseListField(raw string) []string {
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

// ParseIntField parses an integer from a string field.
func ParseIntField(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}
