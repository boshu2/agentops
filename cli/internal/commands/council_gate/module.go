// Package council_gate owns Cobra presentation for council-gate.
package council_gate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/councilgate"
)

type UseCases interface {
	Evaluate(context.Context, councilgate.Request) councilgate.Result
}

type Module struct {
	useCases UseCases
}

func NewModule(useCases UseCases) Module { return Module{useCases: useCases} }

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.council-gate",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:    clicontract.ArgsPolicy{Name: "minimum-2", Validate: cobra.MinimumNArgs(2)},
		Output:  clicontract.OutputText,
		Effects: clicontract.EffectFilesystem,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess, 1: clicontract.ExitFailure,
			councilgate.ExitCouncil: clicontract.ExitConflict, councilgate.ExitDisagree: clicontract.ExitConflict,
		},
	}
}

func (module Module) Command() *cobra.Command       { return module.command() }
func (module Module) LegacyCommand() *cobra.Command { return module.command() }

func (module Module) command() *cobra.Command {
	command := &cobra.Command{
		Use:   "council-gate <verdict1> <verdict2> [...]",
		Short: "Fail-closed two-plus judge verdict aggregation",
		Args:  cobra.MinimumNArgs(2),
	}
	command.RunE = func(command *cobra.Command, paths []string) error {
		result := module.useCases.Evaluate(command.Context(), councilgate.Request{
			Paths: append([]string(nil), paths...), Stdin: command.InOrStdin(),
		})
		if result.Outcome == councilgate.OutcomePass {
			_, err := fmt.Fprintf(command.OutOrStdout(), "COUNCIL PASS: %d/%d judges unanimous across %d distinct contexts (%d model families)\n", result.Pass, result.Total, result.Contexts, result.Families)
			return err
		}
		command.SilenceErrors = true
		command.SilenceUsage = true
		return renderFailure(command, result)
	}
	return command
}

type exitError struct{ code int }

func (failure *exitError) Error() string { return "" }
func (failure *exitError) ExitCode() int { return failure.code }

func renderFailure(command *cobra.Command, result councilgate.Result) error {
	writer := command.ErrOrStderr()
	switch result.Outcome {
	case councilgate.OutcomeUnverified:
		fmt.Fprintf(writer, "FAIL-CLOSED: %d/%d verdict(s) unverified (no COMMANDS RUN / identity gap)\n", result.Unverified, result.Total)
		return &exitError{code: councilgate.ExitCouncil}
	case councilgate.OutcomeDuplicateContext:
		if result.Duplicate != "" {
			fmt.Fprintf(writer, "FAIL-CLOSED: duplicate judge context %q does not count as an independent judge\n", result.Duplicate)
		} else {
			fmt.Fprintf(writer, "FAIL-CLOSED: PASS quorum has %d distinct judge context; need at least 2 independent contexts\n", result.Contexts)
		}
		return &exitError{code: councilgate.ExitCouncil}
	case councilgate.OutcomeDuplicateJudge:
		fmt.Fprintf(writer, "FAIL-CLOSED: duplicate judge %q does not count as an independent judge\n", result.Duplicate)
		return &exitError{code: councilgate.ExitCouncil}
	case councilgate.OutcomeCrossFamily:
		fmt.Fprintf(writer, "FAIL-CLOSED: --require-cross-family set but PASS quorum spans %d model family; need at least 2 (cross-family)\n", result.Families)
		return &exitError{code: councilgate.ExitCouncil}
	case councilgate.OutcomeAllFail:
		fmt.Fprintf(writer, "FAIL-CLOSED: %d/%d judges FAIL\n", result.Fail, result.Total)
		return &exitError{code: councilgate.ExitCouncil}
	default:
		fmt.Fprintf(writer, "DISAGREEMENT: %d PASS / %d FAIL - fail-closed; dispatch tie-break\n", result.Pass, result.Fail)
		return &exitError{code: councilgate.ExitDisagree}
	}
}
