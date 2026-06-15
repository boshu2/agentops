package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// pawlVerdict is the subset of fields we parse from a pawl-verdict JSON file.
// We read only the fields needed to construct provenance edges: bead_id,
// head_sha, and disposition. Everything else is the verdict schema's concern.
type pawlVerdict struct {
	BeadID      string `json:"bead_id"`
	HeadSHA     string `json:"head_sha"`
	Disposition string `json:"disposition"`
}

// extractVerdict reads a pawl-verdict JSON file and returns the provenance-
// relevant fields. Pure except for the file read — the parsing itself is
// deterministic and under test.
func extractVerdict(path string) (pawlVerdict, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pawlVerdict{}, fmt.Errorf("read verdict file: %w", err)
	}
	var v pawlVerdict
	if err := json.Unmarshal(data, &v); err != nil {
		return pawlVerdict{}, fmt.Errorf("parse verdict JSON: %w", err)
	}
	if v.BeadID == "" {
		return pawlVerdict{}, fmt.Errorf("verdict missing required field bead_id")
	}
	if v.HeadSHA == "" || len(v.HeadSHA) < 7 {
		return pawlVerdict{}, fmt.Errorf("verdict missing or too short head_sha (need >=7 chars)")
	}
	if v.Disposition == "" {
		return pawlVerdict{}, fmt.Errorf("verdict missing required field disposition")
	}
	return v, nil
}

// buildVerdictCommitEdge constructs the PROV-O edge: the verdict was derived
// from reviewing the commit at head_sha. Node id for the verdict encodes the
// bead and the short commit hash so each (bead, commit) pair produces a
// distinct, stable node id.
func buildVerdictCommitEdge(v pawlVerdict) provenancegraph.Edge {
	verdictNodeID := v.BeadID + "@" + shortHash7(v.HeadSHA)
	return provenancegraph.Edge{
		FromID:      verdictNodeID,
		FromType:    "verdict",
		ToID:        v.HeadSHA,
		ToType:      "commit",
		Relation:    "wasDerivedFrom",
		TrustTier:   "inferred",
		EvidenceRef: "pawl-verdict " + v.BeadID + " disposition=" + v.Disposition,
	}
}

var (
	provEmitVerdictFile   string
	provEmitVerdictJSON   bool
	provEmitVerdictDryRun bool
)

var provenanceEmitVerdictCmd = &cobra.Command{
	Use:   "emit-verdict",
	Short: "Emit verdict→commit provenance edges from a pawl-verdict file",
	Long: `Read a pawl-verdict JSON artifact (.agents/pawl-verdicts/<bead>.json) and
append a schema-valid, hash-chained provenance edge
(verdict --wasDerivedFrom--> commit) to the ledger. This is the verdict half
of the milestone-1 SENSOR (ag-cm8nd): the landed-bead emitter (ag-62jrm) feeds
on landings; this feeds on verdicts so the navigator knows which commits were
reviewed and by what disposition.

The verdict node id is <bead_id>@<head_sha[:7]> — stable per (bead, commit)
pair. trust_tier is inferred (a deterministic read of the verdict artifact).
Idempotent: re-emitting the same edge is a no-op.

Examples:
  ao provenance emit-verdict --file .agents/pawl-verdicts/ag-abc.json
  ao provenance emit-verdict --file .agents/pawl-verdicts/ag-abc.json --dry-run`,
	Args: cobra.NoArgs,
	RunE: runProvenanceEmitVerdict,
}

func init() {
	provenanceCmd.AddCommand(provenanceEmitVerdictCmd)
	provenanceEmitVerdictCmd.Flags().StringVar(&provEmitVerdictFile, "file", "", "Path to the pawl-verdict JSON file (required)")
	provenanceEmitVerdictCmd.Flags().BoolVar(&provEmitVerdictJSON, "json", false, "Emit appended edges as JSON")
	provenanceEmitVerdictCmd.Flags().BoolVar(&provEmitVerdictDryRun, "dry-run", false, "Resolve and print edges without writing the ledger")
	_ = provenanceEmitVerdictCmd.MarkFlagRequired("file")
}

func runProvenanceEmitVerdict(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	out := cmd.OutOrStdout()

	v, err := extractVerdict(provEmitVerdictFile)
	if err != nil {
		return err
	}

	edge := buildVerdictCommitEdge(v)
	edge.TS = time.Now().UTC().Format(time.RFC3339)

	if provEmitVerdictDryRun {
		fmt.Fprintf(out, "would emit %s --%s--> %s\n", edge.FromID, edge.Relation, shortHash7(edge.ToID))
		return nil
	}

	store := provenancegraph.NewStore(resolveLedgerPath())
	res, err := store.Append(edge)
	if err != nil {
		return fmt.Errorf("emit verdict edge: %w", err)
	}

	if res.Skipped {
		if !provEmitVerdictJSON {
			fmt.Fprintf(out, "provenance emit-verdict: already present (idempotent no-op)\n")
		}
		return nil
	}

	if !provEmitVerdictJSON {
		fmt.Fprintf(out, "emitted %s --%s--> %s\n", res.Edge.FromID, res.Edge.Relation, shortHash7(res.Edge.ToID))
	}
	return nil
}
