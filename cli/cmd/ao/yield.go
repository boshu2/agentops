// practices: [dora-metrics, hexagonal-architecture]
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/parser"
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

// yieldGaugeCmd computes and reports the yield vector for one run.
var yieldGaugeCmd = &cobra.Command{
	Use:   "gauge --run <id> [--json] [--c-delta <float>]",
	Short: "Compute the yield gauges (A, Q, A/R, E, L) for a run + print shadow-mode hypotheses",
	Long: `Load the yield ledger, compute the dynamo yield vector for one run, and
print a readable report including the five gauges AND the pre-registered
shadow-mode actuation hypotheses (the knob each gauge WOULD move). The hypotheses
are PRINTED, never auto-steered — auto-tuning is ag-qpg99 (deferred).

Gauges (see .agents/specs/2026-06-14-yield-vector-and-ledger-event-gap.md):
  A      accepted beads (count of accept events)
  Q      difficulty-weighted first-pass yield  [LEAD]
  C      corpus delta, consumed from ag-8p8o (pending if unpublished) [LEAD]
  A/R    accepted / raw input  [WATCH ONLY — Goodhart, never a tuning target]
  E      (ESCALATE+HOLD verdicts) / accepts
  L      loss spend / raw input (read-time join)

C is consumed, never recomputed. Pass --c-delta to supply ag-8p8o's published
delta; omit it to report C as pending.

  ao yield gauge --run run-2026-06-14-dynamo-dogfood
  ao yield gauge --run r1 --json
  ao yield gauge --run r1 --c-delta 0.12`,
	Args: cobra.NoArgs,
	RunE: runYieldGauge,
}

// yieldTokensCmd derives a session's real token footprint from its transcript.
// It is the bronze->silver bridge (age-membrane-memory-arch-tz2s.3.2): a
// bead-tied usage event needs real tokens_in/tokens_out, and the truth lives in
// the session transcript's per-message usage blocks (captured by the parser in
// E4.1). A caller (e.g. reconcile-pr.sh, or `ao orchestrate` once age-tlj6
// lands) feeds the resulting numbers into `ao yield emit usage` instead of the
// env default of 0.
var yieldTokensCmd = &cobra.Command{
	Use:   "tokens --transcript <path> [--json]",
	Short: "Sum the real tokens_in/tokens_out from a session transcript (for a usage event)",
	Long: `Parse a Claude Code or Codex session transcript and sum the real token
footprint from its per-message usage blocks. tokens_in is the full input
footprint (fresh + cache-creation + cache-read); tokens_out is generated tokens.

This derives the truth a bead-tied yield-usage event should carry, replacing the
hardcoded 0 default. Pipe into 'ao yield emit usage':

  read ti to < <(ao yield tokens --transcript "$T" --pair)
  ao yield emit usage --bead "$BEAD" --run "$RUN" \
    --json "{\"tokens_in\":$ti,\"tokens_out\":$to,...}"`,
	Args: cobra.NoArgs,
	RunE: runYieldTokens,
}

// deriveTranscriptTokens parses a session transcript and returns the summed
// real token footprint: tokensIn is the full input footprint across all
// messages (fresh + cache-creation + cache-read), tokensOut is generated
// tokens. It is the testable core of `ao yield tokens`.
func deriveTranscriptTokens(path string) (tokensIn, tokensOut int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	parsed, err := parser.NewParser().Parse(f)
	if err != nil {
		return 0, 0, fmt.Errorf("parse transcript: %w", err)
	}
	// The parser skips malformed lines, so a non-JSONL file parses "successfully"
	// to zero messages. Treat content-that-yields-nothing as an error so a caller
	// (reconcile-pr.sh) takes a VISIBLE fail-open path instead of silently
	// emitting a derived 0 that looks real. A Codex transcript can carry token
	// totals (FinalUsage) with few/no summed messages, so that counts as parseable.
	if len(parsed.Messages) == 0 && parsed.FinalUsage == nil &&
		(parsed.MalformedLines > 0 || parsed.TotalLines > 0) {
		return 0, 0, fmt.Errorf("transcript %s: no parseable messages (%d lines, %d malformed)",
			path, parsed.TotalLines, parsed.MalformedLines)
	}
	// TokenTotals picks the right aggregation per runtime: Codex cumulative
	// total, or Claude per-message usage de-duped by response id.
	tokensIn, tokensOut = parsed.TokenTotals()

	// Usage-absent vs usage-zero: if the transcript carried real work (an
	// assistant/agent turn) but NO usage data was found in any recognized shape
	// (no Codex FinalUsage, no Claude per-message usage), the format's usage went
	// unrecognized — a derived 0 here would be the same silent-0 this command
	// exists to kill, just for an unknown shape. Error so reconcile-pr.sh takes a
	// VISIBLE fail-open path. A transcript that genuinely reports zero usage still
	// carries a usage block (FinalUsage set, or a message.Usage), so it is NOT
	// caught here. forge_tier1 (historical mining) deliberately does NOT apply
	// this — it records 0 for usage-less old sessions rather than failing ingest.
	if parsed.FinalUsage == nil && tokensIn == 0 && tokensOut == 0 && !anyMessageUsage(parsed) {
		if transcriptHasAssistantTurn(parsed) {
			return 0, 0, fmt.Errorf("transcript %s: %d messages with an assistant turn but no usage data found (unrecognized format?)",
				path, len(parsed.Messages))
		}
	}
	return tokensIn, tokensOut, nil
}

