// This file implements wiki.FreshnessPolicy — the claim-level, evidence-driven
// freshness model born from the 2026-05-17 dueling-idea-wizards brainstorm.
//
// SPIKE-FIRST. The freshness model is new and unproven; this is a deliberately
// small, deterministic first cut. The scoring below is documented inline so a
// later wave can revise the constants without re-deriving the intent.
//
// The model: freshness = volatility_class × authority_class × change_signal.
//
//   - VolatilityClass (from claim.go) sets how fast a Claim decays. It governs
//     the base review interval and how aggressively a change signal ages the
//     Claim.
//   - AuthorityClass (from claim.go) is a damping factor: a high-authority
//     source (code) is trusted to stay correct longer than a low-authority one
//     (external prose), so it lengthens the review interval.
//   - ChangeSignal is observed evidence that the world moved (a newer release,
//     a changed file hash, a bumped schema version). A signal that post-dates
//     the Claim's last verification is what actually pushes a Claim out of the
//     fresh state.
//
// The output is a closed set of three FreshnessState values (fresh, aging,
// stale) plus a machine-readable Reason, a next_review_at timestamp, and a
// suggested RefreshAction. Stale never means "delete" — a stale Claim is still
// renderable, just flagged for re-verification.
package wiki

import (
	"fmt"
	"strings"
	"time"
)

// FreshnessState is the closed set of freshness verdicts a FreshnessPolicy can
// return for a Claim. The set is intentionally small for the spike: a later
// wave may add "expired" or "unknown", but three states are enough to satisfy
// the W3 acceptance criterion and keep the model legible.
type FreshnessState string

const (
	// FreshnessFresh marks a Claim whose backing evidence has not been
	// contradicted by any observed change signal and whose review interval
	// has not elapsed.
	FreshnessFresh FreshnessState = "fresh"
	// FreshnessAging marks a Claim that is past or near its review interval,
	// or that has a change signal of moderate concern. It is still usable but
	// should be re-verified soon.
	FreshnessAging FreshnessState = "aging"
	// FreshnessStale marks a Claim whose evidence is very likely outdated — a
	// change signal directly invalidates it, or its review interval is far
	// exceeded. A stale Claim is re-verified, not deleted.
	FreshnessStale FreshnessState = "stale"
)

// validFreshnessState is the closed set of recognized freshness states.
var validFreshnessState = map[FreshnessState]struct{}{
	FreshnessFresh: {},
	FreshnessAging: {},
	FreshnessStale: {},
}

// Valid reports whether s is a recognized FreshnessState.
func (s FreshnessState) Valid() bool {
	_, ok := validFreshnessState[s]
	return ok
}

// ChangeSignalKind discriminates the source of a ChangeSignal — the evidence
// that the world has moved since a Claim was last verified.
type ChangeSignalKind string

const (
	// ChangeSignalNone is the zero value: no change has been observed. A
	// Claim evaluated against a none-kind signal can only age via its review
	// interval, never via contradiction.
	ChangeSignalNone ChangeSignalKind = ""
	// ChangeSignalRelease marks a newer software release than the one the
	// Claim was verified against (the W3 acceptance-criterion signal).
	ChangeSignalRelease ChangeSignalKind = "release"
	// ChangeSignalFileHash marks a changed content hash for a file the Claim
	// cites as a source.
	ChangeSignalFileHash ChangeSignalKind = "file-hash"
	// ChangeSignalSchemaVersion marks a bumped schema/contract version.
	ChangeSignalSchemaVersion ChangeSignalKind = "schema-version"
)

// ChangeSignal is the observed evidence input to FreshnessPolicy.Evaluate. A
// signal is "relevant" when ObservedAt post-dates the Claim's VerifiedAt: only
// changes that happened after the Claim was last checked can invalidate it.
//
// The zero ChangeSignal (Kind == ChangeSignalNone) means "no change observed";
// Evaluate treats it as evidence of nothing, not evidence of staleness.
type ChangeSignal struct {
	// Kind is the source of the change signal. The zero value
	// (ChangeSignalNone) means no change was observed.
	Kind ChangeSignalKind
	// ObservedAt is when the change was detected. A signal is only relevant
	// when ObservedAt is after the Claim's VerifiedAt.
	ObservedAt time.Time
	// Detail is an opaque human-readable description of the change (e.g. a
	// version string or a commit hash). Optional; not interpreted.
	Detail string
}

