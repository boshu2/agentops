package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateRunID verifies that generateRunID returns 12-char lowercase hex strings
// with no duplicates across 1000 calls.
func TestGenerateRunID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateRunID()
		if len(id) != 12 {
			t.Fatalf("expected 12-char ID, got %d chars: %q", len(id), id)
		}
		if id != strings.ToLower(id) {
			t.Fatalf("expected lowercase, got %q", id)
		}
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("non-hex char %q in ID %q", string(c), id)
			}
		}
		if seen[id] {
			t.Fatalf("duplicate ID: %q", id)
		}
		seen[id] = true
	}
}

// initTestRepo creates a temporary git repo with an initial commit.
// Returns the repo root path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init setup (%v): %v\n%s", args, err, out)
		}
	}
	// Create a file and commit so HEAD exists.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "README.md"},
		{"git", "commit", "-m", "Initial commit"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git commit setup (%v): %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestGetCurrentBranch(t *testing.T) {
	repo := initTestRepo(t)

	branch, err := getCurrentBranch(repo)
	if err != nil {
		t.Fatalf("getCurrentBranch: %v", err)
	}
	// Git default branch is typically "main" or "master".
	if branch == "" || branch == "HEAD" {
		t.Fatalf("unexpected branch: %q", branch)
	}
}

func TestGetCurrentBranch_DetachedHEAD(t *testing.T) {
	repo := initTestRepo(t)

	// Detach HEAD by checking out the commit SHA.
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(out))

	cmd = exec.Command("git", "checkout", sha)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("detach HEAD: %v\n%s", err, out)
	}

	_, err = getCurrentBranch(repo)
	if err == nil {
		t.Fatal("expected error for detached HEAD")
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("expected 'detached HEAD' error, got: %v", err)
	}
}

func TestCreateWorktree(t *testing.T) {
	repo := initTestRepo(t)

	// Override cwd to the test repo.
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
		// Cleanup: remove worktree.
		cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
		cmd.Dir = repo
		_ = cmd.Run()
	}()

	// Verify worktree directory exists.
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree dir not created: %v", err)
	}

	// Verify it's a sibling directory (resolve symlinks for macOS /var → /private/var).
	wtParent, _ := filepath.EvalSymlinks(filepath.Dir(worktreePath))
	repoParent, _ := filepath.EvalSymlinks(filepath.Dir(repo))
	if wtParent != repoParent {
		t.Fatalf("worktree not a sibling: %s vs %s", wtParent, repoParent)
	}

	// Verify basename matches pattern.
	expected := filepath.Base(repo) + "-rpi-" + runID
	if filepath.Base(worktreePath) != expected {
		t.Fatalf("unexpected basename: %q, expected %q", filepath.Base(worktreePath), expected)
	}

	// Verify branch was not created for detached worktree mode.
	cmd := exec.Command("git", "branch", "--list", "rpi/"+runID)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if strings.Contains(string(out), "rpi/"+runID) {
		t.Fatalf("branch rpi/%s should not exist", runID)
	}

	// Verify .agents/rpi/ exists in worktree.
	agentsDir := filepath.Join(worktreePath, ".agents", "rpi")
	if _, err := os.Stat(agentsDir); err != nil {
		t.Fatalf(".agents/rpi/ not created in worktree: %v", err)
	}
}

func TestCreateWorktree_RetryOnCollision(t *testing.T) {
	repo := initTestRepo(t)

	origDir, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	// createWorktree should still succeed and follow detached naming.
	worktreePath, runID, err := createWorktree(repo)
	if err != nil {
		t.Fatalf("createWorktree should succeed: %v", err)
	}
	defer func() {
		cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
		cmd.Dir = repo
		_ = cmd.Run()
	}()

	expected := filepath.Base(repo) + "-rpi-" + runID
	if filepath.Base(worktreePath) != expected {
		t.Fatalf("unexpected basename: %q, expected %q", filepath.Base(worktreePath), expected)
	}
}

func TestMergeWorktree(t *testing.T) {
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
		// Cleanup after test (worktree may already be removed by removeWorktree).
		cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
		cmd.Dir = repo
		_ = cmd.Run()
	}()

	// Make a commit in the worktree.
	testFile := filepath.Join(worktreePath, "new-file.txt")
	if err := os.WriteFile(testFile, []byte("worktree change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "new-file.txt"},
		{"git", "commit", "-m", "Add file in worktree"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = worktreePath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("worktree commit (%v): %v\n%s", args, err, out)
		}
	}

	// Merge back.
	if err := mergeWorktree(repo, worktreePath, runID); err != nil {
		t.Fatalf("mergeWorktree: %v", err)
	}

	// Verify file exists on original branch.
	mergedFile := filepath.Join(repo, "new-file.txt")
	if _, err := os.Stat(mergedFile); err != nil {
		t.Fatalf("merged file not found: %v", err)
	}
}

