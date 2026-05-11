// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------- writePhaseResult / validatePriorPhaseResult ----------

func TestState_WritePhaseResult_RoundTrip(t *testing.T) {
	tmp := t.TempDir()

	want := &phaseResult{
		SchemaVersion:   1,
		RunID:           "abc123",
		Phase:           2,
		PhaseName:       "implementation",
		Status:          "completed",
		Retries:         1,
		Backend:         "claude",
		Artifacts:       map[string]string{"plan": ".agents/plans/plan.md"},
		Verdicts:        map[string]string{"pre-mortem": "PASS"},
		StartedAt:       time.Now().UTC().Format(time.RFC3339),
		CompletedAt:     time.Now().UTC().Format(time.RFC3339),
		DurationSeconds: 42.5,
	}

	if err := writePhaseResult(tmp, want); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".agents", "rpi", "phase-2-result.json"))
	if err != nil {
		t.Fatalf("read result file: %v", err)
	}

	var got phaseResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.RunID != want.RunID {
		t.Errorf("RunID = %q, want %q", got.RunID, want.RunID)
	}
	if got.Phase != want.Phase {
		t.Errorf("Phase = %d, want %d", got.Phase, want.Phase)
	}
	if got.Status != want.Status {
		t.Errorf("Status = %q, want %q", got.Status, want.Status)
	}
	if got.DurationSeconds != want.DurationSeconds {
		t.Errorf("DurationSeconds = %f, want %f", got.DurationSeconds, want.DurationSeconds)
	}
	if got.Verdicts["pre-mortem"] != "PASS" {
		t.Errorf("Verdicts[pre-mortem] = %q, want %q", got.Verdicts["pre-mortem"], "PASS")
	}
}

func TestState_WritePhaseResult_CreatesDir(t *testing.T) {
	tmp := t.TempDir()

	result := &phaseResult{Phase: 1, PhaseName: "discovery", Status: "completed"}
	if err := writePhaseResult(tmp, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	dir := filepath.Join(tmp, ".agents", "rpi")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("state dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected .agents/rpi to be a directory")
	}
}

func TestState_ValidatePriorPhaseResult_Completed(t *testing.T) {
	tmp := t.TempDir()

	result := &phaseResult{Phase: 1, PhaseName: "discovery", Status: "completed"}
	if err := writePhaseResult(tmp, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	if err := validatePriorPhaseResult(tmp, 1); err != nil {
		t.Errorf("validatePriorPhaseResult returned error for completed: %v", err)
	}
}

func TestState_ValidatePriorPhaseResult_TimeBoxed(t *testing.T) {
	tmp := t.TempDir()

	result := &phaseResult{Phase: 1, PhaseName: "discovery", Status: "time_boxed"}
	if err := writePhaseResult(tmp, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	if err := validatePriorPhaseResult(tmp, 1); err != nil {
		t.Errorf("validatePriorPhaseResult returned error for time_boxed: %v", err)
	}
}

func TestState_ValidatePriorPhaseResult_InvalidStatus(t *testing.T) {
	tmp := t.TempDir()

	result := &phaseResult{Phase: 1, PhaseName: "discovery", Status: "failed"}
	if err := writePhaseResult(tmp, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	err := validatePriorPhaseResult(tmp, 1)
	if err == nil {
		t.Error("expected error for status=failed, got nil")
	}
}

// ---------- savePhasedState / loadPhasedState ----------

func TestSavePhasedState_DualRoot(t *testing.T) {
	tmp := t.TempDir()

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "dual root test",
		Phase:         2,
		Cycle:         1,
		RunID:         "run123",
		Verdicts:      map[string]string{"pre-mortem": "PASS"},
		Attempts:      map[string]int{"phase-1": 1},
		Opts:          defaultPhasedEngineOptions(),
	}

	if err := savePhasedState(tmp, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Check flat path exists
	flatPath := filepath.Join(tmp, ".agents", "rpi", phasedStateFile)
	if _, err := os.Stat(flatPath); err != nil {
		t.Errorf("flat state file not found: %v", err)
	}

	// Check registry path exists
	registryPath := filepath.Join(tmp, ".agents", "rpi", "runs", "run123", phasedStateFile)
	if _, err := os.Stat(registryPath); err != nil {
		t.Errorf("registry state file not found: %v", err)
	}
}

func TestLoadPhasedState_RegistryPreference(t *testing.T) {
	tmp := t.TempDir()

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "registry test",
		Phase:         2,
		Cycle:         1,
		RunID:         "run456",
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
		Opts:          defaultPhasedEngineOptions(),
	}

	if err := savePhasedState(tmp, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Update registry copy with newer data
	registryDir := filepath.Join(tmp, ".agents", "rpi", "runs", "run456")
	updatedState := *state
	updatedState.Goal = "registry updated"
	data, _ := json.MarshalIndent(&updatedState, "", "  ")
	data = append(data, '\n')

	// Brief sleep to ensure modtime difference
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(registryDir, phasedStateFile), data, 0600); err != nil {
		t.Fatalf("write updated registry state: %v", err)
	}

	loaded, err := loadPhasedState(tmp)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}

	if loaded.Goal != "registry updated" {
		t.Errorf("Goal = %q, want %q (registry should be preferred)", loaded.Goal, "registry updated")
	}
}

func TestLoadPhasedState_FlatFallback(t *testing.T) {
	tmp := t.TempDir()

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "flat fallback",
		Phase:         1,
		Cycle:         1,
		RunID:         "",
		Verdicts:      map[string]string{},
		Attempts:      map[string]int{},
		Opts:          defaultPhasedEngineOptions(),
	}

	// Write only the flat file (no registry because RunID is empty)
	if err := savePhasedState(tmp, state); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	loaded, err := loadPhasedState(tmp)
	if err != nil {
		t.Fatalf("loadPhasedState: %v", err)
	}

	if loaded.Goal != "flat fallback" {
		t.Errorf("Goal = %q, want %q", loaded.Goal, "flat fallback")
	}
}

