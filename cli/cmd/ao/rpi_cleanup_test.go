// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanupStaleRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a stale run: no heartbeat, non-terminal phase, worktree points to nonexistent dir.
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "stale-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version": 1,
		"run_id":         "stale-run",
		"goal":           "test stale",
		"phase":          2,
		"worktree_path":  "/nonexistent/path",
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	statePath := filepath.Join(runDir, phasedStateFile)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	if len(staleRuns) != 1 {
		t.Fatalf("expected 1 stale run, got %d", len(staleRuns))
	}
	if staleRuns[0].RunID != "stale-run" {
		t.Errorf("expected stale-run, got %s", staleRuns[0].RunID)
	}
	if staleRuns[0].Reason != "worktree missing" {
		t.Errorf("expected reason 'worktree missing', got %q", staleRuns[0].Reason)
	}

	// Mark it stale.
	if err := markRunStale(staleRuns[0]); err != nil {
		t.Fatalf("markRunStale: %v", err)
	}

	// Verify terminal metadata was written.
	updated, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var updatedState map[string]any
	if err := json.Unmarshal(updated, &updatedState); err != nil {
		t.Fatal(err)
	}
	if updatedState["terminal_status"] != "stale" {
		t.Errorf("expected terminal_status 'stale', got %v", updatedState["terminal_status"])
	}
	if updatedState["terminal_reason"] != "worktree missing" {
		t.Errorf("expected terminal_reason 'worktree missing', got %v", updatedState["terminal_reason"])
	}
	if updatedState["terminated_at"] == nil || updatedState["terminated_at"] == "" {
		t.Error("expected terminated_at to be set")
	}
}

func TestCleanupActiveRunUntouched(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an active run with a fresh heartbeat.
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "active-run",
		phase:  2,
		schema: 1,
		goal:   "active goal",
		hbAge:  1 * time.Minute, // fresh
	})

	staleRuns := findStaleRuns(tmpDir)
	for _, sr := range staleRuns {
		if sr.RunID == "active-run" {
			t.Fatal("active run should not be detected as stale")
		}
	}
}

func TestCleanupDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a stale run.
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "dry-run-test")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version": 1,
		"run_id":         "dry-run-test",
		"goal":           "dry run test",
		"phase":          1,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	statePath := filepath.Join(runDir, phasedStateFile)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	if len(staleRuns) == 0 {
		t.Fatal("expected at least 1 stale run for dry-run test")
	}

	// Verify the state file is NOT modified (simulating dry-run by not calling markRunStale).
	originalData, _ := os.ReadFile(statePath)
	var originalState map[string]any
	_ = json.Unmarshal(originalData, &originalState)

	if originalState["terminal_status"] != nil {
		t.Error("dry run should not have written terminal_status")
	}
}

func TestCleanupByRunID(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two stale runs.
	for _, id := range []string{"target-run", "other-run"} {
		runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", id)
		if err := os.MkdirAll(runDir, 0755); err != nil {
			t.Fatal(err)
		}
		state := map[string]any{
			"schema_version": 1,
			"run_id":         id,
			"goal":           id + " goal",
			"phase":          1,
			"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		}
		data, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			t.Fatalf("marshal state: %v", marshalErr)
		}
		if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	allStale := findStaleRuns(tmpDir)
	if len(allStale) != 2 {
		t.Fatalf("expected 2 stale runs, got %d", len(allStale))
	}

	// Filter for specific run ID (simulating --run-id).
	var filtered []staleRunEntry
	for _, sr := range allStale {
		if sr.RunID == "target-run" {
			filtered = append(filtered, sr)
		}
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered run, got %d", len(filtered))
	}
	if filtered[0].RunID != "target-run" {
		t.Errorf("expected target-run, got %s", filtered[0].RunID)
	}
}

func TestCleanupSkipsTerminalRuns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a run that already has terminal metadata.
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "already-stale")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version":  1,
		"run_id":          "already-stale",
		"goal":            "already marked",
		"phase":           1,
		"terminal_status": "stale",
		"terminal_reason": "previously marked",
		"terminated_at":   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"started_at":      time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	for _, sr := range staleRuns {
		if sr.RunID == "already-stale" {
			t.Fatal("run with existing terminal_status should be skipped")
		}
	}
}

