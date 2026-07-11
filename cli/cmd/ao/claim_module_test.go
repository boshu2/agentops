package main

import (
	"strings"
	"testing"
)

func TestClaimModuleRegisteredWithCorrectedCheckArgs(t *testing.T) {
	out, err := executeCommand("claim", "check", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Report read-only proof cards for changed public claim markers.") {
		t.Fatalf("claim check help = %q", out)
	}
	for _, command := range rootCmd.Commands() {
		if command.Name() == "claim" {
			if command.GroupID != "core" {
				t.Fatalf("claim group = %q", command.GroupID)
			}
			return
		}
	}
	t.Fatal("claim command not registered")
}