// ---------- parsePhasedState ----------

func TestParsePhasedState_NilMaps(t *testing.T) {
	// JSON with no verdicts/attempts fields — should initialize to empty maps
	raw := `{"schema_version":1,"goal":"nil maps test","phase":2,"cycle":1}`

	state, err := parsePhasedState([]byte(raw))
	if err != nil {
		t.Fatalf("parsePhasedState: %v", err)
	}

	if state.Verdicts == nil {
		t.Error("Verdicts should be initialized, got nil")
	}
	if state.Attempts == nil {
		t.Error("Attempts should be initialized, got nil")
	}
	if len(state.Verdicts) != 0 {
		t.Errorf("Verdicts should be empty, got %d entries", len(state.Verdicts))
	}
	if len(state.Attempts) != 0 {
		t.Errorf("Attempts should be empty, got %d entries", len(state.Attempts))
	}
}

func TestParsePhasedState_DefaultsPhaseAndCycle(t *testing.T) {
	raw := `{"schema_version":1}`

	state, err := parsePhasedState([]byte(raw))
	if err != nil {
		t.Fatalf("parsePhasedState: %v", err)
	}

	if state.Phase != 1 {
		t.Errorf("Phase = %d, want 1 (default)", state.Phase)
	}
	if state.Cycle != 1 {
		t.Errorf("Cycle = %d, want 1 (default)", state.Cycle)
	}
	if state.Goal != "unknown-goal" {
		t.Errorf("Goal = %q, want %q (default)", state.Goal, "unknown-goal")
	}
}

// ---------- readRunHeartbeat ----------

func TestReadRunHeartbeat_RFC3339Nano(t *testing.T) {
	tmp := t.TempDir()
	runID := "hb-nano"

	runDir := filepath.Join(tmp, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0750); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)
	if err := os.WriteFile(filepath.Join(runDir, "heartbeat.txt"), []byte(ts+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got := readRunHeartbeat(tmp, runID)
	if got.IsZero() {
		t.Fatal("readRunHeartbeat returned zero time for RFC3339Nano format")
	}

	// Allow 1 second tolerance for rounding
	if got.Sub(now).Abs() > time.Second {
		t.Errorf("timestamp drift too large: got %v, want ~%v", got, now)
	}
}

func TestReadRunHeartbeat_RFC3339(t *testing.T) {
	tmp := t.TempDir()
	runID := "hb-rfc"

	runDir := filepath.Join(tmp, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0750); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(runDir, "heartbeat.txt"), []byte(ts+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got := readRunHeartbeat(tmp, runID)
	if got.IsZero() {
		t.Fatal("readRunHeartbeat returned zero time for RFC3339 format")
	}

	if !got.Equal(now) {
		t.Errorf("got %v, want %v", got, now)
	}
}

func TestReadRunHeartbeat_EmptyRunID(t *testing.T) {
	tmp := t.TempDir()
	got := readRunHeartbeat(tmp, "")
	if !got.IsZero() {
		t.Errorf("expected zero time for empty runID, got %v", got)
	}
}

func TestReadRunHeartbeat_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	got := readRunHeartbeat(tmp, "nonexistent")
	if !got.IsZero() {
		t.Errorf("expected zero time for missing heartbeat, got %v", got)
	}
}

func TestRunHeartbeatAge_Fresh(t *testing.T) {
	tmp := t.TempDir()
	runID := "test-age-run"

	// Write a heartbeat
	updateRunHeartbeat(tmp, runID)

	age, ok := runHeartbeatAge(tmp, runID)
	if !ok {
		t.Fatal("expected ok=true for existing heartbeat")
	}
	if age < 0 || age > 5*time.Second {
		t.Errorf("expected age between 0 and 5s, got %v", age)
	}
}

func TestRunHeartbeatAge_Missing(t *testing.T) {
	tmp := t.TempDir()

	age, ok := runHeartbeatAge(tmp, "nonexistent")
	if ok {
		t.Fatal("expected ok=false for missing heartbeat")
	}
	if age != -1 {
		t.Errorf("expected age=-1 for missing heartbeat, got %v", age)
	}
}

func TestRunHeartbeatAge_EmptyRunID(t *testing.T) {
	tmp := t.TempDir()

	age, ok := runHeartbeatAge(tmp, "")
	if ok {
		t.Fatal("expected ok=false for empty runID")
	}
	if age != -1 {
		t.Errorf("expected age=-1 for empty runID, got %v", age)
	}
}