func TestCleanupIncludesTerminalRunsWithExistingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "failed-run")
	worktreePath := filepath.Join(tmpDir, "repo-rpi-failed-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatal(err)
	}

	state := map[string]any{
		"schema_version":  1,
		"run_id":          "failed-run",
		"goal":            "failed run",
		"phase":           2,
		"terminal_status": "failed",
		"terminal_reason": "phase implementation: error",
		"terminated_at":   time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		"started_at":      time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
		"worktree_path":   worktreePath,
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRunsWithMinAge(tmpDir, 1*time.Hour, time.Now())
	if len(staleRuns) != 1 {
		t.Fatalf("expected 1 terminal cleanup candidate, got %d", len(staleRuns))
	}
	if staleRuns[0].RunID != "failed-run" {
		t.Fatalf("expected failed-run, got %s", staleRuns[0].RunID)
	}
	if staleRuns[0].Terminal != "failed" {
		t.Fatalf("expected terminal=failed, got %q", staleRuns[0].Terminal)
	}
}

func TestResolveCleanupRepoRootPrefersSiblingController(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "repo")
	target := filepath.Join(parent, "repo-rpi-stale")
	other := filepath.Join(parent, "repo-rpi-other")

	for _, dir := range []string{cwd, target, other} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	got := resolveCleanupRepoRoot(cwd, target)
	if filepath.Clean(got) == filepath.Clean(target) {
		t.Fatalf("expected controller root different from target worktree, got %q", got)
	}
	if filepath.Dir(filepath.Clean(got)) != filepath.Dir(filepath.Clean(target)) {
		t.Fatalf("expected sibling controller root, got %q for target %q", got, target)
	}
}

func TestCleanupSkipsCompletedRuns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a completed run (phase 3, schema v1 = terminal).
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "done-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version": 1,
		"run_id":         "done-run",
		"goal":           "completed",
		"phase":          3,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	for _, sr := range staleRuns {
		if sr.RunID == "done-run" {
			t.Fatal("completed run should not be detected as stale")
		}
	}
}

func TestFindStaleRunsWithMinAge_SkipsRecentRuns(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "recent-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"schema_version": 1,
		"run_id":         "recent-run",
		"goal":           "recent",
		"phase":          2,
		"started_at":     now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRunsWithMinAge(tmpDir, 1*time.Hour, now)
	if len(staleRuns) != 0 {
		t.Fatalf("expected 0 stale runs with age filter, got %d", len(staleRuns))
	}
}

func TestExecuteRPICleanup_StaleAfterOnlyMarksOldRuns(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	makeRun := func(runID string, startedAt time.Time) {
		t.Helper()
		runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", runID)
		if err := os.MkdirAll(runDir, 0755); err != nil {
			t.Fatal(err)
		}
		state := map[string]any{
			"schema_version": 1,
			"run_id":         runID,
			"goal":           runID,
			"phase":          2,
			"started_at":     startedAt.Format(time.RFC3339),
		}
		data, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			t.Fatalf("marshal state: %v", marshalErr)
		}
		if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	makeRun("old-run", now.Add(-2*time.Hour))
	makeRun("new-run", now.Add(-10*time.Minute))

	if err := executeRPICleanup(tmpDir, "", true, false, false, false, 1*time.Hour); err != nil {
		t.Fatalf("executeRPICleanup: %v", err)
	}

	readTerminalStatus := func(runID string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "rpi", "runs", runID, phasedStateFile))
		if err != nil {
			t.Fatalf("read %s state: %v", runID, err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshal %s state: %v", runID, err)
		}
		if v, ok := raw["terminal_status"].(string); ok {
			return v
		}
		return ""
	}

	if got := readTerminalStatus("old-run"); got != "stale" {
		t.Fatalf("expected old-run terminal_status=stale, got %q", got)
	}
	if got := readTerminalStatus("new-run"); got != "" {
		t.Fatalf("expected new-run terminal_status to be empty, got %q", got)
	}
}

