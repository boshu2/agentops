// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStateMirrorWrite verifies that savePhasedState writes the state file to
// the mirror root (supervisor) when running from an isolated worktree path.
func TestStateMirrorWrite(t *testing.T) {
	parent := t.TempDir()
	runID := "mir-state-01"
	supervisor := filepath.Join(parent, "repo")
	worktree := supervisor + "-rpi-" + runID

	for _, dir := range []string{supervisor, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	state := &phasedState{
		SchemaVersion: 1,
		RunID:         runID,
		WorktreePath:  worktree,
		Goal:          "mirror state test",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
		Opts:          defaultPhasedEngineOptions(),
	}

	if err := savePhasedState(worktree, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Verify state file exists in the worktree (primary root).
	wtFlat := filepath.Join(worktree, ".agents", "rpi", phasedStateFile)
	if _, err := os.Stat(wtFlat); err != nil {
		t.Errorf("worktree flat state file not found: %v", err)
	}

	// Verify state file was mirrored to the supervisor root.
	svFlat := filepath.Join(supervisor, ".agents", "rpi", phasedStateFile)
	if _, err := os.Stat(svFlat); err != nil {
		t.Errorf("supervisor flat state file not found: %v", err)
	}

	svRegistry := filepath.Join(supervisor, ".agents", "rpi", "runs", runID, phasedStateFile)
	if _, err := os.Stat(svRegistry); err != nil {
		t.Errorf("supervisor registry state file not found: %v", err)
	}

	// Verify a state.mirrored C2 event was emitted.
	events, err := loadRPIC2Events(worktree, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.Type == "state.mirrored" {
			found = true
			var details map[string]any
			if err := json.Unmarshal(ev.Details, &details); err != nil {
				t.Fatalf("unmarshal state.mirrored details: %v", err)
			}
			if mr, ok := details["mirror_root"].(string); !ok || mr == "" {
				t.Errorf("state.mirrored event missing mirror_root in details")
			}
			if f, ok := details["file"].(string); !ok || f != phasedStateFile {
				t.Errorf("state.mirrored event file = %q, want %q", f, phasedStateFile)
			}
			break
		}
	}
	if !found {
		t.Errorf("no state.mirrored event found in C2 events log")
	}
}

// TestStateMirrorSkipsSameRoot verifies that the primary root is not double-
// written by mirrorStateToPeers (the primary is already written by the main
// savePhasedState loop).
func TestStateMirrorSkipsSameRoot(t *testing.T) {
	tmp := t.TempDir()
	runID := "mir-skip-01"

	state := &phasedState{
		SchemaVersion: 1,
		RunID:         runID,
		Goal:          "no-double-write",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
		Opts:          defaultPhasedEngineOptions(),
	}

	if err := savePhasedState(tmp, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// When there is no worktree path, mirrorRootsForEvent returns only the
	// primary root (or the same root). mirrorStateToPeers should skip it,
	// so no state.mirrored event should be emitted.
	events, err := loadRPIC2Events(tmp, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}

	for _, ev := range events {
		if ev.Type == "state.mirrored" || ev.Type == "state.mirror.failed" {
			t.Errorf("unexpected mirror event %q when no mirror root exists", ev.Type)
		}
	}
}

// TestStateMirrorFailureEvent verifies that a state.mirror.failed C2 event is
// emitted when the mirror root is not writable.
func TestStateMirrorFailureEvent(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}

	parent := t.TempDir()
	runID := "mir-fail-01"
	supervisor := filepath.Join(parent, "repo")
	worktree := supervisor + "-rpi-" + runID

	for _, dir := range []string{supervisor, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	state := &phasedState{
		SchemaVersion: 1,
		RunID:         runID,
		WorktreePath:  worktree,
		Goal:          "mirror failure test",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
		Opts:          defaultPhasedEngineOptions(),
	}

	// First save to seed the state in the worktree (so mirrorRootsForEvent
	// can find the supervisor root via artifactRootsForRun).
	if err := savePhasedState(worktree, state); err != nil {
		t.Fatalf("initial savePhasedState: %v", err)
	}

	// Make the supervisor directory read-only so mirror writes fail.
	supervisorAgents := filepath.Join(supervisor, ".agents")
	if err := os.Chmod(supervisorAgents, 0o500); err != nil {
		t.Fatalf("chmod supervisor .agents: %v", err)
	}
	// Also lock the inner directories so writes truly fail.
	supervisorRPI := filepath.Join(supervisorAgents, "rpi")
	_ = os.Chmod(supervisorRPI, 0o500)
	supervisorRuns := filepath.Join(supervisorRPI, "runs")
	_ = os.Chmod(supervisorRuns, 0o500)
	supervisorRunDir := filepath.Join(supervisorRuns, runID)
	_ = os.Chmod(supervisorRunDir, 0o500)

	defer func() {
		// Restore permissions so t.TempDir cleanup works.
		_ = os.Chmod(supervisorRunDir, 0o750)
		_ = os.Chmod(supervisorRuns, 0o750)
		_ = os.Chmod(supervisorRPI, 0o750)
		_ = os.Chmod(supervisorAgents, 0o750)
	}()

	// Second save should succeed for primary but fail for mirror.
	state.Phase = 2
	if err := savePhasedState(worktree, state); err != nil {
		t.Fatalf("savePhasedState should succeed for primary root: %v", err)
	}

	// Look for a state.mirror.failed event in the worktree C2 events.
	events, err := loadRPIC2Events(worktree, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.Type == "state.mirror.failed" {
			found = true
			var details map[string]any
			if err := json.Unmarshal(ev.Details, &details); err != nil {
				t.Fatalf("unmarshal state.mirror.failed details: %v", err)
			}
			if mr, ok := details["mirror_root"].(string); !ok || mr == "" {
				t.Errorf("state.mirror.failed event missing mirror_root")
			}
			if errMsg, ok := details["error"].(string); !ok || !strings.Contains(errMsg, "permission denied") {
				t.Errorf("state.mirror.failed error = %q, want something containing 'permission denied'", errMsg)
			}
			break
		}
	}
	if !found {
		// Dump all event types for debugging.
		types := make([]string, len(events))
		for i, ev := range events {
			types[i] = ev.Type
		}
		t.Errorf("no state.mirror.failed event found; event types: %v", types)
	}
}
