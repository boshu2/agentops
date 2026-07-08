package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestMembraneCalibrate_Registered pins that `ao membrane calibrate` is wired under
// the membrane command family and forwards flags verbatim (DisableFlagParsing) to
// scripts/membrane-calibrate.sh — the standing calibration harness (age-e508.2).
func TestMembraneCalibrate_Registered(t *testing.T) {
	var found *cobra.Command
	for _, c := range membraneCmd.Commands() {
		if c.Name() == "calibrate" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("`ao membrane calibrate` is not registered under membraneCmd")
	}
	if !found.DisableFlagParsing {
		t.Error("calibrate must DisableFlagParsing so calibration flags forward to the harness verbatim")
	}
	if defaultMembraneCalibrateScript != "scripts/membrane-calibrate.sh" {
		t.Errorf("calibrate script const = %q, want scripts/membrane-calibrate.sh", defaultMembraneCalibrateScript)
	}
}

// TestMembraneCalibrate_UntrustedRepoRefuses pins the RCE boundary the wrapper
// shares with the pawl live-script path: with the trust escape hatch cleared and
// the go-test binary living outside the checkout, the command refuses to run the
// repo script and names the escape hatch — it never execs bash. (No models.)
func TestMembraneCalibrate_UntrustedRepoRefuses(t *testing.T) {
	t.Setenv(trustRepoEnvVar, "") // force-untrust regardless of the dev's env
	// A bare RUN invocation (not --help) must hit the trust boundary and refuse.
	out, err := executeCommand("membrane", "calibrate")
	if err == nil {
		t.Fatalf("an untrusted checkout must refuse to run the calibration script; got success:\n%s", out)
	}
	if !strings.Contains(err.Error(), trustRepoEnvVar) {
		t.Errorf("the refusal should name the %s escape hatch; got: %v", trustRepoEnvVar, err)
	}
}

// TestMembraneCalibrate_HelpIsStatic pins that `--help` prints the STATIC command help
// WITHOUT touching the repo-trust boundary — so the command-surface doc generator records
// path-independent text, not a checkout-path-leaking RCE-guard error (age-e508.2 land: that
// leak made cli/docs/COMMANDS.md non-deterministic across worktrees, breaking derived.changed-scope).
func TestMembraneCalibrate_HelpIsStatic(t *testing.T) {
	t.Setenv(trustRepoEnvVar, "") // even untrusted, --help must print help, never the RCE error
	out, err := executeCommand("membrane", "calibrate", "--help")
	if err != nil {
		t.Fatalf("--help must print help without error even in an untrusted checkout; got: %v\n%s", err, out)
	}
	if !strings.Contains(out, "standing membrane calibration harness") {
		t.Errorf("--help should print the static Long; got:\n%s", out)
	}
	if strings.Contains(out, "refusing to run repo script") {
		t.Errorf("--help must NOT leak the RCE-guard error (path-dependent, non-deterministic); got:\n%s", out)
	}
}
