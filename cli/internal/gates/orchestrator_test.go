package gates

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

type fakeFiles struct {
	files []string
	err   error
}

func (f fakeFiles) Changed(context.Context, Scope) ([]string, error) { return f.files, f.err }

// testOrch builds an orchestrator with a deterministic clock and a fixed set of
// backing verdicts.
func testOrch(t *testing.T, reg *Registry, files ChangedFilesPort, verdicts map[ports.GateName]ports.GateVerdict) *Orchestrator {
	t.Helper()
	runner := ports.NewInMemoryGateRunner(verdicts)
	o := NewOrchestrator(reg, runner, files, "/repo")
	tick := time.Unix(0, 0)
	o.now = func() time.Time { tick = tick.Add(time.Second); return tick }
	return o
}

func sampleRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	must := func(c Check) {
		if err := r.Add(c); err != nil {
			t.Fatalf("Add(%s): %v", c.ID, err)
		}
	}
	must(Check{ID: "go", Tiers: Fast | Full, Match: []string{"cli/**"}, Blocking: true, Backing: "go"})
	must(Check{ID: "docs", Tiers: Fast | Full, Match: []string{"docs/**"}, Blocking: false, Backing: "docs"})
	must(Check{ID: "always", Tiers: Fast | Full, Blocking: true, Backing: "always"})
	must(Check{ID: "fullonly", Tiers: Full, Blocking: true, Backing: "fullonly"})
	return r
}

var sampleVerdicts = map[ports.GateName]ports.GateVerdict{
	"go":       {Status: ports.GateStatusFail, Reason: "vet failed"},
	"docs":     {Status: ports.GateStatusWarn, Reason: "advisory"},
	"always":   {Status: ports.GateStatusPass, Reason: "ok"},
	"fullonly": {Status: ports.GateStatusPass, Reason: "ok"},
}

func ranIDs(r *Report) map[string]bool {
	m := map[string]bool{}
	for _, res := range r.Results {
		m[res.Check.ID] = true
	}
	return m
}

func resultByID(r *Report, id string) (CheckResult, bool) {
	for _, res := range r.Results {
		if res.Check.ID == id {
			return res, true
		}
	}
	return CheckResult{}, false
}

func skippedByID(r *Report, id string) (SkippedCheck, bool) {
	for _, skip := range r.Skipped {
		if skip.Check.ID == id {
			return skip, true
		}
	}
	return SkippedCheck{}, false
}

func TestOrchestrator_FullModeRunsAllTierMatched(t *testing.T) {
	o := testOrch(t, sampleRegistry(t), fakeFiles{}, sampleVerdicts)
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := ranIDs(rep)
	for _, id := range []string{"go", "docs", "always", "fullonly"} {
		if !got[id] {
			t.Errorf("Full mode should run %q", id)
		}
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 (blocking 'go' FAIL)", rep.ExitCode())
	}
}

func TestGateOrchestrator_FastModeRoutesByChangedFiles(t *testing.T) {
	o := testOrch(t, sampleRegistry(t), fakeFiles{files: []string{"cli/cmd/ao/main.go"}}, sampleVerdicts)
	rep, err := o.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := ranIDs(rep)
	if !got["go"] || !got["always"] {
		t.Errorf("fast/cli change should run go+always; got %v", got)
	}
	if got["docs"] {
		t.Error("docs check should NOT run on a cli-only change")
	}
	if got["fullonly"] {
		t.Error("Full-only check should NOT run in fast mode")
	}
	goResult, ok := resultByID(rep, "go")
	if !ok {
		t.Fatal("missing go result")
	}
	if want := `selected: changed file "cli/cmd/ao/main.go" matched "cli/**"`; goResult.SelectedReason != want {
		t.Errorf("go selected reason = %q, want %q", goResult.SelectedReason, want)
	}
	docsSkip, ok := skippedByID(rep, "docs")
	if !ok {
		t.Fatal("docs should be reported as route-skipped")
	}
	if !strings.Contains(docsSkip.Reason, "no changed file matched") || !strings.Contains(docsSkip.Reason, "docs/**") {
		t.Errorf("docs skip reason = %q, want route explanation with docs/**", docsSkip.Reason)
	}
	fullOnlySkip, ok := skippedByID(rep, "fullonly")
	if !ok {
		t.Fatal("fullonly should be reported as tier-skipped")
	}
	if !strings.Contains(fullOnlySkip.Reason, "do not include active mode fast") {
		t.Errorf("fullonly skip reason = %q, want tier explanation", fullOnlySkip.Reason)
	}
}

