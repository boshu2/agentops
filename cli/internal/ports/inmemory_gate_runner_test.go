// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Sibling pattern: inmemory_citation_test.go (cycle 81). Same shape —
// table-driven where helpful, L1-style assertions for behavior + port
// contract.

func TestInMemoryGateRunner_ReturnsConfiguredVerdict(t *testing.T) {
	r := NewInMemoryGateRunner(map[GateName]GateVerdict{
		"registry-check": {Status: GateStatusPass, Reason: "ok"},
		"bats-tests":     {Status: GateStatusFail, Reason: "5 tests failed", LogTail: "not ok 533"},
	})
	cases := []struct {
		name       GateName
		wantStatus GateStatus
		wantReason string
	}{
		{"registry-check", GateStatusPass, "ok"},
		{"bats-tests", GateStatusFail, "5 tests failed"},
	}
	for _, tc := range cases {
		t.Run(string(tc.name), func(t *testing.T) {
			v, err := r.Run(context.Background(), GateRunRequest{Name: tc.name})
			if err != nil {
				t.Fatal(err)
			}
			if v.Status != tc.wantStatus {
				t.Fatalf("Status = %q, want %q", v.Status, tc.wantStatus)
			}
			if v.Reason != tc.wantReason {
				t.Fatalf("Reason = %q, want %q", v.Reason, tc.wantReason)
			}
		})
	}
}

func TestInMemoryGateRunner_EmptyNameReturnsUnknown(t *testing.T) {
	r := NewInMemoryGateRunner(nil)
	v, err := r.Run(context.Background(), GateRunRequest{Name: ""})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != GateStatusUnknown {
		t.Fatalf("Status = %q, want UNKNOWN", v.Status)
	}
	if !strings.Contains(v.Reason, "empty GateName") {
		t.Fatalf("Reason = %q, want substring 'empty GateName'", v.Reason)
	}
}

func TestInMemoryGateRunner_UnknownGateDefaultsToUnknownStatus(t *testing.T) {
	r := NewInMemoryGateRunner(map[GateName]GateVerdict{
		"known-gate": {Status: GateStatusPass, Reason: "ok"},
	})
	v, err := r.Run(context.Background(), GateRunRequest{Name: "never-configured"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != GateStatusUnknown {
		t.Fatalf("Status = %q, want UNKNOWN (default unknown-name policy)", v.Status)
	}
	if !strings.Contains(v.Reason, "never-configured") {
		t.Fatalf("Reason = %q, want to name the missing gate", v.Reason)
	}
}

func TestInMemoryGateRunner_UnknownIsFailFlag(t *testing.T) {
	r := NewInMemoryGateRunner(nil)
	r.UnknownIsFail = true
	v, err := r.Run(context.Background(), GateRunRequest{Name: "never-configured"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != GateStatusFail {
		t.Fatalf("Status = %q, want FAIL (UnknownIsFail=true)", v.Status)
	}
}

func TestInMemoryGateRunner_NilVerdictsArgumentIsSafe(t *testing.T) {
	r := NewInMemoryGateRunner(nil)
	v, err := r.Run(context.Background(), GateRunRequest{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != GateStatusUnknown {
		t.Fatalf("Status = %q, want UNKNOWN", v.Status)
	}
}

func TestInMemoryGateRunner_HonorsContextCancellation(t *testing.T) {
	r := NewInMemoryGateRunner(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Run(ctx, GateRunRequest{Name: "x"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
