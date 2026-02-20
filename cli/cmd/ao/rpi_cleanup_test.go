package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "stale-run",
		"goal":           "test stale",
		"phase":          2,
		"worktree_path":  "/nonexistent/path",
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	statePath := filepath.Join(runDir, phasedStateFile)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	if len(staleRuns) != 1 {
		t.Fatalf("expected 1 stale run, got %d", len(staleRuns))
	}
	if staleRuns[0].runID != "stale-run" {
		t.Errorf("expected stale-run, got %s", staleRuns[0].runID)
	}
	if staleRuns[0].reason != "worktree missing" {
		t.Errorf("expected reason 'worktree missing', got %q", staleRuns[0].reason)
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
	var updatedState map[string]interface{}
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
		if sr.runID == "active-run" {
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
	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "dry-run-test",
		"goal":           "dry run test",
		"phase":          1,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
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
	var originalState map[string]interface{}
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
		state := map[string]interface{}{
			"schema_version": 1,
			"run_id":         id,
			"goal":           id + " goal",
			"phase":          1,
			"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		}
		data, _ := json.Marshal(state)
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
		if sr.runID == "target-run" {
			filtered = append(filtered, sr)
		}
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered run, got %d", len(filtered))
	}
	if filtered[0].runID != "target-run" {
		t.Errorf("expected target-run, got %s", filtered[0].runID)
	}
}

func TestCleanupSkipsTerminalRuns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a run that already has terminal metadata.
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "already-stale")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]interface{}{
		"schema_version":  1,
		"run_id":          "already-stale",
		"goal":            "already marked",
		"phase":           1,
		"terminal_status": "stale",
		"terminal_reason": "previously marked",
		"terminated_at":   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"started_at":      time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	for _, sr := range staleRuns {
		if sr.runID == "already-stale" {
			t.Fatal("run with existing terminal_status should be skipped")
		}
	}
}

func TestCleanupSkipsCompletedRuns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a completed run (phase 3, schema v1 = terminal).
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "done-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "done-run",
		"goal":           "completed",
		"phase":          3,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	staleRuns := findStaleRuns(tmpDir)
	for _, sr := range staleRuns {
		if sr.runID == "done-run" {
			t.Fatal("completed run should not be detected as stale")
		}
	}
}
