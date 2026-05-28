// Package skills scores the skills/ catalog against a free-text intent so the
// CLI can answer "which skill handles X?" without relying on oral tradition.
//
// The scoring engine (Score) is pure: it takes an already-loaded slice of
// SkillMeta and never touches the filesystem, which keeps it table-testable.
// Loading SKILL.md frontmatter from disk lives in load.go.
package skills

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// SkillMeta is the scoring input for one skill: its name, one-line
// description, optional triggers, and source path. Triggers are best-effort —
// most SKILL.md files carry intent as prose in Description, so Description is
// the primary signal and Triggers an optional boost.
type SkillMeta struct {
	Name        string
	Description string
	Triggers    []string
	Path        string
}

// Match is one scored result. Score is normalized to [0,1], where 1.0 means
// every query token matched a skill-name token.
type Match struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Path        string  `json:"path"`
	Score       float64 `json:"score"`
}

// Field weights: a query token that hits the skill name is the strongest
// signal, a trigger hit is next, and a description hit is the baseline. The
// trigger weight is deliberately below 2×weightDesc so that covering two query
// tokens via the description outranks a single trigger hit — broad coverage of
// the intent beats one narrow signal.
const (
	weightName    = 3.0
	weightTrigger = 1.5
	weightDesc    = 1.0
)

// stopwords are dropped from both the query and the skill haystack so common
// connective words do not inflate scores.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "on": true, "for": true, "is": true, "it": true,
	"with": true, "by": true, "at": true, "as": true, "be": true, "this": true,
}

// Score ranks every skill in metas against query, returning Matches sorted by
// score descending with ties broken by name ascending (stable, deterministic).
// An empty query yields all-zero scores. The score is normalized so results
// are comparable across queries of different lengths.
func Score(query string, metas []SkillMeta) []Match {
	qTokens := tokenize(query)
	matches := make([]Match, 0, len(metas))
	for _, m := range metas {
		nameToks := tokenize(m.Name)
		trigToks := tokenize(strings.Join(m.Triggers, " "))
		descToks := tokenize(m.Description)

		var raw float64
		for _, qt := range qTokens {
			switch {
			case tokenMatches(qt, nameToks):
				raw += weightName
			case tokenMatches(qt, trigToks):
				raw += weightTrigger
			case tokenMatches(qt, descToks):
				raw += weightDesc
			}
		}

		score := 0.0
		if len(qTokens) > 0 {
			score = raw / (float64(len(qTokens)) * weightName)
		}
		matches = append(matches, Match{
			Name:        m.Name,
			Description: m.Description,
			Path:        m.Path,
			Score:       roundScore(score),
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Name < matches[j].Name
	})
	return matches
}

// tokenize lowercases, splits on non-alphanumeric runes, and drops stopwords
// and single-character tokens, returning a deduplicated, order-preserving
// slice of meaningful tokens.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := make(map[string]bool, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 || stopwords[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// tokenMatches reports whether query token qt hits any token in toks. A hit is
// an exact match or a prefix-stem match (one token is a prefix of the other and
// both are at least 4 runes), which tolerates plurals and simple suffixes
// ("loop" ↔ "loops") without a full stemmer.
func tokenMatches(qt string, toks []string) bool {
	for _, t := range toks {
		if t == qt {
			return true
		}
		if len(qt) >= 4 && len(t) >= 4 && (strings.HasPrefix(t, qt) || strings.HasPrefix(qt, t)) {
			return true
		}
	}
	return false
}

// roundScore clamps float noise to 4 decimal places for stable output and
// deterministic comparisons.
func roundScore(f float64) float64 {
	return math.Round(f*10000) / 10000
}
