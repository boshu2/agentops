// practices: [agile-manifesto, dora-metrics]
package main

import (
	"testing"
)

func TestRpi_ParentCommandExists(t *testing.T) {
	if rpiCmd == nil {
		t.Fatal("rpiCmd should not be nil")
	}
	if rpiCmd.Use != "rpi" {
		t.Errorf("rpiCmd.Use = %q, want %q", rpiCmd.Use, "rpi")
	}
	if rpiCmd.Short == "" {
		t.Error("rpiCmd.Short should not be empty")
	}
	if rpiCmd.GroupID != "workflow" {
		t.Errorf("rpiCmd.GroupID = %q, want %q", rpiCmd.GroupID, "workflow")
	}
}

func TestRpi_ParentHasSubcommands(t *testing.T) {
	if !rpiCmd.HasSubCommands() {
		t.Error("rpiCmd should have subcommands")
	}
}

func TestRpi_ParentIsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "rpi" {
			found = true
			break
		}
	}
	if !found {
		t.Error("rpiCmd should be registered on rootCmd")
	}
}
