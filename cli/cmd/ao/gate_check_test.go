package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestGateRepoRoot_ResolvesToScriptsParent(t *testing.T) {
	root, err := gateRepoRoot()
	if err != nil {
		t.Fatalf("gateRepoRoot: %v", err)
	}
	if root == "" {
		t.Fatal("gateRepoRoot returned empty root")
	}
	if _, err := os.Stat(filepath.Join(root, "scripts")); err != nil {
		t.Errorf("resolved root %q has no scripts/ dir: %v (orchestrator joins scripts/ off this)", root, err)
	}
}

func nativeCheck(id string, status ports.GateStatus, blocking bool) gates.Check {
	return gates.Check{
		ID: id, Tiers: gates.Full, Blocking: blocking,
		Run: func(context.Context, gates.RunContext) (ports.GateVerdict, error) {
			return ports.GateVerdict{Status: status, Reason: "test"}, nil
		},
	}
}

// runGateCheckWith executes runGateCheck against an isolated registry, restoring
// the package globals afterwards.
func runGateCheckWith(t *testing.T, reg *gates.Registry, full, jsonOut bool) (string, error) {
	t.Helper()
	save := func() func() {
		r, fl, j, ff, sc := gateCheckRegistry, gateCheckFull, gateCheckJSON, gateCheckFailFast, gateCheckScope
		wc, req, wp := gateCheckWorkflowCoverage, gateCheckRequireWorkflowParity, gateCheckWorkflowPath
		return func() {
			gateCheckRegistry, gateCheckFull, gateCheckJSON, gateCheckFailFast, gateCheckScope = r, fl, j, ff, sc
			gateCheckWorkflowCoverage, gateCheckRequireWorkflowParity, gateCheckWorkflowPath = wc, req, wp
		}
	}()
	defer save()
	gateCheckRegistry, gateCheckFull, gateCheckJSON, gateCheckFailFast, gateCheckScope = reg, full, jsonOut, false, "head"
	gateCheckWorkflowCoverage, gateCheckRequireWorkflowParity, gateCheckWorkflowPath = false, false, ".github/workflows/validate.yml"

	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetContext(context.Background())
	err := runGateCheck(c, nil)
	return buf.String(), err
}

func TestGateCheckMode(t *testing.T) {
	if gateCheckMode(true) != gates.Full {
		t.Error("--full should map to Full")
	}
	if gateCheckMode(false) != gates.Fast {
		t.Error("default should map to Fast")
	}
}

func TestGateCheck_RegisteredUnderGate(t *testing.T) {
	var found bool
	for _, c := range gateCmd.Commands() {
		if c.Name() == "check" {
			found = true
		}
	}
	if !found {
		t.Fatal("`ao gate check` not registered under gate")
	}
}

func TestGateCheck_HasModeFlags(t *testing.T) {
	for _, name := range []string{
		"fast",
		"full",
		"json",
		"github-annotations",
		"fail-fast",
		"scope",
		"workflow-coverage",
		"require-workflow-parity",
		"workflow-path",
	} {
		if gateCheckCmd.Flags().Lookup(name) == nil {
			t.Errorf("ao gate check missing --%s flag", name)
		}
	}
}

func TestGateCheck_FullJSONReportsVerdicts(t *testing.T) {
	reg := gates.NewRegistry()
	if err := reg.Add(nativeCheck("n.pass", ports.GateStatusPass, true)); err != nil {
		t.Fatal(err)
	}
	out, err := runGateCheckWith(t, reg, true, true)
	if err != nil {
		t.Fatalf("runGateCheck: %v", err)
	}
	var parsed struct {
		Run struct {
			Mode    string `json:"mode"`
			Summary struct {
				Total, Passed int
			} `json:"summary"`
		} `json:"run"`
		Gates []struct {
			Name string `json:"name"`
		} `json:"gates"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if parsed.Run.Mode != "full" {
		t.Errorf("mode = %q, want full", parsed.Run.Mode)
	}
	if parsed.Run.Summary.Total != 1 || parsed.Run.Summary.Passed != 1 {
		t.Errorf("summary = %+v, want total=1 passed=1", parsed.Run.Summary)
	}
	if len(parsed.Gates) != 1 || parsed.Gates[0].Name != "n.pass" {
		t.Errorf("gates = %+v, want one n.pass", parsed.Gates)
	}
}

func TestGateCheck_BlockingFailReturnsExitOne(t *testing.T) {
	reg := gates.NewRegistry()
	if err := reg.Add(nativeCheck("n.fail", ports.GateStatusFail, true)); err != nil {
		t.Fatal(err)
	}
	_, err := runGateCheckWith(t, reg, true, false)
	if err == nil {
		t.Fatal("blocking FAIL should return a gateExitError, got nil")
	}
	var ge *gateExitError
	if !errors.As(err, &ge) {
		t.Fatalf("err = %T, want *gateExitError", err)
	}
	if ge.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1", ge.ExitCode())
	}
}
