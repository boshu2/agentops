// Package governor is the membrane's SPC control-loop governor (control-loop-model.md
// §3-§4): the slow-loop setpoint that keeps the self-improving membrane from
// oscillating. SPC.1 (this file) is the error-budget burn-rate ledger — the one
// number that decides ship-vs-harden.
package governor

import (
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// SPC.1 error-budget defaults (control-loop-model.md §4). The error budget is the
// TOP governor: inside tolerance -> keep shipping (hardening would be tampering);
// budget burned -> stop the line and harden. The numbers below are the fixed
// defaults; every one is config-overridable so the rule stays DETERMINISTIC
// (§6.1: no free-form LLM self-grade).
const (
	// DefaultWindowSize is the rolling count of most-recent gate-verdict events the
	// governor judges over. SPC needs a WINDOW (not all-time inertia, not a single
	// point) to see current process behavior — a trend, not noise.
	DefaultWindowSize = 20

	// DefaultToleranceEscapeRate (T) is the tolerated escape rate (escapes /
	// confirmed) within the window. An escape is a membrane MISS: a CONFIRMED a
	// later higher-attempt verdict REFUTED (yieldledger.DetectEscapes / age-6ty).
	DefaultToleranceEscapeRate = 0.10

	// DefaultMinConfirmed is the special-cause floor: the window must hold at least
	// this many CONFIRMED verdicts before "harden" can fire. Without it a single
	// escape in a near-empty window reads as a 100% rate and triggers hardening on
	// common-cause noise — that is tampering, the exact failure SPC forbids.
	DefaultMinConfirmed = 5
)

// Ship and Harden are the two ship-vs-harden decisions.
const (
	Ship   = "ship"
	Harden = "harden"
)

// BudgetConfig parameterizes the SPC error budget. Zero/negative fields fall back
// to the package defaults via Resolve, so callers can pass a partial override.
type BudgetConfig struct {
	WindowSize          int
	ToleranceEscapeRate float64
	MinConfirmed        int
}

// DefaultBudgetConfig returns the canonical SPC.1 configuration.
func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		WindowSize:          DefaultWindowSize,
		ToleranceEscapeRate: DefaultToleranceEscapeRate,
		MinConfirmed:        DefaultMinConfirmed,
	}
}

// Resolve fills any non-positive field from the defaults, so a partial config is
// always usable and the predicate never divides by a zero tolerance.
func (c BudgetConfig) Resolve() BudgetConfig {
	out := c
	if out.WindowSize <= 0 {
		out.WindowSize = DefaultWindowSize
	}
	if out.ToleranceEscapeRate <= 0 {
		out.ToleranceEscapeRate = DefaultToleranceEscapeRate
	}
	if out.MinConfirmed <= 0 {
		out.MinConfirmed = DefaultMinConfirmed
	}
	return out
}

// BudgetVerdict is the deterministic SPC.1 output. Decision is the single
// ship-vs-harden authority; the remaining fields make it auditable.
type BudgetVerdict struct {
	Decision          string  `json:"decision"` // "ship" | "harden"
	BurnRate          float64 `json:"burn_rate"`
	RollingEscapeRate float64 `json:"rolling_escape_rate"`
	EscapesInWindow   int     `json:"escapes_in_window"`
	ConfirmedInWindow int     `json:"confirmed_in_window"`
	VerdictsInWindow  int     `json:"verdicts_in_window"`
	WindowSize        int     `json:"window_size"`
	ToleranceRate     float64 `json:"tolerance_escape_rate"`
	MinConfirmed      int     `json:"min_confirmed"`
	Reason            string  `json:"reason"`
}

// vrow is a flattened gate-verdict marked with the two facts the budget needs.
type vrow struct {
	confirmed bool // disposition == CONFIRMED
	escape    bool // disposition == REFUTED at a strictly higher attempt than a
	// prior CONFIRMED for the SAME bead — i.e. this verdict CAUGHT an earlier
	// false-done the membrane let through (an escape becoming known).
}

