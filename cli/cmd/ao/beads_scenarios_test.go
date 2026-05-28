package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// withStubbedBD swaps execBD/bdAvailable for the duration of a test and
// restores them afterward. The stub records every bd argv so tests can assert
// the dry-run contract (no `bd update`).
func withStubbedBD(t *testing.T, available bool, handler func(args ...string) ([]byte, error)) *[][]string {
	t.Helper()
	origExec, origAvail := execBD, bdAvailable
	t.Cleanup(func() { execBD, bdAvailable = origExec, origAvail })

	var calls [][]string
	bdAvailable = func() bool { return available }
	execBD = func(args ...string) ([]byte, error) {
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

	err = runBeadsScenariosExtract(beadsScenariosExtractCmd, []string{id})
	return out.String(), errBuf.String(), err
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
		t.Fatalf("execBD must not be called when bd is unavailable: %v", args)
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

func TestParseAcceptanceFromBDJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{
			name: "array form prefers acceptance_criteria",
			in:   `[{"acceptance_criteria":"crit text","description":"desc text"}]`,
			want: "crit text",
		},
		{
			name: "single object form",
			in:   `{"acceptance_criteria":"crit text"}`,
			want: "crit text",
		},
		{
			name: "falls back to description when acceptance empty",
			in:   `[{"acceptance_criteria":"  ","description":"desc text"}]`,
			want: "desc text",
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
			got, err := parseAcceptanceFromBDJSON([]byte(tt.in))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAcceptanceFromBDJSON(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