// anyMessageUsage reports whether any parsed message carried a usage block.
func anyMessageUsage(parsed *parser.ParseResult) bool {
	for i := range parsed.Messages {
		if parsed.Messages[i].Usage != nil {
			return true
		}
	}
	return false
}

// transcriptHasAssistantTurn reports whether the transcript contains a model
// turn — the signal that work happened and usage should have been recorded.
func transcriptHasAssistantTurn(parsed *parser.ParseResult) bool {
	for i := range parsed.Messages {
		if parsed.Messages[i].Type == "assistant" || parsed.Messages[i].Role == "assistant" {
			return true
		}
	}
	return false
}

func runYieldTokens(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("transcript")
	if path == "" {
		return fmt.Errorf("--transcript <path> is required")
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	asPair, _ := cmd.Flags().GetBool("pair")

	in, out, err := deriveTranscriptTokens(path)
	if err != nil {
		return err
	}
	switch {
	case asJSON:
		b, _ := json.Marshal(map[string]int{"tokens_in": in, "tokens_out": out})
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
	case asPair:
		// Two whitespace-separated values for `read ti to < <(...)`.
		fmt.Fprintf(cmd.OutOrStdout(), "%d %d\n", in, out)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "tokens_in=%d tokens_out=%d\n", in, out)
	}
	return nil
}

func init() {
	yieldEmitCmd.Flags().String("bead", "", "bead id this event is keyed to (required)")
	yieldEmitCmd.Flags().String("run", "", "factory run/cycle id (required)")
	yieldEmitCmd.Flags().String("json", "", "the typed body object as a single JSON blob")
	yieldEmitCmd.Flags().String("ts", "", "optional RFC3339 timestamp; defaults to now (UTC)")
	yieldCmd.AddCommand(yieldEmitCmd)

	yieldGaugeCmd.Flags().String("run", "", "factory run/cycle id to compute gauges for (required)")
	yieldGaugeCmd.Flags().Bool("json", false, "emit the computed gauges as JSON for machine consumption")
	yieldGaugeCmd.Flags().Float64("c-delta", 0, "ag-8p8o's published corpus delta (C); omit to report C as pending")
	yieldCmd.AddCommand(yieldGaugeCmd)

	yieldTokensCmd.Flags().String("transcript", "", "path to a session transcript (JSONL) to sum tokens from (required)")
	yieldTokensCmd.Flags().Bool("json", false, "emit {\"tokens_in\":N,\"tokens_out\":M} as JSON")
	yieldTokensCmd.Flags().Bool("pair", false, "emit two whitespace-separated values: tokens_in tokens_out")
	yieldCmd.AddCommand(yieldTokensCmd)

	rootCmd.AddCommand(yieldCmd)
}

