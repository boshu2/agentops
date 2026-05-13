// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// ClaimEvidenceRequest is the input to ClaimEvidencePort.Derive. It
// names the claim, the evidence file that backs it, and the gate to
// run for verdict-based promotion.
type ClaimEvidenceRequest struct {
	Claim        ClaimID
	EvidenceFile string
	Gate         GateName
}

// ClaimEvidenceResult is the materialized binding plus its derived
// promotion level. Level is computed mechanically from the gate
// verdict: PASS → caller-specified TargetLevel (default PG2 for
// pointer→annotated promotion); WARN → MAX(existing, PG1); FAIL or
// UNKNOWN → existing (no promotion). Binding is the EvidenceBinding
// that would be passed to ClaimEvidenceBinderPort.Bind for
// persistence.
type ClaimEvidenceResult struct {
	Binding EvidenceBinding
	Verdict GateVerdict
}

// ClaimEvidencePort composes BC2's GateRunnerPort + BC4's
// ClaimEvidenceBinderPort into a single "claim → gate → evidence
// binding" promotion pipeline. Callers — `ao claims promote`, the
// soc-2klg.3 auto-promote ledger work, and dream's compounding loop
// — depend on this port so the policy logic ("which gate verdict
// promotes to which evidence level?") lives in one place.
//
// The composition is intentionally narrow:
//   - Derive runs the gate via the (caller-injected) GateRunner.
//   - The verdict + claim's existing level determine the new level.
//   - The returned Binding can be passed to a ClaimEvidenceBinder
//     for persistence, OR retained as a "what-if" without writing.
//
// Contract:
//
//   - Derive MUST be deterministic given the same (claim, gate,
//     verdict) tuple.
//   - Verdict promotion rules (the policy):
//     PASS → promote to PG2 minimum; if caller passes a higher
//     targetLevel, use it
//     WARN → promote to PG1 minimum (advisory evidence)
//     FAIL → keep existing level (no promotion); reasons surfaced
//     SKIP/UNKNOWN → keep existing level
//   - Adapters MUST NOT downgrade. If the policy yields a lower
//     level than existing, return existing.
//   - Empty Claim or empty EvidenceFile → structural-rejection error.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC4 row); soc-2klg
// epic; this is port scaffold .2 of 4 children.
type ClaimEvidencePort interface {
	// Derive runs the gate and returns the would-be binding + verdict.
	// existingLevel lets the caller seed the starting evidence level
	// for the upgrade-only check; pass EvidenceLevelNone for new bindings.
	// targetLevel lets PASS verdicts promote past PG2 (e.g., PG3 or PG4
	// when the gate is a strong-verify check).
	Derive(ctx context.Context, req ClaimEvidenceRequest, existingLevel, targetLevel EvidenceLevel) (ClaimEvidenceResult, error)
}
