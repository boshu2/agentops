// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	gccommands "github.com/boshu2/agentops/cli/internal/commands/gc"
)

func init() {
	rootCmd.AddCommand(newGCCommand())
}

// newGCCommand wires the Gas City maintainer command module to its host seams.
// The filesystem and gc-subprocess effects are delegated to
// internal/gcmaintainer by the module; no capabilities contract is attached,
// matching the skills composition.
func newGCCommand() *cobra.Command {
	module := gccommands.NewModule(clicontract.HostOptions{
		DryRun:     GetDryRun,
		OutputMode: GetOutput,
	})
	return module.Command()
}
