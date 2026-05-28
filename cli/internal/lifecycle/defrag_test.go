package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsHashNamed(t *testing.T) {
	cases := map[string]bool{
		"2026-02-23-4556c2b4.md":         true,
		"2026-02-23-ABCDEF12.md":         false, // uppercase hex not accepted
		"2026-02-23-name.md":             false,
		"just-four-part-x.md":            false,
		"2026-02-23-4556c2b4aabbccdd.md": false, // too long last part
		"a-b-c.md":                       false, // only 3 parts
	}
	for name, want := range cases {
		if got := IsHashNamed(name); got != want {
			t.Errorf("IsHashNamed(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestBuildTrigrams(t *testing.T) {
	tg := BuildTrigrams("abcd")
	if len(tg) != 2 {
		t.Errorf("expected 2 trigrams, got %d: %v", len(tg), tg)
	}
	if !tg["abc"] || !tg["bcd"] {
		t.Errorf("missing expected trigrams: %v", tg)
	}

	empty := BuildTrigrams("")
	if len(empty) != 0 {
		t.Errorf("empty input should yield 0 trigrams")
	}
	short := BuildTrigrams("ab")
	if len(short) != 0 {
		t.Errorf("input <3 chars should yield 0 trigrams")
	}
}

func TestTrigramOverlap(t *testing.T) {
	a := BuildTrigrams("abcdef")
	b := BuildTrigrams("abcdef")
	if got := TrigramOverlap(a, b); got != 1.0 {
		t.Errorf("identical overlap = %v, want 1.0", got)
	}

	c := BuildTrigrams("xyz123")
	if got := TrigramOverlap(a, c); got != 0 {
		t.Errorf("disjoint overlap = %v, want 0", got)
	}

	// Empty maps
	empty := map[string]bool{}
	if got := TrigramOverlap(empty, empty); got != 0 {
		t.Errorf("empty×empty = %v, want 0", got)
	}
}

func TestFindOrphanLearnings_NoLearnings(t *testing.T) {
	tmp := t.TempDir()
	r, err := FindOrphanLearnings(tmp, 30)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.TotalLearnings != 0 {
		t.Errorf("TotalLearnings = %d", r.TotalLearnings)
	}
}

func TestFindOrphanLearnings_DetectsOrphan(t *testing.T) {
	tmp := t.TempDir()
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	patDir := filepath.Join(tmp, ".agents", "patterns")
	_ = os.MkdirAll(learnDir, 0o755)
	_ = os.MkdirAll(patDir, 0o755)

	orphanName := "orphan-learning.md"
	orphanPath := filepath.Join(learnDir, orphanName)
	if err := os.WriteFile(orphanPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make it stale (older than cutoff)
	old := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(orphanPath, old, old)

	referencedName := "linked.md"
	refPath := filepath.Join(learnDir, referencedName)
	_ = os.WriteFile(refPath, []byte("x"), 0o600)
	_ = os.Chtimes(refPath, old, old)

	// Pattern references the linked file
	_ = os.WriteFile(filepath.Join(patDir, "p1.md"), []byte("see linked.md"), 0o600)

	r, err := FindOrphanLearnings(tmp, 30)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.TotalLearnings != 2 {
		t.Errorf("TotalLearnings = %d, want 2", r.TotalLearnings)
	}
	if r.StaleCount != 2 {
		t.Errorf("StaleCount = %d, want 2", r.StaleCount)
	}
	// Only orphan is an orphan (linked is referenced)
	if len(r.Orphans) != 1 {
		t.Errorf("Orphans = %v, want len 1", r.Orphans)
	}
	if len(r.Orphans) > 0 && !strings.HasSuffix(r.Orphans[0], orphanName) {
		t.Errorf("orphan should be %q, got %v", orphanName, r.Orphans)
	}
}

func TestExecutePrune_DryRunKeepsFiles(t *testing.T) {
	tmp := t.TempDir()
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	_ = os.MkdirAll(learnDir, 0o755)
	orphan := filepath.Join(learnDir, "orphan.md")
	_ = os.WriteFile(orphan, []byte("x"), 0o600)
	old := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(orphan, old, old)

	r, err := ExecutePrune(tmp, true, 30)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.Orphans) != 1 {
		t.Errorf("should detect 1 orphan, got %d", len(r.Orphans))
	}
	if len(r.Deleted) != 0 {
		t.Errorf("dry-run should not delete; got %v", r.Deleted)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Errorf("file should still exist after dry-run")
	}
}

func TestExecutePrune_ActuallyDeletes(t *testing.T) {
	tmp := t.TempDir()
	learnDir := filepath.Join(tmp, ".agents", "learnings")
	_ = os.MkdirAll(learnDir, 0o755)
	orphan := filepath.Join(learnDir, "orphan.md")
	_ = os.WriteFile(orphan, []byte("x"), 0o600)
	old := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(orphan, old, old)

	r, err := ExecutePrune(tmp, false, 30)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.Deleted) != 1 {
		t.Errorf("expected 1 deletion, got %v", r.Deleted)
	}
	if _, err := os.Stat(orphan); err == nil {
		t.Errorf("orphan should be deleted")
	}
}

func TestFindDuplicateLearnings_NoDir(t *testing.T) {
	tmp := t.TempDir()
	r, err := FindDuplicateLearnings(tmp)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.Checked != 0 {
		t.Errorf("Checked = %d", r.Checked)
	}
}

func TestFindDuplicateLearnings_DetectsNearDup(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".agents", "learnings")
	_ = os.MkdirAll(dir, 0o755)

	body := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 10)
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte(body), 0o600)
	// Near-duplicate: same content -> identical trigram sets (overlap 1.0)
	_ = os.WriteFile(filepath.Join(dir, "b.md"), []byte(body), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "c.md"), []byte("completely unrelated content zyxwvu qponml"), 0o600)

	r, err := FindDuplicateLearnings(tmp)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.Checked != 3 {
		t.Errorf("Checked = %d, want 3", r.Checked)
	}
	if len(r.DuplicatePairs) == 0 {
		t.Fatalf("expected at least 1 near-duplicate pair")
	}
}
