// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// constraintIndex / constraintEntry struct tests
// ---------------------------------------------------------------------------

func TestConstraintEntry_JSONRoundTrip(t *testing.T) {
	entry := constraintEntry{
		ID:         "test-constraint-1",
		Title:      "Avoid eval in templates",
		Source:     "learnings/mutex-pattern.md",
		Status:     "active",
		CompiledAt: time.Now().Format(time.RFC3339),
		File:       "constraints/test-constraint-1.md",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed constraintEntry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.ID != entry.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, entry.ID)
	}
	if parsed.Title != entry.Title {
		t.Errorf("Title = %q, want %q", parsed.Title, entry.Title)
	}
	if parsed.Status != entry.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, entry.Status)
	}
}

func TestConstraintIndex_JSONRoundTrip(t *testing.T) {
	idx := constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "c-1", Title: "First", Status: "draft", CompiledAt: "2026-01-01"},
			{ID: "c-2", Title: "Second", Status: "active", CompiledAt: "2025-12-01"},
		},
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed constraintIndex
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", parsed.SchemaVersion)
	}
	if len(parsed.Constraints) != 2 {
		t.Errorf("len(Constraints) = %d, want 2", len(parsed.Constraints))
	}
}

// ---------------------------------------------------------------------------
// findConstraint
// ---------------------------------------------------------------------------

func TestFindConstraint_Found(t *testing.T) {
	idx := &constraintIndex{
		Constraints: []constraintEntry{
			{ID: "c-1", Title: "First"},
			{ID: "c-2", Title: "Second"},
			{ID: "c-3", Title: "Third"},
		},
	}

	got := findConstraint(idx, "c-2")
	if got == nil {
		t.Fatal("expected to find constraint c-2")
	}
	if got.Title != "Second" {
		t.Errorf("found constraint title = %q, want Second", got.Title)
	}
}

func TestFindConstraint_NotFound(t *testing.T) {
	idx := &constraintIndex{
		Constraints: []constraintEntry{
			{ID: "c-1"},
		},
	}

	got := findConstraint(idx, "nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent ID, got %v", got)
	}
}

