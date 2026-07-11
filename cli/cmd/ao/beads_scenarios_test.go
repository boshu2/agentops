package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// withStubbedBD swaps beadsTrackerOutput/beadsTrackerAvailable for the duration of a test and
// restores them afterward. The stub records every bd argv so tests can assert
// the dry-run contract (no `bd update`).
func withStubbedBD(t *testing.T, available bool, handler func(args ...string) ([]byte, error)) *[][]string {
	t.Helper()
	origExec, origAvail := beadsTrackerOutput, beadsTrackerAvailable
	t.Cleanup(func() { beadsTrackerOutput, beadsTrackerAvailable = origExec, origAvail })

	var calls [][]string
	beadsTrackerAvailable = func() bool { return available }
	beadsTrackerOutput = func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return handler(args...)
	}
	return &calls
}

func runScenariosExtract(t *testing.T, jsonOut bool, id string) (stdout, stderr string, err error) {
	t.Helper()
	beadsScenariosJSON = jsonOut
	t.Cleanup(func() { beadsScenariosJSON = false })

	var out, errBuf bytes.Buffer
	beadsScenariosExtractCmd.SetOut(&out)
	beadsScenariosExtractCmd.SetErr(&errBuf)
	t.Cleanup(func() {
		beadsScenariosExtractCmd.SetOut(nil)
		beadsScenariosExtractCmd.SetErr(nil)
	})

	err = executeBeadsScenariosExtract(beadsScenariosExtractCmd, []string{id})
	return out.String(), errBuf.String(), err
}

func runScenariosValidate(t *testing.T, jsonOut bool, id string) (stdout, stderr string, err error) {
	t.Helper()
	beadsScenariosValidateJSON = jsonOut
	t.Cleanup(func() { beadsScenariosValidateJSON = false })

	var out, errBuf bytes.Buffer
	beadsScenariosValidateCmd.SetOut(&out)
	beadsScenariosValidateCmd.SetErr(&errBuf)
	t.Cleanup(func() {
		beadsScenariosValidateCmd.SetOut(nil)
		beadsScenariosValidateCmd.SetErr(nil)
	})

	err = executeBeadsScenariosValidate(beadsScenariosValidateCmd, []string{id})
	return out.String(), errBuf.String(), err
}

// runScenariosExtractWrite runs extract in --write mode with a canned stdin
// response for the y/N confirmation prompt.
func runScenariosExtractWrite(t *testing.T, id, stdin string) (stdout, stderr string, err error) {
	t.Helper()
	beadsScenariosWrite = true
	t.Cleanup(func() { beadsScenariosWrite = false })

	var out, errBuf bytes.Buffer
	beadsScenariosExtractCmd.SetOut(&out)
	beadsScenariosExtractCmd.SetErr(&errBuf)
	beadsScenariosExtractCmd.SetIn(strings.NewReader(stdin))
	t.Cleanup(func() {
		beadsScenariosExtractCmd.SetOut(nil)
		beadsScenariosExtractCmd.SetErr(nil)
		beadsScenariosExtractCmd.SetIn(nil)
	})

	err = executeBeadsScenariosExtract(beadsScenariosExtractCmd, []string{id})
	return out.String(), errBuf.String(), err
}

func TestRunBeadsScenariosExtract_WriteUpdatesBeadAfterConfirmation(t *testing.T) {
	var updateDesc string
	calls := withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		switch {
		case len(args) >= 1 && args[0] == "show":
			return []byte(`[{"id":"ag-x","acceptance_criteria":"Given a bead when extract runs then a block is written","description":"original prose"}]`), nil
		case len(args) >= 1 && args[0] == "update":
			for i := 0; i+1 < len(args); i++ {
				if args[i] == "--description" || args[i] == "-d" {
					updateDesc = args[i+1]
				}
			}
			return []byte("ok"), nil
		}
		return nil, fmt.Errorf("unexpected bd call: %v", args)
	})

	_, _, err := runScenariosExtractWrite(t, "ag-x", "y\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated bool
	for _, c := range *calls {
		if len(c) > 0 && c[0] == "update" {
			updated = true
		}
	}
	if !updated {
		t.Fatal("expected 'bd update' after confirmation; bead was not written")
	}
	if !strings.Contains(updateDesc, "## Scenarios") {
		t.Errorf("written description must carry the '## Scenarios' block, got %q", updateDesc)
	}
	// Lossless write: the original description text must be preserved.
	if !strings.Contains(updateDesc, "original prose") {
		t.Errorf("written description must preserve the original prose, got %q", updateDesc)
	}
}

