// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunArtifactsMirrorToSupervisorRepo(t *testing.T) {
	repo := initTestRepo(t)

	worktreePath, runID, err := createWorktree(repo)
	if err != nil {
		t.Fatalf("createWorktree: %v", err)
	}
	defer func() {
		cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
		cmd.Dir = repo
		_ = cmd.Run()
	}()

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "ag-4ca",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		RunID:         runID,
		WorktreePath:  worktreePath,
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
	}
	if err := savePhasedState(worktreePath, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	updateRunHeartbeat(worktreePath, runID)

	for _, root := range []string{worktreePath, repo} {
		flatState := filepath.Join(root, ".agents", "rpi", phasedStateFile)
		if _, err := os.Stat(flatState); err != nil {
			t.Fatalf("state file missing at %s: %v", flatState, err)
		}

		registryState := filepath.Join(root, ".agents", "rpi", "runs", runID, phasedStateFile)
		if _, err := os.Stat(registryState); err != nil {
			t.Fatalf("registry state missing at %s: %v", registryState, err)
		}

		heartbeat := filepath.Join(root, ".agents", "rpi", "runs", runID, "heartbeat.txt")
		if _, err := os.Stat(heartbeat); err != nil {
			t.Fatalf("heartbeat missing at %s: %v", heartbeat, err)
		}
	}
}