func TestGateOrchestrator_FastModeInvalidationRunsAllFast(t *testing.T) {
	o := testOrch(t, sampleRegistry(t), fakeFiles{files: []string{"go.mod"}}, sampleVerdicts)
	rep, err := o.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := ranIDs(rep)
	if !got["go"] || !got["docs"] || !got["always"] {
		t.Errorf("go.mod change should invalidate→run all fast checks; got %v", got)
	}
	if got["fullonly"] {
		t.Error("Full-only check still excluded from fast mode")
	}
	docsResult, ok := resultByID(rep, "docs")
	if !ok {
		t.Fatal("missing docs result after invalidation")
	}
	if !strings.Contains(docsResult.SelectedReason, `changed file "go.mod" invalidates fast routing`) {
		t.Errorf("docs selected reason = %q, want invalidation explanation", docsResult.SelectedReason)
	}
}

func TestReport_ExitCode_NonBlockingFailIsAdvisory(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(Check{ID: "lint", Tiers: Full, Blocking: false, Backing: "lint"}); err != nil {
		t.Fatal(err)
	}
	o := testOrch(t, r, fakeFiles{}, map[ports.GateName]ports.GateVerdict{
		"lint": {Status: ports.GateStatusFail, Reason: "style"},
	})
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0 (non-blocking FAIL is advisory)", rep.ExitCode())
	}
}

// TestReport_ExitCode_BlockingUnknownFailsClosed pins the audit-A1 fix: a
// blocking check that returns UNKNOWN — exactly what ScriptRunner emits when its
// backing script is missing or won't launch (scriptrunner.go:45,67) — must fail
// the run. Before the fix, isBlockingFail matched only FAIL, so a blocking gate
// that could not run silently passed: a fail-OPEN in the release authority
// ("no verdict = not done" violated). UNKNOWN on a blocking check is now
// fail-closed.
func TestReport_ExitCode_BlockingUnknownFailsClosed(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(Check{ID: "guard", Tiers: Full, Blocking: true, Backing: "missing-script"}); err != nil {
		t.Fatal(err)
	}
	o := testOrch(t, r, fakeFiles{}, map[ports.GateName]ports.GateVerdict{
		"missing-script": {Status: ports.GateStatusUnknown, Reason: "no script scripts/missing-script"},
	})
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 (blocking UNKNOWN must fail-closed — audit A1)", rep.ExitCode())
	}
}

// TestReport_ExitCode_NonBlockingUnknownIsAdvisory keeps the blast radius tight:
// only BLOCKING unknowns fail-close. A non-blocking check that can't run is still
// advisory, exactly like a non-blocking FAIL.
func TestReport_ExitCode_NonBlockingUnknownIsAdvisory(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(Check{ID: "advisory", Tiers: Full, Blocking: false, Backing: "missing-script"}); err != nil {
		t.Fatal(err)
	}
	o := testOrch(t, r, fakeFiles{}, map[ports.GateName]ports.GateVerdict{
		"missing-script": {Status: ports.GateStatusUnknown, Reason: "no script"},
	})
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0 (non-blocking UNKNOWN is advisory)", rep.ExitCode())
	}
}

// TestReport_ExitCode_BlockingSkipStaysAdvisory guards against over-correction:
// SKIP is a first-class "not applicable" verdict (exit 75), NOT a failure to
// produce one. A blocking check that legitimately SKIPs must still pass the run.
func TestReport_ExitCode_BlockingSkipStaysAdvisory(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(Check{ID: "skipper", Tiers: Full, Blocking: true, Backing: "skipper"}); err != nil {
		t.Fatal(err)
	}
	o := testOrch(t, r, fakeFiles{}, map[ports.GateName]ports.GateVerdict{
		"skipper": {Status: ports.GateStatusSkip, Reason: "exit 75 (structural skip)"},
	})
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0 (blocking SKIP is a legitimate not-applicable verdict)", rep.ExitCode())
	}
}