func TestRunBeadsScenariosExtract_WriteAbortsWithoutConfirmation(t *testing.T) {
	calls := withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "show" {
			return []byte(`[{"id":"ag-x","acceptance_criteria":"Given a bead when extract runs then a block is written","description":"original prose"}]`), nil
		}
		return nil, fmt.Errorf("unexpected bd call: %v", args)
	})

	_, stderr, err := runScenariosExtractWrite(t, "ag-x", "n\n")
	if err != nil {
		t.Fatalf("declining the prompt should exit gracefully, got error: %v", err)
	}
	for _, c := range *calls {
		if len(c) > 0 && c[0] == "update" {
			t.Errorf("without confirmation the bead must be unchanged; 'bd update' was called: %v", c)
		}
	}
	if !strings.Contains(stderr, "unchanged") {
		t.Errorf("stderr should report the bead was left unchanged, got %q", stderr)
	}
}

func TestRunBeadsScenariosValidate_WellFormedExitsZero(t *testing.T) {
	calls := withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "show" {
			return []byte(`[{"id":"ag-x","description":"prose\n\n## Scenarios\nScenario: ok\n  Given a\n  When b\n  Then c\n"}]`), nil
		}
		return nil, fmt.Errorf("unexpected bd call: %v", args)
	})

	stdout, _, err := runScenariosValidate(t, false, "ag-x")
	if err != nil {
		t.Fatalf("expected exit 0 for well-formed scenarios, got error: %v", err)
	}
	if !strings.Contains(stdout, "well-formed") {
		t.Errorf("stdout should confirm well-formedness, got %q", stdout)
	}
	// Read-only contract: validate must never mutate the bead.
	for _, c := range *calls {
		if len(c) > 0 && c[0] == "update" {
			t.Errorf("validate must not call bd update: %v", c)
		}
	}
}

func TestRunBeadsScenariosValidate_MalformedIsError(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		// Missing the When step — malformed.
		return []byte(`[{"id":"ag-x","description":"## Scenarios\nScenario: broken\n  Given a\n  Then c\n"}]`), nil
	})

	_, _, err := runScenariosValidate(t, false, "ag-x")
	if err == nil {
		t.Fatal("expected a non-nil error (non-zero exit) for malformed scenarios")
	}
	if !strings.Contains(err.Error(), "When") {
		t.Errorf("error should name the parse problem (missing When), got: %v", err)
	}
	if !strings.Contains(err.Error(), "extract") {
		t.Errorf("error should name the corrective command, got: %v", err)
	}
}

func TestRunBeadsScenariosValidate_NoBlockIsError(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"ag-x","description":"just free text","acceptance_criteria":"no block here"}]`), nil
	})

	_, _, err := runScenariosValidate(t, false, "ag-x")
	if err == nil {
		t.Fatal("expected an error when no '## Scenarios' block is present")
	}
	if !strings.Contains(err.Error(), "Scenarios") {
		t.Errorf("error should mention the missing block, got: %v", err)
	}
}

func TestRunBeadsScenariosValidate_JSONVerdict(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"ag-x","description":"## Scenarios\nScenario: ok\n  Given a\n  When b\n  Then c\n"}]`), nil
	})

	stdout, _, err := runScenariosValidate(t, true, "ag-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload struct {
		BeadID    string `json:"bead_id"`
		Valid     bool   `json:"valid"`
		Scenarios []struct {
			Name string `json:"name"`
		} `json:"scenarios"`
	}
	if jErr := json.Unmarshal([]byte(stdout), &payload); jErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jErr, stdout)
	}
	if !payload.Valid || payload.BeadID != "ag-x" || len(payload.Scenarios) != 1 {
		t.Errorf("unexpected verdict: %+v", payload)
	}
}

