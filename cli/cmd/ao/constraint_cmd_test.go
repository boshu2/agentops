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

func TestConstraintIndexPath(t *testing.T) {
	want := filepath.Join(".agents", "constraints", "index.json")
	if got := constraintIndexPath(); got != want {
		t.Fatalf("constraintIndexPath() = %q, want %q", got, want)
	}
}

func TestConstraintLoadSaveFindRoundTrip(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:             "c-1",
				FindingID:      "f-1",
				Title:          "No eval",
				Status:         "draft",
				CompiledAt:     "2026-01-01",
				SourceArtifact: ".agents/findings/f-1.md",
				ReviewFile:     ".agents/constraints/c-1.sh",
				AppliesTo: constraintAppliesTo{
					Scope:      "files",
					IssueTypes: []string{"feature"},
					PathGlobs:  []string{"cli/cmd/ao/*.go"},
				},
				Detector: constraintDetector{
					Kind:    "content_pattern",
					Mode:    "must_contain",
					Pattern: "issue_type",
				},
				File: "notes.md",
			},
		},
	}

	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}
	info, err := os.Stat(constraintIndexPath())
	if err != nil {
		t.Fatalf("stat index: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("index mode = %o, want 600", got)
	}

	got, err := loadConstraintIndex()
	if err != nil {
		t.Fatalf("loadConstraintIndex: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", got.SchemaVersion)
	}
	if gotEntry := findConstraint(got, "c-1"); gotEntry == nil || gotEntry.ID != "c-1" {
		t.Fatalf("findConstraint() = %v, want entry c-1", gotEntry)
	}
	if gotEntry := findConstraint(got, "c-1"); gotEntry == nil || gotEntry.FindingID != "f-1" {
		t.Fatalf("FindingID = %v, want f-1", gotEntry)
	}
	if gotEntry := findConstraint(got, "c-1"); gotEntry == nil || gotEntry.Detector.Kind != "content_pattern" {
		t.Fatalf("Detector.Kind = %v, want content_pattern", gotEntry)
	}
}

func TestSaveConstraintIndex_AtomicWrite(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "c-atomic", Status: "draft", CompiledAt: time.Now().Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	if _, err := os.Stat(constraintLockPath()); !os.IsNotExist(err) {
		t.Fatalf("constraint lock should not remain after save, err=%v", err)
	}

	tmpMatches, err := filepath.Glob(filepath.Join(".agents", "constraints", "index.json.tmp.*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(tmpMatches) != 0 {
		t.Fatalf("expected no temp files after atomic save, got %v", tmpMatches)
	}

	data, err := os.ReadFile(constraintIndexPath())
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("constraint index is not valid JSON: %s", string(data))
	}
}

func TestConstraintLoadMissingIndex(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)

	if _, err := loadConstraintIndex(); err == nil {
		t.Fatal("expected loadConstraintIndex to fail when index file is missing")
	}
}

func TestConstraintActivateDraft(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "c-1", Status: "draft", CompiledAt: time.Now().Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	stdout, err := captureStdout(t, func() error {
		return constraintActivateCmd.RunE(constraintActivateCmd, []string{"c-1"})
	})
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !strings.Contains(stdout, "Constraint \"c-1\" activated") {
		t.Fatalf("expected activation output, got: %q", stdout)
	}

	reloaded, err := loadConstraintIndex()
	if err != nil {
		t.Fatalf("loadConstraintIndex: %v", err)
	}
	if got := findConstraint(reloaded, "c-1").Status; got != "active" {
		t.Fatalf("constraint status = %q, want active", got)
	}
}

func TestConstraintRetireActive(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "c-1", Status: "active", CompiledAt: time.Now().Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	stdout, err := captureStdout(t, func() error {
		return constraintRetireCmd.RunE(constraintRetireCmd, []string{"c-1"})
	})
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	if !strings.Contains(stdout, "Constraint \"c-1\" retired") {
		t.Fatalf("expected retirement output, got: %q", stdout)
	}

	reloaded, err := loadConstraintIndex()
	if err != nil {
		t.Fatalf("loadConstraintIndex: %v", err)
	}
	if got := findConstraint(reloaded, "c-1").Status; got != "retired" {
		t.Fatalf("constraint status = %q, want retired", got)
	}
}

