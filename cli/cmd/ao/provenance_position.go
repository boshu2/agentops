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
	Short: "Report the navigator's current position from the provenance ledger",
	Long: `Read the provenance ledger and report the navigator's current position:
which beads have landed (bead→commit wasGeneratedBy edges). This is the
read-only consumer that makes the fed ledger useful — the navigator's
"where am I" surface.

Agent-ergonomic: --json emits stdout-as-data, diagnostics on stderr.

Examples:
  ao provenance position
  ao provenance position --json`,
	Args: cobra.NoArgs,
	RunE: runProvenancePosition,
}

func init() {
	provenanceCmd.AddCommand(provenancePositionCmd)
	provenancePositionCmd.Flags().BoolVar(&provPositionJSON, "json", false, "Emit machine-readable JSON (stdout-as-data)")
}

// positionReport is the structured output of ao provenance position --json.
type positionReport struct {
	LandedBeads []landedBeadEntry `json:"landed_beads"`
	TotalEdges  int               `json:"total_edges"`
}

// landedBeadEntry is one bead→commit landing from the ledger.
type landedBeadEntry struct {
	BeadID    string `json:"bead_id"`
	CommitRef string `json:"commit_ref"`
	TrustTier string `json:"trust_tier"`
	Timestamp string `json:"ts"`
}

// extractLandedBeads filters wasGeneratedBy bead→commit edges from the ledger.
func extractLandedBeads(edges []provenancegraph.Edge) []landedBeadEntry {
	var landed []landedBeadEntry
	for _, e := range edges {
		if e.Relation == "wasGeneratedBy" && e.FromType == "bead" {
			landed = append(landed, landedBeadEntry{
				BeadID:    e.FromID,
				CommitRef: e.ToID,
				TrustTier: e.TrustTier,
				Timestamp: e.TS,
			})
		}
	}
	return landed
}

// buildPositionReport constructs the full position report from raw ledger edges.
func buildPositionReport(edges []provenancegraph.Edge) positionReport {
	landed := extractLandedBeads(edges)
	if landed == nil {
		landed = []landedBeadEntry{}
	}
	return positionReport{
		LandedBeads: landed,
		TotalEdges:  len(edges),
	}
}

func runProvenancePosition(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	out := cmd.OutOrStdout()

	store := provenancegraph.NewStore(resolveLedgerPath())
	edges, err := store.Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	report := buildPositionReport(edges)

	if provPositionJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	if len(report.LandedBeads) == 0 {
		fmt.Fprintln(out, "no landed arcs yet")
		return nil
	}

	fmt.Fprintf(out, "landed beads (%d):\n", len(report.LandedBeads))
	for _, lb := range report.LandedBeads {
		fmt.Fprintf(out, "  %s → %s [%s] %s\n", lb.BeadID, shortHash7(lb.CommitRef), lb.TrustTier, lb.Timestamp)
	}
	return nil
}
