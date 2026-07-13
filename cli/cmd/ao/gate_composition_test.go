package main

import (
	"strings"
	"testing"
)

func TestGateCompositionOwnsCompleteCommandTree(t *testing.T) {
	command := newGateCommand()
	for _, path := range []string{"ao gate approve", "ao gate bulk-approve", "ao gate check", "ao gate pending", "ao gate reject", "ao gate run"} {
		args := strings.Fields(strings.TrimPrefix(path, "ao gate "))
		child, remaining, err := command.Find(args)
		if err != nil || child == command || len(remaining) != 0 {
			t.Fatalf("missing gate child %q: child=%v remaining=%v err=%v", path, child, remaining, err)
		}
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
