// Tests for the exit-code-as-verdict path of `ao beads verify|lint|audit`.
//
// These commands previously called os.Exit(1) mid-RunE to signal a stale /
// flagged verdict, which skipped deferred cleanup and killed the test binary,
// making the verdict path untestable. They now return a *beadsVerdictError that
// Execute() maps to the process exit code. These tests pin that contract.

// practices: [dora-metrics, lean-startup]
package main

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

// staleBeadShow is a `bd show` body that cites a file which cannot exist from
// any working directory, so verifyBead classifies it STALE deterministically.
const staleBeadShow = `○ na-stale · Stale bead for verdict testing   [● P2 · OPEN]
Owner: Test · Type: task

DESCRIPTION
This bead cites cli/cmd/ao/this-file-does-not-exist-zzz999.go which is missing,
forcing a stale citation regardless of the test's working directory.
`

func TestBeadsExitError_Contract(t *testing.T) {
	e := &beadsVerdictError{code: 1}
	// Error() is intentionally empty: the verdict text already went to stdout
	// and the command silences cobra's error print.
	if e.Error() != "" {
		t.Fatalf("Error() = %q, want empty", e.Error())
	}
	if e.ExitCode() != 1 {
		t.Fatalf("ExitCode() = %d, want 1", e.ExitCode())
	}
	// It must be discoverable via errors.As, the way Execute() maps it.
	var target *beadsVerdictError
	if !errors.As(error(e), &target) {
		t.Fatalf("errors.As failed to match *beadsVerdictError")
	}
}

func TestRunBeadsVerify_ReturnsExitErrorOnStale(t *testing.T) {
	origBD, origAvail := beadsTrackerOutput, beadsTrackerAvailable
	defer func() { beadsTrackerOutput, beadsTrackerAvailable = origBD, origAvail }()

	beadsTrackerAvailable = func() bool { return true }
	beadsTrackerOutput = func(args ...string) ([]byte, error) {
		return []byte(staleBeadShow), nil
	}

	cmd := &cobra.Command{}
	err := executeBeadsVerify(cmd, []string{"na-stale"})

	var bxErr *beadsVerdictError
	if !errors.As(err, &bxErr) {
		t.Fatalf("executeBeadsVerify on stale bead: got %v, want *beadsVerdictError", err)
	}
	if bxErr.ExitCode() != 1 {
		t.Fatalf("verdict exit code = %d, want 1", bxErr.ExitCode())
	}
	// The verdict return must silence cobra so no spurious "Error:" prints.
	if !cmd.SilenceErrors {
		t.Fatalf("expected cmd.SilenceErrors=true on verdict return")
	}
}

func TestRunBeadsVerify_NilCmdDoesNotPanicOnStale(t *testing.T) {
	origBD, origAvail := beadsTrackerOutput, beadsTrackerAvailable
	defer func() { beadsTrackerOutput, beadsTrackerAvailable = origBD, origAvail }()

	beadsTrackerAvailable = func() bool { return true }
	beadsTrackerOutput = func(args ...string) ([]byte, error) {
		return []byte(staleBeadShow), nil
	}

	// Direct callers (and older tests) pass a nil cmd; the nil-guard must hold.
	err := executeBeadsVerify(nil, []string{"na-stale"})
	var bxErr *beadsVerdictError
	if !errors.As(err, &bxErr) {
		t.Fatalf("executeBeadsVerify(nil, ...) on stale: got %v, want *beadsVerdictError", err)
	}
}

func TestRunBeadsLint_ReturnsExitErrorOnStale(t *testing.T) {
	origBD, origAvail := beadsTrackerOutput, beadsTrackerAvailable
	defer func() { beadsTrackerOutput, beadsTrackerAvailable = origBD, origAvail }()

	beadsTrackerAvailable = func() bool { return true }
	beadsTrackerOutput = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list" {
			return []byte("○ na-stale · Stale bead   [● P2 · OPEN]\n"), nil
		}
		// `show` for the listed bead → a stale body.
		return []byte(staleBeadShow), nil
	}

	cmd := &cobra.Command{}
	err := executeBeadsLint(cmd, []string{})

	var bxErr *beadsVerdictError
	if !errors.As(err, &bxErr) {
		t.Fatalf("executeBeadsLint with a stale bead: got %v, want *beadsVerdictError", err)
	}
	if bxErr.ExitCode() != 1 {
		t.Fatalf("verdict exit code = %d, want 1", bxErr.ExitCode())
	}
	if !cmd.SilenceErrors {
		t.Fatalf("expected cmd.SilenceErrors=true on verdict return")
	}
}

func TestRunBeadsLint_CleanReturnsNil(t *testing.T) {
	origBD, origAvail := beadsTrackerOutput, beadsTrackerAvailable
	defer func() { beadsTrackerOutput, beadsTrackerAvailable = origBD, origAvail }()

	beadsTrackerAvailable = func() bool { return true }
	beadsTrackerOutput = func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list" {
			return []byte("○ na-clean · Clean bead   [● P2 · OPEN]\n"), nil
		}
		// A `show` body with no file citations → no stale citations.
		return []byte("○ na-clean · Clean bead   [● P2 · OPEN]\nOwner: Test · Type: task\n\nDESCRIPTION\nNo citations here.\n"), nil
	}

	if err := executeBeadsLint(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("executeBeadsLint with a clean bead: got %v, want nil", err)
	}
}
