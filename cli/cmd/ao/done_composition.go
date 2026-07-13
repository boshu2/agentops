package main

import (
	"context"

	doneadapter "github.com/boshu2/agentops/cli/internal/adapters/done"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	donecommands "github.com/boshu2/agentops/cli/internal/commands/done"
	doneapp "github.com/boshu2/agentops/cli/internal/done"
)

func newDoneService() doneapp.Service {
	tracker := doneadapter.Tracker{Run: func(ctx context.Context, args ...string) ([]byte, error) {
		return beadsTrackerCommandContext(ctx, args...).CombinedOutput()
	}}
	return doneapp.NewService(doneadapter.SystemRepository(), doneadapter.SystemLedger(resolveLedgerPath()), tracker)
}

var doneModule = donecommands.NewModule(newDoneService())
var doneCommand = doneModule.Command()

func init() {
	if err := clicontract.Attach(doneCommand, doneModule.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(doneCommand)
}
