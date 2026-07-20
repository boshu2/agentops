// practices: [hexagonal-architecture, sre, resilience-patterns]
package main

import (
	"github.com/spf13/cobra"

	doctoradapter "github.com/boshu2/agentops/cli/internal/adapters/doctor"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	doctorcommands "github.com/boshu2/agentops/cli/internal/commands/doctor"
	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

func init() {
	rootCmd.AddCommand(newDoctorCommand())
}

func newLegacyDoctorService() doctorapp.LegacyService {
	checks := doctoradapter.SystemLegacyChecks(version, resolveLedgerPath)
	return doctorapp.NewLegacyService(version, checks)
}

// newDoctorCommand wires the doctor command module: its read/mutation/
// maintenance services, the legacy check set, and the host seams (output mode,
// dry-run, and flag-error enrichment). Constructor-scoped like the gate
// composition — no package-level module, command, or service singleton.
//
// Doctor derives its JSON intent from OutputMode == "json"; the previous
// GlobalOptions.JSON seam was redundant because negotiateOutput forces
// output="json" whenever --json is set, so the two were always equal.
func newDoctorCommand() *cobra.Command {
	readService := doctorapp.NewReadService(version, doctoradapter.ReadRuntime{ToolVersion: version}, doctoradapter.ReadGateway{})
	mutationService := doctorapp.NewMutationService(doctoradapter.MutationRuntime{ToolVersion: version}, doctoradapter.MutationGateway{})
	maintenanceService := doctorapp.NewMaintenanceService(doctoradapter.MaintenanceRuntime{}, doctoradapter.MaintenanceGateway{})

	module := doctorcommands.NewModule(doctorcommands.UseCases{
		LegacyChecks:  newLegacyDoctorService().Checks,
		Read:          readService,
		Mutation:      mutationService,
		Maintenance:   maintenanceService,
		DetectorCount: func() int { return len(doctorapp.Detectors()) },
	}, clicontract.HostOptions{
		OutputMode:    GetOutput,
		DryRun:        GetDryRun,
		EnrichFlagErr: flagErrorWithSuggestion,
	})
	command := module.Command()
	command.GroupID = "core"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	return command
}
