package gate

import (
	"context"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type CheckRuntime interface {
	ApplyRangeScope(gates.Scope) error
	WorkflowCoverage(*gates.Registry, string, string) (*gates.WorkflowCoverage, error)
}

type CheckService struct {
	Registry *gates.Registry
	Runner   ports.GateRunnerPort
	Files    gates.ChangedFilesPort
	RepoRoot string
	Runtime  CheckRuntime
}

type CheckRequest struct {
	Full                  bool
	Scope                 string
	FailFast              bool
	WorkflowCoverage      bool
	RequireWorkflowParity bool
	WorkflowPath          string
	// Plan requests a dry-run projection: resolve the checks that would run for
	// this invocation and return them in CheckResult.Plan WITHOUT executing any
	// check. Scope, tier, and changed-file routing are resolved exactly as a real
	// run resolves them, so the plan can never drift from what a run would select.
	Plan bool
}

type CheckResult struct {
	Report                *gates.Report
	ExitCode              int
	WorkflowParityMissing int
	// Plan is populated (and Report is nil) when the request set Plan: it is the
	// dry-run projection of the run. A plan always carries ExitCode 0 — a
	// successful plan is success regardless of what the planned checks would
	// return.
	Plan *gates.Plan
}

func (service CheckService) Execute(ctx context.Context, request CheckRequest) (CheckResult, error) {
	if service.Registry == nil {
		return CheckResult{}, fmt.Errorf("gate check: registry required")
	}
	if service.Runtime == nil {
		return CheckResult{}, fmt.Errorf("gate check: runtime required")
	}
	scope := gates.Scope(request.Scope)
	if scope == "" {
		scope = gates.ScopeHead
	}
	if err := service.Runtime.ApplyRangeScope(scope); err != nil {
		return CheckResult{}, fmt.Errorf("gate check: %w", err)
	}
	mode := gates.Fast
	if request.Full {
		mode = gates.Full
	}
	orchestrator := gates.NewOrchestrator(service.Registry, service.Runner, service.Files, service.RepoRoot)
	if request.Plan {
		// Dry-run: project the selection WITHOUT executing any check. Plan reuses
		// the same tier filter and changed-file routing as Run, so it never runs a
		// script runner and exits 0 on a successful plan.
		plan, planErr := orchestrator.Plan(ctx, gates.RunOptions{Mode: mode, Scope: scope, FailFast: request.FailFast})
		if planErr != nil {
			return CheckResult{}, fmt.Errorf("gate check: %w", planErr)
		}
		return CheckResult{Plan: plan, ExitCode: 0}, nil
	}
	report, err := orchestrator.Run(ctx, gates.RunOptions{Mode: mode, Scope: scope, FailFast: request.FailFast})
	if err != nil {
		return CheckResult{}, fmt.Errorf("gate check: %w", err)
	}
	result := CheckResult{Report: report, ExitCode: report.ExitCode()}
	if request.WorkflowCoverage || request.RequireWorkflowParity {
		coverage, coverageErr := service.Runtime.WorkflowCoverage(service.Registry, service.RepoRoot, request.WorkflowPath)
		if coverageErr != nil {
			return CheckResult{}, fmt.Errorf("gate check: workflow coverage: %w", coverageErr)
		}
		report.Coverage = coverage
		if request.RequireWorkflowParity && coverage.MissingBlockingCount > 0 {
			result.ExitCode = 1
			result.WorkflowParityMissing = coverage.MissingBlockingCount
		}
	}
	return result, nil
}
