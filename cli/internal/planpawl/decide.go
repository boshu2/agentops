// Package planpawl is the deterministic core of the plan-pawl duel gate
// (docs/contracts/pawls.md). It turns a round of model-family judge verdicts over
// a discovery PLAN artifact into one of three decisions — PASS / REDO / BLOCKED —
// applying the quorum (no-FAIL, ≥2 distinct families) rule and the circuit-breaker
// governance inherited verbatim from pawls.md.
//
// This package contains NO model calls and NO I/O of its own: it is the windshield
// — a pure, deterministic decider. Spawning the panes, waiting on verdict files,
// and the re-judge loop are the skill's job (dueling-idea-genies); they feed verdicts in
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

// FailureClass records an infrastructure (not judgment) failure on a judge lane,
// distinguishing a retryable outage from a genuine refutation. A judge that
// provider-timed-out / rate-limited / produced no verdict must NOT be recorded as
// a REFUTED — that is the age-5olx incident this type fixes.
type FailureClass string

const (
	// FailureNone — the lane ran cleanly (no infrastructure failure).
	FailureNone FailureClass = "none"
	// FailureTransient — a retryable outage (rate-limit / timeout / no-verdict). NOT
	// a refutation: excluded from the FAIL tally and from quorum coverage.
	FailureTransient FailureClass = "transient"
	// FailureHard — a non-retryable infrastructure failure. Counted fail-closed as a
	// refutation (unchanged).
	FailureHard FailureClass = "hard"
)

// JudgeVerdict is one model-family pane's verdict for the current round.
type JudgeVerdict struct {
	Family       string      `json:"family"`
	Disposition  Disposition `json:"disposition"`
	WarnClass    WarnClass   `json:"warn_class,omitempty"`
	JudgmentFlag bool        `json:"judgment_flag,omitempty"`
	Note         string      `json:"note,omitempty"`
	// FailureClass / FailureReason record an infrastructure failure on this lane
	// (a provider timeout, rate-limit, or no-verdict). A transient class makes the
	// lane a retryable outage, not a refutation; see ClassifyFailure.
	FailureClass  FailureClass `json:"failure_class,omitempty"`
	FailureReason string       `json:"failure_reason,omitempty"`
}

// Decision is the deterministic outcome of a duel round.
type Decision string

const (
	// DecisionPass — quorum met, no FAIL, no mechanical WARN: the door opens.
	DecisionPass Decision = "PASS"
	// DecisionRedo — the default self-correcting path: re-judge (no human).
	DecisionRedo Decision = "REDO"
	// DecisionBlocked — a circuit breaker tripped: HOLD, then route per the andon
	// ladder (one bounded helper pass; human only past it — pawls.md §Escalation).
	DecisionBlocked Decision = "BLOCKED"
	// DecisionDegraded — no genuine FAIL / mechanical-WARN / breaker fired, but
	// transient lane loss dropped distinct-family coverage below the quorum floor.
	// Retryable: re-run the PANEL (the work is fine; the judges timed out), NOT the
	// work. This is distinct from REDO so a caller never redoes good work over an
	// infrastructure outage.
	DecisionDegraded Decision = "DEGRADED"
)

// Input is the deterministic decider's input for one round.
type Input struct {
	Verdicts  []JudgeVerdict
	Round     int
	MaxRounds int
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
	// Degraded is set when >= 1 judge lane transiently failed (a retryable outage,
	// not a refutation). DegradedFamilies names those lanes' families. On a PASS it
	// means the quorum floor still held on the surviving families; the DEGRADED
	// decision means transient loss dropped coverage below the floor.
	Degraded         bool     `json:"degraded,omitempty"`
	DegradedFamilies []string `json:"degraded_families,omitempty"`
}

// quorumFloor is the minimum number of distinct, roster-validated model families
// required for the multi-model plan-pawl (matches scripts/pawl-verdict.sh).
const quorumFloor = 2

