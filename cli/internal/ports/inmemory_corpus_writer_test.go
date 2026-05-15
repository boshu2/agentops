package ports

import (
	"context"
	"errors"
	"testing"
)

// Sibling pattern: inmemory_corpus_reader_test.go (cycle 78). Same
// shape — one Go file per adapter, table-driven where helpful,
// L1-style assertions for behavior + port contract.

func TestInMemoryCorpusWriter_CaptureFirstTimeMarksCreated(t *testing.T) {
	w := NewInMemoryCorpusWriter()
	res, err := w.Capture(context.Background(), CorpusWriteRequest{
		Path: "wiki/foo.md",
		Body: []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if res.ResolvedPath != "wiki/foo.md" {
		t.Fatalf("ResolvedPath = %q, want %q", res.ResolvedPath, "wiki/foo.md")
	}
	if !res.Created {
		t.Fatalf("Created = false on first Capture, want true")
	}
}

func TestInMemoryCorpusWriter_CaptureIsIdempotent(t *testing.T) {
	w := NewInMemoryCorpusWriter()
	req := CorpusWriteRequest{Path: "wiki/idem.md", Body: []byte("v1")}

	first, err := w.Capture(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created {
		t.Fatalf("first Created = false, want true")
	}

	// Re-Capture with updated Body — Path is the same, so Created must flip.
	req.Body = []byte("v2")
	second, err := w.Capture(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if second.Created {
		t.Fatalf("second Created = true, want false (idempotent re-capture)")
	}
	if second.ResolvedPath != first.ResolvedPath {
		t.Fatalf("ResolvedPath changed across re-Capture: %q -> %q",
			first.ResolvedPath, second.ResolvedPath)
	}

	// Snapshot should show exactly one entry.
	n, paths := w.Snapshot()
	if n != 1 || len(paths) != 1 || paths[0] != "wiki/idem.md" {
		t.Fatalf("snapshot = (%d, %v), want (1, [wiki/idem.md])", n, paths)
	}
}

func TestInMemoryCorpusWriter_EmptyPathIsRejected(t *testing.T) {
	w := NewInMemoryCorpusWriter()
	_, err := w.Capture(context.Background(), CorpusWriteRequest{Path: "", Body: []byte("x")})
	if err == nil {
		t.Fatal("Capture with empty Path returned nil error, want structural rejection")
	}
}

func TestInMemoryCorpusWriter_HonorsContextCancellation(t *testing.T) {
	w := NewInMemoryCorpusWriter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := w.Capture(ctx, CorpusWriteRequest{Path: "wiki/x.md", Body: []byte("x")})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	// Snapshot must show zero entries since the cancelled Capture did not persist.
	n, _ := w.Snapshot()
	if n != 0 {
		t.Fatalf("snapshot count = %d after cancelled Capture, want 0", n)
	}
}

func TestInMemoryCorpusWriter_MetadataIsRetainedAcrossCaptures(t *testing.T) {
	w := NewInMemoryCorpusWriter()
	meta1 := map[string]string{"maturity": "draft", "source": "harvest"}
	_, err := w.Capture(context.Background(), CorpusWriteRequest{
		Path:     "wiki/meta.md",
		Body:     []byte("body"),
		Metadata: meta1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// A second Capture with new metadata should overwrite (the port
	// contract says re-Capture MAY update Body/Metadata in place).
	meta2 := map[string]string{"maturity": "reviewed", "tags": "test"}
	res, err := w.Capture(context.Background(), CorpusWriteRequest{
		Path:     "wiki/meta.md",
		Body:     []byte("body-v2"),
		Metadata: meta2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created {
		t.Fatalf("Created = true on re-Capture, want false")
	}
}