func TestRemoveOrphanedWorktree_PathValidation(t *testing.T) {
	// Create a fake repo root structure: /tmp/xxx/repo/
	parentDir := t.TempDir()
	repoRoot := filepath.Join(parentDir, "myrepo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		worktreePath string
		wantErr      string
	}{
		{
			name:         "sibling directory is allowed",
			worktreePath: filepath.Join(parentDir, "myrepo-rpi-abc123"),
			wantErr:      "", // no path validation error (will fail on git worktree remove, but path check passes)
		},
		{
			name:         "deeply nested outside path is rejected",
			worktreePath: "/tmp/evil/path",
			wantErr:      "not a sibling",
		},
		{
			name:         "root path is rejected",
			worktreePath: "/",
			wantErr:      "not a sibling",
		},
		{
			name:         "repo root itself is rejected",
			worktreePath: repoRoot,
			wantErr:      "repo root",
		},
		{
			name:         "child of repo is rejected (not a sibling)",
			worktreePath: filepath.Join(repoRoot, "subdir"),
			wantErr:      "not a sibling",
		},
		{
			name:         "cousin directory is rejected",
			worktreePath: filepath.Join(parentDir, "other", "nested"),
			wantErr:      "not a sibling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the worktree directory so os.Stat passes in the caller.
			if tt.worktreePath != "/" {
				_ = os.MkdirAll(tt.worktreePath, 0755)
			}

			err := removeOrphanedWorktree(repoRoot, tt.worktreePath, "test-run")
			if tt.wantErr == "" {
				// We expect path validation to pass but git worktree remove will fail.
				// That's fine — we're testing the path guard, not git.
				if err != nil && strings.Contains(err.Error(), "not a sibling") {
					t.Errorf("expected path validation to pass, got: %v", err)
				}
				if err != nil && strings.Contains(err.Error(), "repo root") {
					t.Errorf("expected path validation to pass, got: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestRemoveOrphanedWorktree_RepoRootProtection(t *testing.T) {
	// Ensure we never delete the repo root even with a matching parent.
	parentDir := t.TempDir()
	repoRoot := filepath.Join(parentDir, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a sentinel file inside repo root.
	sentinel := filepath.Join(repoRoot, "DO_NOT_DELETE")
	if err := os.WriteFile(sentinel, []byte("important"), 0644); err != nil {
		t.Fatal(err)
	}

	err := removeOrphanedWorktree(repoRoot, repoRoot, "test-run")
	if err == nil {
		t.Fatal("expected error when worktree path equals repo root")
	}
	if !strings.Contains(err.Error(), "repo root") {
		t.Errorf("expected 'repo root' error, got: %v", err)
	}

	// Verify sentinel file still exists.
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel file was deleted — repo root was removed!")
	}
}

func TestCollectLegacyRPIBranches_RunIDScope(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := tmpDir

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", strings.Join(args, " "), err, string(output))
		}
	}

	runGit("init", "-q")
	runGit("config", "user.email", "noreply@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-q", "-b", "main")
	runGit("commit", "-q", "--allow-empty", "-m", "init")
	runGit("branch", "-q", "rpi/target")
	runGit("branch", "-q", "rpi/other")
	runGit("branch", "-q", "codex/auto-rpi-old")

	branches, err := collectLegacyRPIBranches(repoPath, "target", false)
	if err != nil {
		t.Fatalf("collect branches: %v", err)
	}
	if len(branches) != 1 || branches[0] != "rpi/target" {
		t.Fatalf("expected only runID branch, got %v", branches)
	}

	if err := cleanupLegacyRPIBranches(repoPath, "target", false, true); err != nil {
		t.Fatalf("cleanup dry run: %v", err)
	}
	if err := runGitCheckBranch(repoPath, "rpi/target"); err != nil {
		t.Fatalf("dry-run should preserve branch: %v", err)
	}
}

func TestCleanupLegacyRPIBranches_AllAndActiveSafety(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := tmpDir

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", strings.Join(args, " "), err, string(output))
		}
	}

	runGit("init", "-q")
	runGit("config", "user.email", "noreply@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-q", "-b", "main")
	runGit("commit", "-q", "--allow-empty", "-m", "init")
	runGit("checkout", "-q", "-b", "rpi/active")
	runGit("branch", "-q", "rpi/inactive")
	runGit("branch", "-q", "codex/auto-rpi-old")
	runGit("checkout", "-q", "main")
	worktreeActivePath := filepath.Join(repoPath, "active-worktree")
	runGit("worktree", "add", worktreeActivePath, "rpi/active")

	if err := cleanupLegacyRPIBranches(repoPath, "", true, false); err != nil {
		t.Fatalf("cleanup all: %v", err)
	}

	if err := runGitCheckBranch(repoPath, "rpi/active"); err != nil {
		t.Fatalf("active branch should be preserved: %v", err)
	}
	if err := runGitCheckBranch(repoPath, "codex/auto-rpi-old"); err == nil {
		t.Fatalf("codex/auto-rpi-old branch should be removed")
	}
	if err := runGitCheckBranch(repoPath, "rpi/inactive"); err == nil {
		t.Fatalf("rpi/inactive branch should be removed")
	}
}

func TestExecuteRPICleanup_DryRunWithPruneFlags_PreservesStateAndBranches(t *testing.T) {
	repoPath := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", strings.Join(args, " "), err, string(output))
		}
	}

	runGit("init", "-q")
	runGit("config", "user.email", "noreply@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-q", "-b", "main")
	runGit("commit", "-q", "--allow-empty", "-m", "init")
	runGit("branch", "-q", "rpi/dry-run")

	runDir := filepath.Join(repoPath, ".agents", "rpi", "runs", "dry-run-stale")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(runDir, phasedStateFile)
	state := map[string]any{
		"schema_version": 1,
		"run_id":         "dry-run-stale",
		"goal":           "dry run stale",
		"phase":          2,
		"started_at":     time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := executeRPICleanup(repoPath, "", true, true, true, true, 0); err != nil {
		t.Fatalf("executeRPICleanup dry-run: %v", err)
	}

	if err := runGitCheckBranch(repoPath, "rpi/dry-run"); err != nil {
		t.Fatalf("dry-run should preserve branch: %v", err)
	}

	updated, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after dry-run: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(updated, &raw); err != nil {
		t.Fatalf("unmarshal state after dry-run: %v", err)
	}
	if _, ok := raw["terminal_status"]; ok {
		t.Fatalf("dry-run should not mutate terminal_status, got %v", raw["terminal_status"])
	}
}

