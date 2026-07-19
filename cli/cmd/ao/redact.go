// practices: [supply-chain-integrity]
package main

import (
	"fmt"
	"io"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/redact"
	"github.com/spf13/cobra"
)

// redactCmd is the shell-callable git-safety chokepoint (ag-sz3h). Shell
// callers (notably the compile render-write in skills/compile/scripts/compile.sh)
// cannot invoke redact.Redact directly, so they pipe content through `ao redact`:
// it reads stdin, applies the canonical secret redactor, and writes the
// scrubbed bytes to stdout. Single source of truth for credential patterns.
var redactCmd = &cobra.Command{
	Use:   "redact",
	Short: "Scrub secrets from stdin to stdout (canonical redactor)",
	Long: `Read text on stdin, apply the canonical secret redactor (the same
patterns as the in-process corpus/compile scrub), and write the scrubbed text to
stdout. The git-safety bridge for shell callers that cannot call the Go redactor
directly. Non-sensitive content passes through unchanged.`,
	Args: cobra.NoArgs,
	RunE: runRedact,
}

// redactContract declares redact's real behavior: it takes no positional args,
// emits the scrubbed text to stdout, is a pure deterministic transform of stdin
// (no filesystem, network, or state), and exits 0 on success or 1 on an I/O
// failure reading stdin or writing stdout.
func redactContract() clicontract.CommandContract {
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

func init() {
	redactCmd.GroupID = "core"
	if err := clicontract.Attach(redactCmd, redactContract()); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(redactCmd)
}

func runRedact(cmd *cobra.Command, _ []string) error {
	in, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("ao redact: read stdin: %w", err)
	}
	if _, err := cmd.OutOrStdout().Write(redact.RedactBytes(in)); err != nil {
		return fmt.Errorf("ao redact: write stdout: %w", err)
	}
	return nil
}
