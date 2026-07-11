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

func TestClaimBindCommandDelegatesThroughModule(t *testing.T) {
	old := testProjectDir
	testProjectDir = t.TempDir()
	t.Cleanup(func() { testProjectDir = old })
	out, err := executeCommand("claim", "bind", "--claim", "AOP-X", "--path", "p.md", "--level", "PG2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `bound claim="AOP-X" path="p.md" level=PG2`) {
		t.Fatalf("claim bind output = %q", out)
	}
}
