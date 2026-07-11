package main

import "github.com/spf13/cobra"

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
