// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInMemoryHypothesisLedger_AppendListAndFind(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger(nil)

	got, err := ledger.Append(context.Background(), HypothesisRecord{
		ID:           "H1",
		CycleLanded:  10,
		CheckAtCycle: 13,
		Patch:        "route discovery through explicit phase context",
		Hypothesis:   "token pressure falls",
		Measure:      "compare prompt tokens",
		Verdict:      HypothesisVerdictPending,
		Evidence:     []string{"baseline=100"},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got.ID != "H1" {
		t.Fatalf("Append ID = %q, want H1", got.ID)
	}

	records, err := ledger.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List len = %d, want 1", len(records))
	}
	if records[0].Patch != "route discovery through explicit phase context" {
		t.Fatalf("Patch = %q, want stored patch", records[0].Patch)
	}

	found, ok, err := ledger.Find(context.Background(), "H1")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !ok {
		t.Fatal("Find ok = false, want true")
	}
	if found.Measure != "compare prompt tokens" {
		t.Fatalf("Measure = %q, want compare prompt tokens", found.Measure)
	}
}

func TestInMemoryHypothesisLedger_PreservesAppendOrder(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger(nil)
	for _, id := range []string{"H1", "H2", "H3"} {
		if _, err := ledger.Append(context.Background(), HypothesisRecord{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	records, err := ledger.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"H1", "H2", "H3"} {
		if records[i].ID != want {
			t.Fatalf("records[%d].ID = %q, want %q", i, records[i].ID, want)
		}
	}
}

func TestInMemoryHypothesisLedger_RejectsEmptyAndDuplicateIDs(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger(nil)
	if _, err := ledger.Append(context.Background(), HypothesisRecord{}); err == nil {
		t.Fatal("expected empty-ID error, got nil")
	}
	if _, err := ledger.Append(context.Background(), HypothesisRecord{ID: "H1"}); err != nil {
		t.Fatal(err)
	}
	_, err := ledger.Append(context.Background(), HypothesisRecord{ID: "H1"})
	if err == nil {
		t.Fatal("expected duplicate-ID error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate substring", err)
	}
}

func TestInMemoryHypothesisLedger_FindUnknownReturnsFalse(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger([]HypothesisRecord{{ID: "H1"}})
	record, ok, err := ledger.Find(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Find ok = true, want false")
	}
	if record.ID != "" {
		t.Fatalf("record.ID = %q, want zero-value", record.ID)
	}
}

func TestInMemoryHypothesisLedger_SeedDuplicateStillRejected(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger([]HypothesisRecord{{ID: "H1"}})
	_, err := ledger.Append(context.Background(), HypothesisRecord{ID: "H1"})
	if err == nil {
		t.Fatal("expected duplicate-ID error against seed, got nil")
	}
}

func TestInMemoryHypothesisLedger_ReturnsDefensiveCopies(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger(nil)
	input := HypothesisRecord{ID: "H1", Evidence: []string{"original"}}
	got, err := ledger.Append(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.Evidence[0] = "mutated-input"
	got.Evidence[0] = "mutated-return"

	records, err := ledger.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	records[0].Evidence[0] = "mutated-list"

	found, ok, err := ledger.Find(context.Background(), "H1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Find ok = false, want true")
	}
	if found.Evidence[0] != "original" {
		t.Fatalf("stored Evidence[0] = %q, want original", found.Evidence[0])
	}
}

func TestInMemoryHypothesisLedger_HonorsContextCancellation(t *testing.T) {
	ledger := NewInMemoryHypothesisLedger(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ledger.Append(ctx, HypothesisRecord{ID: "H1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Append error = %v, want context.Canceled", err)
	}
	if _, err := ledger.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v, want context.Canceled", err)
	}
	if _, _, err := ledger.Find(ctx, "H1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Find error = %v, want context.Canceled", err)
	}
}
