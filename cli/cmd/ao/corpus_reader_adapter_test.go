// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestProductionCorpusReaderLookupUsesRootScopedReads(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.md"), []byte("# Alpha\nneedle body\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := newProductionCorpusReader(root)
	got, err := reader.Lookup(context.Background(), ports.LookupOptions{Query: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	if got[0].Title != "Alpha" {
		t.Fatalf("title = %q, want Alpha", got[0].Title)
	}
}

func TestProductionCorpusReaderRejectsSymlinkEscape(t *testing.T) {
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

	reader := newProductionCorpusReader(root)
	_, err := reader.Lookup(context.Background(), ports.LookupOptions{Query: "needle"})
	if err == nil {
		t.Fatal("Lookup succeeded through a symlink escape; want root-scoped read error")
	}
}