// relevant reports whether the signal carries an actual observed change that
// post-dates the Claim's last verification.
func (s ChangeSignal) relevant(verifiedAt time.Time) bool {
	if s.Kind == ChangeSignalNone {
		return false
	}
	// A signal with a zero ObservedAt is treated as "just now" — it is a
	// freshly observed change with no recorded timestamp.
	if s.ObservedAt.IsZero() {
		return true
	}
	return s.ObservedAt.After(verifiedAt)
}

// FreshnessResult is the verdict FreshnessPolicy.Evaluate returns for a Claim.
type FreshnessResult struct {
	// State is the freshness verdict (fresh, aging, or stale).
	State FreshnessState `json:"state"`
	// Reason is a short machine-stable explanation of why State was chosen.
	Reason string `json:"reason"`
	// NextReviewAt is when the Claim should next be re-verified. It is always
	// set to a non-zero time by Evaluate: a fresh Claim gets a far-future
	// review date, an aging/stale one a near-future (or already-elapsed) one.
	NextReviewAt time.Time `json:"next_review_at"`
	// RefreshAction is a suggested follow-up (empty for a fresh Claim).
	RefreshAction string `json:"refresh_action,omitempty"`
}

// baseReviewInterval is the spike's per-volatility review interval — how long a
// Claim of each volatility class stays fresh in the absence of any change
// signal. These constants are the model's primary tuning knobs; a later wave
// should revisit them with real corpus evidence.
//
//   - invariant:    one year   (rarely needs review at all)
//   - release-bound: 30 days   (one typical release cadence)
//   - fast:          3 days
//   - ephemeral:     6 hours
//
// An unrecognized volatility class falls back to the release-bound interval —
// a conservative middle.
func baseReviewInterval(v VolatilityClass) time.Duration {
	switch v {
	case VolatilityInvariant:
		return 365 * 24 * time.Hour
	case VolatilityReleaseBound:
		return 30 * 24 * time.Hour
	case VolatilityFast:
		return 3 * 24 * time.Hour
	case VolatilityEphemeral:
		return 6 * time.Hour
	default:
		return 30 * 24 * time.Hour
	}
}

// authorityMultiplier damps the base review interval by source authority. A
// high-authority Claim (sourced from code) is trusted to stay correct longer,
// so its interval is stretched; a low-authority Claim (external prose) is
// reviewed sooner. The multipliers are deliberately mild (0.5–1.5×) so
// volatility, not authority, remains the dominant term.
//
// An unrecognized authority class falls back to 1.0 (no damping).
func authorityMultiplier(a AuthorityClass) float64 {
	switch a {
	case AuthorityCode:
		return 1.5
	case AuthorityGenerated:
		return 1.25
	case AuthoritySchema:
		return 1.1
	case AuthorityAgents:
		return 1.0
	case AuthorityExternal:
		return 0.5
	default:
		return 1.0
	}
}

// FreshnessPolicy evaluates a Claim against observed change signals and a
// reference clock, returning a FreshnessResult. The zero value is usable: it
// evaluates against the system clock via time.Now.
//
// FreshnessPolicy is the production implementation of
// ports.FreshnessPolicyPort.
type FreshnessPolicy struct {
	// Now, when non-nil, supplies the reference time for Evaluate. It exists
	// so tests can pin a deterministic clock. When nil, time.Now is used.
	Now func() time.Time
}

