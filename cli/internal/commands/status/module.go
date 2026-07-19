// Package status owns Cobra presentation for the `ao status` command. The
// module builds its command with host-provided seams and delegates every
// filesystem and clock effect to internal/statusapp.
package status

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/statusapp"
)

// HostOptions carries the ambient CLI seams the status command reads. Output
// mode comes from the global -o/--output flag rather than a local flag so
// status honors the same output selection as the rest of the CLI.
type HostOptions struct {
	OutputMode func() string
}

// Module owns Cobra presentation for the status command family.
type Module struct {
	host HostOptions
}

// NewModule constructs the status command module from its host seams.
func NewModule(host HostOptions) Module {
	return Module{host: host}
}

// Contract declares status's real behavior: it accepts (and ignores) arbitrary
// positional args exactly as Cobra does today, emits text (JSON under -o json),
// reads the durable evidence stores on the filesystem and stamps recency from
// the clock, and exits 0 on success or 1 on a working-directory failure.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.status",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem | clicontract.EffectClock,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao status` command. The RunE closure delegates entirely
// to statusapp so this module performs no direct effect.
func (module Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show durable AgentOps loop evidence",
		Long: `Display the content-addressed intent and verdict evidence stored by AgentOps.

The command validates artifact names, content identity, and verdict.v2 shape
before counting evidence. It reports recency only; it does not infer an active
runtime phase, elapsed execution time, tool activity, retries, or remaining work.

Examples:
  ao status
  ao status --json`,
		GroupID: "core",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return statusapp.Run(statusapp.RunOptions{
				JSON:   module.host.OutputMode() == "json",
				Stdout: cmd.OutOrStdout(),
			})
		},
	}
}
