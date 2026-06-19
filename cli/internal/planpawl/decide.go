// Package planpawl is the deterministic core of the plan-pawl duel gate
// (docs/contracts/pawls.md). It turns a round of model-family judge verdicts over
// a discovery PLAN artifact into one of three decisions — PASS / REDO / BLOCKED —
// applying the quorum (no-FAIL, ≥2 distinct families) rule and the circuit-breaker
// governance inherited verbatim from pawls.md.
//
// This package contains NO model calls and NO I/O of its own: it is the windshield
// — a pure, deterministic decider. Spawning the panes, waiting on verdict files,
// and the re-judge loop are the skill's job (dual-pane-atm); they feed verdicts in
// and act on the decision out.
package planpawl

import "strings"

// Disposition is a single judge pane's verdict on the plan.
type Disposition string

const (
	// PASS — the judge found no blocking problem with the plan shape.
	PASS Disposition = "PASS"
	// FAIL — the judge refuted the plan; the duel must auto-redo (or break).
	FAIL Disposition = "FAIL"
	// WARN — a non-blocking concern; classified mechanical vs judgment.
	WARN Disposition = "WARN"
)

// WarnClass distinguishes a mechanically-fixable WARN (concrete pseudocode fix,
// auto-applied then re-judged) from a judgment WARN (surfaced, accepted-risk, does
// not block).
type WarnClass string

const (
	// Mechanical — a concrete, auto-applicable fix; triggers auto-apply + re-judge.
	Mechanical WarnClass = "mechanical"
	// Judgment — a value/risk call; surfaced but does not block PASS.
	Judgment WarnClass = "judgment"
)

// JudgeVerdict is one model-family pane's verdict for the current round.
type JudgeVerdict struct {
	Family       string      `json:"family"`
	Disposition  Disposition `json:"disposition"`
	WarnClass    WarnClass   `json:"warn_class,omitempty"`
	JudgmentFlag bool        `json:"judgment_flag,omitempty"`
	Note         string      `json:"note,omitempty"`
}

// Decision is the deterministic outcome of a duel round.
type Decision string

const (
	// DecisionPass — quorum met, no FAIL, no mechanical WARN: the door opens.
	DecisionPass Decision = "PASS"
	// DecisionRedo — the default self-correcting path: re-judge (no human).
	DecisionRedo Decision = "REDO"
	// DecisionBlocked — a circuit breaker tripped: HOLD and escalate (andon).
	DecisionBlocked Decision = "BLOCKED"
)

// Input is the deterministic decider's input for one round.
type Input struct {
	Verdicts    []JudgeVerdict
	Round       int
	MaxRounds   int
	// Oscillation is set by the caller when the SAME failure has repeated across
	// rounds (no-forward-progress); a hard breaker.
	Oscillation bool
}

// Outcome is the decision plus the evidence that produced it.
type Outcome struct {
	Decision       Decision `json:"decision"`
	Round          int      `json:"round"`
	MaxRounds      int      `json:"max_rounds"`
	Families       []string `json:"families"`
	Reason         string   `json:"reason"`
	AutoApplied    []string `json:"auto_applied,omitempty"`
	SurfacedWarns  []string `json:"surfaced_warns,omitempty"`
	BreakerTripped string   `json:"breaker_tripped,omitempty"`
}

// quorumFloor is the minimum number of distinct, roster-validated model families
// required for the multi-model plan-pawl (matches scripts/pawl-verdict.sh).
const quorumFloor = 2

// normalizeFamily collapses aliases to a canonical roster label, mirroring
// scripts/pawl-verdict.sh's normalize_family. An off-roster family returns "".
func normalizeFamily(raw string) string {
	switch raw {
	case "claude", "fable", "anthropic", "Claude", "Fable", "Anthropic":
		return "claude"
	case "gpt", "codex", "openai", "GPT", "Codex", "OpenAI":
		return "gpt"
	case "gemini", "agy", "google", "Gemini", "AGY", "Google":
		return "gemini"
	default:
		return ""
	}
}

