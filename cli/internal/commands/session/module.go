// Package session owns Cobra presentation for the `ao session` evidence
// commands (bootstrap and rehydrate). The module builds its command tree with
// constructor-scoped flag state and delegates every filesystem effect to
// internal/sessionapp, so this package performs no direct effect. The optional
// `ao session handoff` writer is attached by the cmd/ao composition; it is a
// separate command that shares this parent.
package session

import (
	"bytes"
	"io"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/sessionapp"
)

// Module owns Cobra presentation for the session command family.
type Module struct {
	host clicontract.HostOptions
}

// NewModule constructs the session command module. The only ambient seam it
// reads is the negotiated output mode (so the global -o json/-o yaml flags emit
// structured output, not a silent human-table fallback); the working directory
// is resolved inside internal/sessionapp.
func NewModule(host clicontract.HostOptions) Module {
	return Module{host: host}
}

// outputMode returns the negotiated global output mode, or "" when unset.
func (m Module) outputMode() string {
	if m.host.OutputMode == nil {
		return ""
	}
	return m.host.OutputMode()
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
func (m Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:     "session",
		Short:   "Inspect or export session evidence",
		GroupID: "workflow",
	}
	root.AddCommand(m.bootstrapCommand())
	root.AddCommand(m.rehydrateCommand())
	return root
}

// emitStructured runs a sessionapp entry point and honors the negotiated output
// mode. The local --json flag and global -o json both take the JSON path
// directly; -o yaml captures the app's single JSON document and re-emits it as
// YAML (the app emits exactly one JSON document in JSON mode), so `-o yaml` is
// the same data yaml-marshaled rather than a silent human-table fallback.
func (m Module) emitStructured(cmd *cobra.Command, localJSON bool, run func(w io.Writer, asJSON bool) error) error {
	if m.outputMode() == "yaml" {
		var buf bytes.Buffer
		if err := run(&buf, true); err != nil {
			return err
		}
		return clicontract.JSONToYAML(cmd.OutOrStdout(), buf.Bytes())
	}
	return run(cmd.OutOrStdout(), localJSON || m.outputMode() == "json")
}

// bootstrapCommand builds `ao session bootstrap` with a constructor-scoped
// --json flag.
func (m Module) bootstrapCommand() *cobra.Command {
	var jsonOut bool
	command := &cobra.Command{
		Use:   "bootstrap",
		Short: "Report available local orientation files",
		Long: `Report local orientation files without starting runtimes, probing
trackers, selecting work, inspecting queues, or installing hooks.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return m.emitStructured(cmd, jsonOut, func(w io.Writer, asJSON bool) error {
				return sessionapp.Bootstrap(sessionapp.BootstrapOptions{JSON: asJSON, Stdout: w})
			})
		},
	}
	command.Flags().BoolVar(&jsonOut, "json", false, "Emit JSON")
	return command
}

// rehydrateCommand builds `ao session rehydrate` with a constructor-scoped
// --json flag.
func (m Module) rehydrateCommand() *cobra.Command {
	var jsonOut bool
	command := &cobra.Command{
		Use:   "rehydrate",
		Short: "Read the latest caller-authored handoff",
		Long:  "Read a handoff without consuming it, claiming work, or choosing a next action.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return m.emitStructured(cmd, jsonOut, func(w io.Writer, asJSON bool) error {
				return sessionapp.Rehydrate(sessionapp.RehydrateOptions{JSON: asJSON, Stdout: w, Stderr: cmd.ErrOrStderr()})
			})
		},
	}
	command.Flags().BoolVar(&jsonOut, "json", false, "Emit the stored artifact as JSON")
	return command
}
