package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestClaimModuleInjectsCanonicalTrackerLookup(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "br")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'args:%s\\n' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := trackerLookPath
	trackerLookPath = func(name string) (string, error) {
		if name == "br" {
			return script, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { trackerLookPath = original })
	t.Setenv("AGENTOPS_TRACKER", "br")
	t.Setenv("PATH", "")
	t.Setenv("BEADS_DIR", filepath.Join(root, "_beads"))

	out, err := executeCommand("claim", "age-module")
	if err != nil {
		var exit interface{ ExitCode() int }
		if errors.As(err, &exit) {
			t.Fatalf("claim module tracker lookup exited %d: %v", exit.ExitCode(), err)
		}
		t.Fatal(err)
	}
	if !strings.Contains(out, "args:update age-module --claim") {
		t.Fatalf("claim module output = %q", out)
	}
}
