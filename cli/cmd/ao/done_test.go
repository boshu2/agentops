package main

import (
	"strings"
	"testing"
)

func TestDoneCommandCompositionPreservesSurface(t *testing.T) {
	if doneCommand.Use != "done <bead-id>" || doneCommand.GroupID != "" {
		t.Fatalf("done command = use %q group %q", doneCommand.Use, doneCommand.GroupID)
	}
	for _, flag := range []string{"sha", "reason", "force-no-verdict", "json"} {
		if doneCommand.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag %q", flag)
		}
	}
	command, _, err := rootCmd.Find([]string{"done"})
	if err != nil || command != doneCommand {
		t.Fatalf("root registration = %p want %p err=%v", command, doneCommand, err)
	}
}

func TestDoneCommandCompositionRejectsInvalidSHAWithoutEffects(t *testing.T) {
	command := doneModule.Command()
	command.SetArgs([]string{"age-test", "--sha", "bad"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "is not a commit sha") {
		t.Fatalf("error = %v", err)
	}
}
