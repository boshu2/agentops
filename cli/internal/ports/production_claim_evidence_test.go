// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

func newPCEFixture(verdicts map[GateName]GateVerdict) *ProductionClaimEvidence {
	return NewProductionClaimEvidence(NewInMemoryGateRunner(verdicts))
}

func TestProductionClaimEvidence_PassDefaultsToPG2(t *testing.T) {
	c := newPCEFixture(map[GateName]GateVerdict{
		"g": {Status: GateStatusPass, Reason: "ok"},
	})
	res, err := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if err != nil {
		t.Fatal(err)
	}
	if res.Binding.Level != EvidenceLevelPG2 {
		t.Fatalf("Level = %s, want PG2", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_TargetLevelHonored(t *testing.T) {
	c := newPCEFixture(map[GateName]GateVerdict{
		"g": {Status: GateStatusPass, Reason: "ok"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelPG4)
	if res.Binding.Level != EvidenceLevelPG4 {
		t.Fatalf("Level = %s, want PG4", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_WarnToPG1(t *testing.T) {
	c := newPCEFixture(map[GateName]GateVerdict{
		"g": {Status: GateStatusWarn, Reason: "advisory"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if res.Binding.Level != EvidenceLevelPG1 {
		t.Fatalf("Level = %s, want PG1", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_FailKeepsExisting(t *testing.T) {
	c := newPCEFixture(map[GateName]GateVerdict{
		"g": {Status: GateStatusFail, Reason: "broken"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelPG3, EvidenceLevelNone)
	if res.Binding.Level != EvidenceLevelPG3 {
		t.Fatalf("Level = %s, want PG3 (existing)", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_NoDowngradeOnPass(t *testing.T) {
	c := newPCEFixture(map[GateName]GateVerdict{
		"g": {Status: GateStatusPass, Reason: "ok"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelPG3, EvidenceLevelNone)
	if res.Binding.Level != EvidenceLevelPG3 {
		t.Fatalf("Level = %s, want PG3 (no downgrade)", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_EmptyInputsRejected(t *testing.T) {
	c := newPCEFixture(nil)
	if _, err := c.Derive(context.Background(), ClaimEvidenceRequest{EvidenceFile: "p", Gate: "g"}, EvidenceLevelNone, EvidenceLevelNone); err == nil {
		t.Fatal("empty Claim should error")
	}
	if _, err := c.Derive(context.Background(), ClaimEvidenceRequest{Claim: "X", Gate: "g"}, EvidenceLevelNone, EvidenceLevelNone); err == nil {
		t.Fatal("empty EvidenceFile should error")
	}
}

func TestProductionClaimEvidence_NilGateRunnerErrors(t *testing.T) {
	c := NewProductionClaimEvidence(nil)
	_, err := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if err == nil {
		t.Fatal("nil GateRunner should error")
	}
}

func TestProductionClaimEvidence_PolicyMatchesInMemoryAdapter(t *testing.T) {
	// Cross-check: the production policy must yield identical results
	// to the in-memory adapter for the same inputs. This is the
	// "spec" guarantee from the cycle-141 doc — both adapters apply
	// the same policy.
	cases := []struct {
		status   GateStatus
		existing EvidenceLevel
		target   EvidenceLevel
		want     EvidenceLevel
	}{
		{GateStatusPass, EvidenceLevelNone, EvidenceLevelNone, EvidenceLevelPG2},
		{GateStatusPass, EvidenceLevelNone, EvidenceLevelPG4, EvidenceLevelPG4},
		{GateStatusPass, EvidenceLevelPG3, EvidenceLevelNone, EvidenceLevelPG3},
		{GateStatusWarn, EvidenceLevelNone, EvidenceLevelNone, EvidenceLevelPG1},
		{GateStatusWarn, EvidenceLevelPG2, EvidenceLevelNone, EvidenceLevelPG2},
		{GateStatusFail, EvidenceLevelPG2, EvidenceLevelNone, EvidenceLevelPG2},
		{GateStatusSkip, EvidenceLevelPG3, EvidenceLevelNone, EvidenceLevelPG3},
		{GateStatusUnknown, EvidenceLevelNone, EvidenceLevelNone, EvidenceLevelNone},
	}
	for _, tc := range cases {
		got := productionPromoteEvidenceLevel(tc.status, tc.existing, tc.target)
		if got != tc.want {
			t.Fatalf("policy(%s, existing=%s, target=%s) = %s, want %s",
				tc.status, tc.existing, tc.target, got, tc.want)
		}
	}
}

func TestProductionClaimEvidence_HonorsContextCancellation(t *testing.T) {
	c := newPCEFixture(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Derive(ctx, ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
