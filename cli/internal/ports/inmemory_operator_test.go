// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

// Sibling pattern: inmemory_loop_writer_test.go (cycle 103).

func TestInMemoryOperator_RecordAcceptsValidIntent(t *testing.T) {
	a := NewInMemoryOperator()
	err := a.Record(context.Background(), OperatorIntent{Kind: "halt", Subject: "soc-test", Note: "investigating"})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	list, _ := a.List(context.Background())
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0].Kind != "halt" {
		t.Fatalf("Kind = %q, want halt", list[0].Kind)
	}
}

func TestInMemoryOperator_RecordRejectsEmptyKind(t *testing.T) {
	a := NewInMemoryOperator()
	err := a.Record(context.Background(), OperatorIntent{Subject: "x"})
	if err == nil {
		t.Fatal("expected error on empty Kind, got nil")
	}
}

func TestInMemoryOperator_ListReturnsMostRecentFirst(t *testing.T) {
	a := NewInMemoryOperator()
	_ = a.Record(context.Background(), OperatorIntent{Kind: "first"})
	_ = a.Record(context.Background(), OperatorIntent{Kind: "second"})
	_ = a.Record(context.Background(), OperatorIntent{Kind: "third"})
	list, _ := a.List(context.Background())
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	if list[0].Kind != "third" || list[2].Kind != "first" {
		t.Fatalf("order = %v, want [third, second, first]",
			[]string{list[0].Kind, list[1].Kind, list[2].Kind})
	}
}

func TestInMemoryOperator_ListEmptyIsNonNil(t *testing.T) {
	a := NewInMemoryOperator()
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Fatal("List returned nil; should be non-nil empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}

func TestInMemoryOperator_HonorsContextCancellation(t *testing.T) {
	a := NewInMemoryOperator()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Record(ctx, OperatorIntent{Kind: "halt"}); err == nil {
		t.Fatal("Record: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Record error = %v, want context.Canceled", err)
	}
	if _, err := a.List(ctx); err == nil {
		t.Fatal("List: expected cancellation error, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v, want context.Canceled", err)
	}
}

func TestInMemoryOperator_KindSubjectNoteAllPreserved(t *testing.T) {
	a := NewInMemoryOperator()
	in := OperatorIntent{Kind: "rescope", Subject: "soc-2c1p", Note: "split into BC ports"}
	_ = a.Record(context.Background(), in)
	list, _ := a.List(context.Background())
	if list[0] != in {
		t.Fatalf("round-trip lost data: got %+v, want %+v", list[0], in)
	}
}
