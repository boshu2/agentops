package main

import (
	"errors"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	beadscommands "github.com/boshu2/agentops/cli/internal/commands/beads"
	"github.com/spf13/cobra"
)

// White-box behavior tests keep private throwaway commands for stream and flag
// control. Production command ownership lives in internal/commands/beads.
var legacyBeadsDirCommand = &cobra.Command{Use: "dir", RunE: executeBeadsDir}
var legacyBeadsTrackerCommand = &cobra.Command{Use: "tracker", RunE: executeBeadsTracker}
var legacyBeadsExecCommand = &cobra.Command{
	Use:                "exec [args...]",
	DisableFlagParsing: true,
	RunE:               executeBeadsExec,
}

var legacyBeadsResumeCommand = func() *cobra.Command {
	command := &cobra.Command{Use: "resume <bead-id>", RunE: executeBeadsResume}
	command.Flags().StringVar(&beadsResumeAgentID, "agent", "", "")
	command.Flags().StringVar(&beadsResumeLedgerPath, "ledger", "docs/provenance/ledger.jsonl", "")
	command.Flags().BoolVar(&beadsResumeJSON, "json", false, "")
	return command
}()

var legacyBeadsStaleCommand = func() *cobra.Command {
	command := &cobra.Command{Use: "stale-claims", RunE: executeBeadsStale}
	command.Flags().Float64Var(&beadsStaleThresholdHours, "threshold", 4, "")
	command.Flags().BoolVar(&beadsStaleJSON, "json", false, "")
	return command
}()

var beadsScenariosExtractCmd = &cobra.Command{Use: "extract <bead-id>", RunE: executeBeadsScenariosExtract}
var beadsScenariosValidateCmd = &cobra.Command{Use: "validate <bead-id>", RunE: executeBeadsScenariosValidate}

func newLegacyBeadsVerifyAcceptanceCommand() *cobra.Command {
	var strict, asJSON bool
	command := &cobra.Command{
		Use:  "verify-acceptance <bead-id>...",
		Args: cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return executeBeadsVerifyAcceptance(command, args, strict, asJSON)
		},
	}
	command.Flags().BoolVar(&strict, "strict", false, "")
	command.Flags().BoolVar(&asJSON, "json", false, "")
	return command
}

func executeBeadsDir(command *cobra.Command, _ []string) error {
	child := newTestBeadsCommand("dir")
	child.SetOut(command.OutOrStdout())
	var flags []string
	if beadsDirJSON {
		flags = append(flags, "--json")
	}
	if beadsDirRequire {
		flags = append(flags, "--require")
	}
	if err := child.ParseFlags(flags); err != nil {
		return err
	}
	return child.RunE(child, nil)
}

func executeBeadsTracker(command *cobra.Command, _ []string) error {
	child := newTestBeadsCommand("tracker")
	child.SetOut(command.OutOrStdout())
	if beadsTrackerJSON {
		if err := child.ParseFlags([]string{"--json"}); err != nil {
			return err
		}
	}
	return child.RunE(child, nil)
}

func executeBeadsExec(command *cobra.Command, args []string) error {
	child := newTestBeadsCommand("exec")
	child.SetIn(command.InOrStdin())
	child.SetOut(command.OutOrStdout())
	child.SetErr(command.ErrOrStderr())
	err := child.RunE(child, args)
	var exitError *beadsapp.ExitError
	if errors.As(err, &exitError) {
		return &beadsVerdictError{code: exitError.ExitCode()}
	}
	return err
}

func newTestBeadsCommand(name string) *cobra.Command {
	tracker := currentBeadsTracker()
	runtime := beadsadapter.NewRuntime()
	root := beadscommands.NewModule(nil, tracker, tracker, beadsadapter.NewExecutor(tracker), tracker, tracker, runtime, runtime).Command()
	child, _, err := root.Find([]string{name})
	if err != nil || child == nil {
		panic("missing test beads command " + name)
	}
	return child
}