// transientFailureReasons is the reason-token table that classifies an
// infrastructure failure as retryable when no explicit FailureClass is set. It
// mirrors the pawl-review.sh 529-class / no-verdict transport taxonomy.
var transientFailureReasons = map[string]bool{
	"rate_limited":          true,
	"provider_rate_limited": true,
	"provider_unavailable":  true,
	"provider_timeout":      true,
	"temporary_unavailable": true,
	"transport_interrupted": true,
	"timeout":               true,
	"no_verdict":            true,
}

// transientDispositionTokens are the raw disposition sentinels pawl-review.sh
// writes when a lane produced no trustworthy verdict (a stall / no-verdict). A
// disposition equal to one of these IS the transient signal — there is no separate
// FailureClass on the lane.
var transientDispositionTokens = map[string]bool{
	"<timeout>":  true,
	"timeout":    true,
	"no verdict": true,
	"no-verdict": true,
	"no_verdict": true,
}

// ClassifyFailure normalizes a lane's (class, reason) into the none/transient/hard
// contract. Rules, fail-safe by design (never a silent pass, never a false refute):
//   - explicit transient / hard class          -> that class (wins over the reason)
//   - explicit none, or empty class + empty reason -> none (a clean lane)
//   - empty class + a reason in the token table -> transient
//   - empty class + an off-table reason         -> hard (fail-closed)
//   - an UNRECOGNIZED class token WITH a reason  -> transient (an outage was
//     reported; degrade rather than falsely refute)
//   - an UNRECOGNIZED class token with no reason -> hard (no outage evidence)
func ClassifyFailure(rawClass FailureClass, rawReason string) (FailureClass, string) {
	class := normalizeToken(string(rawClass))
	reason := normalizeToken(rawReason)
	if class == "" && reason == "" {
		return FailureNone, ""
	}
	switch FailureClass(class) {
	case FailureNone:
		return FailureNone, reason
	case FailureTransient:
		return FailureTransient, reason
	case FailureHard:
		return FailureHard, reason
	case "":
		if transientFailureReasons[reason] {
			return FailureTransient, reason
		}
		return FailureHard, reason
	default:
		if reason != "" {
			return FailureTransient, reason
		}
		return FailureHard, reason
	}
}

// laneFailureClass is ClassifyFailure over a whole verdict, folding in the raw
// disposition transport sentinels: an explicit transient/hard class from the
// fields wins; otherwise a disposition that is itself a sentinel ("<timeout>", "no
// verdict", ...) makes the lane transient.
func laneFailureClass(v JudgeVerdict) FailureClass {
	class, _ := ClassifyFailure(v.FailureClass, v.FailureReason)
	if class == FailureTransient || class == FailureHard {
		return class
	}
	if isTransientDisposition(v.Disposition) {
		return FailureTransient
	}
	return FailureNone
}

// isTransientDisposition reports whether a raw disposition is a pawl-review.sh
// transport sentinel (case-insensitive).
func isTransientDisposition(d Disposition) bool {
	return transientDispositionTokens[normalizeToken(string(d))]
}

