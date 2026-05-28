package corpus_fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// --- Reader tests ---

func TestReader_LookupRanksTitleHitsHigher(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.md"), []byte("# needle title\nplain body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beta.md"), []byte("# Beta\nneedle in body only\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := NewReader(root)
	got, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "needle"})
	if err != nil {
		t.Fatalf("Lookup err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
	// alpha has a title+body hit (score 3); beta has body only (score 1).
	if got[0].Title != "needle title" {
		t.Fatalf("highest-ranked title = %q, want %q", got[0].Title, "needle title")
	}
	if got[0].Score != 3 {
		t.Fatalf("alpha score = %v, want 3", got[0].Score)
	}
	if got[1].Score != 1 {
		t.Fatalf("beta score = %v, want 1", got[1].Score)
	}
}

func TestReader_LookupEmptyQueryReturnsAllScoreZero(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# A\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte("# B\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Non-md file must be skipped.
	if err := os.WriteFile(filepath.Join(root, "c.txt"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := NewReader(root)
	got, err := r.Lookup(context.Background(), ports.LookupOptions{})
	if err != nil {
		t.Fatalf("Lookup err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2 (non-md skipped)", len(got))
	}
	for _, it := range got {
		if it.Score != 0 {
			t.Fatalf("empty-query score = %v, want 0 for %q", it.Score, it.Path)
		}
	}
}

func TestReader_LookupHonorsLimit(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("# T\nneedle\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := NewReader(root)
	got, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "needle", Limit: 2})
	if err != nil {
		t.Fatalf("Lookup err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2 (Limit honored)", len(got))
	}
}

func TestReader_LookupTitleFallsBackToFilename(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "no-heading.md"), []byte("just needle body, no h1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewReader(root)
	got, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "needle"})
	if err != nil {
		t.Fatalf("Lookup err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	if got[0].Title != "no-heading.md" {
		t.Fatalf("Title = %q, want filename fallback %q", got[0].Title, "no-heading.md")
	}
}

func TestReader_LookupMissingRootReturnsEmpty(t *testing.T) {
	r := NewReader(filepath.Join(t.TempDir(), "does-not-exist"))
	got, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "x"})
	if err != nil {
		t.Fatalf("missing root should be tolerated, got err: %v", err)
	}
	if got == nil {
		t.Fatal("Lookup returned nil slice; contract requires non-nil")
	}
	if len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}

func TestReader_LookupEmptyRootReturnsEmpty(t *testing.T) {
	r := NewReader("")
	got, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "x"})
	if err != nil {
		t.Fatalf("empty root err: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %v, want empty non-nil slice", got)
	}
}

func TestReader_LookupRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup needs elevated privileges on some Windows hosts")
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside\nneedle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.md")); err != nil {
		t.Fatal(err)
	}
	r := NewReader(root)
	_, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "needle"})
	if err == nil {
		t.Fatal("Lookup succeeded through a symlink escape; want root-scoped read error")
	}
}

func TestReader_LookupHonorsContextCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# A\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := NewReader(root)
	_, err := r.Lookup(ctx, ports.LookupOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// --- Writer tests ---

func TestWriter_CaptureCreatesFile(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
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
		t.Fatalf("file body = %q, want %q", body, "hello world")
	}
}

func TestWriter_CaptureIsIdempotent(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	req := ports.CorpusWriteRequest{Path: "x.md", Body: []byte("v1")}
	r1, err := w.Capture(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !r1.Created {
		t.Fatal("first call should Created=true")
	}
	r2, err := w.Capture(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Created {
		t.Fatal("second Capture should report Created=false (idempotent)")
	}
	if r1.ResolvedPath != r2.ResolvedPath {
		t.Fatalf("ResolvedPath drifted: %q vs %q", r1.ResolvedPath, r2.ResolvedPath)
	}
}

func TestWriter_CaptureUpdatesInPlace(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	if _, err := w.Capture(context.Background(), ports.CorpusWriteRequest{Path: "x.md", Body: []byte("v1")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Capture(context.Background(), ports.CorpusWriteRequest{Path: "x.md", Body: []byte("v2")}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "x.md"))
	if string(body) != "v2" {
		t.Fatalf("update did not overwrite: got %q, want %q", body, "v2")
	}
}

func TestWriter_MetadataRenderedAsSortedFrontmatter(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
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
	// Sorted: date before tag.
	want := "---\ndate: 2026-05-12\ntag: evolve\n---\nbody content"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriter_PreExistingFrontmatterPreserved(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
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

func TestWriter_RoundTripWithReader(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	if _, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "deep/nested/foo.md",
		Body: []byte("# foo title\n\nmatches query"),
	}); err != nil {
		t.Fatal(err)
	}
	r := NewReader(root)
	items, err := r.Lookup(context.Background(), ports.LookupOptions{Query: "matches"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("round-trip Lookup len = %d, want 1", len(items))
	}
	if items[0].Title != "foo title" {
		t.Fatalf("Title = %q, want %q", items[0].Title, "foo title")
	}
}

func TestWriter_AbsolutePathRejected(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "/etc/passwd",
		Body: []byte("nope"),
	})
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("err = %v, want errors.Is(err, ErrPathEscapesRoot)", err)
	}
}

func TestWriter_ParentTraversalRejected(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{
		Path: "../escape.md",
		Body: []byte("nope"),
	})
	if !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("err = %v, want errors.Is(err, ErrPathEscapesRoot)", err)
	}
}

func TestWriter_EmptyPathErrors(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{Body: []byte("x")})
	if !errors.Is(err, ErrPathRequired) {
		t.Fatalf("err = %v, want errors.Is(err, ErrPathRequired)", err)
	}
}

func TestWriter_EmptyRootErrors(t *testing.T) {
	w := NewWriter("")
	_, err := w.Capture(context.Background(), ports.CorpusWriteRequest{Path: "x.md", Body: []byte("x")})
	if !errors.Is(err, ErrRootRequired) {
		t.Fatalf("err = %v, want errors.Is(err, ErrRootRequired)", err)
	}
}

func TestWriter_HonorsContextCancellation(t *testing.T) {
	root := t.TempDir()
	w := NewWriter(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.Capture(ctx, ports.CorpusWriteRequest{Path: "x.md", Body: []byte("x")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
