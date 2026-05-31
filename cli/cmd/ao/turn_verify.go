// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/evidencedturn"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/turnstate"
)

var (
	turnVerifyInput     string
	turnVerifyLedger    string
	turnVerifyGraph     string
	turnVerifyJSON      bool
	turnVerifyAllowSelf bool
)

// turnCmd is the parent group for Evidenced-Turn operations (ag-lmdx). A "turn"
// is one unit of context-work; its done-ness is an enforced assurance contract.
var turnCmd = &cobra.Command{
	Use:   "turn",
	Short: "Evidenced-Turn operations (the enforced unit-of-work definition-of-done)",
	Long: `The 'turn' command group operates on Evidenced Turns — the ag-lmdx
unit of work. An Evidenced Turn is the assurance contract that lets a bead
legally transition validated->closed: a hash-chained state log, covered
scenarios, resolving Evidence, a provenance event, and no orphan.`,
}

// turnInputFile is the on-disk shape consumed by `ao turn verify --input`. It
// carries the facts that are NOT in the committed provenance ledger: the bead's
// state_transition log and its Closes-scenario coverage. Provenance edges and
// orphans are read from the ledger (or --ledger) so they stay the audit
// authority, not a hand-supplied claim.
type turnInputFile struct {
	BeadID      string                   `json:"bead_id"`
	Transitions []turnstate.Transition   `json:"transitions"`
	Scenarios   []evidencedturn.Scenario `json:"scenarios"`
	// AuthorID and JudgeID carry the no-self-grading invariant (ag-lmdx.4):
	// the identity of the context that AUTHORED the artifact vs. the context
	// that PRODUCED the acceptance verdict. The author_neq_validator predicate
	// fails when they are equal (a self-graded, autocorrelated verdict) unless
	// --allow-self waives it. Empty judge_id also fails: independence that was
	// never recorded cannot be asserted.
	AuthorID string `json:"author_id,omitempty"`
	JudgeID  string `json:"judge_id,omitempty"`
}

var turnVerifyCmd = &cobra.Command{
	Use:   "verify <bead>",
	Short: "Verify a bead's Evidenced-Turn definition-of-done (legible pass/fail)",
	Long: `Evaluate the legible Definition-of-Done predicate for one bead's
Evidenced Turn and report a single done/not-done verdict with a per-predicate
breakdown. This is the sufficiency check the live AP#7 gate omits: AP#7 verifies
an Evidence trailer is PRESENT; this verifies the turn is actually DONE.

A turn is DONE iff every predicate passes (validated->closed is legal):

  chain_intact       the bead's state_transition log hash-verifies and folds
                     (cli/internal/turnstate FoldVerified)
  terminal_state     the folded lifecycle state is at the validated->closed
                     boundary
  scenarios_covered  every Closes-scenario claim has a passing test
  evidence_resolves  every Evidence line resolves to a gate log that exercised
                     THAT scenario (presence->sufficiency)
  provenance_event   a provenance edge in the ledger references the bead
  no_orphan          no artifact the turn produced is a provenance orphan
  author_neq_validator
                     the acceptance verdict came from a judge context distinct
                     from the author context (author_id != judge_id) — the
                     no-self-grading invariant (ag-lmdx.4). A self-graded
                     verdict is autocorrelated and fails; --allow-self waives it
                     for the inline fallback (default OFF).

Inputs:
  --input   turn-input JSON file: the bead's state_transition log + its
            Closes-scenario coverage (the facts not in the committed ledger).
  --ledger  the provenance EDGE ledger (schema agentops-sdlc-provenance.v1);
            defaults to docs/provenance/ledger.jsonl. Used for provenance_event.
  --graph   an OPTIONAL provenance trace-graph projection (node/edge "record"
            JSONL, the shape ao provenance trace reads). Used for no_orphan.
            Without it, no_orphan is reported as not-yet-checked and fails.

  --allow-self  waive the no-self-grading invariant (author_neq_validator),
            permitting a self-graded verdict (judge_id == author_id) on the
            inline fallback path. Default OFF: the default requires an
            independent judge context.

Exit is non-zero when the turn is NOT done, so this is usable as a
validated->closed transition guard.

Output:
  default   a legible checklist with one line per predicate and a verdict.
  --json    the full Verdict object (schema agentops-evidenced-turn.v1).

Examples:
  ao turn verify ag-lmdx.5 --input turn.json
  ao turn verify ag-lmdx.5 --input turn.json --json
  ao turn verify ag-lmdx.5 --input turn.json --graph graph.jsonl`,
	Args: cobra.ExactArgs(1),
	RunE: runTurnVerify,
}

