// practices: [design-by-contract]
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// provenanceReaderVersionCmd prints the running binary's ledger-reader capability
// level (provenancegraph.LedgerReaderVersion) as a single bare integer on stdout.
//
// It is the ao-version FLOOR probe the portable pre-push hook (`ao verify init`,
// age-rk3r.6) runs BEFORE it trusts a chain verify. It lives under `provenance`
// (NOT `verify`) on purpose: `verify` sets DisableFlagParsing and forwards every
// unknown token to the pawl-review engine, so on a binary too old to have this
// subcommand `ao verify ledger-reader-version` would be MISREAD as a bead id and
// launch a review. `provenance` has no such forwarding, so an old binary answers
// `ao provenance ledger-reader-version` with a clean "unknown command" non-zero
// exit and NO side effects — exactly the signal the hook needs to say "too old,
// upgrade".
var provenanceReaderVersionCmd = &cobra.Command{
	Use:   "ledger-reader-version",
	Short: "Print this binary's ledger-reader capability level (the pre-push hook's ao-version floor probe)",
	Long: `Print, as a single bare integer, the ledger-reader capability level this ao
binary understands (provenancegraph.LedgerReaderVersion). This is the durable
ao-version FLOOR the portable pre-push hook installed by 'ao verify init' probes
before it trusts 'ao provenance verify': a reader below the floor would report a
SPURIOUS "broken chain" on the age-rk3r.3 v1.1 verdict records instead of
verifying them, so the hook must refuse-with-upgrade rather than false-alarm.

The integer is monotonic; a higher value is a strict superset of readers'
capability, so a hook floor of N is satisfied by any binary printing >= N.

Exit status is always 0 on a binary new enough to have this subcommand; a binary
too old to have it exits non-zero via cobra's unknown-command path, which the
hook treats as "below floor — upgrade ao".

Example:
  ao provenance ledger-reader-version   # -> 1`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		fmt.Fprintf(cmd.OutOrStdout(), "%d\n", provenancegraph.LedgerReaderVersion)
		return nil
	},
}

func init() {
	provenanceCmd.AddCommand(provenanceReaderVersionCmd)
}
