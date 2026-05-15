package ports

import (
	"context"
	"errors"
	"testing"
)

// Sibling pattern: cli/internal/corpus/fitness_test.go — single-file
// per-package test with focused table-driven cases. Asserts the
// CorpusReaderPort contract from the consumer side: a future adapter
// that violates these invariants will fail this test once the same
// table is reused via a port-conformance helper (deferred to the
// per-adapter cycles).

func TestInMemoryCorpusReader_LookupSatisfiesPortContract(t *testing.T) {
	items := []CorpusItem{
		{Path: "a.md", Title: "Alpha", Body: "hello world"},
		{Path: "b.md", Title: "Beta", Body: "world world"},
		{Path: "c.md", Title: "Gamma", Body: "hello again"},
	}
	r := NewInMemoryCorpusReader(items)

	tests := []struct {
		name           string
		opts           LookupOptions
		wantLen        int
		wantFirstPath  string
		wantAllScoreGT float64
	}{
		{
			name:           "empty query returns all items with score 1",
			opts:           LookupOptions{Query: "", Limit: 0},
			wantLen:        3,
			wantFirstPath:  "a.md",
			wantAllScoreGT: 0,
		},
		{
			name:           "non-empty query filters by substring",
			opts:           LookupOptions{Query: "world", Limit: 0},
			wantLen:        2,
			wantFirstPath:  "b.md",
			wantAllScoreGT: 0,
		},
		{
			name:          "query with zero matches returns empty slice",
			opts:          LookupOptions{Query: "noMatchSentinel", Limit: 0},
			wantLen:       0,
			wantFirstPath: "",
		},
		{
			name:          "limit > 0 truncates the result",
			opts:          LookupOptions{Query: "hello", Limit: 1},
			wantLen:       1,
			wantFirstPath: "a.md",
		},
		{
			name:          "case-insensitive query matches title",
			opts:          LookupOptions{Query: "GAMMA", Limit: 0},
			wantLen:       1,
			wantFirstPath: "c.md",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Lookup(context.Background(), tc.opts)
			if err != nil {
				t.Fatalf("Lookup error: %v", err)
			}
			if got == nil {
				t.Fatalf("Lookup returned nil slice; port contract requires non-nil even when empty")
			}
			if len(got) != tc.wantLen {
				t.Fatalf("len(got) = %d, want %d (got: %+v)", len(got), tc.wantLen, got)
			}
			if tc.wantLen > 0 && got[0].Path != tc.wantFirstPath {
				t.Fatalf("got[0].Path = %q, want %q (full ranking: %+v)", got[0].Path, tc.wantFirstPath, got)
			}
			for i, item := range got {
				if item.Score <= tc.wantAllScoreGT {
					t.Fatalf("item %d score = %f, want > %f", i, item.Score, tc.wantAllScoreGT)
				}
			}
		})
	}
}

func TestInMemoryCorpusReader_LookupResultsSortedByScoreDescending(t *testing.T) {
	items := []CorpusItem{
		{Path: "low.md", Title: "x", Body: "hello"},              // 1 match
		{Path: "mid.md", Title: "x", Body: "hello hello"},        // 2 matches
		{Path: "high.md", Title: "x", Body: "hello hello hello"}, // 3 matches
		{Path: "noise.md", Title: "x", Body: "irrelevant"},       // 0
		{Path: "tie.md", Title: "x", Body: "hello"},              // 1 match (tie with low.md)
	}
	r := NewInMemoryCorpusReader(items)
	got, err := r.Lookup(context.Background(), LookupOptions{Query: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 (noise.md filtered)", len(got))
	}
	if got[0].Path != "high.md" {
		t.Fatalf("got[0].Path = %q, want high.md", got[0].Path)
	}
	if got[1].Path != "mid.md" {
		t.Fatalf("got[1].Path = %q, want mid.md", got[1].Path)
	}
	// low.md vs tie.md is a score tie; sort.SliceStable preserves insertion order.
	if got[2].Path != "low.md" || got[3].Path != "tie.md" {
		t.Fatalf("tie order: got %q,%q want low.md,tie.md", got[2].Path, got[3].Path)
	}
}

func TestInMemoryCorpusReader_LookupHonorsContextCancellation(t *testing.T) {
	r := NewInMemoryCorpusReader([]CorpusItem{{Path: "a.md", Title: "x"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := r.Lookup(ctx, LookupOptions{})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Fatalf("on error, returned slice should be nil; got %+v", got)
	}
}

func TestInMemoryCorpusReader_NilItemsArgumentIsSafe(t *testing.T) {
	r := NewInMemoryCorpusReader(nil)
	got, err := r.Lookup(context.Background(), LookupOptions{})
	if err != nil {
		t.Fatalf("Lookup over nil items errored: %v", err)
	}
	if got == nil {
		t.Fatal("nil items + Lookup must still return a non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}