func TestExecuteRPICleanup_PruneBranchesPreservesActiveCheckedOutWorktree(t *testing.T) {
	repoPath := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", strings.Join(args, " "), err, string(output))
		}
	}

	runGit("init", "-q")
	runGit("config", "user.email", "noreply@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "commit.gpgsign", "false")
	runGit("checkout", "-q", "-b", "main")
	runGit("commit", "-q", "--allow-empty", "-m", "init")
	runGit("checkout", "-q", "-b", "rpi/active")
	runGit("checkout", "-q", "main")

	activeWorktreePath := filepath.Join(repoPath, "active-worktree")
	runGit("worktree", "add", activeWorktreePath, "rpi/active")

	runDir := filepath.Join(repoPath, ".agents", "rpi", "runs", "active-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(runDir, phasedStateFile)
	state := map[string]any{
		"schema_version": 1,
		"run_id":         "active-run",
		"goal":           "active run",
		"phase":          2,
		"worktree_path":  activeWorktreePath,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		t.Fatalf("marshal state: %v", marshalErr)
	}
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}
	hbPath := filepath.Join(runDir, "heartbeat.txt")
	hb := time.Now().UTC().Format(time.RFC3339Nano) + "\n"
	if err := os.WriteFile(hbPath, []byte(hb), 0644); err != nil {
		t.Fatal(err)
	}

	if err := executeRPICleanup(repoPath, "", true, false, true, false, 0); err != nil {
		t.Fatalf("executeRPICleanup: %v", err)
	}

	if _, err := os.Stat(activeWorktreePath); err != nil {
		t.Fatalf("active checked-out worktree should be preserved: %v", err)
	}
	if err := runGitCheckBranch(repoPath, "rpi/active"); err != nil {
		t.Fatalf("active checked-out branch should be preserved: %v", err)
	}
}

