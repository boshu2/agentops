// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// ProductionClaimEvidence satisfies ClaimEvidencePort by
// composing any GateRunnerPort (typically the cycle 115
// productionGateRunner) with the same promotion policy as the
// in-memory adapter (cycle 141 promoteEvidenceLevel helper).
//
// The production composer wraps a real gate-runner so calling code
// can `Derive` a claim→evidence binding from a real gate invocation
// without re-implementing the policy.
type ProductionClaimEvidence struct {
	gateRunner GateRunnerPort
}

func NewProductionClaimEvidence(gateRunner GateRunnerPort) *ProductionClaimEvidence {
	return &ProductionClaimEvidence{gateRunner: gateRunner}
}

// Derive runs the gate via the wrapped GateRunner and applies the
// same promotion policy as the in-memory adapter. This wrapper
// exists so production callers get the policy without recreating
// it; the policy lives in the in-memory adapter (test surface) and
// here (production surface) — kept in sync via the contract enumerated
// in cli/internal/ports/claim_evidence.go and the cycle-141
// inmemory_claim_evidence_test.go suite.
func (a *ProductionClaimEvidence) Derive(ctx context.Context, req ClaimEvidenceRequest, existingLevel, targetLevel EvidenceLevel) (ClaimEvidenceResult, error) {
	if err := ctx.Err(); err != nil {
		return ClaimEvidenceResult{}, err
	}
	if req.Claim == "" {
		return ClaimEvidenceResult{}, errors.New("ProductionClaimEvidence: Claim required")
	}
	if req.EvidenceFile == "" {
		return ClaimEvidenceResult{}, errors.New("ProductionClaimEvidence: EvidenceFile required")
	}
	if a.gateRunner == nil {
		return ClaimEvidenceResult{}, errors.New("ProductionClaimEvidence: GateRunner required")
	}

	verdict, err := a.gateRunner.Run(ctx, GateRunRequest{Name: req.Gate})
	if err != nil {
		return ClaimEvidenceResult{}, err
	}

	newLevel := productionPromoteEvidenceLevel(verdict.Status, existingLevel, targetLevel)
	return ClaimEvidenceResult{
		Binding: EvidenceBinding{
			Claim: req.Claim,
			Path:  req.EvidenceFile,
			Level: newLevel,
		},
		Verdict: verdict,
	}, nil
}

// productionPromoteEvidenceLevel mirrors the in-memory adapter's
// promotion policy. Kept as a separate function (not imported from
// internal/ports) so the production-side stays self-contained — the
// in-memory adapter's helper is package-private. Both implementations
// must stay in sync; the cycle-141 test suite is the spec.
func productionPromoteEvidenceLevel(status GateStatus, existing, target EvidenceLevel) EvidenceLevel {
	var candidate EvidenceLevel
	switch status {
	case GateStatusPass:
		candidate = target
		if candidate == EvidenceLevelNone {
			candidate = EvidenceLevelPG2
		}
	case GateStatusWarn:
		candidate = EvidenceLevelPG1
	default:
		// FAIL, SKIP, UNKNOWN — no promotion
		return existing
	}
	if productionEvidenceLevelOrd(candidate) > productionEvidenceLevelOrd(existing) {
		return candidate
	}
	return existing
}

func productionEvidenceLevelOrd(l EvidenceLevel) int {
	switch l {
	case EvidenceLevelPG1:
		return 1
	case EvidenceLevelPG2:
		return 2
	case EvidenceLevelPG3:
		return 3
	case EvidenceLevelPG4:
		return 4
	}
	return 0
}

// Compile-time assertion.
var _ ClaimEvidencePort = (*ProductionClaimEvidence)(nil)
