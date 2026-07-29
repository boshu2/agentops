package gate

import (
	"context"
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type changedFilesStub struct {
	files []string
	err   error
}

func (stub changedFilesStub) Changed(context.Context, gates.Scope) ([]string, error) {
	return stub.files, stub.err
}

type checkRuntimeStub struct {
	scope    gates.Scope
	applyErr error
	coverage *gates.WorkflowCoverage
	coverErr error
}

func (runtime *checkRuntimeStub) ApplyRangeScope(scope gates.Scope) error {
	runtime.scope = scope
	return runtime.applyErr
}

func (runtime *checkRuntimeStub) WorkflowCoverage(*gates.Registry, string, string) (*gates.WorkflowCoverage, error) {
	return runtime.coverage, runtime.coverErr
}

func registryWithVerdict(t *testing.T, status ports.GateStatus, blocking bool) *gates.Registry {
	t.Helper()
	registry := gates.NewRegistry()
	if err := registry.Add(gates.Check{
		ID: "native", Tiers: gates.Fast | gates.Full, Blocking: blocking,
		Run: func(context.Context, gates.RunContext) (ports.GateVerdict, error) {
			return ports.GateVerdict{Status: status, Reason: "test"}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestCheckServiceRunsFullAndReturnsBlockingExit(t *testing.T) {
	runtime := &checkRuntimeStub{}
	result, err := (CheckService{
		Registry: registryWithVerdict(t, ports.GateStatusFail, true),
		Files:    changedFilesStub{},
		Runtime:  runtime,
	}).Execute(context.Background(), CheckRequest{Full: true, Scope: "head"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Report.Mode != gates.Full || result.ExitCode != 1 || runtime.scope != gates.ScopeHead {
		t.Fatalf("result=%+v scope=%q", result, runtime.scope)
	}
}

func TestCheckServiceRunsFastWithChangedFileRouting(t *testing.T) {
	result, err := (CheckService{
		Registry: registryWithVerdict(t, ports.GateStatusPass, true),
		Files:    changedFilesStub{files: []string{"cli/main.go"}},
		Runtime:  &checkRuntimeStub{},
	}).Execute(context.Background(), CheckRequest{Scope: "head"})
	if err != nil || result.Report.Mode != gates.Fast || result.Report.ChangedCount != 1 || result.ExitCode != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestCheckServiceWorkflowParityCanFailResult(t *testing.T) {
	runtime := &checkRuntimeStub{coverage: &gates.WorkflowCoverage{MissingBlockingCount: 2}}
	result, err := (CheckService{
		Registry: registryWithVerdict(t, ports.GateStatusPass, true),
		Files:    changedFilesStub{},
		Runtime:  runtime,
	}).Execute(context.Background(), CheckRequest{Full: true, WorkflowCoverage: true, RequireWorkflowParity: true, WorkflowPath: "validate.yml"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ExitCode != 1 || result.Report.Coverage != runtime.coverage || result.WorkflowParityMissing != 2 {
		t.Fatalf("result=%+v", result)
	}
}

// registryWithSideEffect registers a native check that records whether it ran
// (a stand-in for any real filesystem/process effect) and returns status. Under
// a dry-run plan the recorder must stay false.
func registryWithSideEffect(t *testing.T, ran *bool, status ports.GateStatus, blocking bool) *gates.Registry {
	t.Helper()
	registry := gates.NewRegistry()
	if err := registry.Add(gates.Check{
		ID: "native", Tiers: gates.Fast | gates.Full, Blocking: blocking,
		Run: func(context.Context, gates.RunContext) (ports.GateVerdict, error) {
			*ran = true
			return ports.GateVerdict{Status: status, Reason: "side effect fired"}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	return registry
}

// TestCheckServicePlanExecutesNothingAndExitsZero is witness (1)+(3): under
// --dry-run a check that would FAIL and fire a side effect does neither, the
// plan is returned with exit 0, and the same request WITHOUT --dry-run is
// unchanged (the check runs, the blocking FAIL maps to exit 1, no plan).
func TestCheckServicePlanExecutesNothingAndExitsZero(t *testing.T) {
	var ran bool
	service := CheckService{
		Registry: registryWithSideEffect(t, &ran, ports.GateStatusFail, true),
		Files:    changedFilesStub{files: []string{"cli/main.go"}},
		Runtime:  &checkRuntimeStub{},
	}

	planResult, err := service.Execute(context.Background(), CheckRequest{Full: true, Scope: "head", Plan: true})
	if err != nil {
		t.Fatalf("Execute(plan): %v", err)
	}
	if ran {
		t.Fatal("dry-run executed the check: the side effect fired")
	}
	if planResult.Plan == nil {
		t.Fatal("plan result carried no plan")
	}
	if planResult.Report != nil {
		t.Fatal("plan result must not carry a run report")
	}
	if planResult.ExitCode != 0 {
		t.Fatalf("plan exit = %d, want 0 even though the check would FAIL", planResult.ExitCode)
	}
	if len(planResult.Plan.Selected) != 1 || planResult.Plan.Selected[0].Name != "native" {
		t.Fatalf("plan selected = %+v, want the native check", planResult.Plan.Selected)
	}
	if !planResult.Plan.Selected[0].Blocking {
		t.Fatal("plan entry lost the blocking fact")
	}

	ran = false
	runResult, err := service.Execute(context.Background(), CheckRequest{Full: true, Scope: "head"})
	if err != nil {
		t.Fatalf("Execute(run): %v", err)
	}
	if !ran {
		t.Fatal("non-dry-run did not execute the check")
	}
	if runResult.Plan != nil {
		t.Fatal("non-dry-run must not carry a plan")
	}
	if runResult.Report == nil || runResult.ExitCode != 1 {
		t.Fatalf("non-dry-run result=%+v, want a report with exit 1", runResult)
	}
}

// TestCheckServicePlanSelectionDiffersByInvocation is witness (2): the plan
// lists the expected selection for two different invocations (default fast vs
// --full).
func TestCheckServicePlanSelectionDiffersByInvocation(t *testing.T) {
	registry := gates.NewRegistry()
	add := func(c gates.Check) {
		if err := registry.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	noop := func(context.Context, gates.RunContext) (ports.GateVerdict, error) {
		return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "ok"}, nil
	}
	add(gates.Check{ID: "always", Tiers: gates.Fast | gates.Full, Blocking: true, Run: noop})
	add(gates.Check{ID: "fullonly", Tiers: gates.Full, Blocking: true, Run: noop})

	service := CheckService{Registry: registry, Files: changedFilesStub{}, Runtime: &checkRuntimeStub{}}

	fast, err := service.Execute(context.Background(), CheckRequest{Scope: "head", Plan: true})
	if err != nil {
		t.Fatalf("Execute(fast plan): %v", err)
	}
	full, err := service.Execute(context.Background(), CheckRequest{Full: true, Plan: true})
	if err != nil {
		t.Fatalf("Execute(full plan): %v", err)
	}

	if got := selectedNames(fast.Plan); !equalStrings(got, []string{"always"}) {
		t.Fatalf("fast plan selected %v, want [always]", got)
	}
	if got := selectedNames(full.Plan); !equalStrings(got, []string{"always", "fullonly"}) {
		t.Fatalf("full plan selected %v, want [always fullonly]", got)
	}
	if got := skippedNames(fast.Plan); !containsName(got, "fullonly") {
		t.Fatalf("fast plan should skip fullonly (tier); skipped=%v", got)
	}
}

func selectedNames(p *gates.Plan) []string {
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(p.Selected))
	for _, c := range p.Selected {
		out = append(out, c.Name)
	}
	return out
}

func skippedNames(p *gates.Plan) []string {
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(p.Skipped))
	for _, c := range p.Skipped {
		out = append(out, c.Name)
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func TestCheckServiceReturnsRuntimeAndOrchestratorErrors(t *testing.T) {
	t.Run("range environment", func(t *testing.T) {
		_, err := (CheckService{Registry: gates.NewRegistry(), Runtime: &checkRuntimeStub{applyErr: errors.New("bad range")}}).Execute(context.Background(), CheckRequest{Scope: "range:HEAD"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("changed files", func(t *testing.T) {
		_, err := (CheckService{Registry: gates.NewRegistry(), Files: changedFilesStub{err: errors.New("git failed")}, Runtime: &checkRuntimeStub{}}).Execute(context.Background(), CheckRequest{Scope: "head"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