// now returns the policy's reference time, defaulting to time.Now.
func (p FreshnessPolicy) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Evaluate computes the FreshnessState of claim given an observed ChangeSignal.
//
// verifiedAt is when the Claim's evidence was last confirmed; pass the zero
// time if unknown (the Claim is then treated as verified at evaluation time,
// so only a relevant change signal can age it).
//
// The decision procedure (spike model):
//
//  1. Compute reviewInterval = baseReviewInterval(volatility) ×
//     authorityMultiplier(authority).
//  2. If a relevant change signal exists (one observed after verifiedAt):
//     - release-bound / schema-version / ephemeral claims go STALE — the
//     signal directly contradicts a version- or time-pinned claim.
//     - all other claim/signal combinations go AGING — the signal is a
//     reason to re-verify but not a direct contradiction.
//     NextReviewAt is set to "now" (review is due immediately).
//  3. With no relevant signal, age purely by the clock:
//     - elapsed >= reviewInterval        → STALE
//     - elapsed >= reviewInterval × 0.75 → AGING
//     - otherwise                        → FRESH
//
// NextReviewAt is always set to a non-zero time. Evaluate never returns an
// error: an unrecognized volatility or authority class is handled by the
// fallbacks in baseReviewInterval / authorityMultiplier, so the function is
// total over all inputs.
func (p FreshnessPolicy) Evaluate(claim Claim, signal ChangeSignal, verifiedAt time.Time) FreshnessResult {
	now := p.now()

	// An unknown verification time is treated as "verified now": without a
	// baseline, only a change signal — not the clock — can age the Claim.
	if verifiedAt.IsZero() {
		verifiedAt = now
	}

	interval := time.Duration(float64(baseReviewInterval(claim.VolatilityClass)) *
		authorityMultiplier(claim.AuthorityClass))

	if signal.relevant(verifiedAt) {
		return p.evaluateWithSignal(claim, signal, now)
	}

	return evaluateByClock(now, verifiedAt, interval)
}

// evaluateWithSignal handles the case where a relevant change signal exists.
// Version- or time-pinned claims (release-bound, schema-version-signalled,
// ephemeral) are contradicted outright and go stale; everything else ages.
func (p FreshnessPolicy) evaluateWithSignal(claim Claim, signal ChangeSignal, now time.Time) FreshnessResult {
	contradicts := claim.VolatilityClass == VolatilityReleaseBound ||
		claim.VolatilityClass == VolatilityEphemeral ||
		signal.Kind == ChangeSignalSchemaVersion

	state := FreshnessAging
	if contradicts {
		state = FreshnessStale
	}

	reason := fmt.Sprintf("change signal %q observed after last verification", signal.Kind)
	if detail := strings.TrimSpace(signal.Detail); detail != "" {
		reason = fmt.Sprintf("%s (%s)", reason, detail)
	}

	return FreshnessResult{
		State:         state,
		Reason:        reason,
		NextReviewAt:  now, // a signalled claim is due for review immediately
		RefreshAction: refreshActionFor(signal.Kind),
	}
}

// evaluateByClock handles the no-signal case: the Claim ages purely by how much
// of its review interval has elapsed.
func evaluateByClock(now, verifiedAt time.Time, interval time.Duration) FreshnessResult {
	elapsed := now.Sub(verifiedAt)
	nextReview := verifiedAt.Add(interval)

	switch {
	case elapsed >= interval:
		return FreshnessResult{
			State:         FreshnessStale,
			Reason:        "review interval exceeded with no re-verification",
			NextReviewAt:  now,
			RefreshAction: "re-verify claim sources",
		}
	case elapsed >= time.Duration(float64(interval)*0.75):
		return FreshnessResult{
			State:         FreshnessAging,
			Reason:        "review interval three-quarters elapsed",
			NextReviewAt:  nextReview,
			RefreshAction: "schedule re-verification",
		}
	default:
		return FreshnessResult{
			State:        FreshnessFresh,
			Reason:       "within review interval, no contradicting change signal",
			NextReviewAt: nextReview,
		}
	}
}

// refreshActionFor maps a change signal kind to a suggested refresh action.
func refreshActionFor(kind ChangeSignalKind) string {
	switch kind {
	case ChangeSignalRelease:
		return "re-verify claim against the newer release"
	case ChangeSignalFileHash:
		return "re-read the changed source file"
	case ChangeSignalSchemaVersion:
		return "re-verify claim against the bumped schema"
	default:
		return "re-verify claim sources"
	}
}
