// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRPIC2EventAppendAndLoad(t *testing.T) {
	root := t.TempDir()
	runID := "run-c2-001"

	if _, err := appendRPIC2Event(root, rpiC2EventInput{
		RunID:   runID,
		Phase:   2,
		Backend: "stream",
		Source:  "runtime_stream",
		Type:    "phase.stream.started",
		Message: "started",
		Details: map[string]any{"attempt": 1},
	}); err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Type != "phase.stream.started" {
		t.Fatalf("event type = %q", events[0].Type)
	}
	if events[0].RunID != runID {
		t.Fatalf("run_id = %q", events[0].RunID)
	}
}

func TestRPIC2Event_RequiresRunAndType(t *testing.T) {
	root := t.TempDir()
	if _, err := appendRPIC2Event(root, rpiC2EventInput{Type: "x"}); err == nil {
		t.Fatal("expected error for missing run_id")
	}
	if _, err := appendRPIC2Event(root, rpiC2EventInput{RunID: "run"}); err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestAppendRPIC2WorkerLogEvents(t *testing.T) {
	root := t.TempDir()
	runID := "run-c2-worker"
	runDir := rpiRunRegistryDir(root, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	logPath := filepath.Join(runDir, "phase-1-exit.w1.jsonl")
	payload := "{\"type\":\"rpi_worker_start\",\"worker\":\"1\"}\nplain line\n"
	if err := os.WriteFile(logPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if err := appendRPIC2WorkerLogEvents(root, runID, 1, "tmux", "1", logPath); err != nil {
		t.Fatalf("appendRPIC2WorkerLogEvents: %v", err)
	}

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Type != "worker.rpi_worker_start" {
		t.Fatalf("first event type = %q", events[0].Type)
	}
	if events[1].Type != "worker.log" {
		t.Fatalf("second event type = %q", events[1].Type)
	}
}

func TestMapStreamEventToRPIC2(t *testing.T) {
	input := mapStreamEventToRPIC2("run-1", 3, StreamEvent{
		Type:      EventTypeAssistant,
		Subtype:   "tool_use",
		ToolName:  "Read",
		Message:   "hello",
		SessionID: "sess-1",
	})
	if input.Type != "stream.assistant" {
		t.Fatalf("type = %q", input.Type)
	}
	if input.RunID != "run-1" {
		t.Fatalf("run_id = %q", input.RunID)
	}
	if input.Backend != "stream" {
		t.Fatalf("backend = %q", input.Backend)
	}
}

func TestRPIC2EventAppend_MirrorsToSupervisorRoot(t *testing.T) {
	parent := t.TempDir()
	runID := "a1b2c3d4"
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
		Goal:          "test goal",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
	}
	if err := savePhasedState(worktree, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	ev, err := appendRPIC2Event(worktree, rpiC2EventInput{
		RunID:   runID,
		Phase:   1,
		Backend: "stream",
		Source:  "test",
		Type:    "phase.stream.started",
		Message: "mirrored",
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}

	wtEvents, err := loadRPIC2Events(worktree, runID)
	if err != nil {
		t.Fatalf("load worktree events: %v", err)
	}
	// savePhasedState now emits a state.mirrored event, so expect 2 events
	// (state.mirrored + phase.stream.started).
	if len(wtEvents) < 1 {
		t.Fatalf("worktree len(events) = %d, want >= 1", len(wtEvents))
	}

	supervisorEvents, err := loadRPIC2Events(supervisor, runID)
	if err != nil {
		t.Fatalf("load supervisor events: %v", err)
	}
	if len(supervisorEvents) < 1 {
		t.Fatalf("supervisor len(events) = %d, want >= 1", len(supervisorEvents))
	}
	// Find the mirrored phase.stream.started event by ID.
	found := false
	for _, sev := range supervisorEvents {
		if sev.EventID == ev.EventID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mirrored event_id %q not found in supervisor events", ev.EventID)
	}
}

func TestRPIC2EventAppend_MirrorFailureIsNonFatal(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses filesystem permissions")
	}

	parent := t.TempDir()
	runID := "b4c5d6e7"
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
		Goal:          "test goal",
		Phase:         1,
		StartPhase:    1,
		Cycle:         1,
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
	}
	if err := savePhasedState(worktree, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	mirrorRunDir := rpiRunRegistryDir(supervisor, runID)
	if err := os.Chmod(mirrorRunDir, 0o500); err != nil {
		t.Fatalf("chmod mirror run dir: %v", err)
	}
	defer func() {
		_ = os.Chmod(mirrorRunDir, 0o750)
	}()

	if _, err := appendRPIC2Event(worktree, rpiC2EventInput{
		RunID:   runID,
		Phase:   1,
		Backend: "stream",
		Source:  "test",
		Type:    "phase.stream.started",
		Message: "primary still writes",
	}); err != nil {
		t.Fatalf("appendRPIC2Event should succeed when mirror write fails: %v", err)
	}

	wtEvents, err := loadRPIC2Events(worktree, runID)
	if err != nil {
		t.Fatalf("load worktree events: %v", err)
	}
	// savePhasedState emits a state.mirrored event before chmod, plus the
	// phase.stream.started event, so expect >= 1.
	if len(wtEvents) < 1 {
		t.Fatalf("worktree len(events) = %d, want >= 1", len(wtEvents))
	}
	// The phase.stream.started event must still be recorded in the worktree.
	foundPrimary := false
	for _, ev := range wtEvents {
		if ev.Type == "phase.stream.started" {
			foundPrimary = true
			break
		}
	}
	if !foundPrimary {
		t.Fatal("phase.stream.started event not found in worktree events after mirror failure")
	}
}
