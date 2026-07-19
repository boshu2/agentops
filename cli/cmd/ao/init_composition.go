// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	initcommands "github.com/boshu2/agentops/cli/internal/commands/init"
)

func init() {
	rootCmd.AddCommand(newInitCommand())
}

// newInitCommand wires the init command module to its host seams. The dry-run
// selection comes from the global --dry-run flag; all filesystem effects are
// delegated to internal/initapp. The init family attaches no CommandContract to
// the command tree, preserving its pre-migration capabilities surface.
func newInitCommand() *cobra.Command {
	module := initcommands.NewModule(initcommands.HostOptions{
		DryRun: GetDryRun,
	})
	return module.Command()
}
