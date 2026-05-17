package ports

import "time"

// FreshnessSignalKind discriminates the source of a freshness change signal at
// the port level. It mirrors wiki.ChangeSignalKind without importing the wiki
// package, so port callers stay decoupled from the concrete policy.
type FreshnessSignalKind string

const (
	// FreshnessSignalNone is the zero value: no observed change.
	FreshnessSignalNone FreshnessSignalKind = ""
	// FreshnessSignalRelease marks a newer software release.
	FreshnessSignalRelease FreshnessSignalKind = "release"
	// FreshnessSignalFileHash marks a changed file content hash.
	FreshnessSignalFileHash FreshnessSignalKind = "file-hash"
	// FreshnessSignalSchemaVersion marks a bumped schema/contract version.
	FreshnessSignalSchemaVersion FreshnessSignalKind = "schema-version"
)

// FreshnessChangeSignal is the port-level shape of an observed change signal —
// the evidence input to a FreshnessPolicyPort. It mirrors wiki.ChangeSignal.
type FreshnessChangeSignal struct {
	// Kind is the source of the change; the zero value means no change.
	Kind FreshnessSignalKind
	// ObservedAt is when the change was detected.
	ObservedAt time.Time
	// Detail is an opaque human-readable description of the change.
	Detail string
}

// FreshnessVerdict is the port-level shape of a freshness evaluation result. It
// mirrors wiki.FreshnessResult. State is one of "fresh", "aging", or "stale".
type FreshnessVerdict struct {
	// State is the freshness verdict: "fresh", "aging", or "stale".
	State string
	// Reason is a short machine-stable explanation of the verdict.
	Reason string
	// NextReviewAt is when the claim should next be re-verified; always set.
	NextReviewAt time.Time
	// RefreshAction is a suggested follow-up; empty for a fresh claim.
	RefreshAction string
}

// FreshnessPolicyPort is the BC seam for claim-level freshness evaluation. It
// exists so the loop and validation BCs can ask "is this Claim still trusted?"
// without importing the wiki package's full surface, and so the policy can be
// exercised against an in-memory adapter.
//
// The model is volatility_class × authority_class × change_signal, per the
// 2026-05-17 wiki-domain brainstorm. This port is SPIKE-FIRST: the freshness
// model is new and unproven, so the contract is intentionally minimal.
//
// Contract:
//
//   - Evaluate MUST be total: it never returns an error. Unrecognized
//     volatility or authority classes are handled by conservative fallbacks.
//   - Evaluate MUST always set a non-zero NextReviewAt.
//   - Given a release-bound claim and a change signal observed after the
//     claim's verifiedAt, State MUST be "aging" or "stale".
//
// claimVolatility and claimAuthority are the wiki.Claim's class strings;
// verifiedAt is when the claim's evidence was last confirmed (zero if unknown).
//
// The production implementation is wiki.FreshnessPolicy.
type FreshnessPolicyPort interface {
	// Evaluate computes the freshness verdict for a claim given an observed
	// change signal and the claim's last-verified time.
	Evaluate(claimVolatility, claimAuthority string, signal FreshnessChangeSignal, verifiedAt time.Time) FreshnessVerdict
}
