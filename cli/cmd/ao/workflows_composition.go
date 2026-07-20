// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	workflowscommands "github.com/boshu2/agentops/cli/internal/commands/workflows"
)

func init() {
	rootCmd.AddCommand(newWorkflowsCommand())
}

// newWorkflowsCommand wires the workflows command module to its host seams.
// The global --dry-run flag drives link/unlink; checkout resolution, target
// resolution, and the link/unlink filesystem sweeps are host effects delegated
// to internal/workflowsapp. Workflows are a Claude-only runtime adapter (the
// skills-codex doctrine, Claude-side), grouped under Knowledge next to skills.
// Like skills, the family attaches no capabilities contract.
func newWorkflowsCommand() *cobra.Command {
	module := workflowscommands.NewModule(clicontract.HostOptions{
		DryRun: GetDryRun,
	})
	return module.Command()
}
