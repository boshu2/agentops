// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Sibling pattern: inmemory_loop_reader_test.go (cycle 102).

func TestInMemoryLoopWriter_AppendAutoAssignsNumberWhenZero(t *testing.T) {
	w := NewInMemoryLoopWriter(nil)
	got, err := w.Append(context.Background(), CycleEntry{Mode: "feature", Result: "improved"})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got.Number != 1 {
		t.Fatalf("Number = %d, want 1 (auto-assigned)", got.Number)
	}
	got, err = w.Append(context.Background(), CycleEntry{Mode: "next", Result: "improved"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 2 {
		t.Fatalf("second auto-assigned Number = %d, want 2", got.Number)
	}
}

func TestInMemoryLoopWriter_AppendHonorsExplicitNumber(t *testing.T) {
	w := NewInMemoryLoopWriter(nil)
	got, err := w.Append(context.Background(), CycleEntry{Number: 42, Result: "improved"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 42 {
		t.Fatalf("Number = %d, want 42 (explicit)", got.Number)
	}
	// Next auto-assigned should be 43
	got, err = w.Append(context.Background(), CycleEntry{Result: "improved"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 43 {
		t.Fatalf("auto-assigned after explicit = %d, want 43", got.Number)
	}
}

func TestInMemoryLoopWriter_AppendRejectsDuplicateNumber(t *testing.T) {
	w := NewInMemoryLoopWriter(nil)
	if _, err := w.Append(context.Background(), CycleEntry{Number: 5, Result: "improved"}); err != nil {
		t.Fatal(err)
	}
	_, err := w.Append(context.Background(), CycleEntry{Number: 5, Result: "improved"})
	if err == nil {
		t.Fatal("expected duplicate-Number error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want substring 'duplicate'", err)
	}
}

func TestInMemoryLoopWriter_SeededEntriesUsedAsInitialState(t *testing.T) {
	seed := []CycleEntry{
		{Number: 1, Result: "improved"},
		{Number: 2, Result: "improved"},
		{Number: 3, Result: "idle"},
	}
	w := NewInMemoryLoopWriter(seed)
	got, err := w.Append(context.Background(), CycleEntry{Result: "improved"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 4 {
		t.Fatalf("auto-assigned after seed = %d, want 4", got.Number)
	}
	snap := w.Snapshot()
	if len(snap) != 4 {
		t.Fatalf("Snapshot len = %d, want 4", len(snap))
	}
}

func TestInMemoryLoopWriter_HonorsContextCancellation(t *testing.T) {
	w := NewInMemoryLoopWriter(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.Append(ctx, CycleEntry{Result: "improved"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestInMemoryLoopWriter_SnapshotIsDefensiveCopy(t *testing.T) {
	w := NewInMemoryLoopWriter(nil)
	_, _ = w.Append(context.Background(), CycleEntry{Number: 1, Result: "improved"})
	snap1 := w.Snapshot()
	if len(snap1) != 1 {
		t.Fatalf("len = %d, want 1", len(snap1))
	}
	// Mutate the returned snapshot — should not affect writer state
	snap1[0].Result = "mutated"
	snap2 := w.Snapshot()
	if snap2[0].Result != "improved" {
		t.Fatalf("Snapshot is not defensive: got mutated state %q", snap2[0].Result)
	}
}
