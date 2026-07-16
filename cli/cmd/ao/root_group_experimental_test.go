package main

import "testing"

// TestExperimentalHelpGroup asserts that optional knowledge commands do not
// become lifecycle authorities merely because they are available.
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
	if title != "Optional knowledge tools:" {
		t.Fatalf("experimental group title = %q, want %q", title, "Optional knowledge tools:")
	}

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
