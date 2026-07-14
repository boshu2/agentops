// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"os"

	claimadapter "github.com/boshu2/agentops/cli/internal/adapters/claim"
	claimapp "github.com/boshu2/agentops/cli/internal/claim"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	claimcommands "github.com/boshu2/agentops/cli/internal/commands/claim"
)

func init() {
	service := claimapp.NewService(
		claimadapter.NewTrackerWith(
			os.Getwd,
			os.Environ,
			func(name string) (string, error) { return trackerLookPath(name) },
			func(code int, message string) error {
				return &tickExitError{code: code, msg: message}
			},
		),
		claimadapter.NewEvidenceStore(repoRootOrCwd),
		claimadapter.NewProof(repoRootOrCwd),
	)
	module := claimcommands.NewModule(service, GetOutput)
	command := module.Command()
	command.GroupID = "core"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(command)
}
