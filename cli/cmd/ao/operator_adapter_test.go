// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Sibling pattern: cycle 109 loop_writer_adapter_test.go.

func TestProductionOperator_RecordCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "intents.jsonl")
	a := newProductionOperator(path)
	err := a.Record(context.Background(), ports.OperatorIntent{
		Kind: "halt", Subject: "soc-test", Note: "investigate",
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"kind":"halt"`) {
		t.Fatalf("file missing kind:halt\n%s", body)
	}
	if !strings.Contains(string(body), `"subject":"soc-test"`) {
		t.Fatalf("file missing subject:soc-test\n%s", body)
	}
}

func TestProductionOperator_RecordRejectsEmptyKind(t *testing.T) {
	dir := t.TempDir()
	a := newProductionOperator(filepath.Join(dir, "x.jsonl"))
	err := a.Record(context.Background(), ports.OperatorIntent{Subject: "x"})
	if err == nil {
		t.Fatal("expected error on empty Kind, got nil")
	}
}

func TestProductionOperator_RecordEmptyPathErrors(t *testing.T) {
	a := newProductionOperator("")
	err := a.Record(context.Background(), ports.OperatorIntent{Kind: "halt"})
	if err == nil {
		t.Fatal("expected error on empty path, got nil")
	}
}

func TestProductionOperator_ListReturnsMostRecentFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "intents.jsonl")
	a := newProductionOperator(path)
	for _, kind := range []string{"first", "second", "third"} {
		_ = a.Record(context.Background(), ports.OperatorIntent{Kind: kind})
	}
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	if list[0].Kind != "third" || list[2].Kind != "first" {
		t.Fatalf("order = %v, want [third, second, first]",
			[]string{list[0].Kind, list[1].Kind, list[2].Kind})
	}
}

func TestProductionOperator_ListMissingFileReturnsEmpty(t *testing.T) {
	a := newProductionOperator(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Fatal("missing-file List should return non-nil empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}

func TestProductionOperator_ListMalformedLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "intents.jsonl")
	body := `{"kind":"good1"}
not json
{"broken json:
{"kind":"good2"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newProductionOperator(path)
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2 (malformed skipped)", len(list))
	}
}

func TestProductionOperator_HonorsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	a := newProductionOperator(filepath.Join(dir, "x.jsonl"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Record(ctx, ports.OperatorIntent{Kind: "x"}); err == nil {
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

func TestProductionOperator_RoundTripPreservesAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "intents.jsonl")
	a := newProductionOperator(path)
	in := ports.OperatorIntent{Kind: "rescope", Subject: "soc-2c1p", Note: "split into ports"}
	if err := a.Record(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	list, err := a.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list[0] != in {
		t.Fatalf("round-trip lost data: got %+v, want %+v", list[0], in)
	}
}
