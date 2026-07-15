package main

import (
	"strings"
	"testing"
)

func TestCathedralCutTombstonesAreInert(t *testing.T) {
	withProductionSpine := func(t *testing.T) {
		removed := pruneToDefaultSpine(rootCmd)
		t.Cleanup(func() { restorePrunedCommands(rootCmd, removed) })
	}
	withProductionSpine(t)
	for name := range cathedralCutCommands {
		command, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("%s tombstone missing: %v", name, err)
		}
		if command.RunE == nil {
			t.Fatalf("%s tombstone has no failing handler", name)
		}
		var output strings.Builder
		command.SetErr(&output)
		t.Cleanup(func() { command.SetErr(nil) })
		err = command.RunE(command, nil)
		if err == nil || !strings.Contains(output.String(), "no longer exists") {
			t.Fatalf("%s tombstone result=%v output=%q", name, err, output.String())
		}
	}
}

func TestQuickStartNamesOnlySurvivingResponsibilities(t *testing.T) {
	text := quickstartCmd.Long
	for _, forbidden := range []string{"ao land", "ao verify", "ao beads", "ao pawl"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("quick-start advertises removed responsibility %q", forbidden)
		}
	}
}
