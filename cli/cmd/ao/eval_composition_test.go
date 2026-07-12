package main

import "testing"

func TestEvalCompositionOwnsCompleteCommandTree(t *testing.T) {
	command := newEvalCommand()
	want := []string{"baseline", "baseline-audit", "bench", "chaos", "cleanup", "compare", "coverage", "outcomes", "run", "scenario", "scenario-ab", "scenario-moat", "scorecard", "session-outcome", "suite", "task"}
	for _, name := range want {
		child, _, err := command.Find([]string{name})
		if err != nil || child == command || child.Name() != name {
			t.Fatalf("missing eval child %q: child=%v err=%v", name, child, err)
		}
	}
}
