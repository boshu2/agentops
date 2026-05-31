// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var (
	provTraceOrphans bool
	provTraceStrict  bool
	provTraceJSON    bool
	provTraceGraph   string
)

var provenanceTraceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Audit the provenance graph for orphan artifacts (no inbound edge)",
	Long: `Audit a provenance trace-graph for orphans: engineered artifact nodes
that have NO inbound authored/inferred provenance edge. This generalizes
'ao goals trace --orphans' (the directive→scenario→bead→artifact→learning
chain-gap audit) onto the provenance graph, where the orphan condition is "an
artifact node that nothing produces".

The graph is a JSONL file using the goalstrace Node/Edge JSON contract
(cli/internal/goalstrace/graph.go) with a leading "record" discriminator. Read
the committed audit authority (docs/provenance/ledger.jsonl is the ledger; a
trace-graph is the node/edge projection) or a fixture passed with --graph.

  --orphans   audit for artifact nodes with no inbound edge (required mode).
  --strict    exit non-zero when any orphan exists (the CI gate uses this).
  --json      emit one finding object per line (line-delimited JSON).
  --graph     path to the JSONL trace-graph to audit (required).

Without --strict the audit reports orphans but exits 0 (advisory). With
--strict any orphan fails the command, which is how the no-orphan provenance
gate in .github/workflows/validate.yml blocks merges.

Examples:
  ao provenance trace --orphans --graph tests/fixtures/provenance/orphan-stale-65-jobs.jsonl
  ao provenance trace --orphans --strict --graph docs/provenance/graph.jsonl
  ao provenance trace --orphans --json --graph <file>`,
	Args: cobra.NoArgs,
	RunE: runProvenanceTrace,
}

func init() {
	provenanceCmd.AddCommand(provenanceTraceCmd)

	provenanceTraceCmd.Flags().BoolVar(&provTraceOrphans, "orphans", false, "Audit for artifact nodes with no inbound provenance edge")
	provenanceTraceCmd.Flags().BoolVar(&provTraceStrict, "strict", false, "Exit non-zero when any orphan exists")
	provenanceTraceCmd.Flags().BoolVar(&provTraceJSON, "json", false, "Emit each finding as one JSON object per line")
	provenanceTraceCmd.Flags().StringVar(&provTraceGraph, "graph", "", "Path to the JSONL trace-graph to audit (required)")
}

func runProvenanceTrace(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	if !provTraceOrphans {
		return fmt.Errorf("ao provenance trace requires --orphans (the only supported mode)")
	}
	if provTraceGraph == "" {
		return fmt.Errorf("ao provenance trace --orphans requires --graph <path>")
	}

	recs, err := provenancegraph.ReadGraphRecords(provTraceGraph)
	if err != nil {
		return fmt.Errorf("read trace-graph: %w", err)
	}
	findings := provenancegraph.FindOrphans(recs)

	out := cmd.OutOrStdout()
	if provTraceJSON {
		enc := json.NewEncoder(out)
		for _, f := range findings {
			if err := enc.Encode(f); err != nil {
				return fmt.Errorf("encode orphan finding: %w", err)
			}
		}
	} else if len(findings) == 0 {
		fmt.Fprintln(out, "No provenance orphans found.")
	} else {
		for _, f := range findings {
			fmt.Fprintf(out, "ERROR  %-26s %s\n", f.Code, f.Message)
		}
		fmt.Fprintf(out, "\n%d orphan(s)\n", len(findings))
		if !provTraceStrict {
			fmt.Fprintln(out, "(orphans do not fail the command; pass --strict to escalate)")
		}
	}

	if len(findings) > 0 && provTraceStrict {
		return fmt.Errorf("provenance graph has %d orphan(s) (--strict)", len(findings))
	}
	return nil
}
