package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var provPositionJSON bool

var provenancePositionCmd = &cobra.Command{
	Use:   "position",
	Short: "Report the read-only provenance ledger tip",
	Long:  "Report the ledger record count and latest hash without inferring lifecycle state.",
	Args:  cobra.NoArgs,
	RunE:  runProvenancePosition,
}

func init() {
	provenanceCmd.AddCommand(provenancePositionCmd)
	provenancePositionCmd.Flags().BoolVar(&provPositionJSON, "json", false, "Emit machine-readable JSON")
}

type positionEdge struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
	Hash     string `json:"hash"`
}

type positionReport struct {
	TotalEdges int           `json:"total_edges"`
	TipHash    string        `json:"tip_hash"`
	Latest     *positionEdge `json:"latest,omitempty"`
}

func buildPositionReport(edges []provenancegraph.Edge) positionReport {
	report := positionReport{TotalEdges: len(edges)}
	if len(edges) == 0 {
		return report
	}
	edge := edges[len(edges)-1]
	report.TipHash = edge.Hash
	report.Latest = &positionEdge{FromID: edge.FromID, ToID: edge.ToID, Relation: edge.Relation, Hash: edge.Hash}
	return report
}

func runProvenancePosition(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	edges, err := provenancegraph.NewStore(resolveLedgerPath()).Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}
	report := buildPositionReport(edges)
	if provPositionJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	if report.Latest == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "provenance ledger is empty")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "provenance ledger: %d edge(s), tip %s\n", report.TotalEdges, shortHash(report.TipHash))
	fmt.Fprintf(cmd.OutOrStdout(), "latest: %s --%s--> %s\n", report.Latest.FromID, report.Latest.Relation, report.Latest.ToID)
	return nil
}
