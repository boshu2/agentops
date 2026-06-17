// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// TestLearning_CanonicalRoundTrip is the age-ktd reconciliation guard:
// the single canonical learning record shape is search.Learning, and a
// writer→reader round-trip must be lossless for the load-bearing fields
// (ID, Title, Category, source bead).
//
// Fixture Fidelity (.claude/rules/go.md): the fixture is NOT hand-built
// in memory. It is produced by SERIALIZING with the production writer
// (writePendingLearnings) to a real on-disk pending file, then READING
// it back with the production reader (search.ParseLearningFile). This
// asserts against the exact persisted shape forge actually emits.
func TestLearning_CanonicalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	session := &storage.Session{
		ID:   "roundtrip-session-abc123",
		Date: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Knowledge: []string{
			"Use table-driven tests for multi-case functions",
		},
		Decisions: []string{
			"We decided to use auto-promote instead of relay",
		},
	}

	// --- SERIALIZE with the production writer ---
	n, err := writePendingLearnings(session, dir)
	if err != nil {
		t.Fatalf("writePendingLearnings: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 pending files written, got %d", n)
	}

	pendingDir := filepath.Join(dir, ".agents", "knowledge", "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		t.Fatalf("read pending dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 files on disk, got %d", len(entries))
	}

	// Build the expected canonical records by re-deriving the same id/title/
	// category the writer derives, then assert the reader recovers them.
	// (We read the writer's output, not a synthetic fixture.)
	type want struct {
		id       string
		title    string
		category string
	}
	// Files are <date>-<sessionShort>-<i+1>.md, i in writer order:
	// item 0 = the Knowledge string, item 1 = the Decision string.
	wantByID := map[string]want{
		"2026-03-20-roundtr-1": {
			id:       "2026-03-20-roundtr-1",
			title:    "Use table-driven tests for multi-case functions",
			category: "learning",
		},
		"2026-03-20-roundtr-2": {
			id:       "2026-03-20-roundtr-2",
			title:    "We decided to use auto-promote instead of relay",
			category: "decision",
		},
	}

	seen := map[string]bool{}
	for _, e := range entries {
		path := filepath.Join(pendingDir, e.Name())

		// --- READ back with the production reader ---
		got, err := search.ParseLearningFile(path)
		if err != nil {
			t.Fatalf("ParseLearningFile(%s): %v", e.Name(), err)
		}

		w, ok := wantByID[got.ID]
		if !ok {
			t.Fatalf("read unexpected ID %q from %s (round-trip lost the canonical id; reader fell back to filename?)", got.ID, e.Name())
		}
		seen[got.ID] = true

		// ID: canonical, no .md filename fallback.
		if got.ID != w.id {
			t.Errorf("%s: ID = %q, want %q", e.Name(), got.ID, w.id)
		}
		// Title: canonical, no "Learning: " prefix from the body heading.
		if got.Title != w.title {
			t.Errorf("%s: Title = %q, want %q", e.Name(), got.Title, w.title)
		}
		// Category: round-trips via canonical frontmatter.
		if got.Category != w.category {
			t.Errorf("%s: Category = %q, want %q", e.Name(), got.Category, w.category)
		}
		// Summary must carry the actual body text, not be empty.
		if got.Summary == "" {
			t.Errorf("%s: Summary is empty after round-trip", e.Name())
		}
	}

	if len(seen) != 2 {
		t.Fatalf("expected to round-trip 2 distinct canonical records, saw %d: %v", len(seen), seen)
	}
}