// distinctFamilies returns the set of canonical families that actually ran.
func distinctFamilies(vs []JudgeVerdict) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range vs {
		f := normalizeFamily(v.Family)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// Decide applies the deterministic quorum/round/breaker rules. Precedence (high
// to low) — a breaker always wins over the no-FAIL quorum check:
//
//  1. judgment-flag or oscillation  -> BLOCKED (hard breakers)
//  2. round > max-rounds            -> BLOCKED (max-attempts breaker)
//  3. any FAIL                      -> REDO (auto-redo, no human)
//  4. any mechanical WARN           -> REDO (auto-apply the fix, then re-judge)
//  5. fewer than quorumFloor families -> REDO (quorum not met)
//  6. otherwise                     -> PASS (surfacing any judgment WARNs)
func Decide(in Input) Outcome {
	fams := distinctFamilies(in.Verdicts)
	out := Outcome{Round: in.Round, MaxRounds: in.MaxRounds, Families: fams}

	// 0. FAIL-CLOSED on a malformed round budget. Round is 1-based, and the
	// plan-pawl REQUIRES a finite max-attempts breaker (pawls.md) — so round < 1 or
	// max-rounds < 1 is a malformed duel config. BLOCK (andon) rather than risk a
	// silent PASS or an unbounded loop. (The CLI defaults round=1/max-rounds=3, so
	// this guards library callers.)
	if in.Round < 1 || in.MaxRounds < 1 {
		out.Decision = DecisionBlocked
		out.BreakerTripped = "invalid-input"
		out.Reason = "malformed duel config: round and max-rounds must both be >= 1"
		return out
	}

	// 1. Hard breakers — an explicit judgment flag is an immediate escalation, and
	// oscillation is no-forward-progress. Both win over everything else.
	for _, v := range in.Verdicts {
		if v.JudgmentFlag {
			out.Decision = DecisionBlocked
			out.BreakerTripped = "judgment-flag"
			out.Reason = "a reviewer raised an explicit value/irreversibility judgment — models must not decide this alone"
			return out
		}
	}
	if in.Oscillation {
		out.Decision = DecisionBlocked
		out.BreakerTripped = "oscillation"
		out.Reason = "the same failure repeated across rounds (no forward progress)"
		return out
	}

	// 2. max-attempts breaker (max-rounds is guaranteed >= 1 by the step-0 guard).
	if in.Round > in.MaxRounds {
		out.Decision = DecisionBlocked
		out.BreakerTripped = "max-attempts"
		out.Reason = "exhausted the round budget without convergence"
		return out
	}

	// Tally FAILs and classify WARNs. FAIL-CLOSED: any disposition that is not a
	// recognized PASS/FAIL/WARN (missing, empty, or garbage — e.g. a malformed
	// --dir verdict JSON) is counted as a FAIL, never silently treated as clean.
	// The decider is the windshield: it must not trust its inputs.
	var fails int
	for _, v := range in.Verdicts {
		// FAIL-CLOSED on the family too: a pane from an unrecognized/off-roster
		// family is a malformed duel (operator error), not a free pass. It is
		// counted as a refutation so a junk pane can never pad a quorum — e.g.
		// claude:PASS + gpt:PASS + llama:PASS must REDO (fix the setup), never PASS.
		if normalizeFamily(v.Family) == "" {
			fails++
			continue
		}
		switch normDisposition(v.Disposition) {
		case PASS:
			// no-op: a clean pane
		case FAIL:
			fails++
		case WARN:
			// FAIL-CLOSED on the warn_class too: a WARN is non-blocking ONLY when the
			// judge EXPLICITLY declares it. A mechanical WARN auto-applies + re-judges
			// (REDO); a judgment WARN is surfaced + accepted (can PASS). A WARN with a
			// missing/unknown warn_class must NOT silently get the lenient
			// judgment-and-PASS path — it counts as a blocking concern (REDO).
			switch normWarnClass(v.WarnClass) {
			case Mechanical:
				out.AutoApplied = append(out.AutoApplied, normalizeOrRaw(v.Family))
			case Judgment:
				out.SurfacedWarns = append(out.SurfacedWarns, normalizeOrRaw(v.Family))
			default:
				fails++
			}
		default:
			// Unrecognized/missing disposition -> fail-closed.
			fails++
		}
	}

	// 3. Any FAIL -> auto-redo (the default self-correcting path).
	if fails > 0 {
		out.Decision = DecisionRedo
		out.Reason = "at least one family refuted the plan — auto-redo with the findings"
		return out
	}

	// 4. A mechanical WARN -> auto-apply the concrete fix, then re-judge.
	if len(out.AutoApplied) > 0 {
		out.Decision = DecisionRedo
		out.Reason = "mechanical WARN(s) auto-applied — re-judge the corrected plan"
		return out
	}

	// 5. Quorum floor: the multi-model plan-pawl needs >= 2 distinct families.
	if len(fams) < quorumFloor {
		out.Decision = DecisionRedo
		out.Reason = "quorum not met — the multi-model plan-pawl needs >= 2 distinct roster families to have run"
		return out
	}

	// 6. No FAIL, no mechanical WARN, quorum met -> PASS (judgment WARNs surfaced).
	out.Decision = DecisionPass
	if len(out.SurfacedWarns) > 0 {
		out.Reason = "quorum met, no FAIL — PASS with accepted-risk judgment WARN(s) surfaced"
	} else {
		out.Reason = "quorum met, no FAIL, no blocking WARN"
	}
	return out
}

// normDisposition canonicalizes a disposition case-insensitively to PASS/FAIL/WARN.
// Anything else (empty or unrecognized) returns a sentinel that the caller treats
// as fail-closed.
func normDisposition(d Disposition) Disposition {
	switch strings.ToUpper(strings.TrimSpace(string(d))) {
	case "PASS":
		return PASS
	case "FAIL":
		return FAIL
	case "WARN":
		return WARN
	default:
		return Disposition("") // unrecognized -> fail-closed at the call site
	}
}

// normWarnClass canonicalizes a warn_class case-insensitively to mechanical/judgment.
// Anything else (empty or unrecognized) returns "" so the caller fail-closes.
func normWarnClass(w WarnClass) WarnClass {
	switch strings.ToLower(strings.TrimSpace(string(w))) {
	case "mechanical":
		return Mechanical
	case "judgment":
		return Judgment
	default:
		return WarnClass("")
	}
}

// normalizeOrRaw returns the canonical family or, for an off-roster label, the raw
// value (so surfaced/auto-applied evidence still names who reported it).
func normalizeOrRaw(raw string) string {
	if f := normalizeFamily(raw); f != "" {
		return f
	}
	return raw
}
