package scenarios

import (
	"fmt"
	"strings"
)

// Validate reports whether text carries a well-formed "## Scenarios" Gherkin
// block. It returns nil when every scenario parses cleanly, or an error naming
// the first well-formedness problem (no block, no scenario, a scenario missing
// its name or a Given/When/Then step, steps out of order, or an empty step
// body). It is the inverse acceptance gate of Extract/Render: any block Render
// produces passes Validate.
func Validate(text string) error {
	_, err := ParseBlock(text)
	return err
}

// gherkinStep is one parsed step line: a canonical lowercase keyword
// (given/when/then/and/but) and its body text.
type gherkinStep struct {
	kw   string
	body string
}

// rawScenario is a scenario's heading name and its collected step lines before
// well-formedness has been checked.
type rawScenario struct {
	name  string
	steps []gherkinStep
}

// ParseBlock parses the authored "## Scenarios" block embedded in text — the
// multi-line Gherkin form that Render produces — into structured scenarios. It
// returns an error naming the first well-formedness problem. Each scenario must
// declare a name and contain a Given, a When, and a Then step in that order,
// each with a non-empty body; And/But lines are accepted as continuations of
// the preceding primary step.
func ParseBlock(text string) ([]Scenario, error) {
	block, ok := scenariosBlock(text)
	if !ok {
		return nil, fmt.Errorf("no '## Scenarios' block found")
	}
	raws := groupScenarios(block)
	if len(raws) == 0 {
		return nil, fmt.Errorf("'## Scenarios' block contains no 'Scenario:' entries")
	}
	out := make([]Scenario, 0, len(raws))
	for i, r := range raws {
		s, err := r.assemble(i + 1)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// groupScenarios splits block lines into scenarios keyed by their "Scenario:"
// headings. Blank lines, comments, and step-shaped lines before the first
// heading are ignored — they belong to no scenario.
func groupScenarios(block []string) []rawScenario {
	var raws []rawScenario
	curIdx := -1
	for _, line := range block {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if name, ok := scenarioHeading(t); ok {
			raws = append(raws, rawScenario{name: name})
			curIdx = len(raws) - 1
			continue
		}
		if curIdx < 0 {
			continue
		}
		if kw, body, ok := stepLine(t); ok {
			raws[curIdx].steps = append(raws[curIdx].steps, gherkinStep{kw: kw, body: body})
		}
	}
	return raws
}

// assemble validates one raw scenario and returns its structured form. n is the
// 1-based position used to identify an unnamed scenario in errors.
func (r rawScenario) assemble(n int) (Scenario, error) {
	if r.name == "" {
		return Scenario{}, fmt.Errorf("scenario #%d: missing name after 'Scenario:'", n)
	}
	var prevPrimary string
	firstGiven, firstWhen, firstThen := -1, -1, -1
	var givenBody, whenBody, thenBody string
	for si, st := range r.steps {
		phase := st.kw
		if phase == "and" || phase == "but" {
			if prevPrimary == "" {
				return Scenario{}, fmt.Errorf("scenario %q: '%s' step before any Given/When/Then", r.name, st.kw)
			}
			phase = prevPrimary
		} else {
			prevPrimary = phase
		}
		if st.body == "" {
			return Scenario{}, fmt.Errorf("scenario %q: empty %s step body", r.name, title(st.kw))
		}
		switch phase {
		case "given":
			if firstGiven < 0 {
				firstGiven, givenBody = si, st.body
			}
		case "when":
			if firstWhen < 0 {
				firstWhen, whenBody = si, st.body
			}
		case "then":
			if firstThen < 0 {
				firstThen, thenBody = si, st.body
			}
		}
	}
	if err := checkStepOrder(r.name, firstGiven, firstWhen, firstThen); err != nil {
		return Scenario{}, err
	}
	return Scenario{Name: r.name, Given: givenBody, When: whenBody, Then: thenBody}, nil
}

// checkStepOrder verifies a scenario has a Given, a When, and a Then step, in
// that order. The arguments are the first-occurrence indices of each step type,
// or -1 when absent.
func checkStepOrder(name string, given, when, then int) error {
	if given < 0 {
		return fmt.Errorf("scenario %q: missing Given step", name)
	}
	if when < 0 {
		return fmt.Errorf("scenario %q: missing When step", name)
	}
	if then < 0 {
		return fmt.Errorf("scenario %q: missing Then step", name)
	}
	if given >= when || when >= then {
		return fmt.Errorf("scenario %q: steps out of order (expected Given before When before Then)", name)
	}
	return nil
}

// scenariosBlock returns the lines of the "## Scenarios" block — those after
// the heading whose first word is "Scenarios" (case-insensitive), up to the
// next markdown heading or end of text. ok is false when no such heading
// exists.
func scenariosBlock(text string) ([]string, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(t, "#"))
		word := heading
		if j := strings.IndexAny(heading, " \t"); j >= 0 {
			word = heading[:j]
		}
		if strings.EqualFold(word, "Scenarios") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	var block []string
	for _, line := range lines[start+1:] {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			break
		}
		block = append(block, line)
	}
	return block, true
}

// scenarioHeading reports whether a trimmed line opens a scenario and returns
// its (possibly empty) name. It matches "Scenario:" and "Scenario Outline:"
// case-insensitively.
func scenarioHeading(t string) (string, bool) {
	lower := strings.ToLower(t)
	for _, prefix := range []string{"scenario outline:", "scenario:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(t[len(prefix):]), true
		}
	}
	return "", false
}

// stepLine reports whether a trimmed line is a Gherkin step and returns its
// canonical lowercase keyword and body. The keyword must be a whole word
// (Given/When/Then/And/But, case-insensitive) followed by whitespace or end of
// line; the body is the remainder, trimmed. A keyword with no body returns an
// empty body so the caller can flag it.
func stepLine(t string) (kw, body string, ok bool) {
	lower := strings.ToLower(t)
	for _, k := range []string{"given", "when", "then", "and", "but"} {
		if lower == k {
			return k, "", true
		}
		if strings.HasPrefix(lower, k) {
			rest := t[len(k):]
			if rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
				return k, strings.TrimSpace(rest), true
			}
		}
	}
	return "", "", false
}

// title upper-cases the first letter of a step keyword for human-facing error
// messages ("when" -> "When").
func title(kw string) string {
	if kw == "" {
		return kw
	}
	return strings.ToUpper(kw[:1]) + kw[1:]
}
