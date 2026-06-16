package lifecycle

import (
	"os"
	"strings"
)

// The deposit chokepoint — the pawl made structural (knowledge-field S1).
//
// A knowledge trail's strength may only increase on a *gate-passed outcome*.
// Mere retrieval / co-presence / popularity must NEVER license a deposit — that
// is the positive-feedback loop that drives the ant-mill / confident-slop
// death-spiral, made worse by LLM agents that hallucinate trails. Every reward
// deposit MUST route through GuardDeposit, which requires a GateVerdict.
//
// SCOPE (S1, honest — cross-family review confirmed bypasses): this is the guard
// + the `ao feedback` deposit path. It is NOT yet the single chokepoint —
// other reward-mutating callers still bypass it and must be routed through
// GuardDeposit before strict mode is meaningful: flywheel_citation_feedback,
// feedback_loop (normal + drain), close_loop callback, maturity recalibrate,
// metrics cite, task_sync. That routing is the named S1 follow-up.
//
// Rollout follows the warn-first ratchet: default mode is warn (tolerate a
// missing verdict, but log it) so existing feedback flows keep working; flip to
// strict (AO_DEPOSIT_GATE=strict) only AFTER every deposit path routes through
// here.

// DepositMode controls how a missing gate verdict is treated.
type DepositMode int

const (
	// DepositModeWarn tolerates a missing verdict (logs a warning). The S1
	// rollout default.
	DepositModeWarn DepositMode = iota
	// DepositModeStrict refuses any deposit without a passed verdict. The
	// blocking end-state.
	DepositModeStrict
)

// depositGateEnv selects the deposit mode at runtime.
const depositGateEnv = "AO_DEPOSIT_GATE"

// GateVerdict is the proof that a reward deposit is licensed: the outcome the
// trail contributed to passed a gate. Source names the producer (e.g.
// "pre-land-refuters", "holdout", "hindsight-critic") for provenance.
type GateVerdict struct {
	Passed bool
	Source string
}

// ResolveDepositMode reads AO_DEPOSIT_GATE (warn|strict); defaults to warn.
func ResolveDepositMode() DepositMode {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(depositGateEnv)), "strict") {
		return DepositModeStrict
	}
	return DepositModeWarn
}

// GuardDeposit is the single chokepoint every reward deposit must pass through.
//
//   - a FAILED verdict NEVER deposits, in ANY mode (a gate that rejected the
//     outcome must not strengthen the trail — the anti-death-spiral invariant);
//   - a MISSING (nil) verdict is refused in strict mode, tolerated-with-warning
//     in warn mode (the rollout default);
//   - a PASSED verdict deposits.
//
// Returns (allowed, reason); reason is always set for logging/telemetry.
func GuardDeposit(verdict *GateVerdict, mode DepositMode) (allowed bool, reason string) {
	if verdict == nil {
		if mode == DepositModeStrict {
			return false, "refused: no gate verdict (strict mode)"
		}
		return true, "warn: deposit without gate verdict (warn mode; strict will refuse)"
	}
	if !verdict.Passed {
		return false, "refused: gate verdict did not pass"
	}
	src := verdict.Source
	if src == "" {
		src = "unspecified"
	}
	return true, "allowed: gate verdict passed (" + src + ")"
}
