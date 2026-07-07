package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// pawlVerdict is the subset of fields we parse from a pawl-verdict JSON file.
// We read the fields needed to construct provenance edges: bead_id, head_sha,
// and disposition, plus the refuter panel and council_artifact that back the
// v1.1 additive enrichment (reviewer_family, evidence_path). Everything else is
// the verdict schema's concern.
type pawlVerdict struct {
	BeadID          string        `json:"bead_id"`
	HeadSHA         string        `json:"head_sha"`
	Disposition     string        `json:"disposition"`
	Refuters        []pawlRefuter `json:"refuters"`
	CouncilArtifact string        `json:"council_artifact"`
	Attempt         int           `json:"attempt"`
	Degraded        bool          `json:"degraded"`
	Cost            *pawlCost     `json:"cost"`
}

// pawlCost is the verification-economics meter object pawl-verdict.sh write
// attaches when the caller supplies --wall-seconds (age-verification-economics-
// ebec.1). tokens_est is a transcript-bytes/4 estimate over the refuter
// evidence files unless the harness reported exact usage (estimated=false).
type pawlCost struct {
	WallSeconds float64 `json:"wall_seconds"`
	TokensEst   int     `json:"tokens_est"`
	Estimated   bool    `json:"estimated"`
}

// pawlRefuter is the subset of a pawl-verdict refuter entry the sensor reads:
// the model family (for reviewer_family) and the evidence path (for
// evidence_path). Matches the refuter shape in schemas/pawl-verdict.v1.schema.json.
type pawlRefuter struct {
	Family   string `json:"family"`
	Evidence string `json:"evidence"`
}

// normalizeFamily collapses a raw pawl-verdict family label to its canonical
// model family, mirroring normalize_family() in scripts/pawl-verdict.sh so the
// ledger's reviewer_family agrees with the gate's roster: fable/anthropic ->
// claude, codex/openai -> gpt, agy/google -> gemini. An unknown/off-roster label
// returns "" (dropped) rather than polluting the mix with junk.
func normalizeFamily(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "claude", "fable", "anthropic":
		return "claude"
	case "gpt", "codex", "openai":
		return "gpt"
	case "gemini", "agy", "google":
		return "gemini"
	default:
		return ""
	}
}

// deriveReviewerFamily maps the refuter panel to the edge's reviewer_family: the
// sorted, de-duplicated set of canonical families joined with "+" (e.g. "claude"
// or "claude+gpt"). Returns "" when no refuter carries a roster-valid family, so
// the omitempty field stays absent (a pre-v1.1-shaped edge) rather than empty.
func deriveReviewerFamily(refuters []pawlRefuter) string {
	seen := map[string]bool{}
	var fams []string
	for _, r := range refuters {
		nf := normalizeFamily(r.Family)
		if nf == "" || seen[nf] {
			continue
		}
		seen[nf] = true
		fams = append(fams, nf)
	}
	sort.Strings(fams)
	return strings.Join(fams, "+")
}

// deriveEvidencePath maps the verdict artifact to the edge's evidence_path: the
// first refuter with a non-empty evidence path (the reviewer's real run output),
// falling back to the top-level council_artifact. Returns "" when the verdict
// carries no evidence path, so the omitempty field stays absent.
func deriveEvidencePath(v pawlVerdict) string {
	for _, r := range v.Refuters {
		if p := strings.TrimSpace(r.Evidence); p != "" {
			return p
		}
	}
	return strings.TrimSpace(v.CouncilArtifact)
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
		// v1.1 additive enrichment (age-rk3r.3). bead_id is the structured join
		// key (a non-payload projection of from_id) that lets receipts/mesh stop
		// regex-parsing the free-text evidence_ref for the bead. reviewer_family
		// and evidence_path are hash-protected fields derived from the verdict's
		// refuter panel WHEN PRESENT. degraded/rounds/duration_s have no source in
		// the v1 pawl-verdict file yet, so they stay empty (absent) until the
		// sibling beads populate them (.2 failover label, .16 cost substrate) —
		// additive by construction, no version bump.
		BeadID:         v.BeadID,
		ReviewerFamily: deriveReviewerFamily(v.Refuters),
		EvidencePath:   deriveEvidencePath(v),
		// Verification-economics meter (ebec.1) + the previously-unsourced v1.1
		// fields: all additive/omitempty — a verdict without cost/attempt/degraded
		// produces a byte-identical pre-meter edge payload.
		Degraded:  v.Degraded,
		Rounds:    v.Attempt,
		DurationS: costWallSeconds(v.Cost),
		TokensEst: costTokensEst(v.Cost),
	}
}

// costWallSeconds and costTokensEst project the optional meter object; a nil
// cost yields zero values, which omitempty drops from the payload.
func costWallSeconds(c *pawlCost) float64 {
	if c == nil {
		return 0
	}
	return c.WallSeconds
}

func costTokensEst(c *pawlCost) int {
	if c == nil {
		return 0
	}
	return c.TokensEst
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