func TestRunBeadsScenariosValidate_JSONFailureVerdictOnStdout(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"ag-x","description":"## Scenarios\nScenario: broken\n  Given a\n  Then c\n"}]`), nil
	})

	stdout, _, err := runScenariosValidate(t, true, "ag-x")
	if err == nil {
		t.Fatal("expected a non-nil error (non-zero exit) for malformed scenarios")
	}
	var payload struct {
		BeadID string `json:"bead_id"`
		Valid  bool   `json:"valid"`
		Error  string `json:"error"`
	}
	if jErr := json.Unmarshal([]byte(stdout), &payload); jErr != nil {
		t.Fatalf("failure verdict on stdout is not valid JSON: %v\n%s", jErr, stdout)
	}
	if payload.Valid || payload.BeadID != "ag-x" || payload.Error == "" {
		t.Errorf("unexpected failure verdict: %+v", payload)
	}
}

func TestRunBeadsScenariosValidate_BDUnavailableWarnsAndSucceeds(t *testing.T) {
	withStubbedBD(t, false, func(args ...string) ([]byte, error) {
		t.Fatalf("beadsTrackerOutput must not be called when bd is unavailable: %v", args)
		return nil, nil
	})

	stdout, stderr, err := runScenariosValidate(t, false, "ag-x")
	if err != nil {
		t.Fatalf("expected graceful exit, got error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout when bd unavailable, got %q", stdout)
	}
	if !strings.Contains(stderr, "bd not found") {
		t.Errorf("expected a bd-not-found warning on stderr, got %q", stderr)
	}
}

func TestRunBeadsScenariosExtract_PrintsGherkinToStdout(t *testing.T) {
	calls := withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "show" {
			return []byte(`[{"id":"ag-x","acceptance_criteria":"- Given a bead with free-text bullets, when extract runs, then a Gherkin block is printed to stdout"}]`), nil
		}
		return nil, fmt.Errorf("unexpected bd call: %v", args)
	})

	stdout, _, err := runScenariosExtract(t, false, "ag-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"## Scenarios", "Scenario:", "  Given ", "  When ", "  Then "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n--- stdout ---\n%s", want, stdout)
		}
	}
	// Dry-run contract: the bead must not be modified.
	for _, c := range *calls {
		if len(c) > 0 && c[0] == "update" {
			t.Errorf("dry-run violated: bd update was called: %v", c)
		}
	}
}

func TestRunBeadsScenariosExtract_JSONOutput(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"ag-x","acceptance_criteria":"Given a bead when extract runs then JSON is emitted"}]`), nil
	})

	stdout, _, err := runScenariosExtract(t, true, "ag-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload struct {
		BeadID    string `json:"bead_id"`
		Scenarios []struct {
			Name  string `json:"name"`
			Given string `json:"given"`
			When  string `json:"when"`
			Then  string `json:"then"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if payload.BeadID != "ag-x" {
		t.Errorf("bead_id = %q, want %q", payload.BeadID, "ag-x")
	}
	if len(payload.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(payload.Scenarios))
	}
	if payload.Scenarios[0].Given != "a bead" || payload.Scenarios[0].When != "extract runs" {
		t.Errorf("unexpected scenario clauses: %+v", payload.Scenarios[0])
	}
}

func TestRunBeadsScenariosExtract_UnparseableIsError(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"ag-x","acceptance_criteria":"The CLI exits 0 on success and 1 on failure."}]`), nil
	})

	stdout, _, err := runScenariosExtract(t, false, "ag-x")
	if err == nil {
		t.Fatal("expected an error for unparseable acceptance, got nil")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout on error, got %q", stdout)
	}
	if !strings.Contains(err.Error(), "manually") {
		t.Errorf("error should name the corrective action (manual authoring), got: %v", err)
	}
}

