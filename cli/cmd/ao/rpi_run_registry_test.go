// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSavePhasedState_Atomic verifies that savePhasedState writes atomically
// (no corrupt partial write) and produces a valid JSON file at the flat path.
func TestSavePhasedState_Atomic(t *testing.T) {
	dir := t.TempDir()

	state := newTestPhasedState().
		WithGoal("atomic write test").
		WithEpicID("ag-atm1").
		WithPhase(2).
		WithCycle(1).
		WithRunID("deadbeef").
		WithVerdict("pre_mortem", "PASS").
		WithAttempt("phase_1", 0)

	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	flatPath := filepath.Join(dir, ".agents", "rpi", phasedStateFile)
	data, err := os.ReadFile(flatPath)
	if err != nil {
		t.Fatalf("read flat state: %v", err)
	}

	// Verify content is valid JSON with expected fields.
	loaded, err := parsePhasedState(data)
	if err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if loaded.Goal != state.Goal {
		t.Errorf("Goal: got %q, want %q", loaded.Goal, state.Goal)
	}
	if loaded.EpicID != state.EpicID {
		t.Errorf("EpicID: got %q, want %q", loaded.EpicID, state.EpicID)
	}
	if loaded.RunID != state.RunID {
		t.Errorf("RunID: got %q, want %q", loaded.RunID, state.RunID)
	}

	// Verify no .tmp file remains after successful write.
	entries, err := os.ReadDir(filepath.Join(dir, ".agents", "rpi"))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", entry.Name())
		}
	}
}

// TestSavePhasedState_WritesRunRegistry verifies that savePhasedState creates
// .agents/rpi/runs/<run-id>/phased-state.json when RunID is set.
func TestSavePhasedState_WritesRunRegistry(t *testing.T) {
	dir := t.TempDir()
	runID := "cafe1234"

	state := newTestPhasedState().
		WithGoal("registry write test").
		WithPhase(1).
		WithCycle(1).
		WithRunID(runID)

	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Verify the per-run directory and state file exist.
	registryDir := rpiRunRegistryDir(dir, runID)
	registryStatePath := filepath.Join(registryDir, phasedStateFile)

	data, err := os.ReadFile(registryStatePath)
	if err != nil {
		t.Fatalf("run registry state not written at %s: %v", registryStatePath, err)
	}

	loaded, err := parsePhasedState(data)
	if err != nil {
		t.Fatalf("parse registry state: %v", err)
	}
	if loaded.RunID != runID {
		t.Errorf("RunID: got %q, want %q", loaded.RunID, runID)
	}
	if loaded.Goal != state.Goal {
		t.Errorf("Goal: got %q, want %q", loaded.Goal, state.Goal)
	}
}

// TestSavePhasedState_NoRunID verifies that savePhasedState skips the registry
// write when RunID is empty (backward compatibility path).
func TestSavePhasedState_NoRunID(t *testing.T) {
	dir := t.TempDir()

	state := newTestPhasedState().
		WithGoal("no run id").
		WithPhase(1).
		WithCycle(1)

	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Flat path should be written.
	flatPath := filepath.Join(dir, ".agents", "rpi", phasedStateFile)
	if _, err := os.Stat(flatPath); err != nil {
		t.Fatalf("flat state not written: %v", err)
	}

	// runs/ directory should not be created.
	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")
	if _, err := os.Stat(runsDir); err == nil {
		t.Error("runs/ directory should not exist when RunID is empty")
	}
}

// TestLoadPhasedState_FlatPath verifies that loadPhasedState reads the flat
// phased-state.json when no run registry directory exists.
func TestLoadPhasedState_FlatPath(t *testing.T) {
	dir := t.TempDir()

	state := newTestPhasedState().
		WithGoal("load flat test").
		WithPhase(3).
		WithCycle(1).
		WithVerdict("vibe", "PASS")

	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	loaded, err := loadPhasedState(dir)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}
	if loaded.Goal != state.Goal {
		t.Errorf("Goal: got %q, want %q", loaded.Goal, state.Goal)
	}
	if loaded.Phase != state.Phase {
		t.Errorf("Phase: got %d, want %d", loaded.Phase, state.Phase)
	}
	if loaded.Verdicts["vibe"] != "PASS" {
		t.Errorf("Verdicts: got %v", loaded.Verdicts)
	}
}

