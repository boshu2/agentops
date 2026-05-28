// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Sibling pattern: inmemory_ci_status_test.go (cycle 100). Same shape
// — L1 behavior + port-contract assertions.

func TestInMemoryClaimEvidenceBinder_BindFirstTimeIsAccepted(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	err := a.Bind(context.Background(), EvidenceBinding{
		Claim: "AOP-CLAIM-TEST",
		Path:  ".agents/findings/all-claims-evidence-map.md",
		Level: EvidenceLevelPG1,
	})
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0].Level != EvidenceLevelPG1 {
		t.Fatalf("Level = %q, want PG1", list[0].Level)
	}
}

func TestInMemoryClaimEvidenceBinder_BindIsIdempotent(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	b := EvidenceBinding{Claim: "C1", Path: "p", Level: EvidenceLevelPG2}
	if err := a.Bind(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	// Second call with identical input → no drift
	if err := a.Bind(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	list, _ := a.List(context.Background())
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1 (idempotent re-bind)", len(list))
	}
}

func TestInMemoryClaimEvidenceBinder_BindAllowsLevelUpgrade(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	if err := a.Bind(context.Background(), EvidenceBinding{Claim: "C", Path: "p", Level: EvidenceLevelPG1}); err != nil {
		t.Fatal(err)
	}
	// Upgrade to PG3
	if err := a.Bind(context.Background(), EvidenceBinding{Claim: "C", Path: "p", Level: EvidenceLevelPG3}); err != nil {
		t.Fatalf("upgrade PG1 → PG3 should succeed, got %v", err)
	}
	list, _ := a.List(context.Background())
	if list[0].Level != EvidenceLevelPG3 {
		t.Fatalf("Level = %q after upgrade, want PG3", list[0].Level)
	}
}

func TestInMemoryClaimEvidenceBinder_BindRejectsLevelDowngrade(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	if err := a.Bind(context.Background(), EvidenceBinding{Claim: "C", Path: "p", Level: EvidenceLevelPG3}); err != nil {
		t.Fatal(err)
	}
	err := a.Bind(context.Background(), EvidenceBinding{Claim: "C", Path: "p", Level: EvidenceLevelPG1})
	if err == nil {
		t.Fatal("expected error on PG3 → PG1 downgrade, got nil")
	}
	if !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("error = %v, want substring 'downgrade'", err)
	}
}

func TestInMemoryClaimEvidenceBinder_BindRejectsEmptyClaimOrPath(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	if err := a.Bind(context.Background(), EvidenceBinding{Path: "p", Level: EvidenceLevelPG1}); err == nil {
		t.Fatal("expected error on empty Claim, got nil")
	}
	if err := a.Bind(context.Background(), EvidenceBinding{Claim: "C", Level: EvidenceLevelPG1}); err == nil {
		t.Fatal("expected error on empty Path, got nil")
	}
}

func TestInMemoryClaimEvidenceBinder_ListReturnsMostRecentFirst(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	_ = a.Bind(context.Background(), EvidenceBinding{Claim: "A", Path: "p", Level: EvidenceLevelPG1})
	_ = a.Bind(context.Background(), EvidenceBinding{Claim: "B", Path: "p", Level: EvidenceLevelPG1})
	_ = a.Bind(context.Background(), EvidenceBinding{Claim: "C", Path: "p", Level: EvidenceLevelPG1})
	list, _ := a.List(context.Background())
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	if list[0].Claim != "C" || list[2].Claim != "A" {
		t.Fatalf("order = %v, want [C, B, A]", []ClaimID{list[0].Claim, list[1].Claim, list[2].Claim})
	}
}

func TestInMemoryClaimEvidenceBinder_HonorsContextCancellation(t *testing.T) {
	a := NewInMemoryClaimEvidenceBinder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Bind(ctx, EvidenceBinding{Claim: "C", Path: "p", Level: EvidenceLevelPG1}); err == nil {
		t.Fatal("Bind: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Bind error = %v, want context.Canceled", err)
	}
	if _, err := a.List(ctx); err == nil {
		t.Fatal("List: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v, want context.Canceled", err)
	}
}

func TestInMemoryClaimEvidenceBinder_LevelRankOrdering(t *testing.T) {
	cases := []struct {
		in   EvidenceLevel
		want int
	}{
		{EvidenceLevelNone, 0},
		{EvidenceLevelPG1, 1},
		{EvidenceLevelPG2, 2},
		{EvidenceLevelPG3, 3},
		{EvidenceLevelPG4, 4},
		{EvidenceLevel("garbage"), 0},
	}
	for _, tc := range cases {
		got := evidenceLevelRank(tc.in)
		if got != tc.want {
			t.Fatalf("evidenceLevelRank(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
