// practices: [ai-assisted-dev, llm-eval-harness]
package main

import "testing"

// TestEvalFold_SubcommandsRegistered asserts the eval-family top-level commands
// folded under `ao eval` (age-focus-membrane-bookkeeper-m1wg.16) resolve at
// their canonical `ao eval …` paths, that the `scenario-ab` ADR-0004 revival
// path is preserved as a distinct sibling, and that `session-outcome` does not
// collide with the pre-existing `outcomes` rubric command.
func TestEvalFold_SubcommandsRegistered(t *testing.T) {
	cases := []struct {
		path []string
		leaf string
	}{
		{[]string{"eval", "bench"}, "bench"},
		{[]string{"eval", "chaos"}, "chaos"},
		{[]string{"eval", "session-outcome"}, "session-outcome"},
		{[]string{"eval", "scenario"}, "scenario"},
		// Preserved siblings — must not be shadowed by the fold.
		{[]string{"eval", "scenario-ab"}, "scenario-ab"},
		{[]string{"eval", "outcomes"}, "outcomes"},
		// Reparented children of scenario auto-follow.
		{[]string{"eval", "scenario", "add"}, "add"},
		{[]string{"eval", "scenario", "validate"}, "validate"},
	}
	for _, tc := range cases {
		cmd, rest, err := rootCmd.Find(tc.path)
		if err != nil {
			t.Errorf("%v: not registered: %v", tc.path, err)
			continue
		}
		if len(rest) != 0 {
			t.Errorf("%v: unresolved trailing args %v (path did not fully resolve)", tc.path, rest)
		}
		if cmd.Name() != tc.leaf {
			t.Errorf("%v: found %q, want %q", tc.path, cmd.Name(), tc.leaf)
		}
	}
}

// TestEvalFold_BenchAndSessionOutcomeFlagsBound verifies the eval aliases carry
// the same flag set as their back-compat top-level twins (they bind the same
// package-global vars via the shared bind helpers).
func TestEvalFold_BenchAndSessionOutcomeFlagsBound(t *testing.T) {
	bench, _, err := rootCmd.Find([]string{"eval", "bench"})
	if err != nil {
		t.Fatalf("eval bench not registered: %v", err)
	}
	for _, f := range []string{"live", "corpus", "json", "k"} {
		if bench.Flags().Lookup(f) == nil {
			t.Errorf("eval bench missing --%s flag", f)
		}
	}

	so, _, err := rootCmd.Find([]string{"eval", "session-outcome"})
	if err != nil {
		t.Fatalf("eval session-outcome not registered: %v", err)
	}
	for _, f := range []string{"session", "output"} {
		if so.Flags().Lookup(f) == nil {
			t.Errorf("eval session-outcome missing --%s flag", f)
		}
	}
}

// TestEvalFold_OldSpellingsStillResolveHidden asserts the pre-fold top-level
// spellings remain reachable (hidden) for back-compat, while `scenario` is fully
// reparented off the root.
func TestEvalFold_OldSpellingsStillResolveHidden(t *testing.T) {
	for _, name := range []string{"retrieval-bench", "chaos-test", "session-outcome"} {
		cmd, rest, err := rootCmd.Find([]string{name})
		if err != nil || len(rest) != 0 || cmd.Name() != name {
			t.Errorf("back-compat top-level %q should still resolve (found %q, rest %v, err %v)", name, cmd.Name(), rest, err)
			continue
		}
		if !cmd.Hidden {
			t.Errorf("back-compat top-level %q should be Hidden", name)
		}
	}

	// scenario is reparented under eval — it must NOT remain a top-level command.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "scenario" {
			t.Errorf("scenario should be reparented under eval, still found at top level")
		}
	}
}
