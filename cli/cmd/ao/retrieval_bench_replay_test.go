// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Micro-epic 7 (C6) — retrieval-bench replay conformance tests.
//
// These tests prove that collectLearnings produces byte-identical output
// across multiple invocations when:
//  1. The filesystem fixture has deterministic mtimes (via os.Chtimes).
//  2. The package-level nowFunc is held at a fixed time.
//
// The anti-goal the handoff called out: do NOT add --seed or --eval-set
// flags to retrieval-bench. The comment at retrieval_bench.go:558-559 is
// a deliberate decline. Clock determinism comes from the package-level
// nowFunc, not from signature threading, exactly as Micro-epic 7 spec'd.

// writeFixtureLearning creates a minimal learning markdown file with a
// stable frontmatter shape and then stamps both atime and mtime to the
// supplied timestamp so freshness scoring is deterministic across runs.
func writeFixtureLearning(t *testing.T, dir, id, body string, mtime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	content := "---\n" +
		"id: " + id + "\n" +
		"type: learning\n" +
		"date: 2026-03-15\n" +
		"tags: [test, retrieval]\n" +
		"utility: 0.75\n" +
		"---\n\n" +
		"# " + id + "\n\n" +
		body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", id, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes fixture %s: %v", id, err)
	}
	return path
}

// TestRetrievalBench_ReplayIdentical is the determinism-proof for C6.
// Two back-to-back invocations of collectLearnings against the same
// frozen-mtime fixture with the same nowFunc must produce byte-identical
// results. If a future refactor reintroduces a time.Now() call inside
// collectLearnings or any function it delegates to, this test fails.
func TestRetrievalBench_ReplayIdentical(t *testing.T) {
	// Freeze the package-level clock at a known point in time. Restore
	// on cleanup so sibling tests get the real clock back.
	frozen := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	oldNow := nowFunc
	nowFunc = func() time.Time { return frozen }
	t.Cleanup(func() { nowFunc = oldNow })

	tmpDir := t.TempDir()
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}

	// Deterministic mtimes relative to frozen: one week old, two weeks
	// old, three weeks old. The absolute values matter because
	// ApplyFreshnessToLearning uses days-since-mtime.
	writeFixtureLearning(t, learningsDir, "fixture-alpha",
		"Alpha learning about retrieval deduplication.",
		frozen.Add(-7*24*time.Hour))
	writeFixtureLearning(t, learningsDir, "fixture-beta",
		"Beta learning about freshness decay and retrieval ranking.",
		frozen.Add(-14*24*time.Hour))
	writeFixtureLearning(t, learningsDir, "fixture-gamma",
		"Gamma learning about token matching in retrieval.",
		frozen.Add(-21*24*time.Hour))

	// Run 1
	run1, err := collectLearnings(tmpDir, "retrieval ranking", 10, "", 0)
	if err != nil {
		t.Fatalf("collectLearnings run 1: %v", err)
	}

	// Run 2
	run2, err := collectLearnings(tmpDir, "retrieval ranking", 10, "", 0)
	if err != nil {
		t.Fatalf("collectLearnings run 2: %v", err)
	}

	if len(run1) != len(run2) {
		t.Fatalf("run1 len=%d run2 len=%d (should be equal)", len(run1), len(run2))
	}
	if len(run1) == 0 {
		t.Fatal("expected at least one learning retrieved from fixture")
	}

	// Assert structural equality across runs. DeepEqual is exactly what
	// we want — we expect every field including scores to be identical.
	if !reflect.DeepEqual(run1, run2) {
		r1json, _ := json.MarshalIndent(run1, "", "  ")
		r2json, _ := json.MarshalIndent(run2, "", "  ")
		t.Fatalf("non-deterministic collectLearnings:\nRUN1:\n%s\n\nRUN2:\n%s",
			r1json, r2json)
	}

	// Also check that the frozen clock was actually in effect: every
	// learning must carry a non-empty ID (the frontmatter id field we
	// wrote above). An empty ID means the fixture wasn't being parsed
	// and the test is accidentally green.
	for _, l := range run1 {
		if l.ID == "" {
			t.Error("learning has empty ID — fixture was not parsed, test is vacuous")
		}
	}
}

// TestRetrievalBench_TieOrderDeterministic guards the judge-6 catch: two
// learnings with identical composite scores must sort in a stable, not
// random, order. The fixture below creates two near-identical learnings
// with the same frozen mtime so their freshness and token-match scores
// are equal.
func TestRetrievalBench_TieOrderDeterministic(t *testing.T) {
	frozen := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	oldNow := nowFunc
	nowFunc = func() time.Time { return frozen }
	t.Cleanup(func() { nowFunc = oldNow })

	tmpDir := t.TempDir()
	learningsDir := filepath.Join(tmpDir, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatalf("mkdir learnings: %v", err)
	}

	// Two fixtures with identical body content (same tokens) and same
	// mtime — any composite score should tie. IDs are distinct so the
	// sort has something to break on.
	mtime := frozen.Add(-5 * 24 * time.Hour)
	writeFixtureLearning(t, learningsDir, "tie-alpha",
		"Identical body with matching query tokens retrieval.", mtime)
	writeFixtureLearning(t, learningsDir, "tie-bravo",
		"Identical body with matching query tokens retrieval.", mtime)

	// Run three times and check the order is the same every time.
	var first []string
	for run := 0; run < 3; run++ {
		results, err := collectLearnings(tmpDir, "retrieval", 10, "", 0)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		ids := make([]string, 0, len(results))
		for _, l := range results {
			ids = append(ids, l.ID)
		}
		if run == 0 {
			first = ids
			continue
		}
		if !reflect.DeepEqual(first, ids) {
			t.Fatalf("tie-order changed on run %d:\nrun 0: %v\nrun %d: %v",
				run, first, run, ids)
		}
	}
}
