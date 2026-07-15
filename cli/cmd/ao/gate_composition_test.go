package main

import (
	"strings"
	"testing"
)

func TestGateCompositionOwnsCompleteCommandTree(t *testing.T) {
	command := newGateCommand()
	for _, path := range []string{"ao gate check"} {
		args := strings.Fields(strings.TrimPrefix(path, "ao gate "))
		child, remaining, err := command.Find(args)
		if err != nil || child == command || len(remaining) != 0 {
			t.Fatalf("missing gate child %q: child=%v remaining=%v err=%v", path, child, remaining, err)
		}
	}
	if len(command.Commands()) != 1 {
		t.Fatalf("gate exposes %d children, want only check", len(command.Commands()))
	}
}

func TestGateCompositionRegistersExactlyOneRootOwner(t *testing.T) {
	owners := 0
	for _, command := range rootCmd.Commands() {
		if command.Name() == "gate" {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("gate root owners = %d, want 1", owners)
	}
}
