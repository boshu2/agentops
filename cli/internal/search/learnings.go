package search

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// ValidPhases is the set of canonical RPI phase values for source_phase.
var ValidPhases = map[string]bool{
	"research": true, "plan": true, "implement": true, "validate": true,
}

// SanitizeSourcePhase returns the phase if valid, or empty string if not.
func SanitizeSourcePhase(phase string) string {
	p := strings.ToLower(strings.TrimSpace(phase))
	if ValidPhases[p] {
		return p
	}
	return ""
}

// QueryTokens splits a lowercased query into individual search tokens.
// Tokens shorter than 2 characters are dropped to avoid noise.
func QueryTokens(queryLower string) []string {
	words := strings.Fields(queryLower)
	tokens := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 2 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// MatchesQuery returns true if the learning matches at least one query token.
func MatchesQuery(tokens []string, title, summary, body string) bool {
	return MatchRatio(tokens, title, summary, body) > 0
}

// MatchRatio returns the fraction of query tokens found in the text (0.0 to 1.0).
func MatchRatio(tokens []string, title, summary, body string) float64 {
	if len(tokens) == 0 {
		return 1.0
	}
	text := strings.ToLower(title + " " + summary + " " + body)
	matched := 0
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			matched++
		}
	}
	return float64(matched) / float64(len(tokens))
}

// FrontMatter holds parsed YAML front matter fields.
type FrontMatter struct {
	SupersededBy string
	PromotedTo   string
	Utility      float64
	HasUtility   bool
	SourceBead   string
	SourcePhase  string
	Maturity     string
	Stability    string
}

// ParseFrontMatter extracts YAML front matter from markdown content lines.
func ParseFrontMatter(lines []string) (FrontMatter, int) {
	var fm FrontMatter

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, 0
	}

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			return fm, i + 1
		}
		ParseFrontMatterLine(line, &fm)
	}
	return fm, 0
}

// ParseFrontMatterLine parses a single YAML front matter line into fm fields.
func ParseFrontMatterLine(line string, fm *FrontMatter) {
	switch {
	case strings.HasPrefix(line, "superseded_by:"), strings.HasPrefix(line, "superseded-by:"):
		fm.SupersededBy = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "promoted_to:"), strings.HasPrefix(line, "promoted-to:"):
		fm.PromotedTo = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "utility:"):
		utilityStr := strings.TrimSpace(strings.TrimPrefix(line, "utility:"))
		if utility, err := strconv.ParseFloat(utilityStr, 64); err == nil && utility > 0 {
			fm.Utility = utility
			fm.HasUtility = true
		}
	case strings.HasPrefix(line, "source_bead:"), strings.HasPrefix(line, "source-bead:"):
		fm.SourceBead = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "source_phase:"), strings.HasPrefix(line, "source-phase:"):
		fm.SourcePhase = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "maturity:"):
		fm.Maturity = strings.TrimSpace(strings.TrimPrefix(line, "maturity:"))
	case strings.HasPrefix(line, "stability:"):
		fm.Stability = strings.TrimSpace(strings.TrimPrefix(line, "stability:"))
	}
}

// IsSuperseded returns true if the front matter indicates a superseded learning.
func IsSuperseded(fm FrontMatter) bool {
	return fm.SupersededBy != "" && fm.SupersededBy != "null" && fm.SupersededBy != "~"
}

// IsPromoted returns true if the learning was promoted to a global location.
func IsPromoted(fm FrontMatter) bool {
	return fm.PromotedTo != "" && fm.PromotedTo != "null" && fm.PromotedTo != "~"
}

// IsInlineMetadata returns true for lines like "**ID**: L1" or "**Category**: process"
// that are formatting artifacts from older learning/pattern file formats.
func IsInlineMetadata(line string) bool {
	for _, field := range []string{"ID", "Category", "Confidence", "Date", "Source", "Type", "Status"} {
		if strings.HasPrefix(line, "**"+field+"**:") || strings.HasPrefix(line, "**"+field+":**") {
			return true
		}
	}
	return false
}

// ExtractSummary finds the first content paragraph after headings,
// skipping inline metadata lines.
func ExtractSummary(lines []string, startIdx int) string {
	for i := startIdx; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || IsInlineMetadata(line) {
			continue
		}
		summary := line
		for j := i + 1; j < len(lines) && j < i+3; j++ {
			nextLine := strings.TrimSpace(lines[j])
			if nextLine == "" || strings.HasPrefix(nextLine, "#") || IsInlineMetadata(nextLine) {
				break
			}
			summary += " " + nextLine
		}
		return TruncateText(summary, 200)
	}
	return ""
}

