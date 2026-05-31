// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var (
	provExportJSON   bool
	provExportVerify bool
)

var provenanceExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Emit a deterministic, hash-chained rendering of the provenance ledger",
	Long: `Read docs/provenance/ledger.jsonl, canonically sort its edges by
(ts, from_id, to_id, relation), re-seal them into a fresh per-record hash chain,
and emit the result. The canonical sort makes the export byte-identical on
re-run regardless of the ledger's physical append order, so the bytes are
reproducible and the chain is independently verifiable.

The exported chain is verified with NO Dolt server: the committed JSONL is the
audit authority and re-chaining uses only the in-process hashing in
cli/internal/provenancegraph (the same prev_hash discipline as the rpi ledger).

Output:
  default        one compact JSON edge per line (JSONL), trailing newline.
  --json         a single indented JSON array of the sealed edges.
  --verify       re-chain and verify only; print a one-line OK summary and emit
                 nothing on stdout that would vary across runs.

Examples:
  ao provenance export
  ao provenance export --json
  ao provenance export --verify`,
	Args: cobra.NoArgs,
	RunE: runProvenanceExport,
}

func init() {
	provenanceCmd.AddCommand(provenanceExportCmd)

	provenanceExportCmd.Flags().BoolVar(&provExportJSON, "json", false, "Emit a single indented JSON array instead of JSONL")
	provenanceExportCmd.Flags().BoolVar(&provExportVerify, "verify", false, "Verify the re-chained export and print only a one-line summary")
}

func runProvenanceExport(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	store := provenancegraph.NewStore(resolveLedgerPath())
	edges, err := store.Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	// Canonical sort + fresh hash chain. Deterministic for a given edge set.
	chain, err := provenancegraph.ReChain(edges)
	if err != nil {
		return fmt.Errorf("re-chain provenance ledger: %w", err)
	}

	// The re-chained export must itself be an intact chain.
	if idx, verr := provenancegraph.VerifyChain(chain); verr != nil {
		return fmt.Errorf("exported chain failed verification at record %d: %w", idx, verr)
	}

	out := cmd.OutOrStdout()

	if provExportVerify {
		fmt.Fprintf(out, "OK: %d edge(s) re-chained and verified (no Dolt server)\n", len(chain))
		return nil
	}

	if provExportJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		// Always emit a concrete (possibly empty) array, never null.
		if chain == nil {
			chain = []provenancegraph.Edge{}
		}
		return enc.Encode(chain)
	}

	// Default: deterministic JSONL — one compact line per edge, fixed field
	// order (Edge struct tag order), single trailing newline per record.
	for _, e := range chain {
		line, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal edge: %w", err)
		}
		if _, err := fmt.Fprintf(out, "%s\n", line); err != nil {
			return fmt.Errorf("write edge: %w", err)
		}
	}
	return nil
}
