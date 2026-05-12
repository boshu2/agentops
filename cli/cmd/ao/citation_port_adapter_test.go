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

// Sibling pattern: cycle 75's TestVerifyFunctionCitation_Fresh/Stale
// shape (this same dir, beads_test.go lines 204-236). The adapter is
// a thin shim, so the tests verify: (1) each Kind routes to the right
// helper, (2) the verdict translation preserves the underlying status,
// (3) unknown Kind returns UNKNOWN with a useful Reason, (4) context
// cancellation is honored.

func TestProductionCitationAdapter_FunctionCitationFresh(t *testing.T) {
	dir := t.TempDir()
	clidir := filepath.Join(dir, "cli")
	if err := os.MkdirAll(clidir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clidir, "foo.go"),
		[]byte("package foo\nfunc AdapterFreshFn() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newProductionCitationAdapter()
	v, err := a.Verify(context.Background(), ports.CitationRequest{
		Kind: ports.CitationKindFunction,
		Raw:  "func AdapterFreshFn",
		Cwd:  dir,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Status != ports.CitationStatusFresh {
		t.Fatalf("Status = %q, want FRESH (reason: %s)", v.Status, v.Reason)
	}
	if !strings.Contains(v.Reason, "defined at") {
		t.Fatalf("Reason = %q, want substring 'defined at'", v.Reason)
	}
}

func TestProductionCitationAdapter_SymbolCitationStale(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"cli", "skills", "scripts"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a := newProductionCitationAdapter()
	v, err := a.Verify(context.Background(), ports.CitationRequest{
		Kind: ports.CitationKindSymbol,
		Raw:  "`UNIQ_TEST_q7w_NEVER_DEFINED`",
		Cwd:  dir,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Status != ports.CitationStatusStale {
		t.Fatalf("Status = %q, want STALE (reason: %s)", v.Status, v.Reason)
	}
	if !strings.Contains(v.Reason, "zero references") {
		t.Fatalf("Reason = %q, want substring 'zero references'", v.Reason)
	}
}

func TestProductionCitationAdapter_FileCitationFresh(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newProductionCitationAdapter()
	v, err := a.Verify(context.Background(), ports.CitationRequest{
		Kind: ports.CitationKindFile,
		Raw:  "real.go",
		Cwd:  dir,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Status != ports.CitationStatusFresh {
		t.Fatalf("Status = %q, want FRESH (reason: %s)", v.Status, v.Reason)
	}
}

func TestProductionCitationAdapter_UnknownKindReturnsUnknown(t *testing.T) {
	a := newProductionCitationAdapter()
	v, err := a.Verify(context.Background(), ports.CitationRequest{
		Kind: ports.CitationKind("invalid-kind"),
		Raw:  "x",
		Cwd:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.Status != ports.CitationStatusUnknown {
		t.Fatalf("Status = %q, want UNKNOWN", v.Status)
	}
	if !strings.Contains(v.Reason, "invalid-kind") {
		t.Fatalf("Reason = %q, want to name the offending kind", v.Reason)
	}
}

func TestProductionCitationAdapter_HonorsContextCancellation(t *testing.T) {
	a := newProductionCitationAdapter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Verify(ctx, ports.CitationRequest{
		Kind: ports.CitationKindFile,
		Raw:  "x.md",
		Cwd:  t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestProductionCitationAdapter_TranslateCitationStatus(t *testing.T) {
	cases := []struct {
		in   CitationStatus
		want ports.CitationStatusResult
	}{
		{CitationFresh, ports.CitationStatusFresh},
		{CitationStale, ports.CitationStatusStale},
		{CitationUnknown, ports.CitationStatusUnknown},
		{CitationStatus("garbage"), ports.CitationStatusUnknown},
	}
	for _, tc := range cases {
		got := translateCitationStatus(tc.in)
		if got != tc.want {
			t.Fatalf("translateCitationStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
