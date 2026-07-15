// practices: [hexagonal-architecture, sre, resilience-patterns]
package main

import (
	"github.com/spf13/cobra"

	doctoradapter "github.com/boshu2/agentops/cli/internal/adapters/doctor"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	doctorcommands "github.com/boshu2/agentops/cli/internal/commands/doctor"
	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

func newLegacyDoctorService() doctorapp.LegacyService {
	checks := doctoradapter.SystemLegacyChecks(version, resolveLedgerPath)
	return doctorapp.NewLegacyService(version, checks)
}

var doctorReadService = doctorapp.NewReadService(version, doctoradapter.ReadRuntime{ToolVersion: version}, doctoradapter.ReadGateway{})
var doctorMutationService = doctorapp.NewMutationService(doctoradapter.MutationRuntime{ToolVersion: version}, doctoradapter.MutationGateway{})
var doctorMaintenanceService = doctorapp.NewMaintenanceService(doctoradapter.MaintenanceRuntime{}, doctoradapter.MaintenanceGateway{})

var doctorModule = doctorcommands.NewModule(doctorcommands.UseCases{
	LegacyChecks:  newLegacyDoctorService().Checks,
	Read:          doctorReadService,
	Mutation:      doctorMutationService,
	Maintenance:   doctorMaintenanceService,
	DetectorCount: func() int { return len(doctorapp.Detectors()) },
}, doctorcommands.HostOptions{
	Globals: func(command *cobra.Command) doctorcommands.GlobalOptions {
		app := AppFromContext(command.Context())
		return doctorcommands.GlobalOptions{DryRun: app.DryRun, JSON: app.JSON, Output: app.Output}
	},
	EnrichFlagErr: flagErrorWithSuggestion,
})

var doctorCommand = doctorModule.Command()

func init() {
	doctorCommand.GroupID = "core"
	if err := clicontract.Attach(doctorCommand, doctorModule.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(doctorCommand)
}
