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

// Sibling pattern: cycle 112 corpus_reader_adapter_test.go.

func TestProductionCorpusWriter_CaptureCreatesFile(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	res, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "learnings/a.md",
		Body: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created {
		t.Fatal("first Capture should report Created=true")
	}
	want := filepath.Join(root, "learnings/a.md")
	if res.ResolvedPath != want {
		t.Fatalf("ResolvedPath = %q, want %q", res.ResolvedPath, want)
	}
	body, _ := os.ReadFile(want)
	if string(body) != "hello world" {
		t.Fatalf("file body = %q", body)
	}
}

func TestProductionCorpusWriter_CaptureIsIdempotent(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	req := ports.CorpusWriteRequest{Path: "x.md", Body: []byte("v1")}
	r1, _ := w.Capture(context.Background(), req)
	if !r1.Created {
		t.Fatal("first call should Created=true")
	}
	r2, _ := w.Capture(context.Background(), req)
	if r2.Created {
		t.Fatal("second Capture should report Created=false (idempotent)")
	}
	if r1.ResolvedPath != r2.ResolvedPath {
		t.Fatalf("ResolvedPath drifted: %q vs %q", r1.ResolvedPath, r2.ResolvedPath)
	}
}

func TestProductionCorpusWriter_CaptureUpdatesInPlace(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	_, _ = w.Capture(context.Background(), ports.CorpusWriteRequest{Path: "x.md", Body: []byte("v1")})
	_, _ = w.Capture(context.Background(), ports.CorpusWriteRequest{Path: "x.md", Body: []byte("v2")})
	body, _ := os.ReadFile(filepath.Join(root, "x.md"))
	if string(body) != "v2" {
		t.Fatalf("update did not overwrite: got %q", body)
	}
}

func TestProductionCorpusWriter_MetadataRenderedAsFrontmatter(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path:     "x.md",
		Body:     []byte("body content"),
		Metadata: map[string]string{"tag": "evolve", "date": "2026-05-12"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "x.md"))
	got := string(body)
	if !strings.HasPrefix(got, "---\n") {
		t.Fatalf("missing frontmatter open\n%s", got)
	}
	// Sorted: date before tag
	wantPrefix := "---\ndate: 2026-05-12\ntag: evolve\n---\nbody content"
	if got != wantPrefix {
		t.Fatalf("got:\n%s\nwant:\n%s", got, wantPrefix)
	}
}

func TestProductionCorpusWriter_PreExistingFrontmatterPreserved(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	bodyWithFM := []byte("---\nexisting: yes\n---\ncontent")
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path:     "x.md",
		Body:     bodyWithFM,
		Metadata: map[string]string{"injected": "no"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "x.md"))
	if string(got) != string(bodyWithFM) {
		t.Fatalf("frontmatter collision: got\n%s", got)
	}
	if strings.Contains(string(got), "injected") {
		t.Fatal("metadata leaked into body that already had frontmatter")
	}
}

func TestProductionCorpusWriter_RoundTripWithReader(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	_, _ = w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "deep/nested/foo.md",
		Body: []byte("# foo title\n\nmatches query"),
	})
	r := newProductionCorpusReader(root)
	items, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "matches"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("round-trip Lookup len = %d, want 1", len(items))
	}
	if items[0].Title != "foo title" {
		t.Fatalf("Title = %q", items[0].Title)
	}
}

func TestProductionCorpusWriter_AbsolutePathRejected(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "/etc/passwd",
		Body: []byte("nope"),
	})
	if err == nil {
		t.Fatal("expected error on absolute path, got nil")
	}
}

func TestProductionCorpusWriter_ParentTraversalRejected(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "../escape.md",
		Body: []byte("nope"),
	})
	if err == nil {
		t.Fatal("expected error on parent traversal, got nil")
	}
}

func TestProductionCorpusWriter_EmptyPathErrors(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{Body: []byte("x")})
	if err == nil {
		t.Fatal("expected error on empty Path, got nil")
	}
}

func TestProductionCorpusWriter_EmptyRootErrors(t *testing.T) {
	w := newProductionCorpusWriter("")
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{Path: "x.md", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected error on empty rootDir, got nil")
	}
}

func TestProductionCorpusWriter_HonorsContextCancellation(t *testing.T) {
	root := t.TempDir()
	w := newProductionCorpusWriter(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.Capture(ctx, ports.CorpusWriteRequest{Path: "x.md", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