func TestConstraintListAndReviewNoConstraints(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	empty := &constraintIndex{SchemaVersion: 1}
	if err := saveConstraintIndex(empty); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	oldOutput := output
	output = "text"
	t.Cleanup(func() { output = oldOutput })

	listOut, err := captureStdout(t, func() error {
		return constraintListCmd.RunE(constraintListCmd, nil)
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(listOut, "No constraints found.") {
		t.Fatalf("expected no constraints message, got: %q", listOut)
	}

	reviewOut, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !strings.Contains(reviewOut, "No constraints need review.") {
		t.Fatalf("expected no review candidates message, got: %q", reviewOut)
	}
}

func TestConstraintActivateJSONAndReviewStale(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	stale := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:         "old-c1",
				Title:      "Old constraint",
				Status:     "draft",
				CompiledAt: time.Now().AddDate(0, 0, -91).Format(time.RFC3339),
			},
		},
	}
	if err := saveConstraintIndex(stale); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	reviewOut, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !json.Valid([]byte(reviewOut)) {
		t.Fatalf("expected JSON output for review, got: %q", reviewOut)
	}

	stdout, err := captureStdout(t, func() error {
		return constraintActivateCmd.RunE(constraintActivateCmd, []string{"old-c1"})
	})
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected activation JSON: %v", err)
	}
	if got["id"] != "old-c1" || got["status"] != "active" {
		t.Fatalf("unexpected activation payload: %v", got)
	}
}

func TestConstraintActivateRejectsNonDraft(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{ID: "c-1", Status: "active", CompiledAt: time.Now().Format(time.RFC3339)},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	if err := constraintActivateCmd.RunE(constraintActivateCmd, []string{"c-1"}); err == nil {
		t.Fatal("expected activate to reject non-draft constraint")
	}
}

func TestConstraintRetireRejectsMissingID(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	if err := os.WriteFile(constraintIndexPath(), []byte(`{"schema_version":1,"constraints":[{"id":"c-1","status":"draft","compiled_at":"2026-01-01"}]`), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := constraintRetireCmd.RunE(constraintRetireCmd, []string{"unknown"}); err == nil {
		t.Fatal("expected retire missing ID to fail")
	}
}

// chdirTo moved to testutil_test.go.

func TestConstraintReviewJSONRoundTrip(t *testing.T) {
	wd := t.TempDir()
	chdirTo(t, wd)
	mkdirConstraintsDir(t)

	oldOutput := output
	output = "json"
	t.Cleanup(func() { output = oldOutput })

	idx := &constraintIndex{
		SchemaVersion: 1,
		Constraints: []constraintEntry{
			{
				ID:         "c-1",
				Title:      "Recent",
				Status:     "draft",
				CompiledAt: time.Now().Format(time.RFC3339),
			},
			{
				ID:         "c-2",
				Title:      "Old",
				Status:     "active",
				CompiledAt: time.Now().AddDate(0, 0, -100).Format(time.RFC3339),
			},
		},
	}
	if err := saveConstraintIndex(idx); err != nil {
		t.Fatalf("saveConstraintIndex: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return constraintListCmd.RunE(constraintListCmd, nil)
	})
	if err != nil {
		t.Fatalf("list json: %v", err)
	}
	var got []constraintEntry
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("list json unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 constraints in list JSON, got %d", len(got))
	}

	reviewOut, err := captureStdout(t, func() error {
		return constraintReviewCmd.RunE(constraintReviewCmd, nil)
	})
	if err != nil {
		t.Fatalf("review json: %v", err)
	}
	var review []constraintEntry
	if err := json.Unmarshal([]byte(reviewOut), &review); err != nil {
		t.Fatalf("review json unmarshal: %v", err)
	}
	if len(review) != 1 {
		t.Fatalf("expected 1 stale constraint, got %d", len(review))
	}
}

func mkdirConstraintsDir(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(".agents", "constraints"), 0o755); err != nil {
		t.Fatalf("mkdir .agents/constraints: %v", err)
	}
}
