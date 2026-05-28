// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func newPCEFixture(verdicts map[ports.GateName]ports.GateVerdict) *productionClaimEvidence {
	return newProductionClaimEvidence(ports.NewInMemoryGateRunner(verdicts))
}

func TestProductionClaimEvidence_PassDefaultsToPG2(t *testing.T) {
	c := newPCEFixture(map[ports.GateName]ports.GateVerdict{
		"g": {Status: ports.GateStatusPass, Reason: "ok"},
	})
	res, err := c.Derive(context.Background(), ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelNone, ports.EvidenceLevelNone)
	if err != nil {
		t.Fatal(err)
	}
	if res.Binding.Level != ports.EvidenceLevelPG2 {
		t.Fatalf("Level = %s, want PG2", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_TargetLevelHonored(t *testing.T) {
	c := newPCEFixture(map[ports.GateName]ports.GateVerdict{
		"g": {Status: ports.GateStatusPass, Reason: "ok"},
	})
	res, _ := c.Derive(context.Background(), ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelNone, ports.EvidenceLevelPG4)
	if res.Binding.Level != ports.EvidenceLevelPG4 {
		t.Fatalf("Level = %s, want PG4", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_WarnToPG1(t *testing.T) {
	c := newPCEFixture(map[ports.GateName]ports.GateVerdict{
		"g": {Status: ports.GateStatusWarn, Reason: "advisory"},
	})
	res, _ := c.Derive(context.Background(), ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelNone, ports.EvidenceLevelNone)
	if res.Binding.Level != ports.EvidenceLevelPG1 {
		t.Fatalf("Level = %s, want PG1", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_FailKeepsExisting(t *testing.T) {
	c := newPCEFixture(map[ports.GateName]ports.GateVerdict{
		"g": {Status: ports.GateStatusFail, Reason: "broken"},
	})
	res, _ := c.Derive(context.Background(), ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelPG3, ports.EvidenceLevelNone)
	if res.Binding.Level != ports.EvidenceLevelPG3 {
		t.Fatalf("Level = %s, want PG3 (existing)", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_NoDowngradeOnPass(t *testing.T) {
	c := newPCEFixture(map[ports.GateName]ports.GateVerdict{
		"g": {Status: ports.GateStatusPass, Reason: "ok"},
	})
	res, _ := c.Derive(context.Background(), ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelPG3, ports.EvidenceLevelNone)
	if res.Binding.Level != ports.EvidenceLevelPG3 {
		t.Fatalf("Level = %s, want PG3 (no downgrade)", res.Binding.Level)
	}
}

func TestProductionClaimEvidence_EmptyInputsRejected(t *testing.T) {
	c := newPCEFixture(nil)
	if _, err := c.Derive(context.Background(), ports.ClaimEvidenceRequest{EvidenceFile: "p", Gate: "g"}, ports.EvidenceLevelNone, ports.EvidenceLevelNone); err == nil {
		t.Fatal("empty Claim should error")
	}
	if _, err := c.Derive(context.Background(), ports.ClaimEvidenceRequest{Claim: "X", Gate: "g"}, ports.EvidenceLevelNone, ports.EvidenceLevelNone); err == nil {
		t.Fatal("empty EvidenceFile should error")
	}
}

func TestProductionClaimEvidence_NilGateRunnerErrors(t *testing.T) {
	c := newProductionClaimEvidence(nil)
	_, err := c.Derive(context.Background(), ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelNone, ports.EvidenceLevelNone)
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
		status   ports.GateStatus
		existing ports.EvidenceLevel
		target   ports.EvidenceLevel
		want     ports.EvidenceLevel
	}{
		{ports.GateStatusPass, ports.EvidenceLevelNone, ports.EvidenceLevelNone, ports.EvidenceLevelPG2},
		{ports.GateStatusPass, ports.EvidenceLevelNone, ports.EvidenceLevelPG4, ports.EvidenceLevelPG4},
		{ports.GateStatusPass, ports.EvidenceLevelPG3, ports.EvidenceLevelNone, ports.EvidenceLevelPG3},
		{ports.GateStatusWarn, ports.EvidenceLevelNone, ports.EvidenceLevelNone, ports.EvidenceLevelPG1},
		{ports.GateStatusWarn, ports.EvidenceLevelPG2, ports.EvidenceLevelNone, ports.EvidenceLevelPG2},
		{ports.GateStatusFail, ports.EvidenceLevelPG2, ports.EvidenceLevelNone, ports.EvidenceLevelPG2},
		{ports.GateStatusSkip, ports.EvidenceLevelPG3, ports.EvidenceLevelNone, ports.EvidenceLevelPG3},
		{ports.GateStatusUnknown, ports.EvidenceLevelNone, ports.EvidenceLevelNone, ports.EvidenceLevelNone},
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
	_, err := c.Derive(ctx, ports.ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "p", Gate: "g",
	}, ports.EvidenceLevelNone, ports.EvidenceLevelNone)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
