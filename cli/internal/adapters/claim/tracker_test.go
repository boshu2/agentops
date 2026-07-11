package claim

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	claimapp "github.com/boshu2/agentops/cli/internal/claim"
)

type fixedResolver struct{ resolution beadsapp.TrackerResolution }

func (resolver fixedResolver) Resolve() (beadsapp.TrackerResolution, error) {
	return resolver.resolution, nil
}

type exitMarker struct {
	code    int
	message string
}

func (marker *exitMarker) Error() string { return marker.message }

func TestTrackerPreservesMergedOutputArgsAndChildExit(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "br")
	content := "#!/bin/sh\nprintf 'args:%s\\n' \"$*\"\nprintf 'child-stderr\\n' >&2\nexit 7\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	var captured *exitMarker
	tracker := NewTracker(fixedResolver{beadsapp.TrackerResolution{
		Tracker: beadsapp.TrackerBR, Binary: script, WorkDir: root, ChildEnv: os.Environ(),
	}}, func(code int, message string) error {
		captured = &exitMarker{code: code, message: message}
		return captured
	})
	var output bytes.Buffer
	err := tracker.Claim(context.Background(), "age-1", claimapp.Streams{Stdout: &output})
	if !errors.Is(err, captured) || captured.code != 7 || captured.message != "exit status 7" {
		t.Fatalf("error=%v captured=%+v", err, captured)
	}
	if got := output.String(); !strings.Contains(got, "args:update age-1 --claim") || !strings.Contains(got, "child-stderr") {
		t.Fatalf("merged child output = %q", got)
	}
}
