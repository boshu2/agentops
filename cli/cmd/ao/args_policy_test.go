package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestArgsPolicyEveryPublicRunnableDeclaresPolicy(t *testing.T) {
	var missing []string
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, command := range parent.Commands() {
			if command.Hidden {
				continue
			}
			if (command.Run != nil || command.RunE != nil) && command.Args == nil {
				missing = append(missing, command.CommandPath())
			}
			walk(command)
		}
	}
	walk(rootCmd)
	if len(missing) != 0 {
		t.Fatalf("public runnable commands without explicit Args policy (%d):\n%s", len(missing), strings.Join(missing, "\n"))
	}
}

func TestHelpCommandArgsPolicyIsOrderIndependent(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	for _, command := range rootCmd.Commands() {
		if command.Name() != "help" {
			continue
		}
		if command.Args == nil {
			t.Fatal("Cobra help command has nil Args after repeated initialization")
		}
		if err := command.Args(command, []string{"gate", "check"}); err != nil {
			t.Fatalf("Cobra help command rejected multi-segment path: %v", err)
		}
		return
	}
	t.Fatal("Cobra help command is not registered")
}

func TestHelpCommandAcceptsMultiSegmentPath(t *testing.T) {
	out, err := executeCommand("help", "gate", "check")
	if err != nil {
		t.Fatalf("ao help gate check: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ao gate check") {
		t.Fatalf("ao help gate check did not render leaf help:\n%s", out)
	}
}