// ParseLearningBody extracts title, ID, and maturity from markdown body lines.
func ParseLearningBody(lines []string, start int, l *Learning) {
	defaultID := filepath.Base(l.Source)
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "# ") && l.Title == "" {
			l.Title = strings.TrimPrefix(line, "# ")
		} else if (strings.HasPrefix(line, "ID:") || strings.HasPrefix(line, "id:")) && l.ID == defaultID {
			l.ID = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		if l.Maturity == "" {
			trimmed := strings.TrimPrefix(line, "- ")
			if strings.HasPrefix(trimmed, "**Maturity**:") {
				l.Maturity = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Maturity**:"))
			}
		}
	}
}

// ParseLearningFile extracts learning info from a file.
// Sets Superseded=true if superseded_by or promoted_to field is found.
func ParseLearningFile(path string) (Learning, error) {
	if strings.HasSuffix(path, ".jsonl") {
		return ParseLearningJSONL(path)
	}

	l := Learning{
		ID:     filepath.Base(path),
		Source: path,
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return l, err
	}

	lines := strings.Split(string(content), "\n")
	fm, contentStart := ParseFrontMatter(lines)

	if IsSuperseded(fm) {
		l.Superseded = true
		return l, nil
	}
	if IsPromoted(fm) {
		l.Superseded = true
		return l, nil
	}
	if fm.HasUtility {
		l.Utility = fm.Utility
	}
	l.SourceBead = fm.SourceBead
	l.SourcePhase = SanitizeSourcePhase(fm.SourcePhase)
	l.Maturity = fm.Maturity
	l.Stability = fm.Stability

	ParseLearningBody(lines, contentStart, &l)
	l.Summary = ExtractSummary(lines, contentStart)
	l.BodyText = strings.Join(lines[contentStart:], "\n")

	if l.Title == "" {
		l.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return l, nil
}

// PopulateLearningFromJSON fills learning fields from a parsed JSON map.
func PopulateLearningFromJSON(data map[string]any, l *Learning) {
	if id, ok := data["id"].(string); ok {
		l.ID = id
	}
	if title, ok := data["title"].(string); ok {
		l.Title = title
	}
	if summary, ok := data["summary"].(string); ok {
		l.Summary = TruncateText(summary, 200)
	}
	if content, ok := data["content"].(string); ok && l.Summary == "" {
		l.Summary = TruncateText(content, 200)
	}
	if utility, ok := data["utility"].(float64); ok && utility > 0 {
		l.Utility = utility
	}
	if sb, ok := data["source_bead"].(string); ok {
		l.SourceBead = sb
	}
	if sp, ok := data["source_phase"].(string); ok {
		l.SourcePhase = SanitizeSourcePhase(sp)
	}
	if m, ok := data["maturity"].(string); ok {
		l.Maturity = m
	}
	if s, ok := data["stability"].(string); ok {
		l.Stability = s
	}
}

// ParseLearningJSONL extracts learning from JSONL file.
func ParseLearningJSONL(path string) (Learning, error) {
	l := Learning{
		ID:      filepath.Base(path),
		Source:  path,
		Utility: types.InitialUtility,
	}

	f, err := os.Open(path)
	if err != nil {
		return l, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return l, nil
	}

	var data map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &data); err != nil {
		return l, nil
	}

	if supersededBy, ok := data["superseded_by"]; ok && supersededBy != nil && supersededBy != "" {
		l.Superseded = true
		return l, nil
	}

	PopulateLearningFromJSON(data, &l)
	if content, ok := data["content"].(string); ok {
		l.BodyText = content
	}
	return l, nil
}

// PassesQualityGate returns true if a learning meets minimum injection standards.
func PassesQualityGate(l Learning) bool {
	mat := types.Maturity(l.Maturity)
	if mat == "" {
		mat = types.MaturityProvisional
	}
	switch mat {
	case types.MaturityProvisional, types.MaturityCandidate, types.MaturityEstablished:
		// maturity OK
	default:
		return false
	}
	if l.Utility > 0 && l.Utility <= 0.3 {
		return false
	}
	if l.BodyText != "" && len(strings.TrimSpace(l.BodyText)) < 50 {
		return false
	}
	return true
}

// ApplyFreshnessToLearning sets the freshness score based on file modification time.
func ApplyFreshnessToLearning(l *Learning, file string, now time.Time) {
	info, _ := os.Stat(file)
	if info == nil {
		l.FreshnessScore = 0.5
		return
	}
	ageWeeks := now.Sub(info.ModTime()).Hours() / (24 * 7)
	l.AgeWeeks = ageWeeks
	l.FreshnessScore = FreshnessScore(ageWeeks)
}

