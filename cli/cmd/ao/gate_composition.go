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
)

func init() {
	rootCmd.AddCommand(newGateCommand())
}

func newGateCommand() *cobra.Command {
	module := gatecommands.NewModule(gatecommands.UseCases{
		Check: gateCheckUseCases{},
	}, gatecommands.HostOptions{
		DryRun:       GetDryRun,
		OutputFormat: GetOutput,
	})
	return module.Command()
}

type gateCheckUseCases struct{}

func (gateCheckUseCases) Execute(ctx context.Context, request gateapp.CheckRequest) (gateapp.CheckResult, error) {
	service, err := gateCheckService()
	if err != nil {
		return gateapp.CheckResult{}, err
	}
	return service.Execute(ctx, request)
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