// normalizeToken lowercases and trims a class/reason/disposition token.
func normalizeToken(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

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

func tallyVerdicts(verdicts []JudgeVerdict, out *Outcome) (fails int, surviving, degraded []string) {
	seenSurviving := map[string]bool{}
	seenDegraded := map[string]bool{}
	for _, v := range verdicts {
		switch laneFailureClass(v) {
		case FailureTransient:
			fam := normalizeOrRaw(v.Family)
			if !seenDegraded[fam] {
				seenDegraded[fam] = true
				degraded = append(degraded, fam)
			}
			continue
		case FailureHard:
			fails++
			continue
		}
		family := normalizeFamily(v.Family)
		if family == "" {
			fails++
			continue
		}
		if !seenSurviving[family] {
			seenSurviving[family] = true
			surviving = append(surviving, family)
		}
		switch normDisposition(v.Disposition) {
		case PASS:
		case FAIL:
			fails++
		case WARN:
			switch normWarnClass(v.WarnClass) {
			case Mechanical:
				out.AutoApplied = append(out.AutoApplied, normalizeOrRaw(v.Family))
			case Judgment:
				out.SurfacedWarns = append(out.SurfacedWarns, normalizeOrRaw(v.Family))
			default:
				fails++
			}
		default:
			fails++
		}
	}
	return fails, surviving, degraded
}

// Decide applies the deterministic quorum/round/breaker rules. Precedence (high
// to low) — a breaker always wins over the no-FAIL quorum check:
//
//  1. judgment-flag or oscillation      -> BLOCKED (hard breakers)
//  2. round > max-rounds                -> BLOCKED (max-attempts breaker)
//  3. any FAIL or hard infra failure    -> REDO (auto-redo, no human)
//  4. any mechanical WARN               -> REDO (auto-apply the fix, then re-judge)
//  5. surviving distinct families < floor:
//     - if transient lane loss caused it -> DEGRADED (retryable: re-run the panel)
//     - otherwise                        -> REDO (quorum genuinely not met)
//  6. otherwise                         -> PASS (Degraded=true when the floor still
//     held despite a transient lane loss;
//     surfacing any judgment WARNs)
//
// A lane that transiently failed — a retryable outage (an explicit transient
// FailureClass, a reason in the transient token table, or a raw disposition that is
// itself a transport sentinel like "<timeout>" / "no verdict") — is NOT a
// refutation: it is excluded from the FAIL tally AND from quorum coverage, and
// recorded in DegradedFamilies. This is the fix for the age-5olx incident, where a
// warm panel whose codex+agy panes timed out was recorded REFUTED at 1/3 coverage.
// A hard infra failure, by contrast, stays fail-closed (counted as a refutation).
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

	// Tally FAILs, classify WARNs, and separate transient lane loss from genuine
	// refutations. FAIL-CLOSED: any disposition that is not a recognized
	// PASS/FAIL/WARN (missing, empty, or garbage — e.g. a malformed --dir verdict
	// JSON) is counted as a FAIL, never silently treated as clean. The decider is
	// the windshield: it must not trust its inputs.
	fails, surviving, degraded := tallyVerdicts(in.Verdicts, &out)
	out.DegradedFamilies = degraded
	out.Degraded = len(degraded) > 0

	// 3. Any FAIL (incl. a hard infra failure) -> auto-redo (the self-correcting
	// path). A genuine refutation outranks transient degradation.
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

	// 5. Quorum floor over the SURVIVING (non-transient) families. If transient lane
	// loss dropped coverage below the floor, the decision is DEGRADED — retryable,
	// re-run the panel (the work is fine) — NEVER a silent PASS and never a false
	// REDO/REFUTED. Without any transient loss, too few families is the ordinary
	// quorum-not-met REDO (operator setup error).
	if len(surviving) < quorumFloor {
		if out.Degraded {
			out.Decision = DecisionDegraded
			out.Reason = "degraded coverage — transient lane loss (" + strings.Join(degraded, ", ") +
				") dropped distinct-family coverage below quorum; retryable — re-run the panel, not the work"
			return out
		}
		out.Decision = DecisionRedo
		out.Reason = "quorum not met — the multi-model plan-pawl needs >= 2 distinct roster families to have run"
		return out
	}

	// 6. No FAIL, no mechanical WARN, quorum met by the surviving families -> PASS.
	out.Decision = DecisionPass
	switch {
	case out.Degraded && len(out.SurfacedWarns) > 0:
		out.Reason = "quorum met by surviving families despite transient lane loss — PASS (degraded coverage; judgment WARN(s) surfaced)"
	case out.Degraded:
		out.Reason = "quorum met by surviving families despite transient lane loss — PASS with degraded coverage"
	case len(out.SurfacedWarns) > 0:
		out.Reason = "quorum met, no FAIL — PASS with accepted-risk judgment WARN(s) surfaced"
	default:
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
