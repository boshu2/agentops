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

// Sibling pattern: cycle 108 loop_reader_adapter_test.go.

func TestProductionLoopWriter_AppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	w := newProductionLoopWriter(path)
	got, err := w.Append(context.Background(), ports.CycleEntry{
		Mode: "test", Result: "improved", Commit: "abc123",
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got.Number != 1 {
		t.Fatalf("first auto-assigned Number = %d, want 1", got.Number)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"cycle":1`) {
		t.Fatalf("file does not contain cycle:1\n%s", body)
	}
	if !strings.Contains(string(body), `"commit":"abc123"`) {
		t.Fatalf("file does not contain commit:abc123\n%s", body)
	}
}

func TestProductionLoopWriter_AppendAutoAssignsMaxPlusOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	// Seed file with cycle 5
	if err := os.WriteFile(path, []byte(`{"cycle":5,"mode":"seed"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := newProductionLoopWriter(path)
	got, err := w.Append(context.Background(), ports.CycleEntry{Mode: "next"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 6 {
		t.Fatalf("Number = %d, want 6 (max 5 + 1)", got.Number)
	}
}

func TestProductionLoopWriter_AppendHonorsExplicitNumber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	w := newProductionLoopWriter(path)
	got, err := w.Append(context.Background(), ports.CycleEntry{Number: 42, Mode: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 42 {
		t.Fatalf("Number = %d, want 42", got.Number)
	}
}

func TestProductionLoopWriter_RoundTripWithReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	w := newProductionLoopWriter(path)
	for i := 1; i <= 3; i++ {
		if _, err := w.Append(context.Background(), ports.CycleEntry{
			Mode:   "test",
			Result: "improved",
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Reader sees the same file
	r := newProductionLoopReader(path)
	latest, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Number != 3 {
		t.Fatalf("round-trip Latest = %d, want 3", latest.Number)
	}
}

func TestProductionLoopWriter_EmptyPathErrors(t *testing.T) {
	w := newProductionLoopWriter("")
	_, err := w.Append(context.Background(), ports.CycleEntry{Mode: "x"})
	if err == nil {
		t.Fatal("expected error on empty path, got nil")
	}
}

func TestProductionLoopWriter_AppendIsLineSeparated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	w := newProductionLoopWriter(path)
	_, _ = w.Append(context.Background(), ports.CycleEntry{Mode: "one"})
	_, _ = w.Append(context.Background(), ports.CycleEntry{Mode: "two"})
	body, _ := os.ReadFile(path)
	lineCount := strings.Count(string(body), "\n")
	if lineCount != 2 {
		t.Fatalf("line count = %d, want 2", lineCount)
	}
}

// Cycle 162: paired with cycle 161's CycleEntry widening (soc-ckc4).
// Confirms StartedAt + Title round-trip through writer -> reader.
func TestProductionLoopWriter_RoundTripsStartedAtAndTitle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	w := newProductionLoopWriter(path)
	written, err := w.Append(context.Background(), ports.CycleEntry{
		Mode:      "phase2-widen",
		Result:    "improved",
		Commit:    "deadbee",
		Milestone: "round-trip",
		StartedAt: "2026-05-13T08:00:00-04:00",
		Title:     "soc-ckc4 widening regression",
	})
	if err != nil {
		t.Fatal(err)
	}
	if written.StartedAt != "2026-05-13T08:00:00-04:00" || written.Title != "soc-ckc4 widening regression" {
		t.Fatalf("Append returned-entry dropped new fields: %+v", written)
	}
	r := newProductionLoopReader(path)
	got, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.StartedAt != "2026-05-13T08:00:00-04:00" {
		t.Errorf("round-trip StartedAt = %q, want 2026-05-13T08:00:00-04:00", got.StartedAt)
	}
	if got.Title != "soc-ckc4 widening regression" {
		t.Errorf("round-trip Title = %q, want \"soc-ckc4 widening regression\"", got.Title)
	}
}

func TestProductionLoopWriter_HonorsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cycle-history.jsonl")
	w := newProductionLoopWriter(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.Append(ctx, ports.CycleEntry{Mode: "x"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
