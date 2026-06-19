package rpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeRegistryRun writes a phased-state.json under <root>/.agents/rpi/runs/<id>/.
func writeRegistryRun(t *testing.T, root, runID string, state map[string]any) {
	t.Helper()
	dir := filepath.Join(root, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phased-state.json"), data, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func TestScanRegistryRuns_ParsesRegistryFields(t *testing.T) {
	root := t.TempDir()
	writeRegistryRun(t, root, "run-abc", map[string]any{
		"run_id":          "run-abc",
		"goal":            "ship the thing",
		"epic_id":         "age-xyz",
		"schema_version":  2,
		"phase":           3,
		"started_at":      "2026-01-01T00:00:00Z",
		"terminal_status": "completed",
		"worktree_path":   filepath.Join(root, "wt-gone"), // does not exist
	})

	runs := ScanRegistryRuns(root)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %+v", len(runs), runs)
	}
	got := runs[0]
	if got.RunID != "run-abc" || got.Goal != "ship the thing" || got.EpicID != "age-xyz" {
		t.Fatalf("registry fields not parsed: %+v", got)
	}
	// A vanished worktree_path forces not-active liveness.
	if got.IsActive {
		t.Fatalf("run with a missing worktree_path should be inactive, got active: %+v", got)
	}
}

func TestScanRegistryRuns_SkipsInvalidAndMissing(t *testing.T) {
	root := t.TempDir()
	// Missing run_id -> skipped.
	writeRegistryRun(t, root, "run-noID", map[string]any{"goal": "no id"})
	// Non-JSON file -> skipped.
	badDir := filepath.Join(root, ".agents", "rpi", "runs", "run-bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "phased-state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if runs := ScanRegistryRuns(root); len(runs) != 0 {
		t.Fatalf("expected 0 valid runs, got %d: %+v", len(runs), runs)
	}
	// Absent registry dir -> nil, no panic.
	if runs := ScanRegistryRuns(t.TempDir()); runs != nil {
		t.Fatalf("expected nil for absent registry, got %+v", runs)
	}
}

func TestDetermineRunLiveness_VanishedWorktreeIsInactive(t *testing.T) {
	cwd := t.TempDir()
	active, _ := DetermineRunLiveness(cwd, "run-1", filepath.Join(cwd, "does-not-exist"))
	if active {
		t.Fatal("a run whose worktree_path no longer exists must be inactive")
	}
}

func TestDiscoverRuns_EmptyTreeReturnsNoRuns(t *testing.T) {
	if runs := DiscoverRuns(t.TempDir()); len(runs) != 0 {
		t.Fatalf("expected no runs in an empty tree, got %+v", runs)
	}
}
