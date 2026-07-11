package main

import (
	"fmt"

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
	case beadscommands.OperationDir:
		beadsDirJSON, beadsDirRequire = options.JSON, options.Require
		return executeBeadsDir(command, invocation.Args)
	case beadscommands.OperationTracker:
		beadsTrackerJSON = options.JSON
		return executeBeadsTracker(command, invocation.Args)
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
	case beadscommands.OperationExec:
		return executeBeadsExec(command, invocation.Args)
	case beadscommands.OperationResume:
		beadsResumeAgentID, beadsResumeLedgerPath, beadsResumeJSON = options.Agent, options.Ledger, options.JSON
		return executeBeadsResume(command, invocation.Args)
	case beadscommands.OperationScenariosExtract:
		beadsScenariosJSON, beadsScenariosForce, beadsScenariosWrite = options.JSON, options.Force, options.Write
		return executeBeadsScenariosExtract(command, invocation.Args)
	case beadscommands.OperationScenariosCheck:
		beadsScenariosValidateJSON = options.JSON
		return executeBeadsScenariosValidate(command, invocation.Args)
	case beadscommands.OperationStaleClaims:
		beadsStaleThresholdHours, beadsStaleJSON = options.ThresholdHours, options.JSON
		return executeBeadsStale(command, invocation.Args)
	case beadscommands.OperationEpicStatus:
		beadsEpicStatusTerminal, beadsEpicStatusJSON = options.Terminal, options.JSON
		return executeBeadsEpicStatus(command, invocation.Args)
	case beadscommands.OperationAcceptance:
		return executeBeadsVerifyAcceptance(command, invocation.Args, options.Strict, options.JSON)
	default:
		return fmt.Errorf("unsupported beads operation %q", invocation.Operation)
	}
}

func init() {
	module := beadscommands.NewModule(beadsModuleRunner{})
	command := module.Command()
	command.GroupID = "knowledge"
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(command)
}
