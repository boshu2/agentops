package main

import "testing"

// TestExperimentalHelpGroup asserts the corpus/flywheel demotion contract
// (age-h4y3): the experimental cobra group is registered on rootCmd and every
// demoted corpus/flywheel command present in this build variant carries it.
// When age-nzwo archives these commands behind the flywheel tag, shrink or
// tag-split the demoted list alongside that change.
func TestExperimentalHelpGroup(t *testing.T) {
	var title string
	for _, g := range rootCmd.Groups() {
		if g.ID == "experimental" {
			title = g.Title
		}
	}
	if title == "" {
		t.Fatal("experimental cobra group not registered on rootCmd")
	}
	if title != "Experimental (corpus/flywheel):" {
		t.Fatalf("experimental group title = %q, want %q", title, "Experimental (corpus/flywheel):")
	}

	demoted := []string{
		"compile", "corpus", "curate", "dedup", "defrag", "flywheel",
		"maturity", "pool", "ratchet", "store", "temper", "wiki",
	}
	for _, name := range demoted {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Errorf("demoted command %q not registered in this build variant", name)
			continue
		}
		if cmd.GroupID != "experimental" {
			t.Errorf("command %q GroupID = %q, want %q", name, cmd.GroupID, "experimental")
		}
	}
}
