// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	closeadapter "github.com/boshu2/agentops/cli/internal/adapters/close"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	closeapp "github.com/boshu2/agentops/cli/internal/close"
	closecommands "github.com/boshu2/agentops/cli/internal/commands/close"
)

func newCloseService(runtime closeapp.Runtime) *closeapp.Service {
	return closeapp.NewService(runtime, closeadapter.Tracker{}, closeadapter.Repository{})
}

var closeModule = closecommands.NewModule(newCloseService(closeadapter.SystemRuntime{}))

func init() {
	command := closeModule.Command()
	if err := clicontract.Attach(command, closeModule.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(command)
}
