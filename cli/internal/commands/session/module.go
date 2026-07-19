// Package session owns Cobra presentation for the `ao session` evidence
// commands (bootstrap and rehydrate). The module builds its command tree with
// constructor-scoped flag state and delegates every filesystem effect to
// internal/sessionapp, so this package performs no direct effect. The optional
// `ao session handoff` writer is attached by the cmd/ao composition; it is a
// separate command that shares this parent.
package session

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/sessionapp"
)

// Module owns Cobra presentation for the session command family.
type Module struct{}

// NewModule constructs the session command module. The session evidence
// commands read no ambient CLI seams; the working directory is resolved inside
// internal/sessionapp.
func NewModule() Module {
	return Module{}
}

// Contract declares the session family's real behavior for the family
// architecture gate. Session reads local orientation files and the latest
// caller-authored handoff on the filesystem, emits text (JSON under each
// subcommand's --json flag), and exits 0 on success or 1 on a working-directory
// failure. The session family attached no capabilities contract before the
// carve-out, so the composition does not attach this one either.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.session",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao session` command with its bootstrap and rehydrate
// subcommands. The RunE closures delegate entirely to internal/sessionapp so
// this module performs no direct filesystem effect.
func (Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:     "session",
		Short:   "Inspect or export session evidence",
		GroupID: "workflow",
	}
	root.AddCommand(bootstrapCommand())
	root.AddCommand(rehydrateCommand())
	return root
}

// bootstrapCommand builds `ao session bootstrap` with a constructor-scoped
// --json flag.
func bootstrapCommand() *cobra.Command {
	var jsonOut bool
	command := &cobra.Command{
		Use:   "bootstrap",
		Short: "Report available local orientation files",
		Long: `Report local orientation files without starting runtimes, probing
trackers, selecting work, inspecting queues, or installing hooks.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sessionapp.Bootstrap(sessionapp.BootstrapOptions{
				JSON:   jsonOut,
				Stdout: cmd.OutOrStdout(),
			})
		},
	}
	command.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return command
}

// rehydrateCommand builds `ao session rehydrate` with a constructor-scoped
// --json flag.
func rehydrateCommand() *cobra.Command {
	var jsonOut bool
	command := &cobra.Command{
		Use:   "rehydrate",
		Short: "Read the latest caller-authored handoff",
		Long:  "Read a handoff without consuming it, claiming work, or choosing a next action.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return sessionapp.Rehydrate(sessionapp.RehydrateOptions{
				JSON:   jsonOut,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
		},
	}
	command.Flags().BoolVar(&jsonOut, "json", false, "Emit the stored artifact as JSON")
	return command
}
