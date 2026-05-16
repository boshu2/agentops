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

func TestProductionHypothesisLedger_AppendListFindRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypotheses.jsonl")
	ledger := newProductionHypothesisLedger(path)

	got, err := ledger.Append(context.Background(), ports.HypothesisRecord{
		ID:           "H193.1",
		CycleLanded:  193,
		CheckAtCycle: 196,
		Patch:        "file-backed hypothesis ledger",
		Hypothesis:   "evolve can read empirical claims through a port",
		Measure:      "adapter test passes",
		Verdict:      ports.HypothesisVerdictPending,
		Evidence:     []string{"go test ./cmd/ao"},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got.ID != "H193.1" {
		t.Fatalf("Append ID = %q, want H193.1", got.ID)
	}

	records, err := ledger.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List len = %d, want 1", len(records))
	}
	if records[0].CycleLanded != 193 {
		t.Fatalf("CycleLanded = %d, want 193", records[0].CycleLanded)
	}

	found, ok, err := ledger.Find(context.Background(), "H193.1")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !ok {
		t.Fatal("Find ok = false, want true")
	}
	if found.Measure != "adapter test passes" {
		t.Fatalf("Measure = %q, want adapter test passes", found.Measure)
	}
}

func TestProductionHypothesisLedger_MissingFileReturnsEmpty(t *testing.T) {
	ledger := newProductionHypothesisLedger(filepath.Join(t.TempDir(), "missing.jsonl"))
	records, err := ledger.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("List len = %d, want 0", len(records))
	}
	record, ok, err := ledger.Find(context.Background(), "H404")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if ok {
		t.Fatal("Find ok = true, want false")
	}
	if record.ID != "" {
		t.Fatalf("record.ID = %q, want zero-value", record.ID)
	}
}

func TestProductionHypothesisLedger_SkipsMalformedAndEmptyIDRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypotheses.jsonl")
	content := strings.Join([]string{
		`{"id":"H1","verdict":"PENDING"}`,
		`{bad json`,
		`{"verdict":"PENDING"}`,
		`{"id":"H2","evidence":["kept"]}`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := newProductionHypothesisLedger(path)

	records, err := ledger.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("List len = %d, want 2", len(records))
	}
	if records[0].ID != "H1" || records[1].ID != "H2" {
		t.Fatalf("IDs = %q,%q, want H1,H2", records[0].ID, records[1].ID)
	}
}

func TestProductionHypothesisLedger_RejectsEmptyAndDuplicateIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypotheses.jsonl")
	ledger := newProductionHypothesisLedger(path)

	if _, err := ledger.Append(context.Background(), ports.HypothesisRecord{}); err == nil {
		t.Fatal("expected empty-ID error, got nil")
	}
	if _, err := ledger.Append(context.Background(), ports.HypothesisRecord{ID: "H1"}); err != nil {
		t.Fatal(err)
	}
	_, err := ledger.Append(context.Background(), ports.HypothesisRecord{ID: "H1"})
	if err == nil {
		t.Fatal("expected duplicate-ID error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate substring", err)
	}
}

func TestProductionHypothesisLedger_ReturnsDefensiveCopies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypotheses.jsonl")
	ledger := newProductionHypothesisLedger(path)
	input := ports.HypothesisRecord{ID: "H1", Evidence: []string{"original"}}

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

func TestProductionHypothesisLedger_HonorsContextCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hypotheses.jsonl")
	ledger := newProductionHypothesisLedger(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ledger.Append(ctx, ports.HypothesisRecord{ID: "H1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Append error = %v, want context.Canceled", err)
	}
	if _, err := ledger.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v, want context.Canceled", err)
	}
	if _, _, err := ledger.Find(ctx, "H1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Find error = %v, want context.Canceled", err)
	}
}

func TestProductionHypothesisLedger_EmptyFindIDErrors(t *testing.T) {
	ledger := newProductionHypothesisLedger(filepath.Join(t.TempDir(), "hypotheses.jsonl"))
	if _, _, err := ledger.Find(context.Background(), ""); err == nil {
		t.Fatal("expected empty-ID error, got nil")
	}
}
