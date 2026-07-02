//go:build flywheel

package main

import "testing"

// TestExperimentalHelpGroupFlywheel is the flywheel-tagged half of the
// experimental-group demotion contract (age-h4y3 / age-nzwo). The spine test
// (root_group_experimental_test.go) asserts the 9 corpus/flywheel commands
// still compiled into the default build; this test asserts the 3 archived
// behind //go:build flywheel — `corpus`, `curate`, and `defrag` — are
// registered and carry the experimental GroupID once their tag is active.
// Together the two halves cover the full 12-command demotion set.
func TestExperimentalHelpGroupFlywheel(t *testing.T) {
	var title string
	for _, g := range rootCmd.Groups() {
		if g.ID == "experimental" {
			title = g.Title
		}
	}
	if title != "Experimental (corpus/flywheel):" {
		t.Fatalf("experimental group title = %q, want %q", title, "Experimental (corpus/flywheel):")
	}

	archived := []string{"corpus", "curate", "defrag"}
	for _, name := range archived {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Errorf("flywheel-archived command %q not registered under the flywheel tag", name)
			continue
		}
		if cmd.GroupID != "experimental" {
			t.Errorf("command %q GroupID = %q, want %q", name, cmd.GroupID, "experimental")
		}
	}
}
