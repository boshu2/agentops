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
