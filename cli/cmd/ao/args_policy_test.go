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
