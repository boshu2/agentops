// practices: [continuous-delivery, dora-metrics]
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// --- L1: pure verdict + exit-code mapping ------------------------------------

func TestAggregateVerdict(t *testing.T) {
	tests := []struct {
		name    string
		hasFail bool
		hasWarn bool
		want    verdict
	}{
		{"clean is PASS", false, false, verdictPass},
		{"warnings only is WARN", false, true, verdictWarn},
		{"failure is FAIL", true, false, verdictFail},
		{"failure dominates warnings", true, true, verdictFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateVerdict(tt.hasFail, tt.hasWarn); got != tt.want {
				t.Errorf("aggregateVerdict(%v,%v) = %q, want %q", tt.hasFail, tt.hasWarn, got, tt.want)
			}
		})
	}
}

func TestGateExitForVerdict(t *testing.T) {
	tests := []struct {
		name   string
		v      verdict
		strict bool
		want   int
	}{
		{"PASS -> 0", verdictPass, false, gateExitPass},
		{"PASS strict -> 0", verdictPass, true, gateExitPass},
		{"WARN -> 0", verdictWarn, false, gateExitPass},
		{"WARN strict -> 1", verdictWarn, true, gateExitFail},
		{"FAIL -> 1", verdictFail, false, gateExitFail},
		{"FAIL strict -> 1", verdictFail, true, gateExitFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gateExitForVerdict(tt.v, tt.strict); got != tt.want {
				t.Errorf("gateExitForVerdict(%q, strict=%v) = %d, want %d", tt.v, tt.strict, got, tt.want)
			}
		})
	}
}

func TestGateExitError_ExitCode(t *testing.T) {
	e := &gateExitError{code: gateExitInternal, msg: "boom"}
	if e.ExitCode() != gateExitInternal {
		t.Errorf("ExitCode() = %d, want %d", e.ExitCode(), gateExitInternal)
	}
	if e.Error() != "boom" {
		t.Errorf("Error() = %q, want %q", e.Error(), "boom")
	}
}

// --- L2: full command via runValidate over real artifacts --------------------

// gateTestHarness resets the validate flags and points the validator at a temp
// dir with the given artifact. Returns a configured cobra command + buffers.
func gateTestHarness(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	tmp := t.TempDir()
	t.Chdir(tmp)

	// Reset all package-level validate flags between cases.
	validateGate = true
	validateBead = ""
	validateChanges = nil
	validateStrict = false
	validateWarnAsFail = false
	validateJSONOut = false
	validateLenient = false
	validateLenientExpiry = 90
	t.Cleanup(func() {
		validateGate = false
		validateChanges = nil
		validateStrict = false
		validateJSONOut = false
		output = "table"
	})

	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr, tmp
}

// writeArtifact creates a file under the temp dir, returning its relative path.
func writeArtifact(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

const passArtifact = `---
schema_version: 1
---
# Research: composed gate

## Summary
A deterministic validation gate composed from the existing ratchet validator.

## Key Findings
The investigation surfaced concrete, repository-grounded findings cross-checked
against the existing implementation so the conclusions hold across the surfaces
that matter for downstream consumers and for the validation gate itself.

## Recommendations
Proceed with the composed-validator approach: it reuses the ratchet validator,
confines the new logic to verdict aggregation and exit-code mapping, and stays
well under the cyclomatic complexity budget while remaining fully deterministic.
The gate performs no network calls and invokes no language model, which makes it
safe to run inside the GC check retry loop, inside continuous integration, and
inside the internal validate phase of the ao rpi loop without any flakiness or
external dependency. Each validated artifact contributes its issues and warnings
to a single aggregated verdict, and that verdict is mapped deterministically onto
the process exit code so that any orchestrator can branch on the exit status.

## Sources
- cli/internal/ratchet/validate.go
- cli/cmd/ao/ratchet_validate.go
`

// warnArtifact has schema_version (so it validates) but is short and missing
// recommended sections -> warnings, no hard issues.
const warnArtifact = `---
schema_version: 1
---
# Research: thin

## Summary
Short.
`

// failArtifact lacks schema_version -> strict-mode hard issue -> Valid=false.
const failArtifact = `no frontmatter, no schema_version, no sections
`

func assertGateExit(t *testing.T, err error, want int) {
	t.Helper()
	if want == gateExitPass {
		if err != nil {
			t.Fatalf("expected nil error (exit 0), got %v", err)
		}
		return
	}
	var ge *gateExitError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *gateExitError, got %T: %v", err, err)
	}
	if ge.ExitCode() != want {
		t.Errorf("gate exit = %d, want %d", ge.ExitCode(), want)
	}
}

