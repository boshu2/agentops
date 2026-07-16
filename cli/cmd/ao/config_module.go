// practices: [hexagonal-architecture, twelve-factor-app]
package main

import (
	configadapter "github.com/boshu2/agentops/cli/internal/adapters/config"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	configcommands "github.com/boshu2/agentops/cli/internal/commands/config"
	configapp "github.com/boshu2/agentops/cli/internal/config"
)

var configModule = configcommands.NewModule(configapp.NewCommandService(configadapter.Gateway{}), GetOutput, GetVerbose, GetDryRun)
var configCommand = configModule.Command()

func init() {
	configCommand.GroupID = "config"
	if err := clicontract.Attach(configCommand, configModule.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(configCommand)
}
