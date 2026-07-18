package main

import (
	"strings"
	"testing"
)

// TestCathedralCutVerbsFailWithHint proves each retired lifecycle verb is
// absent from the tree and that invoking it produces an unknown-command error
// the hint layer translates into an actionable replacement pointer.
func TestCathedralCutVerbsFailWithHint(t *testing.T) {
	for name := range removedCommands {
		for _, command := range rootCmd.Commands() {
			if command.Name() == name || command.HasAlias(name) {
				t.Fatalf("%s still resolves to a registered command", name)
			}
		}
		_, err := executeCommand(name)
		if err == nil {
			t.Fatalf("ao %s unexpectedly succeeded", name)
		}
		hint := removedCommandHint(rootCmd, err)
		if !strings.Contains(hint, "was removed from ao") || !strings.Contains(hint, "docs/MIGRATION.md") {
			t.Fatalf("ao %s hint = %q; want removal pointer to docs/MIGRATION.md", name, hint)
		}
	}
}

func TestCathedralCutNestedVerbsFailWithHint(t *testing.T) {
	for parentName, children := range removedChildCommands {
		for name := range children {
			_, err := executeCommand(parentName, name)
			if err == nil {
				t.Fatalf("ao %s %s unexpectedly succeeded", parentName, name)
			}
			hint := removedCommandHint(rootCmd, err)
			if !strings.Contains(hint, "was removed from ao") || !strings.Contains(hint, name) {
				t.Fatalf("ao %s %s hint = %q; want removal pointer", parentName, name, hint)
			}
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
