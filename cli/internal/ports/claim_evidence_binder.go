// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// ClaimID identifies a single AOP-CLAIM in the project's
// .agents/findings/all-claims-evidence-map.md ledger (e.g.
// "AOP-CLAIM-TRUST-FACTORY-FIVE-STEP-PRIMITIVE"). Adapters MUST treat
// values as opaque tokens — no parsing of substructure.
type ClaimID string

// EvidenceLevel enumerates the PG (Promotion Gate) levels per the
// factory-claim-ledger contract. Higher = stronger.
//
//   - PG1 = pointer (a single citation that mentions the claim)
//   - PG2 = annotated pointer (citation + reason)
//   - PG3 = compiled evidence (an artifact file showing the claim
//     materialized)
//   - PG4 = strong-verified (compiled evidence + independent
//     cross-reference)
//
// Adapters MAY return EvidenceLevelNone when the binding hasn't been
// promoted yet.
type EvidenceLevel string

const (
	EvidenceLevelNone EvidenceLevel = ""
	EvidenceLevelPG1  EvidenceLevel = "PG1"
	EvidenceLevelPG2  EvidenceLevel = "PG2"
	EvidenceLevelPG3  EvidenceLevel = "PG3"
	EvidenceLevelPG4  EvidenceLevel = "PG4"
)

// EvidenceBinding is one materialized claim→evidence link. Path is
// the artifact path containing the evidence (relative to repo root);
// Level is the promotion-gate level; Anchors are optional in-file
// anchors (line numbers, named anchors) that adapters MAY populate to
// help readers find the exact evidence span.
type EvidenceBinding struct {
	Claim   ClaimID
	Path    string
	Level   EvidenceLevel
	Anchors []string
}

// ClaimEvidenceBinderPort wraps the bind operation that creates or
// updates an evidence binding for a claim. Callers — the `ao
// claims promote` path, the PG4 strong-verify workflow, dream's
// compounding loop, and any future cross-repo claim auditor — depend
// on this port so the bind machinery can be exercised against an
// in-memory adapter without depending on the real .agents/findings/
// ledger format.
//
// Contract:
//
//   - Bind MUST be idempotent: calling Bind twice with the same
//     (Claim, Path, Level) triple MUST NOT produce drift. The second
//     call MAY refresh Anchors but the resulting binding is the same.
//   - Bind MAY upgrade the Level (e.g. PG1 → PG2) on re-bind. It MUST
//     NOT downgrade (e.g. PG3 → PG1) — adapters MUST return a non-nil
//     error in that case.
//   - List returns all known bindings, most-recently-bound first.
//   - Empty Claim or empty Path is a structural-rejection error on Bind.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC2 row) for the
// canonical Validation context surface. Sibling ports: GateRunnerPort
// (cycle 99), CIStatusPort (cycle 100). soc-wxh5 epic.
type ClaimEvidenceBinderPort interface {
	Bind(ctx context.Context, binding EvidenceBinding) error
	List(ctx context.Context) ([]EvidenceBinding, error)
}
