// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

var (
	provAddFromType  string
	provAddToType    string
	provAddRelation  string
	provAddTrustTier string
	provAddEvidence  string
	provAddTS        string
	provAddJSON      bool

	provListJSON     bool
	provListFromID   string
	provListRelation string
)

var provenanceCmd = &cobra.Command{
	Use:   "provenance",
	Short: "Write and read the SDLC provenance edge ledger",
	Long: `Append-only write model for the SDLC provenance/intent graph
(ag-x31t). Edges are typed, evidence-backed relations between SDLC nodes
(e.g. a decision and the artifact it produced), recorded as per-record
hash-chained events in the committed ledger at docs/provenance/ledger.jsonl.

Per CLAUDE.md the committed JSONL ledger is the AUDIT authority and source of
truth; any Dolt projection is rebuildable and loses on disagreement, so these
subcommands write the JSONL ledger directly.`,
}

var provenanceAddCmd = &cobra.Command{
	Use:   "add <from-id> <to-id>",
	Short: "Append a provenance edge to the ledger",
	Long: `Append one schema-valid, hash-chained provenance edge linking a source
node (<from-id>) to a target node (<to-id>). The edge is sealed onto the
current chain tip (prev_hash = the last record's hash) and validated against
schemas/agentops-sdlc-provenance.v1.schema.json before any write.

The command is idempotent: re-running with the same endpoints, relation,
evidence, and trust tier is a no-op (no duplicate row).

Examples:
  ao provenance add ag-x31t.4 cli/cmd/ao/provenance_add.go \
    --relation wasGeneratedBy --to-type artifact
  ao provenance add soc-byl.3 ag-x31t \
    --relation wasInfluencedBy --from-type bead --to-type decision \
    --trust-tier authored --evidence .agents/council/2026-05-30-debate-provenance-substrate.md`,
	Args: cobra.ExactArgs(2),
	RunE: runProvenanceAdd,
}

var provenanceListCmd = &cobra.Command{
	Use:   "list",
	Short: "Read provenance edges back from the ledger",
	Long: `Read the provenance edges recorded in docs/provenance/ledger.jsonl, in
ledger (chain) order. Optionally filter by source node id or relation.

Examples:
  ao provenance list
  ao provenance list --json
  ao provenance list --from-id ag-x31t.4
  ao provenance list --relation wasGeneratedBy`,
	Args: cobra.NoArgs,
	RunE: runProvenanceList,
}

func init() {
	rootCmd.AddCommand(provenanceCmd)
	provenanceCmd.AddCommand(provenanceAddCmd)
	provenanceCmd.AddCommand(provenanceListCmd)

	provenanceAddCmd.Flags().StringVar(&provAddRelation, "relation", "", "Typed PROV-O relation (required), e.g. wasGeneratedBy")
	provenanceAddCmd.Flags().StringVar(&provAddFromType, "from-type", "decision", "Source node type (decision|artifact|bead|...)")
	provenanceAddCmd.Flags().StringVar(&provAddToType, "to-type", "artifact", "Target node type (decision|artifact|bead|...)")
	provenanceAddCmd.Flags().StringVar(&provAddTrustTier, "trust-tier", "authored", "Trust tier (authored|inferred|mined)")
	provenanceAddCmd.Flags().StringVar(&provAddEvidence, "evidence", "", "Optional evidence pointer (path, commit, CI run URL, event id)")
	provenanceAddCmd.Flags().StringVar(&provAddTS, "ts", "", "Override the UTC RFC3339 timestamp (defaults to now)")
	provenanceAddCmd.Flags().BoolVar(&provAddJSON, "json", false, "Emit the sealed edge as JSON")

	provenanceListCmd.Flags().BoolVar(&provListJSON, "json", false, "Emit machine-readable JSON")
	provenanceListCmd.Flags().StringVar(&provListFromID, "from-id", "", "Filter to edges whose from_id matches")
	provenanceListCmd.Flags().StringVar(&provListRelation, "relation", "", "Filter to edges with this relation")
}

// resolveLedgerPath locates docs/provenance/ledger.jsonl relative to the
// repository root, walking up from cwd to find a directory containing a docs/
// dir or a .git entry. Falls back to a cwd-relative path so the error from the
// store is clear when no repo root is found.
func resolveLedgerPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return provenancegraph.LedgerRelativePath
	}
	dir := cwd
	for i := 0; i < 12; i++ {
		if isDir(filepath.Join(dir, "docs")) && isDir(filepath.Join(dir, "schemas")) {
			return filepath.Join(dir, provenancegraph.LedgerRelativePath)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil {
			return filepath.Join(dir, provenancegraph.LedgerRelativePath)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(cwd, provenancegraph.LedgerRelativePath)
}

func runProvenanceAdd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	ts := provAddTS
	if strings.TrimSpace(ts) == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}

	edge := provenancegraph.Edge{
		FromID:      args[0],
		FromType:    provAddFromType,
		ToID:        args[1],
		ToType:      provAddToType,
		Relation:    provAddRelation,
		EvidenceRef: provAddEvidence,
		TrustTier:   provAddTrustTier,
		TS:          ts,
	}

	store := provenancegraph.NewStore(resolveLedgerPath())
	res, err := store.Append(edge)
	if err != nil {
		return fmt.Errorf("append provenance edge: %w", err)
	}

	out := cmd.OutOrStdout()
	if provAddJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Edge)
	}
	if res.Skipped {
		fmt.Fprintf(out, "edge already present (idempotent no-op): %s --%s--> %s\n",
			res.Edge.FromID, res.Edge.Relation, res.Edge.ToID)
		return nil
	}
	fmt.Fprintf(out, "appended edge %s --%s--> %s (hash %s)\n",
		res.Edge.FromID, res.Edge.Relation, res.Edge.ToID, shortHash(res.Edge.Hash))
	return nil
}

func runProvenanceList(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	store := provenancegraph.NewStore(resolveLedgerPath())
	edges, err := store.Read()
	if err != nil {
		return fmt.Errorf("read provenance ledger: %w", err)
	}

	filtered := make([]provenancegraph.Edge, 0, len(edges))
	for _, e := range edges {
		if provListFromID != "" && e.FromID != provListFromID {
			continue
		}
		if provListRelation != "" && e.Relation != provListRelation {
			continue
		}
		filtered = append(filtered, e)
	}

	out := cmd.OutOrStdout()
	if provListJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}
	if len(filtered) == 0 {
		fmt.Fprintln(out, "no provenance edges")
		return nil
	}
	for _, e := range filtered {
		fmt.Fprintf(out, "%s --%s--> %s [%s] %s\n",
			e.FromID, e.Relation, e.ToID, e.TrustTier, shortHash(e.Hash))
	}
	return nil
}

// shortHash returns the first 12 chars of a hash for human-readable output.
func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
