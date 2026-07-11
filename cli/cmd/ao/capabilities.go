// practices: [twelve-factor-app, ai-assisted-dev, pragmatic-programmer]
package main

import (
	capabilitiesadapter "github.com/boshu2/agentops/cli/internal/adapters/capabilities"
	capabilitiesapp "github.com/boshu2/agentops/cli/internal/capabilities"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	capabilitiescommands "github.com/boshu2/agentops/cli/internal/commands/capabilities"
)

func init() {
	service := capabilitiesapp.NewService(
		version,
		capabilitiesadapter.NewCobraSurface(rootCmd),
		capabilitiesadapter.RuntimePlatform{},
	)
	module := capabilitiescommands.NewModule(service, GetOutput)
	command := module.Command()
	command.GroupID = "core"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(command)
}
