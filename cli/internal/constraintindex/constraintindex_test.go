package constraintindex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConstraintIndexPath(t *testing.T) {
	if got := ConstraintIndexPath(); got != filepath.Join(".agents", "constraints", "index.json") {
		t.Errorf("got %q", got)
	}
}

func TestConstraintLockPath(t *testing.T) {
	if got := ConstraintLockPath(); got != filepath.Join(".agents", "constraints", "compile.lock") {
		t.Errorf("got %q", got)
	}
}

func TestLoadConstraintIndex_Missing(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	_, err := LoadConstraintIndex()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no constraints") {
		t.Errorf("err = %v", err)
	}
}

func TestLoadConstraintIndex_Valid(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	_ = os.MkdirAll(filepath.Join(".agents", "constraints"), 0o755)
	idx := ConstraintIndex{SchemaVersion: 1, Constraints: []ConstraintEntry{{ID: "c1", Title: "t"}}}
	data, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(ConstraintIndexPath(), data, 0o600)

	got, err := LoadConstraintIndex()
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || len(got.Constraints) != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestLoadConstraintIndex_Malformed(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	_ = os.MkdirAll(filepath.Join(".agents", "constraints"), 0o755)
	_ = os.WriteFile(ConstraintIndexPath(), []byte("not json"), 0o600)

	_, err := LoadConstraintIndex()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFindConstraint(t *testing.T) {
	idx := &ConstraintIndex{Constraints: []ConstraintEntry{
		{ID: "c1", Title: "A"},
		{ID: "c2", Title: "B"},
	}}
	if got := FindConstraint(idx, "c2"); got == nil || got.Title != "B" {
		t.Errorf("got %+v", got)
	}
	if got := FindConstraint(idx, "missing"); got != nil {
		t.Errorf("should return nil, got %+v", got)
	}
}

func TestFilterStaleConstraints(t *testing.T) {
	cutoff := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	entries := []ConstraintEntry{
		{ID: "a", Status: "active", CompiledAt: "2026-04-15T00:00:00Z"},  // stale (before cutoff)
		{ID: "b", Status: "active", CompiledAt: "2026-04-25T00:00:00Z"},  // fresh
		{ID: "c", Status: "retired", CompiledAt: "2026-04-01T00:00:00Z"}, // retired -> skipped
		{ID: "d", Status: "draft", CompiledAt: "bogus"},                  // unparseable
		{ID: "e", Status: "draft", CompiledAt: "2026-04-10"},             // date-only, stale
	}
	stale := FilterStaleConstraints(entries, cutoff)
	ids := map[string]bool{}
	for _, s := range stale {
		ids[s.ID] = true
	}
	if !ids["a"] {
		t.Error("a should be stale")
	}
	if ids["b"] {
		t.Error("b should not be stale")
	}
	if ids["c"] {
		t.Error("retired c should be skipped")
	}
	if ids["d"] {
		t.Error("unparseable d should be skipped")
	}
	if !ids["e"] {
		t.Error("date-only e should be stale")
	}
}

func TestSaveConstraintIndexUnlocked(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	idx := &ConstraintIndex{SchemaVersion: 1, Constraints: []ConstraintEntry{{ID: "x", Title: "Saved"}}}
	if err := SaveConstraintIndexUnlocked(idx); err != nil {
		t.Fatal(err)
	}
	// Re-read round-trip
	loaded, err := LoadConstraintIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Constraints) != 1 || loaded.Constraints[0].Title != "Saved" {
		t.Errorf("round-trip failed: %+v", loaded)
	}
}

func TestWithConstraintLock_RunsOnce(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	called := 0
	err := WithConstraintLock(func() error {
		called++
		// Lock file should exist during fn
		if _, statErr := os.Stat(ConstraintLockPath()); statErr != nil {
			t.Errorf("lock file missing during fn")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Errorf("called %d times", called)
	}
	// Lock file removed after fn
	if _, err := os.Stat(ConstraintLockPath()); err == nil {
		t.Error("lock file should be removed")
	}
}

func TestWithConstraintLock_PropagatesError(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	sentinel := errors.New("boom")
	err := WithConstraintLock(func() error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v", err)
	}
}

func TestUpsertConstraintAt_InsertIdempotentForce(t *testing.T) {
	root := t.TempDir()
	idxPath := filepath.Join(root, ConstraintIndexPath())
	e := ConstraintEntry{
		ID: "f-1", Title: "t", Status: "draft", CompiledAt: "2026-06-21T00:00:00Z",
		AppliesTo: ConstraintAppliesTo{PathGlobs: []string{"cli/**"}},
		Detector:  ConstraintDetector{Kind: "regex", Pattern: "panic"},
	}
	// insert into a non-existent index (load-or-init)
	wrote, err := UpsertConstraintAt(root, e, false)
	if err != nil || !wrote {
		t.Fatalf("insert: wrote=%v err=%v", wrote, err)
	}
	idx, err := loadConstraintIndexAtPath(idxPath)
	if err != nil || len(idx.Constraints) != 1 || idx.Constraints[0].Status != "draft" {
		t.Fatalf("after insert: %+v err=%v", idx, err)
	}
	// idempotent: same id, force=false -> no write, original kept
	e2 := e
	e2.Title = "changed"
	if wrote, err = UpsertConstraintAt(root, e2, false); err != nil || wrote {
		t.Fatalf("idempotent: wrote=%v err=%v (want no write)", wrote, err)
	}
	idx, _ = loadConstraintIndexAtPath(idxPath)
	if idx.Constraints[0].Title != "t" {
		t.Fatalf("idempotent upsert overwrote title: %q", idx.Constraints[0].Title)
	}
	// force=true -> replaces in place, still one entry
	if wrote, err = UpsertConstraintAt(root, e2, true); err != nil || !wrote {
		t.Fatalf("force: wrote=%v err=%v", wrote, err)
	}
	idx, _ = loadConstraintIndexAtPath(idxPath)
	if len(idx.Constraints) != 1 || idx.Constraints[0].Title != "changed" {
		t.Fatalf("force didn't replace in place: %+v", idx.Constraints)
	}
}

// EM.2.9: SanitizeForPublish strips every PRIVATE field (finding id + the
// .agents/-pointing artifact/review/file paths) so the tracked published surface
// never leaks private findings/evidence, while preserving the enforceable detector.
func TestSanitizeForPublish_StripsPrivateNoLeak(t *testing.T) {
	in := ConstraintEntry{
		ID: "f-x", Title: "Escape: age-demo confirmed then refuted", Status: "active",
		Source: "finding", SourceType: "escape",
		FindingID:      "f-x",
		SourceArtifact: ".agents/findings/f-x.md",
		ReviewFile:     ".agents/constraints/review/f-x.md",
		File:           ".agents/constraints/f-x.json",
		AppliesTo:      ConstraintAppliesTo{PathGlobs: []string{"cli/**"}},
		Detector:       ConstraintDetector{Kind: "regex", Pattern: `eval\(`, Message: "no eval"},
	}
	out := SanitizeForPublish(in)
	if out.FindingID != "" || out.SourceArtifact != "" || out.ReviewFile != "" || out.File != "" {
		t.Fatalf("private fields not stripped: %+v", out)
	}
	data, _ := json.Marshal(out)
	if strings.Contains(string(data), ".agents") {
		t.Fatalf("published entry leaks a .agents path: %s", data)
	}
	// enforceable surface preserved
	if out.Detector.Pattern != `eval\(` || len(out.AppliesTo.PathGlobs) != 1 || out.Status != "active" {
		t.Fatalf("enforceable detector surface not preserved: %+v", out)
	}
}

// EM.2.9: the allowlist drops EVERY non-enforceable field (not just the known
// private ones), and PublishedLeaks catches a residual .agents/ path that rode
// along in a KEPT field (Title/Message/glob) — defense in depth.
func TestSanitizeForPublish_AllowlistAndLeakGuard(t *testing.T) {
	// A path snuck into a kept field (the detector message) survives the allowlist...
	leaky := ConstraintEntry{
		ID: "f-leak", Title: "t", Status: "active",
		AppliesTo: ConstraintAppliesTo{PathGlobs: []string{"cli/**"}},
		Detector:  ConstraintDetector{Kind: "regex", Pattern: `x`, Message: "see .agents/findings/secret.md"},
		// private structural fields the allowlist must drop:
		FindingID: "f-leak", SourceArtifact: ".agents/findings/f-leak.md", File: ".agents/constraints/f-leak.json",
	}
	out := SanitizeForPublish(leaky)
	if out.FindingID != "" || out.SourceArtifact != "" || out.File != "" {
		t.Fatalf("allowlist must drop structural private fields: %+v", out)
	}
	// ...but PublishedLeaks catches the residual path so publish can refuse.
	idx := &ConstraintIndex{SchemaVersion: 1, Constraints: []ConstraintEntry{out}}
	if leaks := PublishedLeaks(idx); len(leaks) != 1 || leaks[0] != "f-leak" {
		t.Fatalf("PublishedLeaks must catch the residual .agents path, got %v", leaks)
	}
	// separator variants are all caught (the marker is the bare ".agents" segment).
	for _, p := range []string{".agents/findings/x.md", ".agents\\findings\\x.md", "see .agents (bare)"} {
		ve := SanitizeForPublish(ConstraintEntry{ID: "f-v", Title: "t", Status: "active",
			AppliesTo: ConstraintAppliesTo{PathGlobs: []string{"cli/**"}},
			Detector:  ConstraintDetector{Kind: "regex", Pattern: "x", Message: p}})
		if l := PublishedLeaks(&ConstraintIndex{Constraints: []ConstraintEntry{ve}}); len(l) != 1 {
			t.Fatalf("separator variant %q must be caught, got %v", p, l)
		}
	}
	// A clean entry has no leaks.
	clean := SanitizeForPublish(ConstraintEntry{
		ID: "f-ok", Title: "Escape: age-x", Status: "active",
		AppliesTo: ConstraintAppliesTo{PathGlobs: []string{"cli/**"}},
		Detector:  ConstraintDetector{Kind: "regex", Pattern: `eval\(`, Message: "no eval"},
	})
	if leaks := PublishedLeaks(&ConstraintIndex{Constraints: []ConstraintEntry{clean}}); len(leaks) != 0 {
		t.Fatalf("a clean entry must have no leaks, got %v", leaks)
	}
}
