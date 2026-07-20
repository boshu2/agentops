// practices: [dora-metrics, sre]
package main

import (
	"github.com/spf13/cobra"
	"github.com/boshu2/agentops/cli/internal/clicontract"

	flywheelcommands "github.com/boshu2/agentops/cli/internal/commands/flywheel"
)

func init() {
	rootCmd.AddCommand(newFlywheelCommand())
}

// newFlywheelCommand wires the flywheel command module to its host seams: the
// global -o/--output mode and the verbose diagnostic printer. Unlike redact and
// version, flywheel carries no attached CommandContract; this composition does
// not attach one, preserving flywheel's synthesized capabilities surface.
func newFlywheelCommand() *cobra.Command {
	module := flywheelcommands.NewModule(clicontract.HostOptions{
		OutputMode: GetOutput,
		Verbosef:   VerbosePrintf,
	})
	return module.Command()
}
