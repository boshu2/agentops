package beads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrackerPreservesBDLedgerSelectionAndInvocation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bd")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tracker := NewTrackerWith(
		func() (string, error) { return root, nil },
		func() []string { return []string{"PATH=" + root, "BEADS_DIR=/foreign/br"} },
		func(name string) (string, error) {
			if name == "bd" {
				return bin, nil
			}
			return "", errors.New("not found")
		},
	)
	resolved, err := tracker.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Tracker != "bd" || resolved.LedgerDir != filepath.Join(root, ".beads") {
		t.Fatalf("resolution = %+v", resolved)
	}
	for _, entry := range resolved.ChildEnv {
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			t.Fatalf("bd child inherited foreign BEADS_DIR: %v", resolved.ChildEnv)
		}
	}
	output, err := tracker.Output(context.Background(), "ready", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "ready --json\n" {
		t.Fatalf("output = %q", output)
	}
}

func TestTrackerFailsClosedWhenWorkingDirectoryFails(t *testing.T) {
	tracker := NewTrackerWith(
		func() (string, error) { return "", errors.New("cwd unavailable") },
		func() []string { return nil },
		func(string) (string, error) { return "", nil },
	)
	if _, err := tracker.Resolve(); err == nil || !strings.Contains(err.Error(), "cwd unavailable") {
		t.Fatalf("Resolve() error = %v", err)
	}
}
