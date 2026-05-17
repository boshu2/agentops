package wiki

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeCorpusFile writes content to .agents/<rel> under base, creating
// parent directories. It returns the absolute path of the written file.
func writeCorpusFile(t *testing.T, base, rel, content string) string {
	t.Helper()
	full := filepath.Join(base, ".agents", rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	abs, err := filepath.Abs(full)
	if err != nil {
		t.Fatalf("abs %s: %v", rel, err)
	}
	return abs
}

// recordSnapshot is a comparable copy of the identity-bearing fields of a
// persisted index record, used to assert that unchanged records were not
// rewritten.
type recordSnapshot struct {
	contentHash string
	size        int64
	modTimeUnix int64
}

// snapshotRecords reads the index and returns a path → recordSnapshot map.
func snapshotRecords(t *testing.T, idx *WikiIndex) map[string]recordSnapshot {
	t.Helper()
	recs, err := idx.Records()
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	out := make(map[string]recordSnapshot, len(recs))
	for _, r := range recs {
		out[r.Path] = recordSnapshot{
			contentHash: r.ContentHash,
			size:        r.Size,
			modTimeUnix: r.ModTimeUnix,
		}
	}
	return out
}

// TestWikiIndex_Incremental verifies the core acceptance criterion ac-wiki.4.1:
// given an indexed corpus and one changed file, a reindex rewrites ONLY the
// changed record. Every unchanged record's content hash, size, and mod time
// must be byte-identical to the prior snapshot, and the reindex result must
// name exactly the one changed path.
func TestWikiIndex_Incremental(t *testing.T) {
	base := t.TempDir()
	indexPath := filepath.Join(t.TempDir(), "wiki-index.jsonl")

	stablePath := writeCorpusFile(t, base, "learnings/stable.md", "# Stable\noriginal stable body\n")
	otherPath := writeCorpusFile(t, base, "playbooks/other.md", "# Other\noriginal other body\n")
	changedPath := writeCorpusFile(t, base, "concepts/changed.md", "# Changed\noriginal changed body\n")

	idx, err := NewWikiIndex(indexPath, base)
	if err != nil {
		t.Fatalf("NewWikiIndex: %v", err)
	}

	// First reindex: all three files are Added.
	first, err := idx.Reindex(context.Background())
	if err != nil {
		t.Fatalf("first Reindex: %v", err)
	}
	if got, want := len(first.Added), 3; got != want {
		t.Fatalf("first Reindex Added = %d (%v), want %d", got, first.Added, want)
	}
	if len(first.Updated) != 0 || len(first.Removed) != 0 {
		t.Fatalf("first Reindex Updated=%v Removed=%v, want both empty", first.Updated, first.Removed)
	}

	before := snapshotRecords(t, idx)
	if len(before) != 3 {
		t.Fatalf("after first Reindex: %d records, want 3", len(before))
	}

	// Mutate exactly one file. A different byte length guarantees a
	// distinct content hash and size.
	if err := os.WriteFile(changedPath, []byte("# Changed\nMUTATED body with new content\n"), 0o600); err != nil {
		t.Fatalf("mutate changed file: %v", err)
	}

	// Second reindex: only the changed file should be Updated.
	second, err := idx.Reindex(context.Background())
	if err != nil {
		t.Fatalf("second Reindex: %v", err)
	}
	if len(second.Added) != 0 || len(second.Removed) != 0 {
		t.Fatalf("second Reindex Added=%v Removed=%v, want both empty", second.Added, second.Removed)
	}
	if len(second.Updated) != 1 || second.Updated[0] != changedPath {
		t.Fatalf("second Reindex Updated = %v, want exactly [%s]", second.Updated, changedPath)
	}

	after := snapshotRecords(t, idx)
	if len(after) != 3 {
		t.Fatalf("after second Reindex: %d records, want 3", len(after))
	}

	// Unchanged records must be byte-identical to the prior snapshot.
	for _, p := range []string{stablePath, otherPath} {
		b, a := before[p], after[p]
		if b != a {
			t.Errorf("unchanged record %s was rewritten:\n  before %+v\n  after  %+v", p, b, a)
		}
	}

	// The changed record's content hash must differ from before.
	if before[changedPath].contentHash == after[changedPath].contentHash {
		t.Errorf("changed record %s kept stale content hash %s",
			changedPath, after[changedPath].contentHash)
	}
	if after[changedPath].size == before[changedPath].size {
		t.Errorf("changed record %s size unchanged (%d); mutation should alter it",
			changedPath, after[changedPath].size)
	}
}

// TestWikiIndex_Removal verifies that deleting a file drops its record on the
// next reindex and reports it in Removed, leaving surviving records untouched.
func TestWikiIndex_Removal(t *testing.T) {
	base := t.TempDir()
	indexPath := filepath.Join(t.TempDir(), "wiki-index.jsonl")

	keepPath := writeCorpusFile(t, base, "learnings/keep.md", "# Keep\n")
	dropPath := writeCorpusFile(t, base, "learnings/drop.md", "# Drop\n")

	idx, err := NewWikiIndex(indexPath, base)
	if err != nil {
		t.Fatalf("NewWikiIndex: %v", err)
	}
	if _, err := idx.Reindex(context.Background()); err != nil {
		t.Fatalf("first Reindex: %v", err)
	}
	before := snapshotRecords(t, idx)

	if err := os.Remove(dropPath); err != nil {
		t.Fatalf("remove drop file: %v", err)
	}

	res, err := idx.Reindex(context.Background())
	if err != nil {
		t.Fatalf("second Reindex: %v", err)
	}
	if len(res.Removed) != 1 || res.Removed[0] != dropPath {
		t.Fatalf("Removed = %v, want exactly [%s]", res.Removed, dropPath)
	}
	if len(res.Added) != 0 || len(res.Updated) != 0 {
		t.Fatalf("Added=%v Updated=%v, want both empty", res.Added, res.Updated)
	}

	after := snapshotRecords(t, idx)
	if _, present := after[dropPath]; present {
		t.Errorf("removed file %s still present in index", dropPath)
	}
	if before[keepPath] != after[keepPath] {
		t.Errorf("surviving record %s was rewritten on removal reindex", keepPath)
	}
}

// TestWikiIndex_CrossRepoRoots verifies that WikiIndex resolves and indexes
// the .agents/ corpus of every configured base, supporting cross-repo roots.
func TestWikiIndex_CrossRepoRoots(t *testing.T) {
	baseA := t.TempDir()
	baseB := t.TempDir()
	indexPath := filepath.Join(t.TempDir(), "wiki-index.jsonl")

	pathA := writeCorpusFile(t, baseA, "repo-a.md", "repo A content\n")
	pathB := writeCorpusFile(t, baseB, "repo-b.md", "repo B content\n")

	idx, err := NewWikiIndex(indexPath, baseA, baseB)
	if err != nil {
		t.Fatalf("NewWikiIndex: %v", err)
	}
	res, err := idx.Reindex(context.Background())
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	if len(res.Added) != 2 {
		t.Fatalf("Added = %v, want 2 paths across both roots", res.Added)
	}

	recs, err := idx.Records()
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	seen := make(map[string]bool, len(recs))
	for _, r := range recs {
		seen[r.Path] = true
	}
	if !seen[pathA] || !seen[pathB] {
		t.Errorf("cross-repo index missing a root: pathA=%v pathB=%v (records=%v)",
			seen[pathA], seen[pathB], recs)
	}
}

// TestWikiIndex_ConstructorValidation verifies NewWikiIndex rejects an empty
// index path and an empty base list.
func TestWikiIndex_ConstructorValidation(t *testing.T) {
	if _, err := NewWikiIndex(""); err == nil {
		t.Error("NewWikiIndex(\"\") = nil error, want failure for empty indexPath")
	}
	if _, err := NewWikiIndex("/tmp/idx.jsonl"); err == nil {
		t.Error("NewWikiIndex with no bases = nil error, want failure")
	}
}
