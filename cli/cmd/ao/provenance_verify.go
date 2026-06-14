// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var provVerifyJSON bool

var provenanceVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the committed provenance ledger's hash chain IN PLACE",
	Long: `Read docs/provenance/ledger.jsonl exactly as committed and verify its
per-record hash chain without re-sorting or re-chaining. Each non-blank line
must parse as a schema-valid edge AND its prev_hash must link to the prior
record's hash with payload_hash/hash recomputing — so a tampered field, a
forged hash, or a reordered row is CAUGHT and the offending file line is named.

This is the tamper-detection surface for the audit authority: unlike
'ao provenance export --verify' (which canonically re-sorts and re-chains the
edge set), 'verify' checks the committed bytes in place, so an altered ledger
fails loudly instead of being silently re-sealed.

Exit status:
  0   the committed chain is intact (or the ledger is absent/empty)
  1   a broken/tampered/non-conforming record was found (the line is named)

Examples:
  ao provenance verify
  ao provenance verify --json`,
	Args: cobra.NoArgs,
	RunE: runProvenanceVerify,
}

func init() {
	provenanceCmd.AddCommand(provenanceVerifyCmd)
	provenanceVerifyCmd.Flags().BoolVar(&provVerifyJSON, "json", false, "Emit the machine-readable verify result as JSON")
}

func runProvenanceVerify(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	store := provenancegraph.NewStore(resolveLedgerPath())
	res, err := store.VerifyFile()
	if err != nil {
		return fmt.Errorf("verify provenance ledger: %w", err)
	}

	out := cmd.OutOrStdout()
	if provVerifyJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(res); encErr != nil {
			return encErr
		}
	} else if res.Pass {
		fmt.Fprintf(out, "OK: provenance ledger chain intact (%d record(s))\n", res.RecordCount)
	} else {
		fmt.Fprintf(out, "BROKEN: provenance ledger chain breaks at line %d: %s\n",
			res.FirstBrokenLine, res.Message)
	}

	if !res.Pass {
		// Non-zero exit on a broken/tampered ledger, with the line already named.
		return fmt.Errorf("provenance ledger verification failed at line %d: %s",
			res.FirstBrokenLine, res.Message)
	}
	return nil
}
