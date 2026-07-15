package main

import "testing"

// TestExperimentalHelpGroup asserts the corpus/flywheel demotion contract
// (age-h4y3): the experimental cobra group is registered on rootCmd and every
// demoted corpus/flywheel command present in this (default/spine) build variant
// carries it. age-nzwo archived `corpus` and `curate` behind the flywheel tag,
// so they moved out of this default list into the flywheel-tagged variant in
// root_group_experimental_flywheel_test.go (which asserts the full 12).
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

	// corpus + curate + defrag are archived behind //go:build flywheel (age-nzwo)
	// so they are absent from the spine build; the flywheel-tagged sibling test
	// asserts them.
	demoted := []string{"flywheel", "store", "wiki"}
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
