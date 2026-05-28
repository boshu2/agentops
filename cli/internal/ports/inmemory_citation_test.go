package ports

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Sibling pattern: inmemory_finding_compiler_test.go (cycle 80). Same
// shape — table-driven where helpful, L1-style assertions for
// behavior + port contract.

func TestInMemoryCitation_VerifyFreshWhenPresentInKnownSet(t *testing.T) {
	known := []CitationRequest{
		{Kind: CitationKindFunction, Raw: "func TargetFn"},
		{Kind: CitationKindSymbol, Raw: "`UNIQ_SYM`"},
		{Kind: CitationKindFile, Raw: "skills/foo.md"},
	}
	a := NewInMemoryCitation(known)

	for _, req := range known {
		t.Run(string(req.Kind)+"/"+req.Raw, func(t *testing.T) {
			got, err := a.Verify(context.Background(), req)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if got.Status != CitationStatusFresh {
				t.Fatalf("Status = %q, want FRESH (reason: %s)", got.Status, got.Reason)
			}
			if got.Reason == "" {
				t.Fatal("Reason must be non-empty per port contract")
			}
			if got.Resolved != req.Raw {
				t.Fatalf("Resolved = %q, want %q", got.Resolved, req.Raw)
			}
		})
	}
}

func TestInMemoryCitation_VerifyStaleWhenAbsent(t *testing.T) {
	a := NewInMemoryCitation([]CitationRequest{
		{Kind: CitationKindFunction, Raw: "func DoesExist"},
	})
	got, err := a.Verify(context.Background(), CitationRequest{
		Kind: CitationKindFunction,
		Raw:  "func DoesNotExist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CitationStatusStale {
		t.Fatalf("Status = %q, want STALE", got.Status)
	}
	if !strings.Contains(got.Reason, "not in the known-fresh set") {
		t.Fatalf("Reason = %q, want substring 'not in the known-fresh set'", got.Reason)
	}
	if got.Resolved != "" {
		t.Fatalf("Resolved = %q, want \"\" for STALE", got.Resolved)
	}
}

func TestInMemoryCitation_VerifyEmptyRawReturnsUnknown(t *testing.T) {
	a := NewInMemoryCitation(nil)
	cases := []string{"", "  ", "\t\n"}
	for _, raw := range cases {
		t.Run("raw="+raw, func(t *testing.T) {
			got, err := a.Verify(context.Background(), CitationRequest{
				Kind: CitationKindFunction,
				Raw:  raw,
			})
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if got.Status != CitationStatusUnknown {
				t.Fatalf("Status = %q, want UNKNOWN for empty Raw", got.Status)
			}
			if got.Reason == "" {
				t.Fatal("Reason must be non-empty per port contract")
			}
		})
	}
}

func TestInMemoryCitation_VerifyKindMatters(t *testing.T) {
	// Same Raw under one Kind should not match the same Raw under
	// another Kind. This catches an adapter bug where the lookup
	// collapses Kind dimensions.
	a := NewInMemoryCitation([]CitationRequest{
		{Kind: CitationKindFunction, Raw: "ambiguous"},
	})
	got, err := a.Verify(context.Background(), CitationRequest{
		Kind: CitationKindSymbol,
		Raw:  "ambiguous",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CitationStatusStale {
		t.Fatalf("Status = %q, want STALE — Kind/Raw lookup must be exact-tuple", got.Status)
	}
}

func TestInMemoryCitation_VerifyHonorsContextCancellation(t *testing.T) {
	a := NewInMemoryCitation(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Verify(ctx, CitationRequest{Kind: CitationKindFile, Raw: "x.md"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestInMemoryCitation_VerifyEmptyKnownSetReturnsStaleForAny(t *testing.T) {
	a := NewInMemoryCitation(nil)
	got, err := a.Verify(context.Background(), CitationRequest{
		Kind: CitationKindFile,
		Raw:  "any.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CitationStatusStale {
		t.Fatalf("Status = %q, want STALE (empty known-fresh set, non-empty Raw)", got.Status)
	}
}
