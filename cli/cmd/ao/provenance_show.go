// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var provShowJSON bool

var provenanceShowCmd = &cobra.Command{
	Use:   "show <node-id>",
	Short: "Show generic provenance relationships for one exact node",
	Long: `Read the provenance ledger and show every edge whose from_id or to_id
exactly matches the supplied node. This command reports evidence; it does not
infer completion, landing, validation, or a next action.`,
	Args: cobra.ExactArgs(1),
	RunE: runProvenanceShow,
}

func init() {
	provenanceCmd.AddCommand(provenanceShowCmd)
	provenanceShowCmd.Flags().BoolVar(&provShowJSON, "json", false, "Emit machine-readable JSON")
}

type showEdge struct {
	Record      int    `json:"record"`
	Direction   string `json:"direction"`
	Counterpart string `json:"counterpart"`
	Type        string `json:"counterpart_type"`
	Relation    string `json:"relation"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	TrustTier   string `json:"trust_tier"`
	Timestamp   string `json:"ts"`
	Hash        string `json:"hash"`
}

type showReport struct {
	NodeID       string     `json:"node_id"`
	Relationships []showEdge `json:"relationships"`
	TotalRecords int        `json:"total_records"`
}

func buildShowReport(edges []provenancegraph.Edge, nodeID string) (showReport, error) {
	report := showReport{NodeID: nodeID, Relationships: []showEdge{}, TotalRecords: len(edges)}
	for i, edge := range edges {
		view := showEdge{Record: i + 1, Relation: edge.Relation, EvidenceRef: edge.EvidenceRef, TrustTier: edge.TrustTier, Timestamp: edge.TS, Hash: edge.Hash}
		switch {
		case edge.FromID == nodeID:
			view.Direction, view.Counterpart, view.Type = "outbound", edge.ToID, edge.ToType
		case edge.ToID == nodeID:
			view.Direction, view.Counterpart, view.Type = "inbound", edge.FromID, edge.FromType
		default:
			continue
		}
		report.Relationships = append(report.Relationships, view)
	}
	if len(report.Relationships) == 0 {
		return showReport{}, fmt.Errorf("node %q is not present in the provenance ledger", nodeID)
	}
	return report, nil
}

func renderShowReport(out io.Writer, report showReport) {
	fmt.Fprintf(out, "node %s (%d relationship(s))\n", report.NodeID, len(report.Relationships))
	for _, edge := range report.Relationships {
		fmt.Fprintf(out, "  %s --%s--> %s [%s] record %d/%d\n", edge.Direction, edge.Relation, edge.Counterpart, edge.TrustTier, edge.Record, report.TotalRecords)
		if edge.EvidenceRef != "" {
			fmt.Fprintf(out, "    evidence: %s\n", edge.EvidenceRef)
		}
	}
}

func runProvenanceShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	edges, err := provenancegraph.NewStore(resolveLedgerPath()).Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}
	report, err := buildShowReport(edges, args[0])
	if err != nil {
		return err
	}
	if provShowJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	renderShowReport(cmd.OutOrStdout(), report)
	return nil
}
