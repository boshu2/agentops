// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"errors"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionClaimEvidence satisfies ports.ClaimEvidencePort by
// composing any GateRunnerPort (typically the cycle 115
// productionGateRunner) with the same promotion policy as the
// in-memory adapter (cycle 141 promoteEvidenceLevel helper).
//
// The production composer wraps a real gate-runner so calling code
// can `Derive` a claim→evidence binding from a real gate invocation
// without re-implementing the policy.
type productionClaimEvidence struct {
	gateRunner ports.GateRunnerPort
}

func newProductionClaimEvidence(gateRunner ports.GateRunnerPort) *productionClaimEvidence {
	return &productionClaimEvidence{gateRunner: gateRunner}
}

// Derive runs the gate via the wrapped GateRunner and applies the
// same promotion policy as the in-memory adapter. This wrapper
// exists so production callers get the policy without recreating
// it; the policy lives in the in-memory adapter (test surface) and
// here (production surface) — kept in sync via the contract enumerated
// in cli/internal/ports/claim_evidence.go and the cycle-141
// inmemory_claim_evidence_test.go suite.
func (a *productionClaimEvidence) Derive(ctx context.Context, req ports.ClaimEvidenceRequest, existingLevel, targetLevel ports.EvidenceLevel) (ports.ClaimEvidenceResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.ClaimEvidenceResult{}, err
	}
	if req.Claim == "" {
		return ports.ClaimEvidenceResult{}, errors.New("productionClaimEvidence: Claim required")
	}
	if req.EvidenceFile == "" {
		return ports.ClaimEvidenceResult{}, errors.New("productionClaimEvidence: EvidenceFile required")
	}
	if a.gateRunner == nil {
		return ports.ClaimEvidenceResult{}, errors.New("productionClaimEvidence: GateRunner required")
	}

	verdict, err := a.gateRunner.Run(ctx, ports.GateRunRequest{Name: req.Gate})
	if err != nil {
		return ports.ClaimEvidenceResult{}, err
	}

	newLevel := productionPromoteEvidenceLevel(verdict.Status, existingLevel, targetLevel)
	return ports.ClaimEvidenceResult{
		Binding: ports.EvidenceBinding{
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
func productionPromoteEvidenceLevel(status ports.GateStatus, existing, target ports.EvidenceLevel) ports.EvidenceLevel {
	var candidate ports.EvidenceLevel
	switch status {
	case ports.GateStatusPass:
		candidate = target
		if candidate == ports.EvidenceLevelNone {
			candidate = ports.EvidenceLevelPG2
		}
	case ports.GateStatusWarn:
		candidate = ports.EvidenceLevelPG1
	default:
		// FAIL, SKIP, UNKNOWN — no promotion
		return existing
	}
	if productionEvidenceLevelOrd(candidate) > productionEvidenceLevelOrd(existing) {
		return candidate
	}
	return existing
}

func productionEvidenceLevelOrd(l ports.EvidenceLevel) int {
	switch l {
	case ports.EvidenceLevelPG1:
		return 1
	case ports.EvidenceLevelPG2:
		return 2
	case ports.EvidenceLevelPG3:
		return 3
	case ports.EvidenceLevelPG4:
		return 4
	}
	return 0
}

// Compile-time assertion.
var _ ports.ClaimEvidencePort = (*productionClaimEvidence)(nil)