// TestLoadPhasedState_RunRegistryFallback verifies that loadPhasedState falls
// back to the run registry when the flat file is older than the registry file.
func TestLoadPhasedState_RunRegistryFallback(t *testing.T) {
	dir := t.TempDir()
	runID := "aabbccdd"

	// Write old flat state (phase 1).
	oldState := newTestPhasedState().
		WithGoal("old goal").
		WithPhase(1).
		WithCycle(1).
		WithRunID(runID)
	if err := savePhasedState(dir, oldState); err != nil {
		t.Fatalf("savePhasedState (old): %v", err)
	}

	// Ensure some time passes so mtime is different.
	time.Sleep(10 * time.Millisecond)

	// Write newer registry state (phase 2) directly to the registry path.
	runDir := rpiRunRegistryDir(dir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	newState := newTestPhasedState().
		WithGoal("new goal from registry").
		WithPhase(2).
		WithCycle(1).
		WithRunID(runID).
		WithVerdict("pre_mortem", "PASS").
		WithAttempt("phase_1", 1)
	registryPath := filepath.Join(runDir, phasedStateFile)
	// Write via savePhasedState so it uses atomic write.
	if err := savePhasedState(dir, newState); err != nil {
		t.Fatalf("savePhasedState (new): %v", err)
	}
	// Touch the registry file to make it definitely newer.
	now := time.Now().Add(100 * time.Millisecond)
	if err := os.Chtimes(registryPath, now, now); err != nil {
		t.Fatalf("chtimes registry: %v", err)
	}

	loaded, err := loadPhasedState(dir)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}
	// Should have loaded the newer registry state.
	if loaded.Phase != 2 {
		t.Errorf("Phase: got %d, want 2 (registry state)", loaded.Phase)
	}
	if loaded.Verdicts["pre_mortem"] != "PASS" {
		t.Errorf("Verdicts: got %v", loaded.Verdicts)
	}
}

// TestLoadPhasedState_MapsNeverNil verifies that loadPhasedState initializes
// Verdicts and Attempts maps even when missing from JSON.
func TestLoadPhasedState_MapsNeverNil(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write minimal JSON without maps.
	minimal := `{"schema_version":1,"goal":"minimal","phase":1,"cycle":1,"started_at":"2026-02-19T00:00:00Z"}`
	flatPath := filepath.Join(stateDir, phasedStateFile)
	if err := os.WriteFile(flatPath, []byte(minimal), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadPhasedState(dir)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}
	if loaded.Verdicts == nil {
		t.Error("Verdicts should not be nil")
	}
	if loaded.Attempts == nil {
		t.Error("Attempts should not be nil")
	}
	// Should be able to write to maps without panic.
	loaded.Verdicts["test"] = "PASS"
	loaded.Attempts["phase_1"] = 1
}

func TestParsePhasedState_DefaultsStartPhaseToPhase(t *testing.T) {
	stateJSON := `{"schema_version":1,"goal":"x","phase":2,"cycle":1,"started_at":"2026-02-19T00:00:00Z"}`
	loaded, err := parsePhasedState([]byte(stateJSON))
	if err != nil {
		t.Fatalf("parsePhasedState: %v", err)
	}
	if loaded.StartPhase != loaded.Phase {
		t.Fatalf("StartPhase: got %d, want %d", loaded.StartPhase, loaded.Phase)
	}
}

func TestParsePhasedState_NormalizesLegacyZeroValues(t *testing.T) {
	stateJSON := `{"schema_version":1,"goal":"x","phase":0,"start_phase":9,"cycle":0,"started_at":"2026-02-19T00:00:00Z"}`
	loaded, err := parsePhasedState([]byte(stateJSON))
	if err != nil {
		t.Fatalf("parsePhasedState: %v", err)
	}
	if loaded.Phase != 1 {
		t.Fatalf("Phase: got %d, want 1", loaded.Phase)
	}
	if loaded.Cycle != 1 {
		t.Fatalf("Cycle: got %d, want 1", loaded.Cycle)
	}
	if loaded.StartPhase != loaded.Phase {
		t.Fatalf("StartPhase: got %d, want %d", loaded.StartPhase, loaded.Phase)
	}
}

func TestParsePhasedState_NormalizesMissingGoal(t *testing.T) {
	stateJSON := `{"schema_version":1,"phase":1,"cycle":1,"started_at":"2026-02-19T00:00:00Z"}`
	loaded, err := parsePhasedState([]byte(stateJSON))
	if err != nil {
		t.Fatalf("parsePhasedState: %v", err)
	}
	if loaded.Goal != "unknown-goal" {
		t.Fatalf("Goal: got %q, want unknown-goal", loaded.Goal)
	}
}

