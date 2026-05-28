// Package scenarios converts a bead's free-text acceptance criteria into
// structured Gherkin Given/When/Then scenarios.
//
// This package is pure domain logic: it performs no I/O and shells out to
// nothing. Extract applies deterministic rules only — each acceptance line
// that contains the keywords given, when, and then (in that order) becomes a
// single Scenario. Prose that cannot be structured this way is reported as an
// error rather than guessed at; an LLM-assisted fallback is intentionally out
// of scope for this slice and lives in the parent feature (ag-dwq).
package scenarios

import (
	"fmt"
	"strings"
)

// Scenario is one Given/When/Then behavior triple extracted from acceptance
// text. It is the candidate form an operator reviews before it is written
// back to a bead.
type Scenario struct {
	Name  string `json:"name"`
	Given string `json:"given"`
	When  string `json:"when"`
	Then  string `json:"then"`
}

// Extract converts free-text acceptance criteria into structured scenarios
// using deterministic rules. Each non-empty line that contains given, when,
// and then (case-insensitive, in order, as whole words) becomes one Scenario;
// lines that do not match are skipped. Extract returns an error when no line
// can be structured — that signals the acceptance text must be authored
// manually.
func Extract(acceptance string) ([]Scenario, error) {
	var out []Scenario
	for _, line := range strings.Split(acceptance, "\n") {
		if s, ok := parseLine(stripBullet(line)); ok {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no acceptance line could be structured into Given/When/Then")
	}
	return out, nil
}

// Render formats scenarios as a Gherkin "## Scenarios" markdown block matching
// the repository's acceptance convention (two-space indent under each
// Scenario heading).
func Render(scenarios []Scenario) string {
	var b strings.Builder
	b.WriteString("## Scenarios\n")
	for _, s := range scenarios {
		b.WriteString("\nScenario: ")
		b.WriteString(s.Name)
		b.WriteString("\n  Given ")
		b.WriteString(s.Given)
		b.WriteString("\n  When ")
		b.WriteString(s.When)
		b.WriteString("\n  Then ")
		b.WriteString(s.Then)
		b.WriteString("\n")
	}
	return b.String()
}

// stripBullet removes a leading list marker ("-", "*", "•", or "1.", "2)")
// and surrounding whitespace from a single line.
func stripBullet(line string) string {
	s := strings.TrimSpace(line)
	for _, m := range []string{"- ", "* ", "• "} {
		if strings.HasPrefix(s, m) {
			return strings.TrimSpace(s[len(m):])
		}
	}
	// numbered markers: leading digits followed by '.' or ')'
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// parseLine splits a single acceptance line into a Scenario by locating the
// given/when/then keywords as whole words in order. It returns ok=false when
// any keyword is missing or any resulting clause is empty.
func parseLine(line string) (Scenario, bool) {
	lower := strings.ToLower(line)
	gi := findKeyword(lower, "given", 0)
	if gi < 0 {
		return Scenario{}, false
	}
	wi := findKeyword(lower, "when", gi+len("given"))
	if wi < 0 {
		return Scenario{}, false
	}
	ti := findKeyword(lower, "then", wi+len("when"))
	if ti < 0 {
		return Scenario{}, false
	}
	given := cleanClause(line[gi+len("given") : wi])
	when := cleanClause(line[wi+len("when") : ti])
	then := cleanClause(line[ti+len("then"):])
	// Each clause must carry an actual word. This rejects prose that merely
	// mentions "Given/When/Then" as a slash-separated phrase, where the
	// between-keyword clauses collapse to punctuation like "/".
	if !hasLetter(given) || !hasLetter(when) || !hasLetter(then) {
		return Scenario{}, false
	}
	return Scenario{Name: then, Given: given, When: when, Then: then}, true
}

func hasLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if isLetter(s[i]) {
			return true
		}
	}
	return false
}

// findKeyword returns the index of kw in lower at or after from, matching only
// on whole-word boundaries so that "forgiven" does not match "given" and
// "whenever" does not match "when".
func findKeyword(lower, kw string, from int) int {
	for i := from; i+len(kw) <= len(lower); i++ {
		if lower[i:i+len(kw)] != kw {
			continue
		}
		if i > 0 && isLetter(lower[i-1]) {
			continue
		}
		if after := i + len(kw); after < len(lower) && isLetter(lower[after]) {
			continue
		}
		return i
	}
	return -1
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// cleanClause trims whitespace and clause-joining punctuation (commas,
// semicolons, colons) and a single trailing period from an extracted clause.
func cleanClause(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ",;:")
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ".")
	return strings.TrimSpace(s)
}
