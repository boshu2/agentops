package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/governor"
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// governorCmd is the parent for the membrane's SPC control-loop governor
// (control-loop-model.md §3-§4). Today it carries SPC.1 (budget); SPC.2/SPC.3
// (noise-band, consolidated breakers) land under it later.
var governorCmd = &cobra.Command{
	Use:   "governor",
	Short: "The membrane's SPC control-loop governor (error budget, special-cause)",
	Long: `The slow-loop setpoint that keeps the self-improving membrane from oscillating
(control-loop-model.md §3-§4). Subcommands expose the governor's deterministic
decisions; nothing here reasons about the work — it reads ground truth (the yield
ledger) and applies a fixed rule.`,
}

var (
	govBudgetJSON      bool
	govBudgetWindow    int
	govBudgetTolerance float64
	govBudgetMinConf   int
)

var governorBudgetCmd = &cobra.Command{
	Use:   "budget [--json] [--window N] [--tolerance T] [--min-confirmed N]",
	Short: "Error-budget burn-rate: the one number that decides ship-vs-harden",
	Long: `SPC.1 (control-loop-model.md §4). The ERROR BUDGET is the top governor: inside
tolerance -> keep shipping (hardening would be tampering); budget burned -> stop
the line and harden before more work flows.

Reads the yield ledger and computes, over a rolling window of the most-recent
gate-verdicts ACROSS ALL RUNS:

    rolling_escape_rate = escapes_in_window / confirmed_in_window
    burn_rate           = rolling_escape_rate / tolerance
    decision            = HARDEN iff confirmed_in_window >= min_confirmed
                                  AND burn_rate > 1.0,  else SHIP

An escape is a membrane MISS: a CONFIRMED a later, higher-attempt verdict REFUTED.
The min_confirmed floor enforces special-cause-only adjustment — a one-off escape
in a thin window cannot harden (that would be tampering on common-cause noise).

The decision is DETERMINISTIC (no LLM self-grade) and derive-on-read (no second
source of truth). Exit status: 0 = ship, 3 = harden (so "stop the line" is
mechanical for a calling gate/loop).`,
	RunE: runGovernorBudget,
}

var (
	govNoiseJSON   bool
	govNoiseWindow int
	govNoiseLimit  int
)

var governorNoiseBandCmd = &cobra.Command{
	Use:   "noise-band [--json] [--window N] [--limit K]",
	Short: "Special-cause noise-band: should the membrane ADJUST, or is this common-cause noise?",
	Long: `SPC.2 (control-loop-model.md §4). The membrane adjusts ONLY on a special-cause
signal — a repeated escape pattern past a control limit — never on common-cause
noise (a one-off), which is tampering that makes the membrane oscillate (cry-wolf).

Reads the yield ledger and, over the rolling window of the most-recent
gate-verdicts across all runs, counts escapes per DOMAIN (one escape per bead).
A domain with >= K escapes is a special-cause pattern -> ADJUST (a derived gate
is warranted for that domain). Otherwise -> HOLD.

Informational (exit 0); the decision is in the output. This is a recommendation
to the slow loop, not a stop-the-line gate.`,
	RunE: runGovernorNoiseBand,
}

func init() {
	governorBudgetCmd.Flags().BoolVar(&govBudgetJSON, "json", false, "Emit the verdict as JSON")
	governorBudgetCmd.Flags().IntVar(&govBudgetWindow, "window", 0, "Rolling window size (gate-verdicts); 0 = default")
	governorBudgetCmd.Flags().Float64Var(&govBudgetTolerance, "tolerance", 0, "Tolerated escape rate T; 0 = default")
	governorBudgetCmd.Flags().IntVar(&govBudgetMinConf, "min-confirmed", 0, "Special-cause floor: min confirmed in window before harden can fire; 0 = default")
	governorCmd.AddCommand(governorBudgetCmd)

	governorNoiseBandCmd.Flags().BoolVar(&govNoiseJSON, "json", false, "Emit the verdict as JSON")
	governorNoiseBandCmd.Flags().IntVar(&govNoiseWindow, "window", 0, "Rolling window size (gate-verdicts); 0 = default")
	governorNoiseBandCmd.Flags().IntVar(&govNoiseLimit, "limit", 0, "Special-cause control limit K (escapes per domain in window); 0 = default")
	governorCmd.AddCommand(governorNoiseBandCmd)

	rootCmd.AddCommand(governorCmd)
}

func runGovernorNoiseBand(cmd *cobra.Command, _ []string) error {
	root, err := resolveProjectDir()
	if err != nil {
		return err
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	v := governor.ShouldAdjust(ledger, governor.NoiseBandConfig{
		WindowSize:        govNoiseWindow,
		SpecialCauseLimit: govNoiseLimit,
	})
	out := cmd.OutOrStdout()
	if govNoiseJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	fmt.Fprintf(out, "governor noise-band: %s\n", v.Decision)
	fmt.Fprintf(out, "  window %d verdicts, control limit K=%d escapes/domain\n", v.WindowSize, v.SpecialCauseLimit)
	if len(v.SpecialCauseDomains) > 0 {
		fmt.Fprintf(out, "  special-cause domains: %v\n", v.SpecialCauseDomains)
	}
	fmt.Fprintf(out, "  %s\n", v.Reason)
	return nil
}

// hardenExitCode is returned when the budget is burned, so a calling gate/loop can
// mechanically "stop the line". Distinct from 1 (generic error) so callers can tell
// "budget burned" from "command failed".
const hardenExitCode = 3

// governorExitError carries the governor's decision as a process exit code:
// 3 = harden (stop the line). Mapped to os.Exit in root.go's Execute.
type governorExitError struct {
	code int
	msg  string
}

func (e *governorExitError) Error() string { return e.msg }
func (e *governorExitError) ExitCode() int { return e.code }

func runGovernorBudget(cmd *cobra.Command, _ []string) error {
	root, err := resolveProjectDir()
	if err != nil {
		return err
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}

	cfg := governor.BudgetConfig{
		WindowSize:          govBudgetWindow,
		ToleranceEscapeRate: govBudgetTolerance,
		MinConfirmed:        govBudgetMinConf,
	}
	v := governor.EvaluateBudget(ledger, cfg)

	out := cmd.OutOrStdout()
	if govBudgetJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "governor budget: %s\n", v.Decision)
		fmt.Fprintf(out, "  burn_rate           %.3f  (rolling_escape_rate %.3f / tolerance %.3f)\n", v.BurnRate, v.RollingEscapeRate, v.ToleranceRate)
		fmt.Fprintf(out, "  window              %d verdicts (%d in window): %d confirmed, %d escapes\n", v.WindowSize, v.VerdictsInWindow, v.ConfirmedInWindow, v.EscapesInWindow)
		fmt.Fprintf(out, "  min_confirmed floor %d\n", v.MinConfirmed)
		fmt.Fprintf(out, "  %s\n", v.Reason)
	}

	if v.Decision == governor.Harden {
		// Silence cobra's usage/error noise; the verdict is the message.
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return &governorExitError{code: hardenExitCode, msg: "governor budget: harden — error budget burned"}
	}
	return nil
}
