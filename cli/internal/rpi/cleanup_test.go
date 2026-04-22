package rpi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckTerminalRunStale_CompletedSkipped(t *testing.T) {
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	_, stale := CheckTerminalRunStale("r1", "/root", "/state", "completed", "", "", "", "/wt", 0, now)
	if stale {
		t.Error("completed runs should not be stale")
	}
}

func TestCheckTerminalRunStale_NoWorktreeSkipped(t *testing.T) {
	now := time.Now()
	_, stale := CheckTerminalRunStale("r1", "/root", "/state", "interrupted", "", "", "", "", 0, now)
	if stale {
		t.Error("runs without worktree path should not be stale")
	}
}

func TestCheckTerminalRunStale_NonExistentWorktreeSkipped(t *testing.T) {
	now := time.Now()
	_, stale := CheckTerminalRunStale("r1", "/root", "/state", "interrupted", "", "", "", "/nonexistent/xyz/abc", 0, now)
	if stale {
		t.Error("non-existent worktree should not be stale")
	}
}

func TestCheckTerminalRunStale_MinAgeFiltersYoung(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	// Started 10 minutes ago; minAge is 1 hour
	startedAt := now.Add(-10 * time.Minute).Format(time.RFC3339)
	_, stale := CheckTerminalRunStale("r1", "/root", "/state", "interrupted", "", "", startedAt, tmp, time.Hour, now)
	if stale {
		t.Error("young run should be filtered")
	}
}

func TestCheckTerminalRunStale_AcceptsOldEnough(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-2 * time.Hour).Format(time.RFC3339)
	entry, stale := CheckTerminalRunStale("r1", "/root", "/state", "interrupted", "reason", "", startedAt, tmp, time.Hour, now)
	if !stale {
		t.Fatal("old run should be flagged stale")
	}
	if entry.RunID != "r1" {
		t.Errorf("RunID = %q", entry.RunID)
	}
	if entry.Reason != "reason" {
		t.Errorf("reason preserved: %q", entry.Reason)
	}
}

func TestCheckTerminalRunStale_DefaultReasonWhenEmpty(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now()
	entry, stale := CheckTerminalRunStale("r1", "/root", "/state", "interrupted", "", "", "", tmp, 0, now)
	if !stale {
		t.Fatal("should be stale")
	}
	if !strings.Contains(entry.Reason, "terminal status: interrupted") {
		t.Errorf("default reason = %q", entry.Reason)
	}
}

func TestResolveCleanupRepoRoot_PrefersSibling(t *testing.T) {
	target := "/parent/wt-a"
	roots := []string{
		"/parent/wt-b", // sibling
		target,         // self
		"/other/dir",   // unrelated
	}
	got := ResolveCleanupRepoRoot("/cwd", target, roots)
	if got != "/parent/wt-b" {
		t.Errorf("got %q, want sibling", got)
	}
}

func TestResolveCleanupRepoRoot_FallsBackToCwd(t *testing.T) {
	target := "/parent/wt-a"
	roots := []string{target, "/other/dir"}
	got := ResolveCleanupRepoRoot("/fallback", target, roots)
	if got != "/fallback" {
		t.Errorf("got %q, want /fallback", got)
	}
}

func TestValidateWorktreeSibling_ValidSibling(t *testing.T) {
	if err := ValidateWorktreeSibling("/parent/repo", "/parent/worktree"); err != nil {
		t.Errorf("sibling should be valid: %v", err)
	}
}

func TestValidateWorktreeSibling_NotSibling(t *testing.T) {
	err := ValidateWorktreeSibling("/parent/repo", "/other/worktree")
	if err == nil {
		t.Fatal("should reject non-sibling")
	}
	if !strings.Contains(err.Error(), "not a sibling") {
		t.Errorf("err = %v", err)
	}
}

func TestValidateWorktreeSibling_RepoRootRefused(t *testing.T) {
	err := ValidateWorktreeSibling("/parent/repo", "/parent/repo")
	if err == nil {
		t.Fatal("should refuse repo root")
	}
	if !strings.Contains(err.Error(), "repo root") {
		t.Errorf("err = %v", err)
	}
}

func TestPatchStateWithCancelFields(t *testing.T) {
	tmp := t.TempDir()
	state := filepath.Join(tmp, "state.json")
	_ = os.WriteFile(state, []byte(`{"run_id":"r1","phase":2}`), 0o600)

	err := PatchStateWithCancelFields(state, "user cancel", "2026-04-22T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(state)
	body := string(data)
	if !strings.Contains(body, `"terminal_status": "interrupted"`) {
		t.Errorf("terminal_status missing, got: %s", body)
	}
	if !strings.Contains(body, `"terminal_reason": "user cancel"`) {
		t.Errorf("reason missing: %s", body)
	}
}

func TestPatchStateWithCancelFields_MissingFile(t *testing.T) {
	err := PatchStateWithCancelFields("/nonexistent/xyz.json", "r", "t")
	if err == nil {
		t.Error("expected error")
	}
}

func TestPatchStateWithCancelFields_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	state := filepath.Join(tmp, "bad.json")
	_ = os.WriteFile(state, []byte("not json"), 0o600)

	err := PatchStateWithCancelFields(state, "r", "t")
	if err == nil {
		t.Error("expected error on invalid json")
	}
}

func TestUpdateFlatStateIfMatches_MatchingRunID(t *testing.T) {
	tmp := t.TempDir()
	flat := filepath.Join(tmp, "flat.json")
	_ = os.WriteFile(flat, []byte(`{"run_id":"r1","phase":2}`), 0o600)

	UpdateFlatStateIfMatches(flat, "r1", "my reason", "2026-04-22T12:00:00Z")

	data, _ := os.ReadFile(flat)
	body := string(data)
	if !strings.Contains(body, `"terminal_status": "stale"`) {
		t.Errorf("status missing: %s", body)
	}
	if !strings.Contains(body, `"my reason"`) {
		t.Errorf("reason missing: %s", body)
	}
}

func TestUpdateFlatStateIfMatches_WrongRunID(t *testing.T) {
	tmp := t.TempDir()
	flat := filepath.Join(tmp, "flat.json")
	original := `{"run_id":"r1","phase":2}`
	_ = os.WriteFile(flat, []byte(original), 0o600)

	UpdateFlatStateIfMatches(flat, "other-run", "reason", "time")

	data, _ := os.ReadFile(flat)
	if string(data) != original {
		t.Errorf("file modified despite non-matching run_id: %s", string(data))
	}
}

func TestUpdateFlatStateIfMatches_MissingFile(t *testing.T) {
	// Should silently no-op, not panic
	UpdateFlatStateIfMatches("/nonexistent/x.json", "r1", "r", "t")
}