// EvaluateBudget computes the SPC error-budget verdict over the rolling window of
// the most-recent cfg.WindowSize gate-verdict events ACROSS ALL RUNS.
//
// burn_rate = rolling_escape_rate / tolerance; harden iff confirmed_in_window >=
// min_confirmed AND burn_rate > 1.0 (else ship). The min_confirmed floor enforces
// the special-cause-only rule: a one-off escape in a thin window cannot harden.
//
// Escape detection mirrors yieldledger.DetectEscapes' definition (a CONFIRMED then
// a strictly-higher-attempt REFUTED for the same bead) but spans runs: the per-bead
// lowest-CONFIRMED-attempt is tracked over the FULL append-ordered prefix, so a
// REFUTED in the window can catch a CONFIRMED that fell BEFORE the window.
func EvaluateBudget(l *yieldledger.Ledger, cfg BudgetConfig) BudgetVerdict {
	cfg = cfg.Resolve()
	v := BudgetVerdict{
		Decision:      Ship,
		WindowSize:    cfg.WindowSize,
		ToleranceRate: cfg.ToleranceEscapeRate,
		MinConfirmed:  cfg.MinConfirmed,
	}
	if l == nil {
		v.Reason = "no ledger: 0 verdicts in window — ship (no signal)"
		return v
	}

	// Walk all gate-verdicts in append order, marking each as confirmed / escape.
	// minConfAttempt[bead] is the lowest CONFIRMED attempt seen so far for that bead.
	// escaped[bead] guards ONE-escape-per-bead (v1 counting, mirroring
	// yieldledger.DetectEscapes): a single escaped CONFIRMED that several later
	// verdicts refute is ONE membrane miss, not one per refutation — counting every
	// refute would inflate the escape rate and harden on a single false-done.
	minConfAttempt := map[string]int{}
	escaped := map[string]bool{}
	var rows []vrow
	for _, ev := range l.Events {
		if ev.Event != yieldledger.EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		gv := ev.GateVerdict
		r := vrow{}
		switch gv.Disposition {
		case yieldledger.DispositionConfirmed:
			r.confirmed = true
			if cur, ok := minConfAttempt[ev.BeadID]; !ok || gv.Attempt < cur {
				minConfAttempt[ev.BeadID] = gv.Attempt
			}
		case yieldledger.DispositionRefuted:
			if cur, ok := minConfAttempt[ev.BeadID]; ok && gv.Attempt > cur && !escaped[ev.BeadID] {
				r.escape = true // FIRST catch of a prior false-done the membrane confirmed
				escaped[ev.BeadID] = true
			}
		}
		rows = append(rows, r)
	}

	// Window = the last WindowSize gate-verdict events.
	start := 0
	if len(rows) > cfg.WindowSize {
		start = len(rows) - cfg.WindowSize
	}
	win := rows[start:]
	v.VerdictsInWindow = len(win)
	for _, r := range win {
		if r.confirmed {
			v.ConfirmedInWindow++
		}
		if r.escape {
			v.EscapesInWindow++
		}
	}

	if v.ConfirmedInWindow > 0 {
		v.RollingEscapeRate = float64(v.EscapesInWindow) / float64(v.ConfirmedInWindow)
	}
	v.BurnRate = v.RollingEscapeRate / cfg.ToleranceEscapeRate

	switch {
	case v.ConfirmedInWindow < cfg.MinConfirmed:
		v.Reason = "below special-cause floor: confirmed_in_window < min_confirmed — ship (insufficient signal; hardening now would be tampering on common-cause noise)"
	case v.BurnRate > 1.0:
		v.Decision = Harden
		v.Reason = "error budget burned: rolling escape rate exceeds tolerance (burn_rate > 1.0) over the window — stop the line and harden before more work flows"
	default:
		v.Reason = "inside tolerance: burn_rate <= 1.0 over the window — keep shipping (do not harden; that would be tampering)"
	}
	return v
}
