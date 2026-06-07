package gates

import (
	"context"
	"encoding/json"
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

func TestOrchestrator_FastModeRoutesByChangedFiles(t *testing.T) {
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
}

func TestOrchestrator_FastModeInvalidationRunsAllFast(t *testing.T) {
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

func TestOrchestrator_FailFastStopsAfterFirstBlockingFail(t *testing.T) {
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
}

func TestReport_JSONSchema(t *testing.T) {
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
}
