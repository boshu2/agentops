// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Sibling pattern: cycle 111 harness_adapter_test.go.

func mustWriteMarkdown(t *testing.T, root, rel, body string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func TestProductionCorpusReader_LookupReturnsMatch(t *testing.T) {
	root := t.TempDir()
	mustWriteMarkdown(t, root, "a.md", "# Hexagonal architecture\n\nbody mentions cycle")
	mustWriteMarkdown(t, root, "b.md", "# Unrelated\n\nnothing here")
	r := newProductionCorpusReader(root)
	items, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "hexagonal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0].Title != "Hexagonal architecture" {
		t.Fatalf("Title = %q", items[0].Title)
	}
	if items[0].Score == 0 {
		t.Fatal("expected non-zero Score for title hit")
	}
}

func TestProductionCorpusReader_TitleHitOutranksBodyHit(t *testing.T) {
	root := t.TempDir()
	mustWriteMarkdown(t, root, "body-only.md", "# Other\n\nmentions evolve in body")
	mustWriteMarkdown(t, root, "title.md", "# evolve loop\n\nbody")
	r := newProductionCorpusReader(root)
	items, _ := r.Lookup(context.Background(), ports.LookupOptions{Query: "evolve"})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0].Title != "evolve loop" {
		t.Fatalf("title hit should rank first; got %q", items[0].Title)
	}
}

func TestProductionCorpusReader_LimitRespected(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md", "d.md"} {
		mustWriteMarkdown(t, root, name, "# t\n\nfoo")
	}
	r := newProductionCorpusReader(root)
	items, _ := r.Lookup(context.Background(), ports.LookupOptions{Query: "foo", Limit: 2})
	if len(items) != 2 {
		t.Fatalf("Limit not respected: got %d, want 2", len(items))
	}
}

func TestProductionCorpusReader_EmptyQueryReturnsAllScoreZero(t *testing.T) {
	root := t.TempDir()
	mustWriteMarkdown(t, root, "a.md", "# A\n\nbody")
	mustWriteMarkdown(t, root, "b.md", "# B\n\nbody")
	r := newProductionCorpusReader(root)
	items, _ := r.Lookup(context.Background(), ports.LookupOptions{})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.Score != 0 {
			t.Fatalf("empty Query should give Score 0, got %f", item.Score)
		}
	}
}

func TestProductionCorpusReader_NonMarkdownFilesSkipped(t *testing.T) {
	root := t.TempDir()
	mustWriteMarkdown(t, root, "real.md", "# real\n\nbody")
	mustWriteMarkdown(t, root, "noise.txt", "ignored")
	r := newProductionCorpusReader(root)
	items, _ := r.Lookup(context.Background(), ports.LookupOptions{})
	if len(items) != 1 || items[0].Title != "real" {
		t.Fatalf("got %+v, want only real.md", items)
	}
}

func TestProductionCorpusReader_NestedDirectoriesWalked(t *testing.T) {
	root := t.TempDir()
	mustWriteMarkdown(t, root, "nested/sub/deep.md", "# deep\n\nfindme")
	r := newProductionCorpusReader(root)
	items, _ := r.Lookup(context.Background(), ports.LookupOptions{Query: "findme"})
	if len(items) != 1 || items[0].Title != "deep" {
		t.Fatalf("nested walk failed: got %+v", items)
	}
}

func TestProductionCorpusReader_MissingTitleFallsBackToFilename(t *testing.T) {
	root := t.TempDir()
	mustWriteMarkdown(t, root, "no-h1.md", "body without h1, matches query")
	r := newProductionCorpusReader(root)
	items, _ := r.Lookup(context.Background(), ports.LookupOptions{Query: "matches"})
	if len(items) != 1 || items[0].Title != "no-h1.md" {
		t.Fatalf("fallback title wrong: got %+v", items)
	}
}

func TestProductionCorpusReader_MissingRootIsEmpty(t *testing.T) {
	r := newProductionCorpusReader(filepath.Join(t.TempDir(), "does-not-exist"))
	items, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Fatal("missing root should return non-nil empty slice")
	}
	if len(items) != 0 {
		t.Fatalf("len = %d, want 0", len(items))
	}
}

func TestProductionCorpusReader_EmptyRootDirReturnsEmpty(t *testing.T) {
	r := newProductionCorpusReader("")
	items, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("len = %d, want 0", len(items))
	}
}

func TestProductionCorpusReader_HonorsContextCancellation(t *testing.T) {
	r := newProductionCorpusReader(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Lookup(ctx, ports.LookupOptions{Query: "x"})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
