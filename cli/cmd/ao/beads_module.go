package main

import (
	"fmt"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	beadscommands "github.com/boshu2/agentops/cli/internal/commands/beads"
	"github.com/spf13/cobra"
)

// beadsModuleRunner is the composition seam between the explicit driving
// adapter and the existing family use cases. The module owns Cobra parsing;
// this bridge transfers typed options while the remaining use cases move out
// of cmd/ao behind their application ports.
type beadsModuleRunner struct{}

func (beadsModuleRunner) Run(command *cobra.Command, invocation beadscommands.Invocation) error {
	options := invocation.Options
	switch invocation.Operation {
	case beadscommands.OperationVerify:
		beadsVerifyJSON, beadsVerifyVerbose = options.JSON, options.Verbose
		return executeBeadsVerify(command, invocation.Args)
	case beadscommands.OperationLint:
		beadsLintStatus, beadsLintJSON = options.Status, options.JSON
		return executeBeadsLint(command, invocation.Args)
	case beadscommands.OperationHarvest:
		beadsHarvestOutDir, beadsHarvestDryRun = options.OutputDirectory, options.DryRun
		return executeBeadsHarvest(command, invocation.Args)
	case beadscommands.OperationAudit:
		beadsAuditJSON, beadsAuditStrict, beadsAuditAutoClose = options.JSON, options.Strict, options.AutoClose
		return executeBeadsAudit(command, invocation.Args)
	case beadscommands.OperationCluster:
		beadsClusterJSON, beadsClusterApply = options.JSON, options.Apply
		return executeBeadsCluster(command, invocation.Args)
	case beadscommands.OperationScenariosExtract:
		beadsScenariosJSON, beadsScenariosForce, beadsScenariosWrite = options.JSON, options.Force, options.Write
		return executeBeadsScenariosExtract(command, invocation.Args)
	case beadscommands.OperationScenariosCheck:
		beadsScenariosValidateJSON = options.JSON
		return executeBeadsScenariosValidate(command, invocation.Args)
	case beadscommands.OperationAcceptance:
		return executeBeadsVerifyAcceptance(command, invocation.Args, options.Strict, options.JSON)
	default:
		return fmt.Errorf("unsupported beads operation %q", invocation.Operation)
	}
}

func init() {
	tracker := currentBeadsTracker()
	runtime := beadsadapter.NewRuntime()
	module := beadscommands.NewModule(
		beadsModuleRunner{}, tracker, tracker, beadsadapter.NewExecutor(tracker),
		tracker, tracker, runtime, runtime,
	)
	command := module.Command()
	command.GroupID = "knowledge"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(command)
}