// runYieldGauge wires the cobra invocation to the gauge core.
func runYieldGauge(cmd *cobra.Command, args []string) error {
	root, err := resolveProjectDir()
	if err != nil {
		return err
	}
	run, _ := cmd.Flags().GetString("run")
	if run == "" {
		return fmt.Errorf("--run is required")
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	cDelta, _ := cmd.Flags().GetFloat64("c-delta")
	cKnown := cmd.Flags().Changed("c-delta")

	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	g := yieldledger.ComputeGauges(ledger, run, cDelta, cKnown)
	return writeGaugeReport(cmd.OutOrStdout(), g, asJSON)
}

// gaugeJSON is the machine-output envelope: the gauges plus the shadow-mode
// hypotheses table, so a --json consumer gets both in one document.
type gaugeJSON struct {
	Gauges     yieldledger.Gauges                `json:"gauges"`
	Hypotheses []yieldledger.ActuationHypothesis `json:"shadow_mode_hypotheses"`
}

// writeGaugeReport renders the computed gauges either as JSON or as a readable
// human report including the shadow-mode actuation hypotheses table.
func writeGaugeReport(out io.Writer, g yieldledger.Gauges, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(gaugeJSON{Gauges: g, Hypotheses: yieldledger.ActuationHypotheses()})
	}

	fmt.Fprintf(out, "Yield gauges — run %s\n", g.RunID)
	fmt.Fprintf(out, "(spend measure R = %s)\n\n", g.SpendMeasure)

	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "GAUGE\tVALUE\tROLE")
	fmt.Fprintf(tw, "A (accepted)\t%d\tnumerator (gate-admitted only)\n", g.A)
	if g.Unadmitted > 0 {
		fmt.Fprintf(tw, "  ⚠ unadmitted deposits\t%d\tE-G LEAK — accepts with no CONFIRMED verdict; C/self-excitation SUSPECT\n", g.Unadmitted)
	}
	fmt.Fprintf(tw, "R (raw input)\t%d\tdenominator\n", g.R)
	fmt.Fprintf(tw, "Q (first-pass yield)\t%s\tLEAD\n", fmtRatio(g.Q, g.QDefined))
	fmt.Fprintf(tw, "  Q numerator/denominator\t%.3f / %.3f (%d/%d beads clean)\t\n",
		g.QNumerator, g.QDenominator, g.QCleanBeads, g.QAttemptBeads)
	fmt.Fprintf(tw, "C (corpus delta)\t%s\tLEAD (consumed from ag-8p8o)\n", fmtC(g))
	fmt.Fprintf(tw, "A/R (conversion)\t%s\tWATCH ONLY — Goodhart, never tune\n", fmtRatio(g.AOverR, g.AOverRDefined))
	fmt.Fprintf(tw, "E (escalation rate)\t%s\tautonomy watch (%d ESCALATE/HOLD)\n", fmtRatio(g.E, g.EDefined), g.EEscalateHolds)
	fmt.Fprintf(tw, "catch_rate (membrane)\t%s\tin-situ false-done catch (%d refuted / %d adjudicated)\n",
		fmtCatchRate(g.CatchRate, g.CatchRateNote), g.Refuted, g.Refuted+g.Confirmed)
	fmt.Fprintf(tw, "  cross-family catch_rate\t%s\tdiversity-gated subset\n",
		fmtCatchRate(g.CatchRateCrossFamily, ""))
	fmt.Fprintf(tw, "escape_rate (membrane)\t%s\tconfirms later proven wrong (%d escapes / %d confirmed) — rubber-stamp tell\n",
		fmtCatchRate(g.EscapeRate, g.EscapeRateNote), g.Escapes, g.Confirmed)
	fmt.Fprintf(tw, "L (loss)\t%s\twaste watch\n", fmtRatio(g.L, g.LDefined))
	fmt.Fprintf(tw, "  L breakdown (spend)\trejected=%d rework=%d coord=%d productive=%d\t\n",
		g.LCategory.Rejected, g.LCategory.Rework, g.LCategory.Coordination, g.LCategory.Productive)
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintln(out, "\nShadow-mode actuation hypotheses (PRINTED, not auto-steered — ag-qpg99 deferred):")
	htw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(htw, "IF GAUGE READS…\t…WOULD MOVE THIS KNOB\tMODE")
	for _, h := range yieldledger.ActuationHypotheses() {
		fmt.Fprintf(htw, "%s\t%s\t%s\n", h.Trigger, h.Knob, h.Mode)
	}
	return htw.Flush()
}

// fmtRatio renders a ratio gauge, or "n/a (0 denom)" when undefined so a 0/0
// reads as "no signal", not a misleading 0.000.
func fmtRatio(v float64, defined bool) string {
	if !defined {
		return "n/a (0 denominator)"
	}
	return fmt.Sprintf("%.3f", v)
}

// fmtCatchRate renders the in-situ membrane catch-rate, or the divide-guard note
// when nil so a 0/0 reads as "no signal", not a misleading 0.000.
func fmtCatchRate(v *float64, note string) string {
	if v == nil {
		if note != "" {
			return "n/a (" + note + ")"
		}
		return "n/a (0 denominator)"
	}
	return fmt.Sprintf("%.3f", *v)
}

// fmtC renders the consumed corpus-delta gauge, distinguishing a published delta
// from the pending sentinel (never fabricated).
func fmtC(g yieldledger.Gauges) string {
	if g.CPendingFlag {
		return "pending (ag-8p8o unpublished)"
	}
	return fmt.Sprintf("%.3f", g.CDelta)
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
		in := yieldledger.GateVerdictInput{
			BeadID: bead, RunID: run, TS: ts,
			Difficulty: b.Difficulty, PawlVerdictRef: b.PawlVerdictRef,
			Disposition: b.Disposition, HeadSHA: b.HeadSHA, Attempt: b.Attempt, Mode: b.Mode,
			AuthorContextID: b.AuthorContextID, RefuterFamilies: b.RefuterFamilies,
			AuthorFamily: b.AuthorFamily, CrossFamily: b.CrossFamily,
			AuthorNeReviewer: b.AuthorNeReviewer, EvidencePresent: b.EvidencePresent,
			Domain: b.Domain, Reason: b.Reason,
		}
		// EM.2.1: the writer stamps the escape sentinels (UNCLASSIFIED domain /
		// unspecified reason) when this is an overturning-REFUTED missing either —
		// including the fail-safe path on a degraded ledger. Mirror that here to
		// surface the stamp as visible debt: an unclassified escape is NOT success;
		// it must be classified so derive-checks can route by domain.
		existing, lerr := yieldledger.LoadPath(yieldledger.LedgerPath(root))
		if _, substituted := yieldledger.StampEscapeSentinels(existing, lerr, in); substituted {
			fmt.Fprintf(os.Stderr,
				"⚠ yield: escape recorded with placeholder domain/reason (bead %s, attempt %d) — classify it: ao membrane recall --domain %s\n",
				in.BeadID, in.Attempt, yieldledger.DomainUnclassified)
		}
		if _, err := w.AppendGateVerdict(root, in); err != nil {
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
