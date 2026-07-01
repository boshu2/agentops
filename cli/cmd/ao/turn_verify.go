//go:build legacy

// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/adapters/turnverify"
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

	ledgerPath := turnVerifyLedger
	if ledgerPath == "" {
		ledgerPath = resolveLedgerPath()
	}
	return turnverify.Run(turnverify.Options{
		BeadID:     args[0],
		InputPath:  turnVerifyInput,
		LedgerPath: ledgerPath,
		GraphPath:  turnVerifyGraph,
		JSON:       turnVerifyJSON,
		AllowSelf:  turnVerifyAllowSelf,
		Stdout:     cmd.OutOrStdout(),
	})
}
