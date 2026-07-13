package claim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	claimapp "github.com/boshu2/agentops/cli/internal/claim"
)

type exitMarker struct {
	code    int
	message string
}

func (marker *exitMarker) Error() string { return marker.message }

func captureExit(target **exitMarker) ExitErrorFactory {
	return func(code int, message string) error {
		*target = &exitMarker{code: code, message: message}
		return *target
	}
}

func TestTrackerPreservesMergedOutputArgsAndChildExit(t *testing.T) {
	root := t.TempDir()
	script := writeTrackerScript(t, root, "br", "printf 'args:%s\\n' \"$*\"\nprintf 'child-stderr\\n' >&2\nexit 7\n")
	environment := []string{"AGENTOPS_TRACKER=br", "PATH=" + root}
	var captured *exitMarker
	tracker := NewTrackerWith(
		func() (string, error) { return root, nil },
		func() []string { return environment },
		lookPathFor("br", script),
		captureExit(&captured),
	)
	var output bytes.Buffer
	err := tracker.Claim(context.Background(), "age-1", claimapp.Streams{Stdout: &output})
	if !errors.Is(err, captured) || captured.code != 7 || captured.message != "exit status 7" {
		t.Fatalf("error=%v captured=%+v", err, captured)
	}
	if got := output.String(); !strings.Contains(got, "args:update age-1 --claim") || !strings.Contains(got, "child-stderr") {
		t.Fatalf("merged child output = %q", got)
	}
}

func TestTrackerPreservesBDCallerDirectoryAndForeignBeadsDir(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := writeTrackerScript(t, root, "bd", "pwd\nprintf 'beads=%s\\n' \"${BEADS_DIR-unset}\"\nprintf 'args:%s\\n' \"$*\"\n")
	environment := []string{"AGENTOPS_TRACKER=bd", "BEADS_DIR=/foreign", "PATH=" + root}
	tracker := NewTrackerWith(
		func() (string, error) { return subdir, nil },
		func() []string { return environment },
		lookPathFor("bd", script),
		func(code int, message string) error { return fmt.Errorf("exit %d: %s", code, message) },
	)
	var output bytes.Buffer
	if err := tracker.Claim(context.Background(), "age-2", claimapp.Streams{Stdout: &output}); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, subdir) || !strings.Contains(got, "beads=/foreign") || !strings.Contains(got, "args:update age-2 --claim") {
		t.Fatalf("BD execution context = %q", got)
	}
}

func TestTrackerResolutionFailureMapsTo127(t *testing.T) {
	root := t.TempDir()
	var captured *exitMarker
	tracker := NewTrackerWith(
		func() (string, error) { return root, nil },
		func() []string { return []string{"PATH=/missing"} },
		func(string) (string, error) { return "", errors.New("not found") },
		captureExit(&captured),
	)
	err := tracker.Claim(context.Background(), "age-x", claimapp.Streams{})
	if !errors.Is(err, captured) || captured.code != 127 || !strings.Contains(captured.message, "cannot resolve a beads tracker") {
		t.Fatalf("resolution error=%v captured=%+v", err, captured)
	}
}

func writeTrackerScript(t *testing.T, directory, name, body string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func lookPathFor(name, path string) func(string) (string, error) {
	return func(candidate string) (string, error) {
		if candidate == name {
			return path, nil
		}
		return "", errors.New("not found")
	}
}
