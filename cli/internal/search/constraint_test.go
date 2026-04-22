package search

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
	prev, _ := os.Getwd()
	defer func() { _ = os.Chdir(prev) }()
	_ = os.Chdir(tmp)

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
	prev, _ := os.Getwd()
	defer func() { _ = os.Chdir(prev) }()
	_ = os.Chdir(tmp)

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
	prev, _ := os.Getwd()
	defer func() { _ = os.Chdir(prev) }()
	_ = os.Chdir(tmp)

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
		{ID: "a", Status: "active", CompiledAt: "2026-04-15T00:00:00Z"},   // stale (before cutoff)
		{ID: "b", Status: "active", CompiledAt: "2026-04-25T00:00:00Z"},   // fresh
		{ID: "c", Status: "retired", CompiledAt: "2026-04-01T00:00:00Z"},  // retired -> skipped
		{ID: "d", Status: "draft", CompiledAt: "bogus"},                   // unparseable
		{ID: "e", Status: "draft", CompiledAt: "2026-04-10"},              // date-only, stale
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
	prev, _ := os.Getwd()
	defer func() { _ = os.Chdir(prev) }()
	_ = os.Chdir(tmp)

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
	prev, _ := os.Getwd()
	defer func() { _ = os.Chdir(prev) }()
	_ = os.Chdir(tmp)

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
	prev, _ := os.Getwd()
	defer func() { _ = os.Chdir(prev) }()
	_ = os.Chdir(tmp)

	sentinel := errors.New("boom")
	err := WithConstraintLock(func() error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v", err)
	}
}
