// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var provShowJSON bool

var provenanceShowCmd = &cobra.Command{
	Use:   "show <sha|bead-id>",
	Short: "Render the human verdict lineage for one commit or bead",
	Long: `Render the human story of one change from the committed provenance
ledger (docs/provenance/ledger.jsonl): which bead(s) landed as which commit
(bead --wasGeneratedBy--> commit), which verdict(s) reviewed that commit
(verdict --wasDerivedFrom--> commit, with disposition and evidence when
recorded), and each record's chain position in the ledger.

The argument is either a commit sha (full, or a prefix of at least 7 hex
chars) or a bead id. A bead id resolves to every commit it landed as; a sha
renders every bead and verdict bound to it. Multiple verdicts on one commit
all render. A landed-but-unreviewed commit renders honestly with a
"no verdict recorded" line — per the membrane doctrine, no verdict = not done.

Agent-ergonomic (Directive 13): --json emits stdout-as-data, diagnostics on
stderr. Unknown ids exit non-zero with a corrective error naming how to
search.

Examples:
  ao provenance show ag-x31t.4
  ao provenance show 4f2a91c
  ao provenance show 4f2a91cd0e8b7a65331290fedcba9876543210ab --json`,
	Args: cobra.ExactArgs(1),
	RunE: runProvenanceShow,
}

func init() {
	provenanceCmd.AddCommand(provenanceShowCmd)
	provenanceShowCmd.Flags().BoolVar(&provShowJSON, "json", false, "Emit machine-readable JSON (stdout-as-data)")
}

// showReport is the structured output of ao provenance show --json.
type showReport struct {
	// Query is the sha or bead id the report resolved.
	Query string `json:"query"`
	// Commits is the lineage of every commit the query resolved to.
	Commits []commitLineage `json:"commits"`
	// TotalRecords is the ledger record count, the denominator of every
	// record position ("record 3/12").
	TotalRecords int `json:"total_records"`
}

// commitLineage is the full provenance story of one landed commit.
type commitLineage struct {
	CommitSHA string             `json:"commit_sha"`
	Beads     []showBeadEntry    `json:"beads"`
	Verdicts  []showVerdictEntry `json:"verdicts"`
}

// showBeadEntry is one bead→commit landing (wasGeneratedBy) edge.
type showBeadEntry struct {
	BeadID    string `json:"bead_id"`
	TrustTier string `json:"trust_tier"`
	Timestamp string `json:"ts"`
	// Record is the 1-based chain position of this edge in the ledger.
	Record int `json:"record"`
}

