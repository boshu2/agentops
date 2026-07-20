// Package quickstart owns Cobra presentation for the `ao quick-start` command.
// The command prints a static single-pass workflow summary, so it performs no
// filesystem, process, or clock effect.
package quickstart

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// Module owns Cobra presentation for the quick-start command.
type Module struct{}

// NewModule constructs the quick-start command module.
func NewModule() Module {
	return Module{}
}

// Contract declares quick-start's real behavior: it takes no positional args,
// emits a static text summary to stdout, is a pure computation (no filesystem,
// process, or clock effect), and exits 0 on success or 1 on an output-write
// failure.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.quick-start",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao quick-start` command.
func (Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "quick-start",
		Short: "Show the single-pass AgentOps workflow",
		Long: `AgentOps is a small semantic evidence layer around agent work.

Run the RPI skill for one pass:
  Plan -> Implement -> fresh Validate -> durable verdict -> report and stop

The CLI does not claim work, retry, manage Git, or deliver changes. Use
ao gate check for deterministic repository checks and ao provenance for
generic evidence inspection.`,
		Args:    cobra.NoArgs,
		GroupID: "start",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop")
			fmt.Fprintln(cmd.OutOrStdout(), "Deterministic checks: ao gate check")
			fmt.Fprintln(cmd.OutOrStdout(), "Semantic judgment: invoke the Validate skill from a fresh context")
		},
	}
}
