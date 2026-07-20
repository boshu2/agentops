// practices: [hexagonal-architecture, twelve-factor-app]
package main

import (
	"github.com/spf13/cobra"

	configadapter "github.com/boshu2/agentops/cli/internal/adapters/config"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	configcommands "github.com/boshu2/agentops/cli/internal/commands/config"
	configapp "github.com/boshu2/agentops/cli/internal/config"
)

func init() {
	rootCmd.AddCommand(newConfigCommand())
}

// newConfigCommand wires the config command module to its host seams (output
// mode, verbose, dry-run) and attaches its CommandContract. Constructor-scoped
// like the gate composition: no package-level module or command singleton, so
// flag state lives inside internal/commands/config.
func newConfigCommand() *cobra.Command {
	module := configcommands.NewModule(
		configapp.NewCommandService(configadapter.Gateway{}),
		clicontract.HostOptions{OutputMode: GetOutput, Verbose: GetVerbose, DryRun: GetDryRun},
	)
	command := module.Command()
	command.GroupID = "config"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	return command
}
