// Package redact owns Cobra presentation for the `ao redact` command: the
// shell-callable git-safety chokepoint (ag-sz3h). The command is near-pure
// delegation to internal/redact — it reads stdin, applies the canonical secret
// redactor, and writes the scrubbed bytes to stdout — so it performs no
// filesystem, network, or persistent state effect. Unlike most carved
// families, redact carries a real CommandContract that the composition attaches
// to the command tree, preserving its capabilities surface.
package redact

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	redactlib "github.com/boshu2/agentops/cli/internal/redact"
)

// Module owns Cobra presentation for the redact command.
type Module struct{}

// NewModule constructs the redact command module.
func NewModule() Module {
	return Module{}
}

// Contract declares redact's real behavior: it takes no positional args, emits
// the scrubbed text to stdout, is a pure deterministic transform of stdin (no
// filesystem, network, or state), and exits 0 on success or 1 on an I/O failure
// reading stdin or writing stdout. The composition attaches this contract to
// the command tree, preserving redact's declared capabilities surface exactly.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.redact",
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

// Command builds the `ao redact` command. Shell callers (notably the compile
// render-write in skills/compile/scripts/compile.sh) cannot invoke
// redact.Redact directly, so they pipe content through `ao redact`.
func (Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "redact",
		Short: "Scrub secrets from stdin to stdout (canonical redactor)",
		Long: `Read text on stdin, apply the canonical secret redactor (the same
patterns as the in-process corpus/compile scrub), and write the scrubbed text to
stdout. The git-safety bridge for shell callers that cannot call the Go redactor
directly. Non-sensitive content passes through unchanged.`,
		Args:    cobra.NoArgs,
		GroupID: "core",
		RunE:    run,
	}
}

func run(cmd *cobra.Command, _ []string) error {
	in, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("ao redact: read stdin: %w", err)
	}
	if _, err := cmd.OutOrStdout().Write(redactlib.RedactBytes(in)); err != nil {
		return fmt.Errorf("ao redact: write stdout: %w", err)
	}
	return nil
}
