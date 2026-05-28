// practices: [bdd-gherkin, tdd]
package main

import (
	"os"
	"strings"
	"testing"
)

func TestGoalsScenariosUsesCanonicalPracticeSlug(t *testing.T) {
	body, err := os.ReadFile("goals_scenarios.go")
	if err != nil {
		t.Fatalf("read goals_scenarios.go: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "practices: [bdd-gherkin, llm-eval-harness]") {
		t.Fatalf("goals_scenarios.go must cite canonical bdd-gherkin practice slug")
	}
	if strings.Contains(text, "practices: [bdd,") {
		t.Fatalf("goals_scenarios.go cites non-canonical bdd practice slug")
	}
}