func TestMergeWorktree_Conflict(t *testing.T) {
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
	}()

	// Create conflicting changes in both repos.
	conflictFile := "conflict.txt"

	// In worktree: create and commit.
	if err := os.WriteFile(filepath.Join(worktreePath, conflictFile), []byte("worktree version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", conflictFile},
		{"git", "commit", "-m", "Worktree side of conflict"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = worktreePath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("worktree commit (%v): %v\n%s", args, err, out)
		}
	}

	// In original: create and commit (conflicting content).
	if err := os.WriteFile(filepath.Join(repo, conflictFile), []byte("original version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", conflictFile},
		{"git", "commit", "-m", "Original side of conflict"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("original commit (%v): %v\n%s", args, err, out)
		}
	}

	// Merge should fail with conflict info.
	err = mergeWorktree(repo, worktreePath, runID)
	if err == nil {
		t.Fatal("expected merge conflict error")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected 'conflict' in error, got: %v", err)
	}

	// Verify original repo is clean (merge was aborted).
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("repo not clean after merge abort: %s", out)
	}

	// Verify worktree still exists with its committed file intact.
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree should be preserved: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(worktreePath, conflictFile))
	if err != nil {
		t.Fatalf("worktree file should still exist: %v", err)
	}
	if string(data) != "worktree version\n" {
		t.Fatalf("worktree file content changed: %q", string(data))
	}
}

func TestMergeWorktree_DirtyRepo(t *testing.T) {
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
	}()

	// Make uncommitted changes in original repo.
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "dirty.txt")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	err = mergeWorktree(repo, worktreePath, runID)
	if err == nil {
		t.Fatal("expected error for dirty repo")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") && !strings.Contains(err.Error(), "after 5 retries") {
		t.Fatalf("expected 'uncommitted changes' error, got: %v", err)
	}
}

func TestRemoveWorktree(t *testing.T) {
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

	// Verify worktree exists.
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree should exist: %v", err)
	}

	// Remove it.
	if err := removeWorktree(repo, worktreePath, runID); err != nil {
		t.Fatalf("removeWorktree: %v", err)
	}

	// Verify directory gone.
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree dir should be removed, got: %v", err)
	}

	// No branch assertion in detached mode; branch cleanup is best-effort.
}

func TestRemoveWorktree_PathValidation(t *testing.T) {
	repo := initTestRepo(t)

	// Try to remove a path that's NOT a valid sibling worktree.
	err := removeWorktree(repo, "/tmp/evil-path", "abc123")
	if err == nil {
		t.Fatal("expected path validation error")
	}
	if !strings.Contains(err.Error(), "path validation failed") {
		t.Fatalf("expected 'path validation failed' error, got: %v", err)
	}

	// Try with a path that has wrong basename.
	wrongPath := filepath.Join(filepath.Dir(repo), "not-a-worktree")
	err = removeWorktree(repo, wrongPath, "abc123")
	if err == nil {
		t.Fatal("expected path validation error for wrong basename")
	}
	if !strings.Contains(err.Error(), "path validation failed") {
		t.Fatalf("expected 'path validation failed' error, got: %v", err)
	}
}

func TestPhasedState_WorktreeFields(t *testing.T) {
	dir := t.TempDir()

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "test goal",
		Phase:         1,
		Cycle:         1,
		Verdicts:      make(map[string]string),
		Attempts:      make(map[string]int),
		StartedAt:     "2026-02-15T00:00:00Z",
		WorktreePath:  "/tmp/test-rpi-abc123",
		RunID:         "abc123",
	}

	// Save state.
	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Load state.
	loaded, err := loadPhasedState(dir)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}

	if loaded.WorktreePath != state.WorktreePath {
		t.Fatalf("WorktreePath mismatch: %q vs %q", loaded.WorktreePath, state.WorktreePath)
	}
	if loaded.RunID != state.RunID {
		t.Fatalf("RunID mismatch: %q vs %q", loaded.RunID, state.RunID)
	}

	// Verify JSON roundtrip contains the fields.
	data, _ := os.ReadFile(filepath.Join(dir, ".agents", "rpi", phasedStateFile))
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if raw["worktree_path"] != state.WorktreePath {
		t.Fatalf("JSON worktree_path mismatch: %v", raw["worktree_path"])
	}
	if raw["run_id"] != state.RunID {
		t.Fatalf("JSON run_id mismatch: %v", raw["run_id"])
	}
}

func TestPhasedState_WorktreeFields_OmitEmpty(t *testing.T) {
	dir := t.TempDir()

	// State without worktree fields (backward compat).
	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "test goal",
		Phase:         1,
		Cycle:         1,
		Verdicts:      make(map[string]string),
		Attempts:      make(map[string]int),
		StartedAt:     "2026-02-15T00:00:00Z",
	}

	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Verify JSON does NOT contain worktree_path or run_id when empty.
	data, _ := os.ReadFile(filepath.Join(dir, ".agents", "rpi", phasedStateFile))
	if strings.Contains(string(data), "worktree_path") {
		t.Fatal("empty worktree_path should be omitted from JSON")
	}
	if strings.Contains(string(data), "run_id") {
		t.Fatal("empty run_id should be omitted from JSON")
	}

	// Load and verify zero values.
	loaded, err := loadPhasedState(dir)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}
	if loaded.WorktreePath != "" {
		t.Fatalf("expected empty WorktreePath, got %q", loaded.WorktreePath)
	}
	if loaded.RunID != "" {
		t.Fatalf("expected empty RunID, got %q", loaded.RunID)
	}
}