// showVerdictEntry is one verdict→commit review (wasDerivedFrom) edge.
type showVerdictEntry struct {
	// VerdictID is the verdict node id (<bead_id>@<sha7> for pawl verdicts).
	VerdictID string `json:"verdict_id"`
	// Disposition is parsed from the evidence_ref when present ("CONFIRMED",
	// "REFUTED", ...); empty when the record does not carry one.
	Disposition string `json:"disposition,omitempty"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
	TrustTier   string `json:"trust_tier"`
	Timestamp   string `json:"ts"`
	// Record is the 1-based chain position of this edge in the ledger.
	Record int `json:"record"`
	// v1.1 additive enrichment (age-rk3r.3): displayed only when the edge carries
	// them (consumers branch on PRESENCE, never a version string). A v1-shaped
	// verdict edge leaves them empty and they are omitted from the JSON.
	ReviewerFamily string  `json:"reviewer_family,omitempty"`
	Degraded       bool    `json:"degraded,omitempty"`
	Rounds         int     `json:"rounds,omitempty"`
	DurationS      float64 `json:"duration_s,omitempty"`
	EvidencePath   string  `json:"evidence_path,omitempty"`
}

// isHexToken reports whether s is non-empty lowercase/uppercase hex.
func isHexToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// parseDisposition extracts the "disposition=<value>" token from an
// evidence_ref (the emit-verdict evidence convention:
// "pawl-verdict <bead_id> disposition=<D>"). Returns "" when absent.
func parseDisposition(evidenceRef string) string {
	for _, field := range strings.Fields(evidenceRef) {
		if v, ok := strings.CutPrefix(field, "disposition="); ok {
			return v
		}
	}
	return ""
}

// minShaPrefixLen is the shortest commit-sha prefix `show` will resolve.
const minShaPrefixLen = 7

// resolveShowSHAs resolves a query (bead id, or a >=7-char sha prefix) to the
// full commit shas it names, in first-seen ledger order. Pure.
func resolveShowSHAs(edges []provenancegraph.Edge, query string) []string {
	var shas []string
	seen := map[string]bool{}
	add := func(sha string) {
		if sha != "" && !seen[sha] {
			seen[sha] = true
			shas = append(shas, sha)
		}
	}

	// Bead-id resolution first (exact match): landed edges plus verdict nodes
	// (<bead>@<sha7>), so a reviewed-but-never-landed bead still resolves.
	for _, e := range edges {
		if e.Relation == "wasGeneratedBy" && e.FromType == "bead" &&
			(e.FromID == query || e.BeadID == query) {
			add(e.ToID)
		}
		if e.FromType == "verdict" && e.ToType == "commit" &&
			strings.HasPrefix(e.FromID, query+"@") {
			add(e.ToID)
		}
	}
	if len(shas) > 0 {
		return shas
	}

	// SHA-prefix resolution: at least 7 hex chars, matched case-insensitively
	// against every commit-typed node id.
	lq := strings.ToLower(query)
	if len(lq) < minShaPrefixLen || !isHexToken(lq) {
		return nil
	}
	for _, e := range edges {
		if e.ToType == "commit" && strings.HasPrefix(strings.ToLower(e.ToID), lq) {
			add(e.ToID)
		}
		if e.FromType == "commit" && strings.HasPrefix(strings.ToLower(e.FromID), lq) {
			add(e.FromID)
		}
	}
	return shas
}

// buildShowReport resolves query against the ledger edges and assembles the
// per-commit lineage. Returns a corrective error (naming how to search) when
// nothing matches. Pure: no I/O, so it is the unit under test.
func buildShowReport(edges []provenancegraph.Edge, query string) (showReport, error) {
	query = strings.TrimSpace(query)
	shas := resolveShowSHAs(edges, query)
	if len(shas) == 0 {
		hint := "list landings with 'ao provenance position' or browse/filter edges with 'ao provenance list'"
		if isHexToken(query) && len(query) < minShaPrefixLen {
			hint = fmt.Sprintf("a sha prefix needs at least %d chars; %s", minShaPrefixLen, hint)
		}
		return showReport{}, fmt.Errorf("no provenance records match %q — %s", query, hint)
	}

	report := showReport{Query: query, Commits: []commitLineage{}, TotalRecords: len(edges)}
	for _, sha := range shas {
		lin := commitLineage{CommitSHA: sha, Beads: []showBeadEntry{}, Verdicts: []showVerdictEntry{}}
		for i, e := range edges {
			record := i + 1
			if e.ToID != sha {
				continue
			}
			switch {
			case e.Relation == "wasGeneratedBy" && e.FromType == "bead" && e.ToType == "commit":
				lin.Beads = append(lin.Beads, showBeadEntry{
					BeadID:    e.FromID,
					TrustTier: e.TrustTier,
					Timestamp: e.TS,
					Record:    record,
				})
			case e.Relation == "wasDerivedFrom" && e.FromType == "verdict" && e.ToType == "commit":
				lin.Verdicts = append(lin.Verdicts, showVerdictEntry{
					VerdictID:      e.FromID,
					Disposition:    parseDisposition(e.EvidenceRef),
					EvidenceRef:    e.EvidenceRef,
					TrustTier:      e.TrustTier,
					Timestamp:      e.TS,
					Record:         record,
					ReviewerFamily: e.ReviewerFamily,
					Degraded:       e.Degraded,
					Rounds:         e.Rounds,
					DurationS:      e.DurationS,
					EvidencePath:   e.EvidencePath,
				})
			}
		}
		report.Commits = append(report.Commits, lin)
	}
	return report, nil
}

// renderShowReport writes the human-readable lineage story.
func renderShowReport(out io.Writer, r showReport) {
	for _, c := range r.Commits {
		fmt.Fprintf(out, "commit %s\n", c.CommitSHA)
		if len(c.Beads) == 0 {
			fmt.Fprintln(out, "  (no landed-bead edge recorded for this commit)")
		}
		for _, b := range c.Beads {
			fmt.Fprintf(out, "  bead    %s  [%s]  %s  (record %d/%d)\n",
				b.BeadID, b.TrustTier, b.Timestamp, b.Record, r.TotalRecords)
		}
		if len(c.Verdicts) == 0 {
			fmt.Fprintln(out, "  no verdict recorded — landed but unreviewed")
		}
		for _, v := range c.Verdicts {
			disp := v.Disposition
			if disp == "" {
				disp = "(unspecified)"
			}
			fmt.Fprintf(out, "  verdict %s  disposition=%s  [%s]  %s  (record %d/%d)\n",
				v.VerdictID, disp, v.TrustTier, v.Timestamp, v.Record, r.TotalRecords)
			if v.EvidenceRef != "" {
				fmt.Fprintf(out, "          evidence: %s\n", v.EvidenceRef)
			}
			// v1.1 enrichment lines — rendered only when the edge carries them
			// (a v1-shaped edge omits every one, so the output is unchanged).
			if v.ReviewerFamily != "" {
				fmt.Fprintf(out, "          reviewer_family: %s\n", v.ReviewerFamily)
			}
			if v.Degraded {
				fmt.Fprintf(out, "          degraded: true\n")
			}
			if v.Rounds != 0 {
				fmt.Fprintf(out, "          rounds: %d\n", v.Rounds)
			}
			if v.DurationS != 0 {
				fmt.Fprintf(out, "          duration_s: %g\n", v.DurationS)
			}
			if v.EvidencePath != "" {
				fmt.Fprintf(out, "          evidence_path: %s\n", v.EvidencePath)
			}
		}
	}
}

func runProvenanceShow(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	store := provenancegraph.NewStore(resolveLedgerPath())
	edges, err := store.Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	report, err := buildShowReport(edges, args[0])
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if provShowJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	renderShowReport(out, report)
	return nil
}
