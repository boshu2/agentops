package scenarios

import "strings"

// HasScenariosBlock reports whether text already carries an authored Gherkin
// scenarios section — a markdown heading line whose first word is "Scenarios"
// (case-insensitive), e.g. "## Scenarios". It is the idempotency guard for the
// extractor: a bead that already has a "## Scenarios" block should not be
// re-extracted over without an explicit override.
//
// A singular "## Scenario:" heading or a bare "Scenario:" line does not match;
// only the plural section heading that opens a scenarios block does.
func HasScenariosBlock(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(t, "#"))
		word := heading
		if i := strings.IndexAny(heading, " \t"); i >= 0 {
			word = heading[:i]
		}
		if strings.EqualFold(word, "Scenarios") {
			return true
		}
	}
	return false
}
