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
}

type CheckResult struct {
	Report                *gates.Report
	ExitCode              int
	WorkflowParityMissing int
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
