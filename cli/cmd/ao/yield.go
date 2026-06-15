// practices: [dora-metrics, hexagonal-architecture]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// strictUnmarshalBody decodes a typed yield body rejecting unknown fields, so a
// misspelled or extra key on an `ao yield emit ... --json` blob fails loudly
// (closed schema, additionalProperties:false) instead of silently dropping.
func strictUnmarshalBody(jsonBody string, dst any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonBody)))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// yieldCmd is the parent for the dynamo yield ledger surfaces. Today it carries
// one verb — `emit` — a thin wrapper so shell scripts and an orchestrator can
// append a durable, bead-keyed operational event (accept | gate-verdict |
// usage). Computing the gauges (A, R, A/R, Q, E, L) is ag-qzinh, not here.
var yieldCmd = &cobra.Command{
	Use:   "yield",
	Short: "Record dynamo yield-ledger events (per-bead operational stream)",
	Long: `Record durable, append-only, bead-keyed operational events for the
agent factory's yield vector (see .agents/specs/2026-06-14-yield-vector-and-ledger-event-gap.md).

Emission is fail-open observability, NOT a gate: callers guard each invocation
(` + "`|| true`" + `) so a failed emit never blocks a merge.`,
}

// yieldEmitCmd appends one event to the yield ledger.
var yieldEmitCmd = &cobra.Command{
	Use:   "emit <accept|gate-verdict|usage> --bead <id> --run <id> [--json <body> | typed flags]",
	Short: "Append one accept|gate-verdict|usage event to the yield ledger",
	Long: `Append one operational event keyed by bead to the yield ledger
(.agents/yield/yield-ledger.jsonl, append-only JSONL). The body is supplied either as a single
--json blob (the typed body object per the schema) or via the typed flags for
that event kind.

Examples:
  ao yield emit accept --bead ag-x --run r1 \
    --json '{"merge_sha":"def5678","merged_by":"orch","gate_verdict_ref":{"bead_id":"ag-x","head_sha":"abc1234"}}'
  ao yield emit gate-verdict --bead ag-x --run r1 \
    --json '{"difficulty":3,"pawl_verdict_ref":{"bead_id":"ag-x","head_sha":"abc1234"},"disposition":"CONFIRMED","head_sha":"abc1234","attempt":1,"author_context_id":"ctx-1","refuter_families":["claude","gpt"],"author_family":"claude","cross_family":true,"author_ne_reviewer":true,"evidence_present":true}'
  ao yield emit usage --bead ag-x --run r1 \
    --json '{"tokens_in":100,"tokens_out":20,"cost_usd":0.3,"wall_clock_s":60,"model":"claude-opus-4-8","phase":"implement","category_hint":"productive"}'`,
	Args: cobra.ExactArgs(1),
	RunE: runYieldEmit,
}

func init() {
	yieldEmitCmd.Flags().String("bead", "", "bead id this event is keyed to (required)")
	yieldEmitCmd.Flags().String("run", "", "factory run/cycle id (required)")
	yieldEmitCmd.Flags().String("json", "", "the typed body object as a single JSON blob")
	yieldEmitCmd.Flags().String("ts", "", "optional RFC3339 timestamp; defaults to now (UTC)")
	yieldCmd.AddCommand(yieldEmitCmd)
	rootCmd.AddCommand(yieldCmd)
}

// runYieldEmit wires the cobra invocation to the emit core.
func runYieldEmit(cmd *cobra.Command, args []string) error {
	root, err := resolveProjectDir()
	if err != nil {
		return err
	}
	bead, _ := cmd.Flags().GetString("bead")
	run, _ := cmd.Flags().GetString("run")
	jsonBody, _ := cmd.Flags().GetString("json")
	tsRaw, _ := cmd.Flags().GetString("ts")

	ts := time.Now().UTC()
	if tsRaw != "" {
		parsed, perr := time.Parse(time.RFC3339, tsRaw)
		if perr != nil {
			return fmt.Errorf("invalid --ts (want RFC3339): %w", perr)
		}
		ts = parsed
	}

	return emitYieldEvent(root, args[0], bead, run, ts, jsonBody)
}

// emitYieldEvent appends one event of the named kind. It is the testable core:
// it validates the envelope inputs, decodes the typed body, and appends through
// the yieldledger Writer (atomic, append-preserving).
func emitYieldEvent(root, kind, bead, run string, ts time.Time, jsonBody string) error {
	if bead == "" {
		return fmt.Errorf("--bead is required")
	}
	if run == "" {
		return fmt.Errorf("--run is required")
	}
	if jsonBody == "" {
		return fmt.Errorf("--json body is required")
	}
	w := yieldledger.Writer{}

	switch kind {
	case yieldledger.EventAccept:
		var b yieldledger.AcceptBody
		if err := strictUnmarshalBody(jsonBody, &b); err != nil {
			return fmt.Errorf("parse accept body: %w", err)
		}
		if _, err := w.AppendAccept(root, yieldledger.AcceptInput{
			BeadID: bead, RunID: run, TS: ts,
			MergeSHA: b.MergeSHA, MergedBy: b.MergedBy, GateVerdictRef: b.GateVerdictRef,
		}); err != nil {
			return err
		}
	case yieldledger.EventGateVerdict:
		var b yieldledger.GateVerdictBody
		if err := strictUnmarshalBody(jsonBody, &b); err != nil {
			return fmt.Errorf("parse gate-verdict body: %w", err)
		}
		if _, err := w.AppendGateVerdict(root, yieldledger.GateVerdictInput{
			BeadID: bead, RunID: run, TS: ts,
			Difficulty: b.Difficulty, PawlVerdictRef: b.PawlVerdictRef,
			Disposition: b.Disposition, HeadSHA: b.HeadSHA, Attempt: b.Attempt, Mode: b.Mode,
			AuthorContextID: b.AuthorContextID, RefuterFamilies: b.RefuterFamilies,
			AuthorFamily: b.AuthorFamily, CrossFamily: b.CrossFamily,
			AuthorNeReviewer: b.AuthorNeReviewer, EvidencePresent: b.EvidencePresent,
		}); err != nil {
			return err
		}
	case yieldledger.EventUsage:
		var b yieldledger.UsageBody
		if err := strictUnmarshalBody(jsonBody, &b); err != nil {
			return fmt.Errorf("parse usage body: %w", err)
		}
		if _, err := w.AppendUsage(root, yieldledger.UsageInput{
			BeadID: bead, RunID: run, TS: ts,
			TokensIn: b.TokensIn, TokensOut: b.TokensOut, CostUSD: b.CostUSD,
			WallClockS: b.WallClockS, Model: b.Model, Phase: b.Phase, CategoryHint: b.CategoryHint,
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown event kind %q (want accept|gate-verdict|usage)", kind)
	}
	return nil
}
