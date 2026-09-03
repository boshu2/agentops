// Package skills scores the skills/ catalog against a free-text intent so the
// CLI can answer "which skill handles X?" without relying on oral tradition.
//
// The scoring engine (Score) is pure: it takes an already-loaded slice of
// SkillMeta and never touches the filesystem, which keeps it table-testable.
// Loading SKILL.md frontmatter from disk lives in load.go.
package skills

import (
	"math"
	"regexp"
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
		// The exclusion sentence names a sibling's job; it is not part of
		// this skill's haystack. It earns nothing and it costs nothing: a
		// penalty was tried and suppressed skills the caller named outright.
		positive, _ := splitExclusion(m.Description)
		descToks := tokenize(positive)

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
		// A declared trigger phrase quoted whole in the query is the caller
		// using the skill's own words; it outranks a sibling that merely owns
		// one of those words as a name token ("check this change" belongs to
		// validate, not to reality-check).
		for _, phrase := range triggerPhrases(m) {
			if len(phrase) >= 2 && containsPhrase(qTokens, phrase) {
				raw += weightName
			}
		}

		score := 0.0
		if len(qTokens) > 0 {
			score = raw / (float64(len(qTokens)) * weightName)
			if score > 1 {
				score = 1
			}
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

// exclusionMarker opens a description's negative-routing sentence: "Not for
// <sibling's job>; that is <sibling>." The sentence names the neighbouring
// skill's vocabulary on purpose, so it is held out of the haystack: without
// that, premortem ranked first for "is this live decision reversible" because
// its own description said it was not for reversibility.
const exclusionMarker = "Not for "

// splitExclusion separates a description into the text that describes the
// skill's own job and its exclusion sentence, if one is present. The sentence
// runs from the marker to the first period followed by whitespace or the end
// of the text; everything after it (typically the Triggers clause) stays
// positive.
func splitExclusion(desc string) (positive, exclusion string) {
	start := strings.Index(desc, exclusionMarker)
	if start < 0 {
		return desc, ""
	}
	rest := desc[start:]
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		if rest[i] == '.' && (i+1 == len(rest) || rest[i+1] == ' ' || rest[i+1] == '\n') {
			end = i + 1
			break
		}
	}
	positive = strings.TrimSpace(desc[:start])
	if tail := strings.TrimSpace(rest[end:]); tail != "" {
		positive += " " + tail
	}
	return positive, rest[:end]
}

// triggerClause opens the description's declared trigger list.
var triggerClause = regexp.MustCompile(`(?i)\btriggers?:`)

// quotedPhrase matches one quoted trigger in that list.
var quotedPhrase = regexp.MustCompile(`"([^"]+)"`)

// triggerPhrases returns every declared trigger as a token sequence: the
// frontmatter triggers list plus each quoted phrase after "Triggers:" in the
// description. Phrases that tokenize to nothing are dropped.
func triggerPhrases(m SkillMeta) [][]string {
	raw := append([]string(nil), m.Triggers...)
	if loc := triggerClause.FindStringIndex(m.Description); loc != nil {
		for _, q := range quotedPhrase.FindAllStringSubmatch(m.Description[loc[1]:], -1) {
			raw = append(raw, q[1])
		}
	}
	out := make([][]string, 0, len(raw))
	for _, r := range raw {
		if toks := tokenize(r); len(toks) > 0 {
			out = append(out, toks)
		}
	}
	return out
}

// containsPhrase reports whether phrase occurs as a contiguous run inside
// qTokens, token by token under tokenMatches, so "check this change" still
// matches "checking this change".
func containsPhrase(qTokens, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(qTokens) {
		return false
	}
	for start := 0; start+len(phrase) <= len(qTokens); start++ {
		hit := true
		for i, pt := range phrase {
			if !tokenMatches(qTokens[start+i], []string{pt}) {
				hit = false
				break
			}
		}
		if hit {
			return true
		}
	}
	return false
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
