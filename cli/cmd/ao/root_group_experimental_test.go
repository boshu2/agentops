package main

import "testing"

// TestNoExperimentalKnowledgeGroup pins the retirement of the knowledge-
// flywheel command family: the "Optional knowledge tools:" help group and the
// flywheel command tree must stay gone from the root command.
func TestNoExperimentalKnowledgeGroup(t *testing.T) {
	for _, g := range rootCmd.Groups() {
		if g.ID == "experimental" {
			t.Fatalf("experimental cobra group %q is registered; the knowledge-flywheel surface was retired", g.Title)
		}
	}
	if cmd, _, err := rootCmd.Find([]string{"flywheel"}); err == nil && cmd != nil && cmd.Name() == "flywheel" {
		t.Fatal("flywheel command is registered; the knowledge-flywheel family was retired")
	}
}
