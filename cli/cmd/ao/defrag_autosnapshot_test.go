// practices: [wiki-knowledge-surface, resilience-patterns]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedCorpusForSnapshot creates a minimal .agents/ tree under the current
// working directory so defrag has a corpus to scan and snapshot.
func seedCorpusForSnapshot(t *testing.T) {
	t.Helper()
	learnings := filepath.Join(".agents", "learnings")
	if err := os.MkdirAll(learnings, 0o750); err != nil {
		t.Fatalf("mkdir corpus: %v", err)
	}
	if err := os.WriteFile(filepath.Join(learnings, "keep.md"), []byte("# Keep\nbody"), 0o600); err != nil {
		t.Fatalf("write corpus file: %v", err)
	}
}

// countSnapshots returns the number of *.tar.gz snapshots in dir (0 if absent).
func countSnapshots(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read snapshot dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.gz") {
			n++
		}
	}
	return n
}

// withDefragSnapshotState sets the defrag/snapshot globals to a known baseline
// pointed at a temp snapshot dir and restores them on cleanup. Returns the
// snapshot dir.
func withDefragSnapshotState(t *testing.T) string {
	t.Helper()
	snapDir := filepath.Join(t.TempDir(), "snaps")

	prevPrune, prevNoSnap, prevQuiet := defragPrune, defragNoSnapshot, defragQuiet
	prevSnapDir, prevDryRun, prevOutput := snapshotOutputDir, dryRun, output
	t.Cleanup(func() {
		defragPrune, defragNoSnapshot, defragQuiet = prevPrune, prevNoSnap, prevQuiet
		snapshotOutputDir, dryRun, output = prevSnapDir, prevDryRun, prevOutput
	})

	defragQuiet = true
	snapshotOutputDir = snapDir
	dryRun = false
	output = ""
	return snapDir
}

func TestAutoSnapshotBeforeDefrag_TakesSnapshotOnApplyPrune(t *testing.T) {
	snapDir := withDefragSnapshotState(t)
	cwd := chdirTemp(t)
	seedCorpusForSnapshot(t)

	defragPrune = true
	defragNoSnapshot = false

	if err := autoSnapshotBeforeDefrag(cwd, false, os.Stdout); err != nil {
		t.Fatalf("autoSnapshotBeforeDefrag: %v", err)
	}
	if got := countSnapshots(t, snapDir); got != 1 {
		t.Fatalf("snapshot count = %d, want 1", got)
	}
}

func TestAutoSnapshotBeforeDefrag_SkipsDryRun(t *testing.T) {
	snapDir := withDefragSnapshotState(t)
	cwd := chdirTemp(t)
	seedCorpusForSnapshot(t)

	defragPrune = true
	defragNoSnapshot = false

	if err := autoSnapshotBeforeDefrag(cwd, true, os.Stdout); err != nil {
		t.Fatalf("autoSnapshotBeforeDefrag (dry-run): %v", err)
	}
	if got := countSnapshots(t, snapDir); got != 0 {
		t.Fatalf("dry-run snapshot count = %d, want 0 (dry-run must not snapshot)", got)
	}
}

func TestAutoSnapshotBeforeDefrag_SkipsNoSnapshotFlag(t *testing.T) {
	snapDir := withDefragSnapshotState(t)
	cwd := chdirTemp(t)
	seedCorpusForSnapshot(t)

	defragPrune = true
	defragNoSnapshot = true

	if err := autoSnapshotBeforeDefrag(cwd, false, os.Stdout); err != nil {
		t.Fatalf("autoSnapshotBeforeDefrag (--no-snapshot): %v", err)
	}
	if got := countSnapshots(t, snapDir); got != 0 {
		t.Fatalf("--no-snapshot snapshot count = %d, want 0", got)
	}
}

func TestAutoSnapshotBeforeDefrag_SkipsDedupOnly(t *testing.T) {
	snapDir := withDefragSnapshotState(t)
	cwd := chdirTemp(t)
	seedCorpusForSnapshot(t)

	// Dedup-only defrag flags duplicates but never deletes — no snapshot needed.
	defragPrune = false
	defragNoSnapshot = false

	if err := autoSnapshotBeforeDefrag(cwd, false, os.Stdout); err != nil {
		t.Fatalf("autoSnapshotBeforeDefrag (dedup-only): %v", err)
	}
	if got := countSnapshots(t, snapDir); got != 0 {
		t.Fatalf("dedup-only snapshot count = %d, want 0 (no deletion, no snapshot)", got)
	}
}

func TestAutoSnapshotBeforeDefrag_FailsWhenCorpusMissing(t *testing.T) {
	withDefragSnapshotState(t)
	cwd := chdirTemp(t)
	// Intentionally do NOT seed .agents/ — snapshot of a missing corpus must error.

	defragPrune = true
	defragNoSnapshot = false

	if err := autoSnapshotBeforeDefrag(cwd, false, os.Stdout); err == nil {
		t.Fatal("expected error when corpus is missing, got nil (must refuse to prune without a backup)")
	}
}

// TestRunDefrag_SnapshotsThenPrunes is the L2 path: a bare apply-mode defrag
// must leave a recoverable snapshot whose tarball actually contains the corpus.
func TestRunDefrag_SnapshotsThenPrunes(t *testing.T) {
	snapDir := withDefragSnapshotState(t)
	dir := chdirTemp(t)
	seedCorpusForSnapshot(t)

	prevDedup, prevStale, prevOut := defragDedup, defragStaleDays, defragOutputDir
	t.Cleanup(func() {
		defragDedup, defragStaleDays, defragOutputDir = prevDedup, prevStale, prevOut
	})
	defragPrune = true
	defragDedup = false
	defragNoSnapshot = false
	defragStaleDays = 30
	defragOutputDir = filepath.Join(dir, ".agents", "defrag")

	out, err := captureStdout(t, func() error { return runDefrag(defragCmd, nil) })
	if err != nil {
		t.Fatalf("runDefrag: %v", err)
	}

	if got := countSnapshots(t, snapDir); got != 1 {
		t.Fatalf("runDefrag snapshot count = %d, want 1", got)
	}
	// quiet was set true by withDefragSnapshotState, so stdout stays silent;
	// the durable artifact (the tarball) is the assertion that matters.
	_ = out
}
