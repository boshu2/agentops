// practices: [supply-chain-integrity]
package main

import (
	"fmt"
	"io"

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

func init() {
	redactCmd.GroupID = "core"
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
