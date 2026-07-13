// Package close owns Cobra presentation for the close command family.
package close

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

type UseCases interface {
	Execute(context.Context, closeapp.Request) (closeapp.Result, error)
}

type Module struct {
	useCases UseCases
}

func NewModule(useCases UseCases) Module {
	return Module{useCases: useCases}
}

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.close",
		Profiles: clicontract.ProfileDefault |
			clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy |
			clicontract.ProfileCombined,
		Args:    clicontract.ArgsPolicy{Name: "minimum-3", Validate: cobra.MinimumNArgs(3)},
		Output:  clicontract.OutputText,
		Effects: clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectTracker | clicontract.EffectEnvironment,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess, 1: clicontract.ExitFailure, closeapp.ExitRefused: clicontract.ExitConflict,
			closeapp.ExitPersistence: clicontract.ExitPartial, closeapp.ExitTracker: clicontract.ExitFailure,
		},
	}
}

func (module Module) Command() *cobra.Command {
	return module.command(closeapp.ModeEnsure)
}

func (module Module) LegacyCommand() *cobra.Command {
	return module.command(closeapp.ModeStrict)
}

func (module Module) command(mode closeapp.Mode) *cobra.Command {
	command := &cobra.Command{
		Use:   "close <id> <commit-message> <evidence-ref> [paths...]",
		Short: "Close a bead and persist the explicit ledger/evidence paths",
		Args:  cobra.MinimumNArgs(3),
	}
	command.RunE = func(command *cobra.Command, args []string) error {
		result, err := module.useCases.Execute(command.Context(), closeapp.Request{
			ID: args[0], Message: args[1], Evidence: args[2], Paths: append([]string(nil), args[3:]...), Mode: mode,
		})
		if err != nil {
			command.SilenceErrors = true
			command.SilenceUsage = true
			return renderFailure(command.ErrOrStderr(), err)
		}
		verb := "closed"
		if result.AlreadyClosed {
			verb = "already closed"
		}
		_, err = fmt.Fprintf(command.OutOrStdout(), "%s %s @ %s\n", verb, result.ID, closeapp.ShortRef(result.Ref))
		return err
	}
	return command
}

type exitError struct{ code int }

func (failure *exitError) Error() string { return "" }
func (failure *exitError) ExitCode() int { return failure.code }

func renderFailure(stderr io.Writer, err error) error {
	var failure *closeapp.Failure
	if errors.As(err, &failure) {
		if failure.Message != "" {
			fmt.Fprintln(stderr, failure.Message)
		}
		return &exitError{code: failure.Code}
	}
	fmt.Fprintln(stderr, err)
	return &exitError{code: 1}
}