// TestRunRegistry_DirectoryLayout verifies that savePhasedState creates the
// expected .agents/rpi/runs/<run-id>/ directory layout (acceptance criterion:
// content check for "runs/").
func TestRunRegistry_DirectoryLayout(t *testing.T) {
	dir := t.TempDir()
	runID := "12345678"

	state := newTestPhasedState().
		WithGoal("registry layout test").
		WithPhase(1).
		WithCycle(1).
		WithRunID(runID)

	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Verify directory structure: .agents/rpi/runs/<run-id>/
	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")
	runDir := filepath.Join(runsDir, runID)

	if _, err := os.Stat(runsDir); err != nil {
		t.Fatalf("runs/ directory not created: %v", err)
	}
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("runs/%s/ directory not created: %v", runID, err)
	}

	statePath := filepath.Join(runDir, phasedStateFile)
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("runs/%s/%s not created: %v", runID, phasedStateFile, err)
	}

	// Validate the file contains valid JSON with the run ID.
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read registry state: %v", err)
	}
	loaded, err := parsePhasedState(data)
	if err != nil {
		t.Fatalf("parse registry state: %v", err)
	}
	if loaded.RunID != runID {
		t.Errorf("registry RunID: got %q, want %q", loaded.RunID, runID)
	}
}

// TestRunRegistry_MultipleRuns verifies that multiple runs create separate
// directories without interfering with each other.
func TestRunRegistry_MultipleRuns(t *testing.T) {
	dir := t.TempDir()

	for _, runID := range []string{"run0001", "run0002", "run0003"} {
		state := newTestPhasedState().
			WithGoal("goal for " + runID).
			WithPhase(1).
			WithCycle(1).
			WithRunID(runID)
		if err := savePhasedState(dir, state); err != nil {
			t.Fatalf("savePhasedState(%s): %v", runID, err)
		}
	}

	// Each run should have its own directory.
	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("ReadDir runs/: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 run directories, got %d", len(entries))
	}

	for _, runID := range []string{"run0001", "run0002", "run0003"} {
		statePath := filepath.Join(runsDir, runID, phasedStateFile)
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("read %s state: %v", runID, err)
		}
		loaded, err := parsePhasedState(data)
		if err != nil {
			t.Fatalf("parse %s state: %v", runID, err)
		}
		if loaded.RunID != runID {
			t.Errorf("%s: RunID mismatch: got %q", runID, loaded.RunID)
		}
		if loaded.Goal != "goal for "+runID {
			t.Errorf("%s: Goal mismatch: got %q", runID, loaded.Goal)
		}
	}
}

