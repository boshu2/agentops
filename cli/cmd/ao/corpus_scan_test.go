//go:build flywheel

// practices: [fail-closed-safety, test-pyramid]
package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRunCorpusScan_LeakFailsClosed is the L2 over the command entry point: a
// fixture with a fleet marker must return a corpusScanExitError with exit code
// 1 (FAIL CLOSED).
func TestRunCorpusScan_LeakFailsClosed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "leak.md")
	if err := os.WriteFile(p, []byte("Deploy to bushido over tailscale.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	corpusScanJSON = false
	err := runCorpusScan(corpusScanCmd, []string{p})
	if err == nil {
		t.Fatal("expected a fail-closed error for a leak, got nil")
	}
	var scanErr *corpusScanExitError
	if !errors.As(err, &scanErr) {
		t.Fatalf("expected *corpusScanExitError, got %T: %v", err, err)
	}
	if scanErr.ExitCode() != corpusScanLeak {
		t.Fatalf("expected exit code %d (leak), got %d", corpusScanLeak, scanErr.ExitCode())
	}
}

// TestRunCorpusScan_CleanPasses asserts a clean generic learning returns nil
// (exit 0).
func TestRunCorpusScan_CleanPasses(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "clean.md")
	if err := os.WriteFile(p, []byte("# Lesson\n\nGive agents durable context; require evidence.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	corpusScanJSON = false
	if err := runCorpusScan(corpusScanCmd, []string{p}); err != nil {
		t.Fatalf("expected clean (nil error), got %v", err)
	}
}

// TestRunCorpusScan_MissingPathInternal asserts a missing path maps to the
// internal exit code (2), distinct from a leak (1).
func TestRunCorpusScan_MissingPathInternal(t *testing.T) {
	corpusScanJSON = false
	err := runCorpusScan(corpusScanCmd, []string{filepath.Join(t.TempDir(), "nope")})
	if err == nil {
		t.Fatal("expected an internal error for a missing path")
	}
	var scanErr *corpusScanExitError
	if !errors.As(err, &scanErr) {
		t.Fatalf("expected *corpusScanExitError, got %T", err)
	}
	if scanErr.ExitCode() != corpusScanInternal {
		t.Fatalf("expected exit code %d (internal), got %d", corpusScanInternal, scanErr.ExitCode())
	}
}