// TestReport_ExitCode_BlockingEvalErrorFailsClosed covers the native-check twin
// of A1: a blocking Run func that returns an error (could not evaluate at all)
// must fail the run, not silently pass on its zero-value verdict.
func TestReport_ExitCode_BlockingEvalErrorFailsClosed(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(Check{ID: "native", Tiers: Full, Blocking: true, Run: func(context.Context, RunContext) (ports.GateVerdict, error) {
		return ports.GateVerdict{}, errBlockingEval
	}}); err != nil {
		t.Fatal(err)
	}
	o := testOrch(t, r, fakeFiles{}, nil)
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 (blocking eval error must fail-closed)", rep.ExitCode())
	}
}

var errBlockingEval = errorString("native check could not evaluate")

type errorString string

func (e errorString) Error() string { return string(e) }

// TestGateOrchestrator_FailFastStopsAfterBlockingUnknown proves fail-fast also
// trips on a blocking UNKNOWN, not just a blocking FAIL.
func TestGateOrchestrator_FailFastStopsAfterBlockingUnknown(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"a", "b"} {
		if err := r.Add(Check{ID: id, Tiers: Full, Blocking: true, Backing: id}); err != nil {
			t.Fatal(err)
		}
	}
	o := testOrch(t, r, fakeFiles{}, map[ports.GateName]ports.GateVerdict{
		"a": {Status: ports.GateStatusUnknown}, "b": {Status: ports.GateStatusFail},
	})
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full, FailFast: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Results) != 1 {
		t.Errorf("FailFast should stop after the blocking UNKNOWN; ran %d", len(rep.Results))
	}
}

func TestGateOrchestrator_FailFastStopsAfterFirstBlockingFail(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"a", "b", "c"} {
		if err := r.Add(Check{ID: id, Tiers: Full, Blocking: true, Backing: id}); err != nil {
			t.Fatal(err)
		}
	}
	o := testOrch(t, r, fakeFiles{}, map[ports.GateName]ports.GateVerdict{
		"a": {Status: ports.GateStatusFail}, "b": {Status: ports.GateStatusFail}, "c": {Status: ports.GateStatusFail},
	})
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full, FailFast: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Results) != 1 {
		t.Errorf("FailFast should stop after 1 blocking FAIL; ran %d", len(rep.Results))
	}
	if bSkip, ok := skippedByID(rep, "b"); !ok {
		t.Error("FailFast should report later selected checks as skipped")
	} else if !strings.Contains(bSkip.Reason, "fail-fast stopped") {
		t.Errorf("b skip reason = %q, want fail-fast explanation", bSkip.Reason)
	}
}

func TestGateReport_JSONSchema(t *testing.T) {
	o := testOrch(t, sampleRegistry(t), fakeFiles{}, sampleVerdicts)
	rep, err := o.Run(context.Background(), RunOptions{Mode: Full})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw, err := rep.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var parsed jsonReport
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Run.Mode != "full" {
		t.Errorf("run.mode = %q, want full", parsed.Run.Mode)
	}
	if parsed.Run.Summary.Total != 4 {
		t.Errorf("summary.total = %d, want 4", parsed.Run.Summary.Total)
	}
	if parsed.Run.Summary.Failed != 1 || parsed.Run.Summary.Warned != 1 || parsed.Run.Summary.Passed != 2 {
		t.Errorf("summary = %+v, want 2 pass/1 warn/1 fail", parsed.Run.Summary)
	}
	if len(parsed.Gates) != 4 {
		t.Errorf("gates len = %d, want 4", len(parsed.Gates))
	}
	var goGate jsonGate
	for _, gate := range parsed.Gates {
		if gate.Name == "go" {
			goGate = gate
			break
		}
	}
	if goGate.Name == "" {
		t.Fatal("JSON gates missing go")
	}
	if goGate.SelectedReason == "" {
		t.Error("go selected_reason should be populated")
	}
	if goGate.WorkflowBacking != "bash scripts/go" {
		t.Errorf("go workflow_backing = %q, want bash scripts/go", goGate.WorkflowBacking)
	}
	if goGate.ArtifactPath != "scripts/go" {
		t.Errorf("go artifact_path = %q, want scripts/go", goGate.ArtifactPath)
	}
	if goGate.RepairHint != "bash scripts/go" {
		t.Errorf("go repair_hint = %q, want bash scripts/go", goGate.RepairHint)
	}
}