func TestRunBeadsScenariosExtract_BDUnavailableWarnsAndSucceeds(t *testing.T) {
	withStubbedBD(t, false, func(args ...string) ([]byte, error) {
		t.Fatalf("beadsTrackerOutput must not be called when bd is unavailable: %v", args)
		return nil, nil
	})

	stdout, stderr, err := runScenariosExtract(t, false, "ag-x")
	if err != nil {
		t.Fatalf("expected graceful exit, got error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout when bd unavailable, got %q", stdout)
	}
	if !strings.Contains(stderr, "bd not found") {
		t.Errorf("expected a bd-not-found warning on stderr, got %q", stderr)
	}
}

func TestParseBeadFromBDJSON(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantAccept  string
		wantDescrip string
		wantErr     bool
	}{
		{
			name:        "array form prefers acceptance_criteria and keeps description",
			in:          `[{"acceptance_criteria":"crit text","description":"desc text"}]`,
			wantAccept:  "crit text",
			wantDescrip: "desc text",
		},
		{
			name:       "single object form",
			in:         `{"acceptance_criteria":"crit text"}`,
			wantAccept: "crit text",
		},
		{
			name:        "falls back to description when acceptance empty",
			in:          `[{"acceptance_criteria":"  ","description":"desc text"}]`,
			wantAccept:  "desc text",
			wantDescrip: "desc text",
		},
		{
			name:    "empty array is an error",
			in:      `[]`,
			wantErr: true,
		},
		{
			name:    "no acceptance or description is an error",
			in:      `[{"acceptance_criteria":"","description":""}]`,
			wantErr: true,
		},
		{
			name:    "malformed json is an error",
			in:      `{not json`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBeadFromBDJSON([]byte(tt.in))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseBeadFromBDJSON(%q) = %+v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Acceptance != tt.wantAccept {
				t.Errorf("Acceptance = %q, want %q", got.Acceptance, tt.wantAccept)
			}
			if got.Description != tt.wantDescrip {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantDescrip)
			}
		})
	}
}

func TestRunBeadsScenariosExtract_RefusesWhenScenariosExist(t *testing.T) {
	calls := withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		// acceptance_criteria is parseable, but the description already carries
		// an authored '## Scenarios' block — the guard must short-circuit.
		return []byte(`[{"id":"ag-x","acceptance_criteria":"Given a bead when extract runs then a block prints","description":"## Scenarios\nScenario: existing\n  Given a\n  When b\n  Then c"}]`), nil
	})

	stdout, stderr, err := runScenariosExtract(t, false, "ag-x")
	if err != nil {
		t.Fatalf("guard should refuse gracefully, got error: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected no stdout when scenarios already exist, got %q", stdout)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr should name --force as the corrective action, got %q", stderr)
	}
	for _, c := range *calls {
		if len(c) > 0 && c[0] == "update" {
			t.Errorf("dry-run violated: bd update was called: %v", c)
		}
	}
}

func TestRunBeadsScenariosExtract_ForceReExtractsOverExisting(t *testing.T) {
	withStubbedBD(t, true, func(args ...string) ([]byte, error) {
		return []byte(`[{"id":"ag-x","acceptance_criteria":"Given a bead when extract runs then a block prints","description":"## Scenarios\nScenario: existing\n  Given a\n  When b\n  Then c"}]`), nil
	})

	beadsScenariosForce = true
	t.Cleanup(func() { beadsScenariosForce = false })

	stdout, _, err := runScenariosExtract(t, false, "ag-x")
	if err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}
	if !strings.Contains(stdout, "## Scenarios") {
		t.Errorf("--force should print a freshly extracted block, got %q", stdout)
	}
	// The printed block is extracted from acceptance_criteria, not the bead's
	// pre-existing description block.
	if !strings.Contains(stdout, "When extract runs") {
		t.Errorf("--force output should reflect the acceptance text, got %q", stdout)
	}
}
