// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// InMemoryClaimEvidence is the test-double ClaimEvidencePort. It
// holds a reference to a GateRunner (typically the InMemoryGateRunner)
// and applies the documented promotion policy.
//
// Concurrency: not safe for concurrent mutation; tests should
// configure the GateRunner before invoking Derive.
type InMemoryClaimEvidence struct {
	GateRunner GateRunnerPort
}

// NewInMemoryClaimEvidence returns an adapter wired to the given
// gate runner. A nil GateRunner makes Derive return an error so
// tests catch mis-wiring immediately.
func NewInMemoryClaimEvidence(runner GateRunnerPort) *InMemoryClaimEvidence {
	return &InMemoryClaimEvidence{GateRunner: runner}
}

// Derive applies the documented policy:
//
//	PASS → max(existing, targetLevel) where targetLevel defaults to PG2
//	WARN → max(existing, PG1)
//	FAIL/SKIP/UNKNOWN → existing (no promotion)
//
// The returned Binding is a what-if record — caller persists via
// ClaimEvidenceBinderPort.Bind if desired.
func (a *InMemoryClaimEvidence) Derive(ctx context.Context, req ClaimEvidenceRequest, existingLevel, targetLevel EvidenceLevel) (ClaimEvidenceResult, error) {
	if err := ctx.Err(); err != nil {
		return ClaimEvidenceResult{}, err
	}
	if req.Claim == "" {
		return ClaimEvidenceResult{}, errors.New("ports: ClaimEvidenceRequest.Claim required")
	}
	if req.EvidenceFile == "" {
		return ClaimEvidenceResult{}, errors.New("ports: ClaimEvidenceRequest.EvidenceFile required")
	}
	if a.GateRunner == nil {
		return ClaimEvidenceResult{}, errors.New("ports: GateRunner required (in-memory or production)")
	}

	verdict, err := a.GateRunner.Run(ctx, GateRunRequest{Name: req.Gate})
	if err != nil {
		return ClaimEvidenceResult{}, err
	}

	newLevel := promoteEvidenceLevel(verdict.Status, existingLevel, targetLevel)
	return ClaimEvidenceResult{
		Binding: EvidenceBinding{
			Claim: req.Claim,
			Path:  req.EvidenceFile,
			Level: newLevel,
		},
		Verdict: verdict,
	}, nil
}

// promoteEvidenceLevel encodes the policy. Exported via the in-memory
// adapter so the production adapter can share it.
func promoteEvidenceLevel(status GateStatus, existing, target EvidenceLevel) EvidenceLevel {
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
	if evidenceLevelOrd(candidate) > evidenceLevelOrd(existing) {
		return candidate
	}
	return existing
}

// evidenceLevelOrd gives EvidenceLevel an integer ordering for
// max(existing, candidate). None < PG1 < PG2 < PG3 < PG4.
func evidenceLevelOrd(l EvidenceLevel) int {
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
var _ ClaimEvidencePort = (*InMemoryClaimEvidence)(nil)
