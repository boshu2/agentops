// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	statuscommands "github.com/boshu2/agentops/cli/internal/commands/status"
)

func init() {
	rootCmd.AddCommand(newStatusCommand())
}

// newStatusCommand wires the status command module to its host seams. Status
// reads the global -o/--output flag for its output mode and delegates all
// filesystem and clock effects to internal/statusapp.
func newStatusCommand() *cobra.Command {
	module := statuscommands.NewModule(clicontract.HostOptions{
		OutputMode: GetOutput,
	})
	command := module.Command()
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	return command
}