func runGitCheckBranch(repoPath, name string) error {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = repoPath
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// updateFlatStateIfMatches
// ---------------------------------------------------------------------------

func TestUpdateFlatStateIfMatches_MatchingRunID(t *testing.T) {
	tmp := t.TempDir()
	flatPath := filepath.Join(tmp, "phased-state.json")

	state := map[string]any{
		"run_id": "run-abc",
		"phase":  1,
		"status": "running",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(flatPath, data, 0644)

	updateFlatStateIfMatches(flatPath, "run-abc", "stale-worktree", "2026-04-04T12:00:00Z")

	updated, _ := os.ReadFile(flatPath)
	var result map[string]any
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatalf("invalid JSON after update: %v", err)
	}
	if result["terminal_status"] != "stale" {
		t.Errorf("terminal_status = %v, want %q", result["terminal_status"], "stale")
	}
	if result["terminal_reason"] != "stale-worktree" {
		t.Errorf("terminal_reason = %v, want %q", result["terminal_reason"], "stale-worktree")
	}
	if result["terminated_at"] != "2026-04-04T12:00:00Z" {
		t.Errorf("terminated_at = %v, want %q", result["terminated_at"], "2026-04-04T12:00:00Z")
	}
}

func TestUpdateFlatStateIfMatches_NonMatchingRunID(t *testing.T) {
	tmp := t.TempDir()
	flatPath := filepath.Join(tmp, "phased-state.json")

	state := map[string]any{"run_id": "run-abc", "status": "running"}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(flatPath, data, 0644)

	updateFlatStateIfMatches(flatPath, "run-DIFFERENT", "reason", "2026-04-04T12:00:00Z")

	updated, _ := os.ReadFile(flatPath)
	var result map[string]any
	json.Unmarshal(updated, &result)
	if _, exists := result["terminal_status"]; exists {
		t.Error("terminal_status should not be set for non-matching run_id")
	}
}

func TestUpdateFlatStateIfMatches_NonexistentFile(t *testing.T) {
	// Should not panic
	updateFlatStateIfMatches("/nonexistent/path.json", "run-1", "reason", "2026-04-04T12:00:00Z")
}

func TestUpdateFlatStateIfMatches_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	flatPath := filepath.Join(tmp, "bad.json")
	os.WriteFile(flatPath, []byte("not json"), 0644)

	// Should not panic
	updateFlatStateIfMatches(flatPath, "run-1", "reason", "2026-04-04T12:00:00Z")
}

// --- preserveWorktreeCommits (soc-ewne / mc-m3.5-pre3) ---
//
// preserveWorktreeCommits is called BEFORE removeOrphanedWorktree on a failed
// cycle. It detects commits in the worktree's HEAD ancestry that are not
// reachable from main, and creates a `codex/preserve-<runID>` branch on the
// repo so the work is not garbage-collected when the worktree is removed.
//
// This guards against the 2026-05-06 overnight failure mode: 4 cycles produced
// real commits inside detached worktrees; auto-clean orphaned them. Three were
// confirmed unreachable in git fsck and required manual recovery.

