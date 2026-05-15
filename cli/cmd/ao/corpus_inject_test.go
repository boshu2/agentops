// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestCorpusInject_StubReturnsItems(t *testing.T) {
	stub := func(_ context.Context, _ corpusInjectOptions) ([]ports.CorpusItem, error) {
		return []ports.CorpusItem{
			{Path: "/p/a.md", Title: "alpha", Body: "body-a", Score: 2.0},
			{Path: "/p/b.md", Title: "beta", Body: "body-b", Score: 1.0},
		}, nil
	}
	var buf bytes.Buffer
	err := corpusInjectRun(context.Background(), corpusInjectOptions{
		query:    "x",
		writer:   &buf,
		injectFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Title":"alpha"`) {
		t.Fatalf("first line missing alpha: %s", lines[0])
	}
}

func TestCorpusInject_EmptyResultsEmitsZeroLines(t *testing.T) {
	stub := func(_ context.Context, _ corpusInjectOptions) ([]ports.CorpusItem, error) {
		return []ports.CorpusItem{}, nil
	}
	var buf bytes.Buffer
	err := corpusInjectRun(context.Background(), corpusInjectOptions{
		query:    "nomatch",
		writer:   &buf,
		injectFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty result should emit 0 bytes, got %q", buf.String())
	}
}

func TestCorpusInject_ErrorPropagates(t *testing.T) {
	stub := func(_ context.Context, _ corpusInjectOptions) ([]ports.CorpusItem, error) {
		return nil, errors.New("corpus root unavailable")
	}
	err := corpusInjectRun(context.Background(), corpusInjectOptions{
		writer:   nil, // also exercise nil-writer default
		injectFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "corpus inject:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestCorpusInject_LiveRootWalksTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	_ = os.WriteFile(a, []byte("# alpha\n\nhexagonal pattern body"), 0o644)
	_ = os.WriteFile(b, []byte("# beta\n\nunrelated"), 0o644)

	var buf bytes.Buffer
	err := corpusInjectRun(context.Background(), corpusInjectOptions{
		query:  "hexagonal",
		root:   dir,
		limit:  5,
		writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("len = %d, want 1 (only alpha matches)", len(lines))
	}
	if !strings.Contains(lines[0], `"Title":"alpha"`) {
		t.Fatalf("expected alpha as match, got: %s", lines[0])
	}
}

func TestCorpusInject_RespectsLimit(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md", "d.md"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("# t\n\nfoo"), 0o644)
	}
	var buf bytes.Buffer
	err := corpusInjectRun(context.Background(), corpusInjectOptions{
		query:  "foo",
		root:   dir,
		limit:  2,
		writer: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("limit not honored: %d lines", len(lines))
	}
}