func init() {
	rootCmd.AddCommand(turnCmd)
	turnCmd.AddCommand(turnVerifyCmd)

	turnVerifyCmd.Flags().StringVar(&turnVerifyInput, "input", "", "Path to the turn-input JSON file (state log + scenario coverage) (required)")
	turnVerifyCmd.Flags().StringVar(&turnVerifyLedger, "ledger", "", "Path to the provenance EDGE ledger JSONL (default: docs/provenance/ledger.jsonl)")
	turnVerifyCmd.Flags().StringVar(&turnVerifyGraph, "graph", "", "Path to the provenance trace-graph JSONL (node/edge records) for orphan detection")
	turnVerifyCmd.Flags().BoolVar(&turnVerifyJSON, "json", false, "Emit the full Verdict object as JSON")
	turnVerifyCmd.Flags().BoolVar(&turnVerifyAllowSelf, "allow-self", false, "Waive the no-self-grading invariant (permit judge_id == author_id) for the inline fallback; default OFF")
}

func runTurnVerify(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	beadID := strings.TrimSpace(args[0])
	if beadID == "" {
		return fmt.Errorf("a non-empty <bead> is required")
	}
	if turnVerifyInput == "" {
		return fmt.Errorf("ao turn verify requires --input <turn-input.json>")
	}

	tf, err := readTurnInputFile(turnVerifyInput)
	if err != nil {
		return err
	}
	// The positional bead is authoritative; an input file declaring a different
	// bead is a mistake worth surfacing rather than silently overriding.
	if tf.BeadID != "" && tf.BeadID != beadID {
		return fmt.Errorf("input file bead_id %q does not match <bead> %q", tf.BeadID, beadID)
	}

	ledgerPath := turnVerifyLedger
	if ledgerPath == "" {
		ledgerPath = resolveLedgerPath()
	}
	edges, err := readLedgerEdges(ledgerPath)
	if err != nil {
		return err
	}

	var orphans []provenancegraph.OrphanFinding
	orphanChecked := false
	if turnVerifyGraph != "" {
		graph, gerr := provenancegraph.ReadGraphRecords(turnVerifyGraph)
		if gerr != nil {
			return fmt.Errorf("read provenance trace-graph: %w", gerr)
		}
		// Any orphan in the turn's trace-graph means its provenance is
		// incomplete — mirrors the repo-wide `ao provenance trace --orphans`
		// no-orphan gate. A turn with a dangling artifact is not provably done.
		orphans = provenancegraph.FindOrphans(graph)
		orphanChecked = true
	}

	v, err := evidencedturn.Evaluate(evidencedturn.Input{
		BeadID:          beadID,
		Transitions:     tf.Transitions,
		Scenarios:       tf.Scenarios,
		ProvenanceEdges: edges,
		OrphanFindings:  orphans,
		OrphanChecked:   orphanChecked,
		AuthorID:        tf.AuthorID,
		JudgeID:         tf.JudgeID,
		AllowSelf:       turnVerifyAllowSelf,
	})
	if err != nil {
		return err
	}

	if err := renderVerdict(cmd, v); err != nil {
		return err
	}
	if !v.Done {
		// Non-zero exit makes this usable as a validated->closed transition
		// guard. SilenceUsage is set, so cobra won't print usage on this.
		return fmt.Errorf("turn %s is NOT done: %d gap(s)", v.BeadID, len(v.Gaps))
	}
	return nil
}

// readTurnInputFile loads and decodes the turn-input JSON file, rejecting
// unknown fields so a malformed contract fails loudly.
func readTurnInputFile(path string) (turnInputFile, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- operator-supplied input path, same trust model as --graph
	if err != nil {
		return turnInputFile{}, fmt.Errorf("read turn-input file %q: %w", path, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var tf turnInputFile
	if err := dec.Decode(&tf); err != nil {
		return turnInputFile{}, fmt.Errorf("parse turn-input file %q: %w", path, err)
	}
	return tf, nil
}

// readLedgerEdges reads the provenance ledger edges. A missing ledger is not a
// hard error here: the evidencedturn evaluator will simply fail the
// provenance_event predicate with a legible reason rather than crash.
func readLedgerEdges(path string) ([]provenancegraph.Edge, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat provenance ledger %q: %w", path, err)
	}
	store := provenancegraph.NewStore(path)
	edges, err := store.Read()
	if err != nil {
		return nil, fmt.Errorf("read provenance ledger: %w", err)
	}
	return edges, nil
}

// renderVerdict prints the legible checklist (or JSON). The text form is the
// agent-facing default: a status glyph, the predicate, and its reason.
func renderVerdict(cmd *cobra.Command, v evidencedturn.Verdict) error {
	out := cmd.OutOrStdout()
	if turnVerifyJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}

	fmt.Fprintf(out, "Evidenced-Turn DoD for %s\n", v.BeadID)
	for _, p := range v.Predicates {
		glyph := "FAIL"
		if p.Passed {
			glyph = "PASS"
		}
		fmt.Fprintf(out, "  [%s] %-18s %s\n", glyph, p.Name, p.Reason)
	}
	fmt.Fprintln(out)
	if v.Done {
		fmt.Fprintf(out, "VERDICT: DONE — validated->closed is legal for %s\n", v.BeadID)
	} else {
		fmt.Fprintf(out, "VERDICT: NOT DONE — %d gap(s):\n", len(v.Gaps))
		for _, g := range v.Gaps {
			fmt.Fprintf(out, "  - %s\n", g)
		}
	}
	return nil
}
