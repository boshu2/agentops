// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

// Sibling pattern: inmemory_ci_status_test.go (cycle 100). Same shape.

func sampleCycleEntries() []CycleEntry {
	return []CycleEntry{
		{Number: 1, Mode: "stabilization", Result: "improved", Commit: "aaaa"},
		{Number: 2, Mode: "feature", Result: "improved", Commit: "bbbb"},
		{Number: 3, Mode: "idle", Result: "idle", Commit: ""},
		{Number: 4, Mode: "idle", Result: "unchanged", Commit: ""},
	}
}

func TestInMemoryLoopReader_LatestReturnsHighestNumber(t *testing.T) {
	r := NewInMemoryLoopReader(sampleCycleEntries())
	v, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 4 {
		t.Fatalf("Number = %d, want 4", v.Number)
	}
	if v.Result != "unchanged" {
		t.Fatalf("Result = %q, want 'unchanged'", v.Result)
	}
}

func TestInMemoryLoopReader_LatestEmptyLedgerReturnsZeroValue(t *testing.T) {
	r := NewInMemoryLoopReader(nil)
	v, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 0 || v.Mode != "" || v.Result != "" {
		t.Fatalf("expected zero-value, got %+v", v)
	}
}

func TestInMemoryLoopReader_RangeInclusive(t *testing.T) {
	r := NewInMemoryLoopReader(sampleCycleEntries())
	got, err := r.Range(context.Background(), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Number != 2 || got[1].Number != 3 {
		t.Fatalf("got numbers = [%d, %d], want [2, 3]", got[0].Number, got[1].Number)
	}
}

func TestInMemoryLoopReader_RangeOutOfBoundsReturnsEmpty(t *testing.T) {
	r := NewInMemoryLoopReader(sampleCycleEntries())
	got, err := r.Range(context.Background(), 100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("Range returned nil; should be non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestInMemoryLoopReader_IdleStreakCountsTrailing(t *testing.T) {
	r := NewInMemoryLoopReader(sampleCycleEntries())
	got, err := r.IdleStreak(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Sample ends with idle, unchanged → streak of 2
	if got != 2 {
		t.Fatalf("IdleStreak = %d, want 2", got)
	}
}

func TestInMemoryLoopReader_IdleStreakZeroWhenLastIsProductive(t *testing.T) {
	r := NewInMemoryLoopReader([]CycleEntry{
		{Number: 1, Result: "idle"},
		{Number: 2, Result: "improved"}, // most recent
	})
	got, err := r.IdleStreak(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("IdleStreak = %d, want 0", got)
	}
}

func TestInMemoryLoopReader_IdleStreakAllIdleReturnsTotal(t *testing.T) {
	r := NewInMemoryLoopReader([]CycleEntry{
		{Number: 1, Result: "idle"},
		{Number: 2, Result: "unchanged"},
		{Number: 3, Result: "idle"},
	})
	got, err := r.IdleStreak(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("IdleStreak = %d, want 3", got)
	}
}

func TestInMemoryLoopReader_HonorsContextCancellation(t *testing.T) {
	r := NewInMemoryLoopReader(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, fn := range []struct {
		name string
		call func() error
	}{
		{"Latest", func() error { _, err := r.Latest(ctx); return err }},
		{"Range", func() error { _, err := r.Range(ctx, 0, 1); return err }},
		{"IdleStreak", func() error { _, err := r.IdleStreak(ctx); return err }},
	} {
		err := fn.call()
		if err == nil {
			t.Fatalf("%s: expected cancellation error, got nil", fn.name)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s error = %v, want context.Canceled", fn.name, err)
		}
	}
}
