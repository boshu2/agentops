package main

import (
	"strings"
	"testing"
)

// TestRootHelpStatesConditionalReview pins the review rule `ao --help` teaches:
// checks and CI gate an ordinary change, and a fresh read is conditional
// (AGENTS.md; ADR-0017 conditional-review amendment). The retired every-change
// judgment sentence must not come back.
func TestRootHelpStatesConditionalReview(t *testing.T) {
	help := strings.Join(strings.Fields(rootCmd.Long), " ")
	const want = "Checks and CI gate an ordinary change; one fresh author-distinct read is owed only when the caller asks, a mistake cannot be cheaply undone, or no deterministic check covers the change."
	if !strings.Contains(help, want) {
		t.Errorf("root help missing conditional-review sentence %q\ngot: %s", want, help)
	}
	for _, retired := range []string{"exact final change is required", "Fresh independent judgment"} {
		if strings.Contains(help, retired) {
			t.Errorf("root help still states the retired every-change review rule %q", retired)
		}
	}
}
