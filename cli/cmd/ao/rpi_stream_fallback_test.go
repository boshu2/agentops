package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldFallbackToDirect(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "startup timeout", err: os.ErrDeadlineExceeded, want: false},
		{name: "stream startup timeout", err: errors.New("stream startup timeout: no events"), want: true},
		{name: "stream parse error", err: errors.New("stream parse error: malformed"), want: true},
		{name: "stream stall", err: errors.New("phase 1 (stall): stall detected: no stream activity for 10m0s"), want: true},
		{name: "other error", err: errors.New("claude exited with code 1"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldFallbackToDirect(tc.err)
			if got != tc.want {
				t.Fatalf("shouldFallbackToDirect(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestStreamExecutorExecute_FallsBackToDirectOnStartupTimeout(t *testing.T) {
	binDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "direct-ran.txt")
	t.Setenv("STREAM_FALLBACK_MARKER", marker)

	claudePath := filepath.Join(binDir, "claude")
	script := `#!/bin/sh
case "$*" in
  *"--output-format stream-json"*)
    sleep 10
    ;;
  *)
    echo "direct" >> "$STREAM_FALLBACK_MARKER"
    exit 0
    ;;
esac
`
	if err := os.WriteFile(claudePath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	exec := &streamExecutor{
		statusPath:           filepath.Join(t.TempDir(), "live-status.md"),
		allPhases:            []PhaseProgress{{Name: "discovery", CurrentAction: "starting"}},
		phaseTimeout:         0,
		stallTimeout:         0,
		streamStartupTimeout: 120 * time.Millisecond,
		stallCheckInterval:   40 * time.Millisecond,
	}

	if err := exec.Execute("test prompt", t.TempDir(), "run-stream-fallback", 1); err != nil {
		t.Fatalf("expected fallback to direct execution, got error: %v", err)
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("expected direct fallback marker file, read failed: %v", err)
	}
	if !strings.Contains(string(data), "direct") {
		t.Fatalf("expected direct fallback marker content, got: %q", string(data))
	}
}