// setupRepoWithWorktree creates a tiny repo and a sibling detached worktree
// with one optional extra commit on top of HEAD. Returns (repoRoot, worktreePath).
// The worktree's HEAD points to extraCommitSHA when madeCommit is true.
func setupRepoWithWorktree(t *testing.T, runID string, madeCommit bool) (string, string, string) {
	t.Helper()
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Isolate from user/global git config.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
			"HOME="+parent,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run(repoRoot, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repoRoot, "add", ".")
	run(repoRoot, "commit", "-m", "seed")

	// Add a sibling detached worktree on the same parent dir so
	// ValidateWorktreeSibling passes.
	worktreePath := filepath.Join(parent, "repo-rpi-"+runID)
	run(repoRoot, "worktree", "add", "--detach", worktreePath, "HEAD")

	extraSHA := ""
	if madeCommit {
		if err := os.WriteFile(filepath.Join(worktreePath, "work.txt"), []byte("cycle work\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run(worktreePath, "add", ".")
		run(worktreePath, "commit", "-m", "cycle "+runID+" landed")

		out, err := exec.Command("git", "-C", worktreePath, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatalf("rev-parse: %v", err)
		}
		extraSHA = strings.TrimSpace(string(out))
	}

	return repoRoot, worktreePath, extraSHA
}

// branchTipSHA returns the commit sha that branchName points to in repoRoot,
// or "" if the branch does not exist.
func branchTipSHA(t *testing.T, repoRoot, branchName string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", branchName).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestPreserveWorktreeCommits_FailedCycle_CreatesPreserveBranch(t *testing.T) {
	runID := "abcd1234ef56"
	repoRoot, worktreePath, wantSHA := setupRepoWithWorktree(t, runID, true)

	preserved, err := preserveWorktreeCommits(repoRoot, worktreePath, runID)
	if err != nil {
		t.Fatalf("preserveWorktreeCommits: %v", err)
	}
	if preserved != wantSHA {
		t.Errorf("preserved sha = %q, want %q", preserved, wantSHA)
	}

	branchName := "codex/preserve-" + runID
	got := branchTipSHA(t, repoRoot, branchName)
	if got != wantSHA {
		t.Errorf("branch %q tip = %q, want %q", branchName, got, wantSHA)
	}
}

func TestPreserveWorktreeCommits_NoNewCommits_NoBranchCreated(t *testing.T) {
	runID := "ffeeddccbbaa"
	repoRoot, worktreePath, _ := setupRepoWithWorktree(t, runID, false)

	preserved, err := preserveWorktreeCommits(repoRoot, worktreePath, runID)
	if err != nil {
		t.Fatalf("preserveWorktreeCommits: %v", err)
	}
	if preserved != "" {
		t.Errorf("expected empty preserved sha when no extra commits; got %q", preserved)
	}

	branchName := "codex/preserve-" + runID
	if got := branchTipSHA(t, repoRoot, branchName); got != "" {
		t.Errorf("expected no preserve branch; got tip %q", got)
	}
}

func TestPreserveWorktreeCommits_EmptyRunID_Errors(t *testing.T) {
	repoRoot, worktreePath, _ := setupRepoWithWorktree(t, "tmprid", true)
	if _, err := preserveWorktreeCommits(repoRoot, worktreePath, ""); err == nil {
		t.Error("expected error for empty runID; preserve branch name must be deterministic")
	}
}

// removeOrphanedWorktree must call preserveWorktreeCommits before deleting the
// worktree, so failed-cycle commits aren't orphaned (soc-ewne acceptance).
func TestRemoveOrphanedWorktree_PreservesCommitsBeforeRemoval(t *testing.T) {
	runID := "cleanuptest1"
	repoRoot, worktreePath, wantSHA := setupRepoWithWorktree(t, runID, true)

	if err := removeOrphanedWorktree(repoRoot, worktreePath, runID); err != nil {
		t.Fatalf("removeOrphanedWorktree: %v", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree still exists after cleanup: %v", err)
	}

	branchName := "codex/preserve-" + runID
	if got := branchTipSHA(t, repoRoot, branchName); got != wantSHA {
		t.Errorf("preserve branch tip = %q, want %q", got, wantSHA)
	}
}
