package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
	beadscommands "github.com/boshu2/agentops/cli/internal/commands/beads"
	"github.com/spf13/cobra"
)

type beadsVerdictError struct{ code int }

func (err *beadsVerdictError) Error() string { return "" }
func (err *beadsVerdictError) ExitCode() int { return err.code }

var beadsTrackerOutput = func(args ...string) ([]byte, error) {
	return currentBeadsTracker().Output(context.Background(), args...)
}
var beadsTrackerAvailable = func() bool { return currentBeadsTracker().Available() }

func beadMinInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

type acceptBead = beadsapp.AcceptanceBead
type acceptanceVerdict = beadsapp.AcceptanceVerdict

const (
	acPass      = beadsapp.AcceptancePass
	acFail      = beadsapp.AcceptanceFail
	acUndefined = beadsapp.AcceptanceUndefined
)

var parseBeadsFromBRJSON = beadsapp.ParseBeadsFromBRJSON
var checkAcceptanceContract = beadsapp.CheckAcceptanceContract
var validateAcceptanceCriteriaContent = beadsapp.ValidateAcceptanceCriteriaContent

var execBR = func(...string) ([]byte, error) { return nil, errors.New("execBR test seam is not configured") }

type acceptanceRepositoryFunc func([]string) ([]byte, error)

func (function acceptanceRepositoryFunc) ShowAcceptance(ids []string) ([]byte, error) {
	return function(ids)
}

func executeBeadsVerifyAcceptance(command *cobra.Command, ids []string, strict, asJSON bool) error {
	service := beadsapp.AcceptanceService{Repository: acceptanceRepositoryFunc(func(ids []string) ([]byte, error) {
		return execBR(append([]string{"show", "--format", "json"}, ids...)...)
	})}
	results, nonPass, err := service.VerifyAcceptance(ids)
	if err != nil {
		return err
	}
	if asJSON {
		encoder := json.NewEncoder(command.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(results); err != nil {
			return err
		}
	} else {
		for _, result := range results {
			fmt.Fprintf(command.OutOrStdout(), "%s [%s] %s\n", result.Verdict, result.IssueType, result.BeadID)
			for _, missing := range result.Missing {
				fmt.Fprintf(command.OutOrStdout(), "    missing: %s\n", missing)
			}
		}
	}
	if strict && nonPass {
		return &beadsVerdictError{code: 1}
	}
	return nil
}

func asBeadsExit(err error, target **beadsVerdictError) bool { return errors.As(err, target) }

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
	directory := beadsapp.DirectoryService{Resolver: tracker, Inspector: tracker}
	recovery := beadsapp.RecoveryService{StaleSource: tracker, Claims: tracker, Runtime: runtime, Resolver: tracker, Reader: runtime}
	root := beadscommands.NewModule(tracker, beadsadapter.NewExecutor(tracker), directory, recovery, nil, nil, nil, nil).Command()
	child, _, err := root.Find([]string{name})
	if err != nil || child == nil {
		panic("missing test beads command " + name)
	}
	return child
}