// TestRunRegistry_SurvivesInterruption simulates an interrupted write by
// verifying that a corrupt .tmp file does not break run recovery.
func TestRunRegistry_SurvivesInterruption(t *testing.T) {
	dir := t.TempDir()
	runID := "deadcafe"

	// Write a valid state first.
	state := newTestPhasedState().
		WithGoal("interruption test").
		WithPhase(2).
		WithCycle(1).
		WithRunID(runID).
		WithVerdict("pre_mortem", "PASS")
	if err := savePhasedState(dir, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Leave a corrupt .tmp file in the state directory to simulate an
	// interrupted write. This is what would happen if the process crashed
	// after creating the tmp file but before the rename.
	stateDir := filepath.Join(dir, ".agents", "rpi")
	corruptTmp := filepath.Join(stateDir, ".phased-state-CORRUPT.json.tmp")
	if err := os.WriteFile(corruptTmp, []byte("not-json{{{{"), 0644); err != nil {
		t.Fatalf("write corrupt tmp: %v", err)
	}

	// Also leave a corrupt tmp in the run directory.
	runDir := rpiRunRegistryDir(dir, runID)
	corruptRunTmp := filepath.Join(runDir, ".phased-state-CORRUPT.json.tmp")
	if err := os.WriteFile(corruptRunTmp, []byte("not-json{{{{"), 0644); err != nil {
		t.Fatalf("write corrupt run tmp: %v", err)
	}

	// loadPhasedState should succeed because it reads the final renamed file.
	loaded, err := loadPhasedState(dir)
	if err != nil {
		t.Fatalf("loadPhasedState after simulated interruption: %v", err)
	}
	if loaded.Goal != state.Goal {
		t.Errorf("Goal: got %q, want %q", loaded.Goal, state.Goal)
	}
	if loaded.Phase != state.Phase {
		t.Errorf("Phase: got %d, want %d", loaded.Phase, state.Phase)
	}
}

// TestHeartbeat_UpdateAndRead verifies that updateRunHeartbeat writes a valid
// RFC3339Nano timestamp and readRunHeartbeat parses it correctly.
func TestHeartbeat_UpdateAndRead(t *testing.T) {
	dir := t.TempDir()
	runID := "hb000001"

	before := time.Now().UTC().Truncate(time.Second)
	updateRunHeartbeat(dir, runID)
	after := time.Now().UTC().Add(time.Second)

	ts := readRunHeartbeat(dir, runID)
	if ts.IsZero() {
		t.Fatal("readRunHeartbeat returned zero time after updateRunHeartbeat")
	}
	if ts.Before(before) {
		t.Errorf("heartbeat timestamp %v is before test start %v", ts, before)
	}
	if ts.After(after) {
		t.Errorf("heartbeat timestamp %v is after test end %v", ts, after)
	}
}

// TestHeartbeat_CreatesRunDir verifies that updateRunHeartbeat creates the
// run registry directory if it does not exist.
func TestHeartbeat_CreatesRunDir(t *testing.T) {
	dir := t.TempDir()
	runID := "hb000002"

	// Run directory should not exist yet.
	runDir := rpiRunRegistryDir(dir, runID)
	if _, err := os.Stat(runDir); err == nil {
		t.Fatal("run directory should not exist before updateRunHeartbeat")
	}

	updateRunHeartbeat(dir, runID)

	// Verify heartbeat file exists.
	heartbeatPath := filepath.Join(runDir, "heartbeat.txt")
	if _, err := os.Stat(heartbeatPath); err != nil {
		t.Fatalf("heartbeat.txt not created: %v", err)
	}
}

// TestHeartbeat_EmptyRunID verifies that updateRunHeartbeat is a no-op when
// runID is empty.
func TestHeartbeat_EmptyRunID(t *testing.T) {
	dir := t.TempDir()

	// Should not panic or create any files.
	updateRunHeartbeat(dir, "")

	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")
	if _, err := os.Stat(runsDir); err == nil {
		t.Error("runs/ directory should not be created for empty runID")
	}
}

// TestHeartbeat_AtomicWrite verifies that updateRunHeartbeat leaves no .tmp
// files and produces a valid timestamp file.
func TestHeartbeat_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	runID := "hb000003"

	updateRunHeartbeat(dir, runID)

	runDir := rpiRunRegistryDir(dir, runID)
	entries, err := os.ReadDir(runDir)
	if err != nil {
		t.Fatalf("ReadDir run dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", entry.Name())
		}
	}

	// Heartbeat content should be a parseable RFC3339Nano timestamp.
	heartbeatPath := filepath.Join(runDir, "heartbeat.txt")
	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if _, err := time.Parse(time.RFC3339Nano, trimmed); err != nil {
		t.Errorf("heartbeat content is not RFC3339Nano: %q: %v", trimmed, err)
	}
}

// TestHeartbeat_MultipleUpdates verifies that repeated heartbeat updates
// produce monotonically non-decreasing timestamps.
func TestHeartbeat_MultipleUpdates(t *testing.T) {
	dir := t.TempDir()
	runID := "hb000004"

	var timestamps []time.Time
	for i := range 3 {
		updateRunHeartbeat(dir, runID)
		ts := readRunHeartbeat(dir, runID)
		if ts.IsZero() {
			t.Fatalf("iteration %d: got zero time", i)
		}
		timestamps = append(timestamps, ts)
		time.Sleep(2 * time.Millisecond)
	}

	for i := 1; i < len(timestamps); i++ {
		if timestamps[i].Before(timestamps[i-1]) {
			t.Errorf("timestamps not monotonic: %v before %v", timestamps[i], timestamps[i-1])
		}
	}
}

// TestReadRunHeartbeat_Missing verifies that readRunHeartbeat returns zero time
// when the heartbeat file does not exist.
func TestReadRunHeartbeat_Missing(t *testing.T) {
	dir := t.TempDir()
	ts := readRunHeartbeat(dir, "no-such-run")
	if !ts.IsZero() {
		t.Errorf("expected zero time for missing heartbeat, got %v", ts)
	}
}

// TestRPIRunRegistryDir verifies the path construction helper.
func TestRPIRunRegistryDir(t *testing.T) {
	// Non-empty runID.
	got := rpiRunRegistryDir("/tmp/myrepo", "abc123")
	want := "/tmp/myrepo/.agents/rpi/runs/abc123"
	if got != want {
		t.Errorf("rpiRunRegistryDir: got %q, want %q", got, want)
	}

	// Empty runID should return empty string.
	got = rpiRunRegistryDir("/tmp/myrepo", "")
	if got != "" {
		t.Errorf("rpiRunRegistryDir with empty runID: got %q, want empty", got)
	}
}
