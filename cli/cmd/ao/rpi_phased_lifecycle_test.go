package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeFailurePropagation verifies that mergeWorktree errors surface as
// non-zero command results (non-nil error). Uses a real git repo to test the
// dirty-repo rejection path which is the most deterministic failure mode.
func TestMergeFailurePropagation(t *testing.T) {
	repo := initTestRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	worktreePath, runID, err := createWorktree(repo)
	if err != nil {
		t.Fatalf("createWorktree: %v", err)
	}
	defer func() {
		cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
		cmd.Dir = repo
		_ = cmd.Run()
		cmd = exec.Command("git", "branch", "-D", "rpi/"+runID)
		cmd.Dir = repo
		_ = cmd.Run()
	}()

	// Dirty the original repo so merge is rejected.
	dirtyFile := filepath.Join(repo, "uncommitted.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "uncommitted.txt")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	// mergeWorktree must return a non-nil error — callers must propagate it.
	mergeErr := mergeWorktree(repo, runID)
	if mergeErr == nil {
		t.Fatal("expected mergeWorktree to return error for dirty repo; got nil (silent-success violation)")
	}

	// Verify the error message is actionable (not just a wrapped exit code).
	errMsg := mergeErr.Error()
	if !strings.Contains(errMsg, "uncommitted") && !strings.Contains(errMsg, "after 5 retries") {
		t.Errorf("merge failure error should mention uncommitted changes, got: %v", mergeErr)
	}

	t.Logf("merge failure propagated correctly: %v", mergeErr)
}

// TestCleanupFailurePropagation verifies that removeWorktree errors are logged
// with actionable context via logFailureContext and propagated as non-nil errors.
// Tests the specific behavior added in this task: cleanup failures must not
// silently succeed.
func TestCleanupFailurePropagation(t *testing.T) {
	// Test that removeWorktree returns an error for invalid paths (path validation).
	repo := initTestRepo(t)

	// Path that fails validation (not an rpi sibling path).
	rmErr := removeWorktree(repo, "/tmp/not-a-valid-rpi-worktree", "somerunid")
	if rmErr == nil {
		t.Fatal("expected removeWorktree to return error for invalid path; got nil (silent-success violation)")
	}
	if !strings.Contains(rmErr.Error(), "path validation failed") {
		t.Errorf("removeWorktree error should mention path validation, got: %v", rmErr)
	}

	// Verify logFailureContext records the error with actionable context.
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logFailureContext(logPath, "test-run", "cleanup", rmErr)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not written after logFailureContext: %v", err)
	}
	logContent := string(data)

	// Must contain FAILURE_CONTEXT marker.
	if !strings.Contains(logContent, "FAILURE_CONTEXT") {
		t.Errorf("log must contain FAILURE_CONTEXT marker, got: %q", logContent)
	}
	// Must contain actionable guidance.
	if !strings.Contains(logContent, "action:") {
		t.Errorf("log must contain actionable guidance (action:), got: %q", logContent)
	}
	// Must include the phase name.
	if !strings.Contains(logContent, "cleanup") {
		t.Errorf("log must include phase name 'cleanup', got: %q", logContent)
	}
	// Must include the run ID.
	if !strings.Contains(logContent, "test-run") {
		t.Errorf("log must include run ID 'test-run', got: %q", logContent)
	}

	t.Logf("cleanup failure logged with actionable context: %s", logContent)
}