func TestRunValidate_PassExits0(t *testing.T) {
	cmd, stdout, _, dir := gateTestHarness(t)
	validateChanges = []string{writeArtifact(t, dir, "research.md", passArtifact)}

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitPass)
	if got := stdout.String(); got != "PASS\n" {
		t.Errorf("stdout = %q, want %q", got, "PASS\n")
	}
}

func TestRunValidate_WarnExits0(t *testing.T) {
	cmd, stdout, _, dir := gateTestHarness(t)
	validateChanges = []string{writeArtifact(t, dir, "research.md", warnArtifact)}

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitPass)
	if got := stdout.String(); got != "WARN\n" {
		t.Errorf("stdout = %q, want %q", got, "WARN\n")
	}
}

func TestRunValidate_StrictFlipsWarnToFail(t *testing.T) {
	cmd, _, _, dir := gateTestHarness(t)
	validateChanges = []string{writeArtifact(t, dir, "research.md", warnArtifact)}
	validateStrict = true

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitFail)
}

func TestRunValidate_WarnAsFailAlias(t *testing.T) {
	cmd, _, _, dir := gateTestHarness(t)
	validateChanges = []string{writeArtifact(t, dir, "research.md", warnArtifact)}
	validateWarnAsFail = true

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitFail)
}

func TestRunValidate_FailExits1(t *testing.T) {
	cmd, stdout, _, dir := gateTestHarness(t)
	validateChanges = []string{writeArtifact(t, dir, "plan.md", failArtifact)}

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitFail)
	if got := stdout.String(); got != "FAIL\n" {
		t.Errorf("stdout = %q, want %q", got, "FAIL\n")
	}
}

func TestRunValidate_NoTargetsExits2(t *testing.T) {
	cmd, _, _, _ := gateTestHarness(t)
	// No --changes, empty temp dir -> nothing to validate -> internal error.

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitInternal)
}

func TestRunValidate_JSONOutputContract(t *testing.T) {
	cmd, stdout, _, dir := gateTestHarness(t)
	validateChanges = []string{writeArtifact(t, dir, "plan.md", failArtifact)}
	validateJSONOut = true

	err := runValidate(cmd, nil)
	assertGateExit(t, err, gateExitFail)

	var got validateGateResult
	if e := json.Unmarshal(stdout.Bytes(), &got); e != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", e, stdout.String())
	}
	if got.Verdict != verdictFail {
		t.Errorf("verdict = %q, want %q", got.Verdict, verdictFail)
	}
	if got.GateExit != gateExitFail {
		t.Errorf("gate_exit = %d, want %d", got.GateExit, gateExitFail)
	}
	if len(got.Issues) == 0 {
		t.Error("expected issues in FAIL JSON, got none")
	}
}

func TestRunValidate_DefaultModeNoGateNoExitError(t *testing.T) {
	cmd, stdout, _, dir := gateTestHarness(t)
	validateGate = false // default (report) mode
	validateChanges = []string{writeArtifact(t, dir, "plan.md", failArtifact)}

	// Default mode prints the verdict but never carries it in the exit code.
	err := runValidate(cmd, nil)
	if err != nil {
		t.Fatalf("default mode should not return a gate error, got %v", err)
	}
	if got := stdout.String(); got == "" || got[:8] != "Verdict:" {
		t.Errorf("default report should start with 'Verdict:', got %q", got)
	}
}