// RankLearnings applies composite scoring and sorts by score descending.
func RankLearnings(learnings []Learning) {
	items := make([]Scorable, len(learnings))
	for i := range learnings {
		items[i] = &learnings[i]
	}
	ApplyCompositeScoringTo(items, types.DefaultLambda)
}

// ComputeDecayedConfidence applies exponential decay and clamps to a minimum of 0.1.
func ComputeDecayedConfidence(confidence, weeks float64) float64 {
	decayFactor := math.Exp(-weeks * types.ConfidenceDecayRate)
	result := confidence * decayFactor
	if result < 0.1 {
		return 0.1
	}
	return result
}

// JSONFloat extracts a float64 from a map, returning defaultVal if missing or non-positive.
func JSONFloat(data map[string]any, key string, defaultVal float64) float64 {
	if c, ok := data[key].(float64); ok && c > 0 {
		return c
	}
	return defaultVal
}

// JSONTimeField tries to parse a time.Time from the first non-empty string field found among keys.
func JSONTimeField(data map[string]any, keys ...string) time.Time {
	for _, k := range keys {
		if v, ok := data[k].(string); ok && v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// WriteDecayFields updates the data map with new confidence, timestamp, and incremented decay count.
func WriteDecayFields(data map[string]any, newConfidence float64, now time.Time) {
	data["confidence"] = newConfidence
	data["last_decay_at"] = now.Format(time.RFC3339)
	decayCount := 0.0
	if dc, ok := data["decay_count"].(float64); ok {
		decayCount = dc
	}
	data["decay_count"] = decayCount + 1
}

// ParseFrontmatterFromContent extracts specific fields from YAML frontmatter
// in a string. Field extraction is delegated to wiki.FrontmatterCodec.
func ParseFrontmatterFromContent(content string, fields ...string) map[string]string {
	return wiki.FrontmatterCodec{}.ExtractStringFields(strings.Split(content, "\n"), fields...)
}

// Section evidence types and helpers.

// LearningSectionCandidate represents a candidate section for matching.
type LearningSectionCandidate struct {
	Heading string
	Locator string
	Content string
	Snippet string
	Score   float64
	Index   int
}

const (
	SectionCoverageBonusCap    = 0.15
	SectionCoverageBonusWeight = 0.15
	SectionSnippetMaxChars     = 160
)

// BuildLearningSectionCandidates splits a learning's body into section candidates.
func BuildLearningSectionCandidates(l Learning, splitSections func(string) []string) []LearningSectionCandidate {
	body := strings.TrimSpace(l.BodyText)
	if body == "" {
		body = strings.TrimSpace(strings.Join([]string{l.Title, l.Summary}, "\n\n"))
	}
	if body == "" {
		return nil
	}

	rawSections := splitSections(body)
	if len(rawSections) == 0 {
		rawSections = []string{body}
	}

	candidates := make([]LearningSectionCandidate, 0, len(rawSections))
	seenLocators := make(map[string]int, len(rawSections))
	for idx, raw := range rawSections {
		heading, content := ExtractLearningSectionHeading(raw, l.Title, idx)
		locator := BuildLearningSectionLocator(heading, idx, seenLocators)
		candidates = append(candidates, LearningSectionCandidate{
			Heading: heading,
			Locator: locator,
			Content: content,
			Index:   idx,
		})
	}
	return candidates
}

// ExtractLearningSectionHeading extracts heading and content from a markdown section.
func ExtractLearningSectionHeading(section, fallbackTitle string, index int) (string, string) {
	lines := strings.Split(section, "\n")
	heading := strings.TrimSpace(fallbackTitle)
	bodyStart := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			heading = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			bodyStart = i + 1
		} else {
			bodyStart = i
		}
		break
	}

	if heading == "" {
		heading = fmt.Sprintf("Section %d", index+1)
	}

	content := strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	if content == "" {
		content = strings.TrimSpace(section)
	}
	return heading, content
}

// BuildLearningSectionLocator creates a unique locator for a section heading.
func BuildLearningSectionLocator(heading string, index int, seen map[string]int) string {
	slug := SlugifyLearningSectionHeading(heading)
	if slug == "" {
		slug = fmt.Sprintf("section-%d", index+1)
	}
	seen[slug]++
	if seen[slug] == 1 {
		return "heading:" + slug
	}
	return fmt.Sprintf("heading:%s#%d", slug, seen[slug])
}

// SlugifyLearningSectionHeading converts a heading to a URL-safe slug.
func SlugifyLearningSectionHeading(heading string) string {
	var sb strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			lastDash = false
		case !lastDash && sb.Len() > 0:
			sb.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(sb.String(), "-")
}
