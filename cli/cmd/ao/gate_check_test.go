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

// nativeCheckReason builds a native FAIL/PASS check that carries a reason but no
// LogTail — the shape (changelog.sync, learning.coherence, silent backing
// scripts) that emitted an empty --json log_tail before age-wy2t.
func nativeCheckReason(id string, status ports.GateStatus, reason string) gates.Check {
	return gates.Check{
		ID: id, Tiers: gates.Full, Blocking: true,
		Run: func(context.Context, gates.RunContext) (ports.GateVerdict, error) {
			return ports.GateVerdict{Status: status, Reason: reason}, nil
		},
	}
}

// TestGateCheck_FailJSONHasNonEmptyLogTail is the SLICE 2 acceptance end-to-end:
// `ao gate check --full --json` on a FAILing native check emits a non-empty
// log_tail carrying the reason, so a landing loop never has to re-run + tee-log
// to learn which gate failed and why.
func TestGateCheck_FailJSONHasNonEmptyLogTail(t *testing.T) {
	reg := gates.NewRegistry()
	if err := reg.Add(nativeCheckReason("n.fail", ports.GateStatusFail, "CHANGELOG.md != docs/CHANGELOG.md")); err != nil {
		t.Fatal(err)
	}
	// A blocking FAIL returns a gateExitError; the JSON is still written first.
	out, err := runGateCheckWith(t, reg, true, true)
	if err == nil {
		t.Fatal("blocking FAIL should return a gateExitError")
	}
	var parsed struct {
		Gates []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			LogTail string `json:"log_tail"`
		} `json:"gates"`
	}
	if uerr := json.Unmarshal([]byte(out), &parsed); uerr != nil {
		t.Fatalf("unmarshal %q: %v", out, uerr)
	}
	if len(parsed.Gates) != 1 {
		t.Fatalf("gates = %+v, want one", parsed.Gates)
	}
	g := parsed.Gates[0]
	if g.Status != "FAIL" {
		t.Fatalf("status = %q, want FAIL", g.Status)
	}
	if g.LogTail != "CHANGELOG.md != docs/CHANGELOG.md" {
		t.Fatalf("log_tail = %q, want the reason (non-empty detail)", g.LogTail)
	}
}

func TestApplyRangeScope_ExportsEnv(t *testing.T) {
	t.Setenv("AGENTOPS_GATE_RANGE", "") // baseline + auto-restore
	if err := applyRangeScope("range:origin/main..HEAD"); err != nil {
		t.Fatalf("applyRangeScope: %v", err)
	}
	if got := os.Getenv("AGENTOPS_GATE_RANGE"); got != "origin/main..HEAD" {
		t.Fatalf("AGENTOPS_GATE_RANGE = %q, want origin/main..HEAD", got)
	}
}

func TestApplyRangeScope_NonRangeLeavesEnv(t *testing.T) {
	t.Setenv("AGENTOPS_GATE_RANGE", "sentinel")
	for _, scope := range []gates.Scope{gates.ScopeHead, gates.ScopeStaged, gates.ScopeUpstream} {
		if err := applyRangeScope(scope); err != nil {
			t.Fatalf("applyRangeScope(%q): %v", scope, err)
		}
	}
	if got := os.Getenv("AGENTOPS_GATE_RANGE"); got != "sentinel" {
		t.Fatalf("AGENTOPS_GATE_RANGE = %q, want unchanged sentinel", got)
	}
}

func TestApplyRangeScope_InvalidRangeErrors(t *testing.T) {
	t.Setenv("AGENTOPS_GATE_RANGE", "")
	if err := applyRangeScope("range:HEAD"); err == nil {
		t.Fatal("range without .. should error")
	}
	if err := applyRangeScope("range:"); err == nil {
		t.Fatal("empty range should error")
	}
}

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

func TestGateCheck_RequireWorkflowParityIgnoresDeferredAndAdvisoryMissing(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workflowPath := filepath.Join(workflowDir, "validate.yml")
	workflow := []byte(`name: Validate
jobs:
  advisory:
    runs-on: ubuntu-latest
    steps:
      - name: advisory missing
        continue-on-error: true
        run: scripts/lint-evidence-lines.sh 123
  deferred:
    runs-on: ubuntu-latest
    steps:
      - name: ap7
        run: scripts/verify-gate-claim.sh --log /tmp/run.log pr-1 "claim"
`)
	if err := os.WriteFile(workflowPath, workflow, 0o644); err != nil {
		t.Fatal(err)
	}

	reg := gates.NewRegistry()
	if err := reg.Add(nativeCheck("n.pass", ports.GateStatusPass, true)); err != nil {
		t.Fatal(err)
	}

	save := func() func() {
		r, fl, j, ff, sc := gateCheckRegistry, gateCheckFull, gateCheckJSON, gateCheckFailFast, gateCheckScope
		wc, req, wp := gateCheckWorkflowCoverage, gateCheckRequireWorkflowParity, gateCheckWorkflowPath
		return func() {
			gateCheckRegistry, gateCheckFull, gateCheckJSON, gateCheckFailFast, gateCheckScope = r, fl, j, ff, sc
			gateCheckWorkflowCoverage, gateCheckRequireWorkflowParity, gateCheckWorkflowPath = wc, req, wp
		}
	}()
	defer save()
	gateCheckRegistry = reg
	gateCheckFull = true
	gateCheckJSON = true
	gateCheckFailFast = false
	gateCheckScope = "head"
	gateCheckWorkflowCoverage = true
	gateCheckRequireWorkflowParity = true
	gateCheckWorkflowPath = workflowPath

	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetContext(context.Background())
	if err := runGateCheck(c, nil); err != nil {
		t.Fatalf("runGateCheck returned error for deferred/advisory-only parity gaps: %v", err)
	}
	var parsed struct {
		Coverage struct {
			MissingBlockingCount int `json:"missing_blocking_count"`
			MissingAdvisoryCount int `json:"missing_advisory_count"`
			MissingDeferredCount int `json:"missing_deferred_count"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal %q: %v", buf.String(), err)
	}
	if parsed.Coverage.MissingBlockingCount != 0 {
		t.Fatalf("MissingBlockingCount = %d, want 0", parsed.Coverage.MissingBlockingCount)
	}
	if parsed.Coverage.MissingAdvisoryCount != 0 {
		t.Fatalf("MissingAdvisoryCount = %d, want 0; lint-evidence-lines is deferred by workflow context", parsed.Coverage.MissingAdvisoryCount)
	}
	if parsed.Coverage.MissingDeferredCount != 2 {
		t.Fatalf("MissingDeferredCount = %d, want 2", parsed.Coverage.MissingDeferredCount)
	}
}
