// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	gateadapter "github.com/boshu2/agentops/cli/internal/adapters/gate"
	gatecommands "github.com/boshu2/agentops/cli/internal/commands/gate"
	gateapp "github.com/boshu2/agentops/cli/internal/gate"
	"github.com/boshu2/agentops/cli/internal/gates"
	_ "github.com/boshu2/agentops/cli/internal/gates/checks"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func init() {
	rootCmd.AddCommand(newGateCommand())
}

func newGateCommand() *cobra.Command {
	module := gatecommands.NewModule(gatecommands.UseCases{
		Review: gateReviewUseCases{},
		Run:    gateRunUseCases{},
		Check:  gateCheckUseCases{},
	}, gatecommands.HostOptions{
		DryRun:       GetDryRun,
		OutputFormat: GetOutput,
	})
	return module.Command()
}

type gateReviewUseCases struct{}

func (gateReviewUseCases) Pending(ctx context.Context, request gateapp.PendingRequest) (gateapp.PendingResult, error) {
	service, err := gateReviewService()
	if err != nil {
		return gateapp.PendingResult{}, err
	}
	return service.Pending(ctx, request)
}

func (gateReviewUseCases) Approve(ctx context.Context, input gateapp.ApproveInput) (gateapp.ApproveResult, error) {
	service, err := gateReviewService()
	if err != nil {
		return gateapp.ApproveResult{}, err
	}
	return service.Approve(ctx, input)
}

func (gateReviewUseCases) Reject(ctx context.Context, input gateapp.RejectInput) (gateapp.RejectResult, error) {
	service, err := gateReviewService()
	if err != nil {
		return gateapp.RejectResult{}, err
	}
	return service.Reject(ctx, input)
}

func (gateReviewUseCases) BulkApprove(ctx context.Context, input gateapp.BulkApproveInput) (gateapp.BulkApproveResult, error) {
	service, err := gateReviewService()
	if err != nil {
		return gateapp.BulkApproveResult{}, err
	}
	return service.BulkApprove(ctx, input)
}

type gateRunUseCases struct{}

func (gateRunUseCases) Execute(ctx context.Context, request gateapp.RunRequest) (ports.GateVerdict, error) {
	root, err := resolveProjectDir()
	if err != nil {
		return ports.GateVerdict{}, fmt.Errorf("gate run: resolve project: %w", err)
	}
	return (gateapp.RunService{Runner: gateadapter.NewRunner(root)}).Execute(ctx, request)
}

type gateCheckUseCases struct{}

func (gateCheckUseCases) Execute(ctx context.Context, request gateapp.CheckRequest) (gateapp.CheckResult, error) {
	service, err := gateCheckService()
	if err != nil {
		return gateapp.CheckResult{}, err
	}
	return service.Execute(ctx, request)
}

func gateReviewService() (gateapp.ReviewService, error) {
	root, err := resolveProjectDir()
	if err != nil {
		return gateapp.ReviewService{}, fmt.Errorf("gate: resolve project: %w", err)
	}
	return gateapp.ReviewService{Port: gateadapter.NewReviewPool(root, GetCurrentUser)}, nil
}

func gateCheckService() (gateapp.CheckService, error) {
	start, err := resolveProjectDir()
	if err != nil {
		return gateapp.CheckService{}, fmt.Errorf("gate check: resolve project: %w", err)
	}
	root, err := gateadapter.ResolveRepoRoot(start)
	if err != nil {
		return gateapp.CheckService{}, err
	}
	return gateapp.CheckService{
		Registry: gates.Default,
		Runner:   gates.NewScriptRunner(root),
		Files:    gates.NewGitChangedFiles(root),
		RepoRoot: root,
		Runtime:  gateadapter.CheckRuntime{},
	}, nil
}