func TestFindConstraint_EmptyIndex(t *testing.T) {
	idx := &constraintIndex{}
	got := findConstraint(idx, "any-id")
	if got != nil {
		t.Errorf("expected nil for empty index, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// findConstraint returns a mutable pointer
// ---------------------------------------------------------------------------

func TestFindConstraint_MutablePointer(t *testing.T) {
	idx := &constraintIndex{
		Constraints: []constraintEntry{
			{ID: "c-1", Status: "draft"},
		},
	}

	got := findConstraint(idx, "c-1")
	if got == nil {
		t.Fatal("expected to find constraint")
	}

	// Modifying through the pointer should update the original
	got.Status = "active"
	if idx.Constraints[0].Status != "active" {
		t.Error("expected findConstraint to return a mutable pointer to the original slice element")
	}
}

// ---------------------------------------------------------------------------
// saveConstraintIndex / loadConstraintIndex edge cases
// ---------------------------------------------------------------------------

func TestSaveConstraintIndex_NewlineTerminated(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{SchemaVersion: 1}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	data, err := os.ReadFile(constraintIndexPath())
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.HasSuffix(string(data), "\n") {
		t.Error("expected index file to be newline-terminated")
	}
}

func TestLoadConstraintIndex_CorruptJSON(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	// Write invalid JSON
	if err := os.WriteFile(constraintIndexPath(), []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := loadConstraintIndex()
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("expected parsing error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// activate edge cases
// ---------------------------------------------------------------------------

func TestConstraintActivate_NotFound(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints:   []constraintEntry{{ID: "c-1", Status: "draft"}},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	err := constraintActivateCmd.RunE(constraintActivateCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent constraint")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestConstraintActivate_AlreadyRetired(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints:   []constraintEntry{{ID: "c-1", Status: "retired", CompiledAt: "2026-01-01"}},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	err := constraintActivateCmd.RunE(constraintActivateCmd, []string{"c-1"})
	if err == nil {
		t.Fatal("expected error for retired constraint")
	}
	if !strings.Contains(err.Error(), "retired") {
		t.Errorf("expected status error mentioning 'retired', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// retire edge cases
// ---------------------------------------------------------------------------

func TestConstraintRetire_NotFound(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints:   []constraintEntry{{ID: "c-1", Status: "active"}},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	err := constraintRetireCmd.RunE(constraintRetireCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent constraint")
	}
}

func TestConstraintRetire_DraftCannotRetire(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints:   []constraintEntry{{ID: "c-1", Status: "draft", CompiledAt: "2026-01-01"}},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	err := constraintRetireCmd.RunE(constraintRetireCmd, []string{"c-1"})
	if err == nil {
		t.Fatal("expected error for draft constraint")
	}
	if !strings.Contains(err.Error(), "draft") {
		t.Errorf("expected error mentioning 'draft', got: %v", err)
	}
}

func TestConstraintRetire_JSONOutput(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints:   []constraintEntry{{ID: "c-1", Status: "active", CompiledAt: time.Now().Format(time.RFC3339)}},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintRetireCmd.RunE(constraintRetireCmd, []string{"c-1"})
	})
	if err != nil {
		t.Fatalf("retire: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	if got["status"] != "retired" {
		t.Errorf("expected status=retired, got %v", got["status"])
	}
}

// ---------------------------------------------------------------------------
// review edge cases
// ---------------------------------------------------------------------------

func TestConstraintReview_SkipsRetired(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "old-active", Status: "active", CompiledAt: time.Now().AddDate(0, 0, -100).Format(time.RFC3339)},
			{ID: "old-retired", Status: "retired", CompiledAt: time.Now().AddDate(0, 0, -100).Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	var stale []constraintEntry
	if err := json.Unmarshal([]byte(stdout), &stale); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Only old-active should appear (old-retired is skipped)
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale constraint, got %d", len(stale))
	}
	if stale[0].ID != "old-active" {
		t.Errorf("expected old-active, got %q", stale[0].ID)
	}
}

func TestConstraintReview_DateOnlyFormat(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	// Use date-only format instead of RFC3339
	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "date-only", Status: "draft", CompiledAt: "2025-01-01"},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	var stale []constraintEntry
	if err := json.Unmarshal([]byte(stdout), &stale); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(stale) != 1 {
		t.Fatalf("expected 1 stale constraint with date-only format, got %d", len(stale))
	}
}

func TestConstraintReview_InvalidDate(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	// Constraint with unparseable date should be skipped
	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "bad-date", Status: "draft", CompiledAt: "not-a-date"},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	var stale []constraintEntry
	if err := json.Unmarshal([]byte(stdout), &stale); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(stale) != 0 {
		t.Errorf("expected 0 stale constraints for unparseable date, got %d", len(stale))
	}
}

func TestConstraintReview_TableOutput(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "stale-1", Title: "Old constraint", Status: "active", CompiledAt: time.Now().AddDate(0, 0, -100).Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	if !strings.Contains(stdout, "stale-1") {
		t.Errorf("expected stale-1 in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "1 constraint(s) need review") {
		t.Errorf("expected review count, got: %q", stdout)
	}
}

// ---------------------------------------------------------------------------
// list edge cases
// ---------------------------------------------------------------------------

func TestConstraintList_TableOutput(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "c-1", Title: "First", Status: "draft", CompiledAt: "2026-01-01"},
			{ID: "c-2", Title: "Second", Status: "active", CompiledAt: "2026-02-01"},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintListCmd.RunE(constraintListCmd, nil)
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if !strings.Contains(stdout, "c-1") {
		t.Errorf("expected c-1 in list, got: %q", stdout)
	}
	if !strings.Contains(stdout, "c-2") {
		t.Errorf("expected c-2 in list, got: %q", stdout)
	}
	if !strings.Contains(stdout, "2 constraint(s) total") {
		t.Errorf("expected total count, got: %q", stdout)
	}
}

func TestConstraintList_TruncatesLongValues(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	longID := strings.Repeat("x", 35)
	longTitle := strings.Repeat("y", 55)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: longID, Title: longTitle, Status: "active", CompiledAt: "2026-01-01T00:00:00+00:00Z-extra-chars"},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintListCmd.RunE(constraintListCmd, nil)
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// Long values should be truncated with "..."
	if !strings.Contains(stdout, "...") {
		t.Errorf("expected truncation with '...' in output, got: %q", stdout)
	}
}

func TestConstraintList_MissingIndex(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)

	// No constraints directory
	err := constraintListCmd.RunE(constraintListCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing index")
	}
	if !strings.Contains(err.Error(), "no constraints found") {
		t.Errorf("expected 'no constraints found' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// constraintIndexPath
// ---------------------------------------------------------------------------

func TestConstraintIndexPath_IsStable(t *testing.T) {
	want := filepath.Join(".agents", "constraints", "index.json")
	got := constraintIndexPath()
	if got != want {
		t.Errorf("constraintIndexPath() = %q, want %q", got, want)
	}

	// Call twice to verify it's deterministic
	if got2 := constraintIndexPath(); got2 != got {
		t.Errorf("constraintIndexPath() not deterministic: %q != %q", got, got2)
	}
}

// ---------------------------------------------------------------------------
// Full lifecycle: draft -> activate -> retire
// ---------------------------------------------------------------------------

func TestConstraintLifecycle_DraftActivateRetire(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "lifecycle", Title: "Lifecycle test", Status: "draft", CompiledAt: time.Now().Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Step 1: Activate
	_, err := captureStdout(t, func() error {
		return constraintActivateCmd.RunE(constraintActivateCmd, []string{"lifecycle"})
	})
	if err != nil {
		t.Fatalf("activate: %v", err)
	}

	reloaded, err := loadConstraintIndex()
	if err != nil {
		t.Fatalf("load after activate: %v", err)
	}
	if got := findConstraint(reloaded, "lifecycle").Status; got != "active" {
		t.Fatalf("after activate: status = %q, want active", got)
	}

	// Step 2: Retire
	_, err = captureStdout(t, func() error {
		return constraintRetireCmd.RunE(constraintRetireCmd, []string{"lifecycle"})
	})
	if err != nil {
		t.Fatalf("retire: %v", err)
	}

	reloaded, err = loadConstraintIndex()
	if err != nil {
		t.Fatalf("load after retire: %v", err)
	}
	if got := findConstraint(reloaded, "lifecycle").Status; got != "retired" {
		t.Fatalf("after retire: status = %q, want retired", got)
	}
}
