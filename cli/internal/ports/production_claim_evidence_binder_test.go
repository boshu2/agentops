// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// Sibling pattern: cycle 115 gate_runner_adapter_test.go.

func newTempBinder(t *testing.T) *ProductionClaimEvidenceBinder {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evidence-bindings.jsonl")
	return NewProductionClaimEvidenceBinder(path)
}

func TestProductionClaimEvidenceBinder_BindCreatesEntry(t *testing.T) {
	b := newTempBinder(t)
	err := b.Bind(context.Background(), EvidenceBinding{
		Claim: "AOP-CLAIM-X",
		Path:  ".agents/findings/x.md",
		Level: EvidenceLevelPG2,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := b.List(context.Background())
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0].Claim != "AOP-CLAIM-X" || list[0].Level != EvidenceLevelPG2 {
		t.Fatalf("got %+v", list[0])
	}
}

func TestProductionClaimEvidenceBinder_UpgradeAllowed(t *testing.T) {
	b := newTempBinder(t)
	_ = b.Bind(context.Background(), EvidenceBinding{
		Claim: "AOP-CLAIM-X",
		Path:  "p",
		Level: EvidenceLevelPG1,
	})
	err := b.Bind(context.Background(), EvidenceBinding{
		Claim: "AOP-CLAIM-X",
		Path:  "p",
		Level: EvidenceLevelPG3,
	})
	if err != nil {
		t.Fatalf("PG1 → PG3 upgrade rejected: %v", err)
	}
	list, _ := b.List(context.Background())
	if list[0].Level != EvidenceLevelPG3 {
		t.Fatalf("Level = %s, want PG3 (upgrade not visible)", list[0].Level)
	}
}

func TestProductionClaimEvidenceBinder_DowngradeRejected(t *testing.T) {
	b := newTempBinder(t)
	_ = b.Bind(context.Background(), EvidenceBinding{
		Claim: "AOP-CLAIM-X", Path: "p", Level: EvidenceLevelPG3,
	})
	err := b.Bind(context.Background(), EvidenceBinding{
		Claim: "AOP-CLAIM-X", Path: "p", Level: EvidenceLevelPG1,
	})
	if err == nil {
		t.Fatal("PG3 → PG1 downgrade should error, got nil")
	}
}

func TestProductionClaimEvidenceBinder_IdempotentSameLevel(t *testing.T) {
	b := newTempBinder(t)
	req := EvidenceBinding{Claim: "X", Path: "p", Level: EvidenceLevelPG2}
	_ = b.Bind(context.Background(), req)
	_ = b.Bind(context.Background(), req) // second call — should NOT append
	list, _ := b.List(context.Background())
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1 (idempotent no-op)", len(list))
	}
}

func TestProductionClaimEvidenceBinder_DifferentAnchorsCausesAppend(t *testing.T) {
	b := newTempBinder(t)
	_ = b.Bind(context.Background(), EvidenceBinding{
		Claim: "X", Path: "p", Level: EvidenceLevelPG2,
		Anchors: []string{"L10"},
	})
	_ = b.Bind(context.Background(), EvidenceBinding{
		Claim: "X", Path: "p", Level: EvidenceLevelPG2,
		Anchors: []string{"L20"},
	})
	list, _ := b.List(context.Background())
	if list[0].Anchors[0] != "L20" {
		t.Fatalf("Anchors[0] = %v, want L20", list[0].Anchors)
	}
}

func TestProductionClaimEvidenceBinder_ListMostRecentFirst(t *testing.T) {
	b := newTempBinder(t)
	for _, claim := range []ClaimID{"A", "B", "C"} {
		_ = b.Bind(context.Background(), EvidenceBinding{
			Claim: claim, Path: "p", Level: EvidenceLevelPG1,
		})
	}
	list, _ := b.List(context.Background())
	if list[0].Claim != "C" || list[2].Claim != "A" {
		t.Fatalf("order wrong: %v %v %v", list[0].Claim, list[1].Claim, list[2].Claim)
	}
}

func TestProductionClaimEvidenceBinder_EmptyClaimRejected(t *testing.T) {
	b := newTempBinder(t)
	err := b.Bind(context.Background(), EvidenceBinding{Path: "p", Level: EvidenceLevelPG1})
	if err == nil {
		t.Fatal("expected empty-claim rejection, got nil")
	}
}

func TestProductionClaimEvidenceBinder_EmptyPathRejected(t *testing.T) {
	b := newTempBinder(t)
	err := b.Bind(context.Background(), EvidenceBinding{Claim: "X", Level: EvidenceLevelPG1})
	if err == nil {
		t.Fatal("expected empty-path rejection, got nil")
	}
}

func TestProductionClaimEvidenceBinder_EmptyFilePathErrors(t *testing.T) {
	b := NewProductionClaimEvidenceBinder("")
	err := b.Bind(context.Background(), EvidenceBinding{Claim: "X", Path: "p"})
	if err == nil {
		t.Fatal("expected error on empty file path, got nil")
	}
}

func TestProductionClaimEvidenceBinder_HonorsContextCancellation(t *testing.T) {
	b := newTempBinder(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.Bind(ctx, EvidenceBinding{Claim: "X", Path: "p"}); err == nil {
		t.Fatal("Bind: expected cancellation, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Bind error = %v, want context.Canceled", err)
	}
	if _, err := b.List(ctx); err == nil {
		t.Fatal("List: expected cancellation, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v, want context.Canceled", err)
	}
}

func TestProductionClaimEvidenceBinder_ListMissingFileEmpty(t *testing.T) {
	b := NewProductionClaimEvidenceBinder(filepath.Join(t.TempDir(), "missing.jsonl"))
	list, err := b.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Fatal("missing-file List should be non-nil empty")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}
