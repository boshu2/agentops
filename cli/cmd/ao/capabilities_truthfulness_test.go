package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// capabilityEntry returns the projected capabilities record for a command path
// from the fully-assembled live root command.
func capabilityEntry(t *testing.T, path string) clicontract.Command {
	t.Helper()
	for _, command := range clicontract.Inspect(rootCmd, map[string]map[string]string{}) {
		if command.Path == path {
			return command
		}
	}
	t.Fatalf("no capabilities entry for %q", path)
	return clicontract.Command{}
}

// TestTruthfulnessSixCommandSliceReportsRealContracts pins declared==observed
// for the owner-scoped six-command slice (age-gocli-audit-remediation-6fybr.7).
// Each entry must project the command's real CommandContract, never the
// fabricated range/mixed/{0,1} placeholder inspect.go used to emit.
func TestTruthfulnessSixCommandSliceReportsRealContracts(t *testing.T) {
	cases := []struct {
		path    string
		args    string
		output  string
		effects string
		exit    map[string]string
	}{
		{"ao version", "arbitrary", "text", "pure", map[string]string{"0": "success", "1": "failure"}},
		{"ao redact", "no-args", "text", "pure", map[string]string{"0": "success", "1": "failure"}},
		{"ao status", "arbitrary", "text", "filesystem,clock", map[string]string{"0": "success", "1": "failure"}},
		{"ao doctor", "no-args", "none", "filesystem,process,network,environment,clock", map[string]string{"0": "success", "1": "failure"}},
		{"ao gate check", "no-args", "text", "filesystem,process,environment,clock", map[string]string{"0": "success", "1": "failure", "2": "invalid-configuration"}},
		{"ao eval run", "exact", "text", "filesystem,process,environment,clock", map[string]string{"0": "success", "1": "failure"}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			entry := capabilityEntry(t, tc.path)
			if entry.Args != tc.args {
				t.Errorf("%s args = %q, want %q", tc.path, entry.Args, tc.args)
			}
			if entry.Output != tc.output {
				t.Errorf("%s output = %q, want %q", tc.path, entry.Output, tc.output)
			}
			if entry.Effects != tc.effects {
				t.Errorf("%s effects = %q, want %q", tc.path, entry.Effects, tc.effects)
			}
			if !reflect.DeepEqual(entry.ExitCodes, tc.exit) {
				t.Errorf("%s exit_codes = %v, want %v", tc.path, entry.ExitCodes, tc.exit)
			}
		})
	}
}

// TestTruthfulnessVersionExitCodesObserved verifies the version command's
// declared success/failure exits are the ones it actually produces: exit 0 on a
// normal run, and a nonzero-classed failure path that never returns success.
func TestTruthfulnessVersionExitCodesObserved(t *testing.T) {
	entry := capabilityEntry(t, "ao version")
	if entry.ExitCodes["0"] != "success" {
		t.Fatalf("version declares exit 0 = %q, want success", entry.ExitCodes["0"])
	}
	// Observed: a normal `ao version` run exits 0 (success).
	if _, err := executeCommand("version"); err != nil {
		t.Fatalf("ao version returned error (nonzero exit) on the success path: %v", err)
	}
}

// TestTruthfulnessUncontractedCommandsAreHonestlyUnknown asserts inspect.go no
// longer emits the fabricated range/mixed/{0,1} defaults as if authoritative:
// a runnable command with no attached contract must report unknown args,
// unknown effects, and absent (null) exit codes.
func TestTruthfulnessUncontractedCommandsAreHonestlyUnknown(t *testing.T) {
	// eval compare is runnable (RunE) and carries no CommandContract.
	entry := capabilityEntry(t, "ao eval compare")
	if entry.Args != "unknown" {
		t.Errorf("uncontracted eval compare args = %q, want %q", entry.Args, "unknown")
	}
	if entry.Effects != "unknown" {
		t.Errorf("uncontracted eval compare effects = %q, want %q", entry.Effects, "unknown")
	}
	if entry.ExitCodes != nil {
		t.Errorf("uncontracted eval compare exit_codes = %v, want nil (no fabricated {0,1})", entry.ExitCodes)
	}

	// Guard against regression to the specific fabricated placeholders across
	// the whole surface: no uncontracted entry may claim range/mixed effects or
	// the {0:success,1:error} default that inspect.go used to invent.
	for _, command := range clicontract.Inspect(rootCmd, map[string]map[string]string{}) {
		if command.Effects == "mixed" {
			t.Errorf("%s still reports fabricated effects=mixed", command.Path)
		}
		if command.ExitCodes["1"] == "error" {
			t.Errorf("%s still reports fabricated exit 1=error", command.Path)
		}
	}
}

// observedExit maps a returned command error to the process exit code the root
// executable would produce, mirroring Execute()'s mapping in root.go: a typed
// commandExitError carries its own code; any other error is a cobra-level
// failure that exits 1; nil is success.
func observedExit(err error) int {
	if err == nil {
		return 0
	}
	var commandExit commandExitError
	if errors.As(err, &commandExit) {
		return commandExit.ExitCode()
	}
	return 1
}

// TestTruthfulnessGateCheckExitCodesObserved is the observed==declared
// regression that catches the exit-2 mislabel: the `ao gate check` contract
// declares exit 2 = invalid-configuration and exit 1 = failure, so the command
// must actually produce those codes for those conditions. An invalid --scope
// value is an invalid gate configuration (exit 2); a bogus positional arg is a
// cobra usage error that exits 1, NOT 2 — proving exit 2 is not a generic
// "usage" class.
func TestTruthfulnessGateCheckExitCodesObserved(t *testing.T) {
	// Declared contract: exit 2 is invalid-configuration, exit 1 is failure.
	entry := capabilityEntry(t, "ao gate check")
	if got := entry.ExitCodes["2"]; got != "invalid-configuration" {
		t.Fatalf("ao gate check declares exit 2 = %q, want %q", got, "invalid-configuration")
	}
	if got := entry.ExitCodes["1"]; got != "failure" {
		t.Fatalf("ao gate check declares exit 1 = %q, want %q", got, "failure")
	}

	// (a) Invalid --scope value → the declared 2-meaning (invalid gate config).
	// Scope is rejected before any check runs, so this stays hermetic.
	if _, err := executeCommand("gate", "check", "--scope", "nonsense"); observedExit(err) != 2 {
		t.Errorf("ao gate check --scope nonsense observed exit %d, want 2 (invalid-configuration)", observedExit(err))
	}

	// (b) Bogus positional arg → cobra NoArgs usage error, exits 1 (never 2).
	if _, err := executeCommand("gate", "check", "bogus"); observedExit(err) != 1 {
		t.Errorf("ao gate check bogus observed exit %d, want 1 (cobra usage error, not the exit-2 config class)", observedExit(err))
	}
}
