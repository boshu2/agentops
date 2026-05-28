// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

func newCEFixture(verdicts map[GateName]GateVerdict) *InMemoryClaimEvidence {
	return NewInMemoryClaimEvidence(NewInMemoryGateRunner(verdicts))
}

func TestInMemoryClaimEvidence_PassPromotesToPG2_Default(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{
		"check-foo": {Status: GateStatusPass, Reason: "exit 0"},
	})
	res, err := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "AOP-CLAIM-X", EvidenceFile: ".agents/findings/x.md", Gate: "check-foo",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if err != nil {
		t.Fatal(err)
	}
	if res.Binding.Level != EvidenceLevelPG2 {
		t.Fatalf("PASS default → Level = %s, want PG2", res.Binding.Level)
	}
	if res.Verdict.Status != GateStatusPass {
		t.Fatalf("Verdict.Status = %s, want PASS", res.Verdict.Status)
	}
}

func TestInMemoryClaimEvidence_PassPromotesToTargetLevel(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{
		"check-strong": {Status: GateStatusPass, Reason: "exit 0"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "AOP-CLAIM-X", EvidenceFile: "x", Gate: "check-strong",
	}, EvidenceLevelNone, EvidenceLevelPG4)
	if res.Binding.Level != EvidenceLevelPG4 {
		t.Fatalf("PASS w/ targetPG4 → Level = %s, want PG4", res.Binding.Level)
	}
}

func TestInMemoryClaimEvidence_WarnPromotesToPG1(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{
		"check-advisory": {Status: GateStatusWarn, Reason: "exit 2"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "x", Gate: "check-advisory",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if res.Binding.Level != EvidenceLevelPG1 {
		t.Fatalf("WARN → Level = %s, want PG1", res.Binding.Level)
	}
}

func TestInMemoryClaimEvidence_FailKeepsExisting(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{
		"check-broken": {Status: GateStatusFail, Reason: "exit 1"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "x", Gate: "check-broken",
	}, EvidenceLevelPG3, EvidenceLevelNone)
	if res.Binding.Level != EvidenceLevelPG3 {
		t.Fatalf("FAIL → Level = %s, want PG3 (existing)", res.Binding.Level)
	}
}

func TestInMemoryClaimEvidence_NoDowngradeOnPass(t *testing.T) {
	// existing=PG3, PASS yields PG2 candidate — must not downgrade
	c := newCEFixture(map[GateName]GateVerdict{
		"check-ok": {Status: GateStatusPass, Reason: "exit 0"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "x", Gate: "check-ok",
	}, EvidenceLevelPG3, EvidenceLevelNone)
	if res.Binding.Level != EvidenceLevelPG3 {
		t.Fatalf("PASS w/ existing=PG3 → Level = %s, want PG3 (no downgrade)", res.Binding.Level)
	}
}

func TestInMemoryClaimEvidence_SkipAndUnknownKeepExisting(t *testing.T) {
	for _, status := range []GateStatus{GateStatusSkip, GateStatusUnknown} {
		c := newCEFixture(map[GateName]GateVerdict{
			"check-x": {Status: status, Reason: "test"},
		})
		res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
			Claim: "X", EvidenceFile: "x", Gate: "check-x",
		}, EvidenceLevelPG2, EvidenceLevelNone)
		if res.Binding.Level != EvidenceLevelPG2 {
			t.Fatalf("%s → Level = %s, want PG2 (existing)", status, res.Binding.Level)
		}
	}
}

func TestInMemoryClaimEvidence_EmptyClaimRejected(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{})
	_, err := c.Derive(context.Background(), ClaimEvidenceRequest{
		EvidenceFile: "x", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if err == nil {
		t.Fatal("empty Claim should error")
	}
}

func TestInMemoryClaimEvidence_EmptyEvidenceFileRejected(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{})
	_, err := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if err == nil {
		t.Fatal("empty EvidenceFile should error")
	}
}

func TestInMemoryClaimEvidence_NilGateRunnerRejected(t *testing.T) {
	c := NewInMemoryClaimEvidence(nil)
	_, err := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "x", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if err == nil {
		t.Fatal("nil GateRunner should error")
	}
}

func TestInMemoryClaimEvidence_BindingCarriesClaimAndPath(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{
		"g": {Status: GateStatusPass, Reason: "ok"},
	})
	res, _ := c.Derive(context.Background(), ClaimEvidenceRequest{
		Claim: "AOP-CLAIM-Y", EvidenceFile: "p/q.md", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if res.Binding.Claim != "AOP-CLAIM-Y" {
		t.Fatalf("Binding.Claim = %q", res.Binding.Claim)
	}
	if res.Binding.Path != "p/q.md" {
		t.Fatalf("Binding.Path = %q", res.Binding.Path)
	}
}

func TestInMemoryClaimEvidence_HonorsContextCancellation(t *testing.T) {
	c := newCEFixture(map[GateName]GateVerdict{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Derive(ctx, ClaimEvidenceRequest{
		Claim: "X", EvidenceFile: "x", Gate: "g",
	}, EvidenceLevelNone, EvidenceLevelNone)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
