package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writePawlTestRepo builds a minimal repo root resolveAgentsRepoRoot() accepts
// (docs/contracts/agents-write-surfaces.md + skills/) plus a stub scripts/pawl-review.sh
// that exits with the given code, and points testProjectDir at it. Restores all shared
// global state via t.Cleanup (the cmd/ao test-isolation contract).
func writePawlTestRepo(t *testing.T, exitCode int) {
	t.Helper()
	repo := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(repo, "docs", "contracts"), 0o755))
	must(os.WriteFile(filepath.Join(repo, "docs", "contracts", "agents-write-surfaces.md"), []byte("ok"), 0o644))
	must(os.MkdirAll(filepath.Join(repo, "skills"), 0o755))
	must(os.MkdirAll(filepath.Join(repo, "scripts"), 0o755))
	stub := "#!/usr/bin/env bash\nexit " + strconv.Itoa(exitCode) + "\n"
	must(os.WriteFile(filepath.Join(repo, "scripts", "pawl-review.sh"), []byte(stub), 0o755))

	prevDir := testProjectDir
	testProjectDir = repo
	// runPawlReview mutates these shared cobra-command flags on error — restore them.
	prevSU, prevSE := pawlReviewCmd.SilenceUsage, pawlReviewCmd.SilenceErrors
	t.Cleanup(func() {
		testProjectDir = prevDir
		pawlReviewCmd.SilenceUsage = prevSU
		pawlReviewCmd.SilenceErrors = prevSE
	})
}

// The exit code IS the verdict in `ao pawl review`; it must propagate the wrapped
// script's code verbatim (0 CONFIRMED · 3 REFUTED · 4 converge-advisory · 2 usage).
func TestRunPawlReview_PropagatesScriptExitCode(t *testing.T) {
	cases := []struct {
		name     string
		code     int
		wantNil  bool
		wantCode int
	}{
		{"CONFIRMED exit 0 -> nil", 0, true, 0},
		{"REFUTED exit 3 -> exitErr 3", 3, false, 3},
		{"converge advisory exit 4 -> exitErr 4", 4, false, 4},
		{"usage exit 2 -> exitErr 2", 2, false, 2},
		{"hard error exit 1 -> exitErr 1", 1, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writePawlTestRepo(t, tc.code)
			err := runPawlReview(pawlReviewCmd, []string{"age-test", "--scope", "head"})
			if tc.wantNil {
				if err != nil {
					t.Fatalf("exit %d: want nil error, got %v", tc.code, err)
				}
				return
			}
			var exitErr *pawlReviewExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("exit %d: want *pawlReviewExitError, got %T: %v", tc.code, err, err)
			}
			if exitErr.ExitCode() != tc.wantCode {
				t.Fatalf("ExitCode() = %d, want %d", exitErr.ExitCode(), tc.wantCode)
			}
		})
	}
}

// --help is for ao's command, not the wrapped script — it must NOT forward (which would
// run the exit-3 stub) and must return nil.
func TestRunPawlReview_HelpDoesNotForwardToScript(t *testing.T) {
	writePawlTestRepo(t, 3)
	if err := runPawlReview(pawlReviewCmd, []string{"--help"}); err != nil {
		t.Fatalf("--help should return nil (print help, not forward), got %v", err)
	}
}

// A missing script is a hard error, not a silent success.
func TestRunPawlReview_MissingScriptErrors(t *testing.T) {
	writePawlTestRepo(t, 0)
	repo := testProjectDir
	if err := os.Remove(filepath.Join(repo, "scripts", "pawl-review.sh")); err != nil {
		t.Fatal(err)
	}
	err := runPawlReview(pawlReviewCmd, []string{"age-test"})
	if err == nil {
		t.Fatal("missing pawl-review.sh should error, got nil")
	}
	var exitErr *pawlReviewExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("missing script should be a plain error, not a propagated exit code, got %v", err)
	}
}
