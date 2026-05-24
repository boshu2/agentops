package llmwiki

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectStage_PrefersIngestWhenRawHasNew(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "raw", "foo.md"), []byte("raw"), 0o644); err != nil {
		t.Fatalf("write raw/foo.md: %v", err)
	}
	got := SelectStage(vault, 24, time.Now())
	if got != StageIngest {
		t.Fatalf("SelectStage = %q, want %q", got, StageIngest)
	}
}

func TestSelectStage_PrefersLintWhenStale(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	// raw/ is empty → no Ingest preference.
	// wiki/.last-lint exists but is old → Lint should win.
	sentinel := filepath.Join(vault, "wiki", ".last-lint")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(sentinel, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	got := SelectStage(vault, 24, time.Now())
	if got != StageLint {
		t.Fatalf("SelectStage = %q, want %q", got, StageLint)
	}
}

func TestSelectStage_DefaultsToIngestWhenNothingDue(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "raw"), 0o755); err != nil {
		t.Fatalf("mkdir raw: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "wiki"), 0o755); err != nil {
		t.Fatalf("mkdir wiki: %v", err)
	}
	// Fresh lint sentinel; empty raw/. Conservative default is StageIngest.
	if err := os.WriteFile(filepath.Join(vault, "wiki", ".last-lint"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write last-lint: %v", err)
	}
	got := SelectStage(vault, 24, time.Now())
	if got != StageIngest {
		t.Fatalf("SelectStage = %q, want %q", got, StageIngest)
	}
}
