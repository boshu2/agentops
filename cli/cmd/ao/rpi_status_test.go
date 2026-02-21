package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper: write a run registry entry (state + optional heartbeat)
type registryRunSpec struct {
	runID  string
	phase  int
	schema int
	goal   string
	hbAge  time.Duration // 0 = no heartbeat; negative = stale; positive = fresh
}

func writeRegistryRun(t *testing.T, rootDir string, spec registryRunSpec) {
	t.Helper()
	runDir := filepath.Join(rootDir, ".agents", "rpi", "runs", spec.runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatalf("mkdir registry run dir: %v", err)
	}
	state := map[string]interface{}{
		"schema_version": spec.schema,
		"run_id":         spec.runID,
		"goal":           spec.goal,
		"phase":          spec.phase,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatalf("write registry state: %v", err)
	}
	if spec.hbAge != 0 {
		ts := time.Now().Add(-spec.hbAge).UTC().Format(time.RFC3339Nano) + "\n"
		if err := os.WriteFile(filepath.Join(runDir, "heartbeat.txt"), []byte(ts), 0644); err != nil {
			t.Fatalf("write heartbeat: %v", err)
		}
	}
}

func TestRPIStatusDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "abc123def456",
		"goal":           "test goal",
		"phase":          2,
		"epic_id":        "ag-test",
		"started_at":     time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(stateDir, "phased-state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	run, ok := loadRPIRun(tmpDir)
	if !ok {
		t.Fatal("expected loadRPIRun to return a run")
	}

	if run.RunID != "abc123def456" {
		t.Errorf("expected run_id abc123def456, got %s", run.RunID)
	}
	if run.PhaseName != "implementation" {
		t.Errorf("expected phase implementation, got %s", run.PhaseName)
	}
	if run.EpicID != "ag-test" {
		t.Errorf("expected epic ag-test, got %s", run.EpicID)
	}
	if run.Goal != "test goal" {
		t.Errorf("expected goal 'test goal', got %s", run.Goal)
	}
	// Without tmux, non-terminal phased runs are "unknown".
	if run.Status != "unknown" {
		t.Errorf("expected status 'unknown', got %s", run.Status)
	}
}

func TestRPIStatusMissingState(t *testing.T) {
	tmpDir := t.TempDir()
	_, ok := loadRPIRun(tmpDir)
	if ok {
		t.Fatal("expected loadRPIRun to return false for empty dir")
	}
}

func TestRPIStatusCorruptState(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "phased-state.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := loadRPIRun(tmpDir)
	if ok {
		t.Fatal("expected loadRPIRun to return false for corrupt state")
	}
}

func TestRPIStatusPhaseNames(t *testing.T) {
	tests := []struct {
		schema   int
		phase    int
		expected string
	}{
		{1, 1, "discovery"},
		{1, 2, "implementation"},
		{1, 3, "validation"},
		{1, 99, "phase-99"},
		{0, 1, "research"},
		{0, 2, "plan"},
		{0, 3, "pre-mortem"},
		{0, 4, "crank"},
		{0, 5, "vibe"},
		{0, 6, "post-mortem"},
	}

	for _, tt := range tests {
		tmpDir := t.TempDir()
		stateDir := filepath.Join(tmpDir, ".agents", "rpi")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatal(err)
		}

		state := map[string]interface{}{
			"schema_version": tt.schema,
			"run_id":         "test-run",
			"phase":          tt.phase,
		}
		data, _ := json.Marshal(state)
		if err := os.WriteFile(filepath.Join(stateDir, "phased-state.json"), data, 0644); err != nil {
			t.Fatal(err)
		}

		run, ok := loadRPIRun(tmpDir)
		if !ok {
			t.Fatalf("expected run for phase %d", tt.phase)
		}
		if run.PhaseName != tt.expected {
			t.Errorf("phase %d: expected %s, got %s", tt.phase, tt.expected, run.PhaseName)
		}
	}
}

func TestRPIStatusEmptyRunID(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := map[string]interface{}{
		"goal":  "no run id",
		"phase": 1,
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(stateDir, "phased-state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := loadRPIRun(tmpDir)
	if ok {
		t.Fatal("expected loadRPIRun to return false when run_id is empty")
	}
}

func TestRPIStatusCompletedFinalPhase(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "completed-run",
		"goal":           "finished goal",
		"phase":          3,
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(stateDir, "phased-state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	run, ok := loadRPIRun(tmpDir)
	if !ok {
		t.Fatal("expected loadRPIRun to return a run")
	}
	if run.Status != "completed" {
		t.Errorf("expected status 'completed' for terminal phase, got %s", run.Status)
	}
}

func TestRPIStatusDetermineRunStatus(t *testing.T) {
	// No tmux in test environment, so all sessions are "not alive"
	tests := []struct {
		name     string
		schema   int
		phase    int
		expected string
	}{
		{"schema v1 phase 1 no tmux", 1, 1, "unknown"},
		{"schema v1 phase 3 completed", 1, 3, "completed"},
		{"legacy phase 3 no tmux", 0, 3, "unknown"},
		{"legacy phase 6 completed", 0, 6, "completed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := phasedState{
				SchemaVersion: tt.schema,
				RunID:         "test-run",
				Phase:         tt.phase,
			}
			status := determineRunStatus(state)
			if status != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestRPIStatusSiblingDiscovery(t *testing.T) {
	// Create a parent directory with cwd and a sibling worktree
	parent := t.TempDir()
	cwd := filepath.Join(parent, "myrepo")
	sibling := filepath.Join(parent, "myrepo-rpi-abc123")

	for _, dir := range []string{cwd, sibling} {
		stateDir := filepath.Join(dir, ".agents", "rpi")
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write state in cwd
	cwdState := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "main-run",
		"goal":           "main goal",
		"phase":          2,
	}
	cwdData, _ := json.Marshal(cwdState)
	if err := os.WriteFile(filepath.Join(cwd, ".agents", "rpi", "phased-state.json"), cwdData, 0644); err != nil {
		t.Fatal(err)
	}

	// Write state in sibling
	sibState := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "sibling-run",
		"goal":           "sibling goal",
		"phase":          3,
	}
	sibData, _ := json.Marshal(sibState)
	if err := os.WriteFile(filepath.Join(sibling, ".agents", "rpi", "phased-state.json"), sibData, 0644); err != nil {
		t.Fatal(err)
	}

	runs := discoverRPIRuns(cwd)
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	// Verify both runs are present
	foundMain, foundSibling := false, false
	for _, r := range runs {
		if r.RunID == "main-run" {
			foundMain = true
		}
		if r.RunID == "sibling-run" {
			foundSibling = true
		}
	}
	if !foundMain {
		t.Error("expected to find main-run")
	}
	if !foundSibling {
		t.Error("expected to find sibling-run")
	}
}

// --- Log parser tests ---

func TestRPIStatusParseLogNewFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T10:00:00Z] [abc123] start: goal="add user auth" from=discovery
[2026-02-15T10:05:00Z] [abc123] discovery: completed in 5m0s
[2026-02-15T10:06:00Z] [abc123] discovery: pre-mortem verdict: PASS
[2026-02-15T10:25:00Z] [abc123] implementation: completed in 19m0s
[2026-02-15T10:30:00Z] [abc123] validation: vibe verdict: PASS
[2026-02-15T10:35:00Z] [abc123] validation: completed in 10m0s
[2026-02-15T10:35:00Z] [abc123] complete: epic=ag-test verdicts=map[pre_mortem:PASS vibe:PASS]
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.RunID != "abc123" {
		t.Errorf("expected run_id abc123, got %s", run.RunID)
	}
	if run.Goal != "add user auth" {
		t.Errorf("expected goal 'add user auth', got %q", run.Goal)
	}
	if run.Status != "completed" {
		t.Errorf("expected status completed, got %s", run.Status)
	}
	if run.EpicID != "ag-test" {
		t.Errorf("expected epic ag-test, got %s", run.EpicID)
	}
	if len(run.Phases) != 7 {
		t.Errorf("expected 7 phase entries, got %d", len(run.Phases))
	}
	// Check verdicts
	if run.Verdicts["pre_mortem"] != "PASS" {
		t.Errorf("expected pre_mortem verdict PASS, got %q", run.Verdicts["pre_mortem"])
	}
	if run.Verdicts["vibe"] != "PASS" {
		t.Errorf("expected vibe verdict PASS, got %q", run.Verdicts["vibe"])
	}
	// Check duration (35 min from start to complete)
	expectedDur := 35 * time.Minute
	if run.Duration != expectedDur {
		t.Errorf("expected duration %s, got %s", expectedDur, run.Duration)
	}
}

func TestRPIStatusParseLogOldFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T09:00:00Z] start: goal="fix typo" from=discovery
[2026-02-15T09:02:00Z] research: completed in 2m0s
[2026-02-15T09:04:00Z] plan: completed in 2m0s
[2026-02-15T09:06:00Z] complete: epic=ag-typo verdicts=map[vibe:PASS]
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.Goal != "fix typo" {
		t.Errorf("expected goal 'fix typo', got %q", run.Goal)
	}
	if run.Status != "completed" {
		t.Errorf("expected status completed, got %s", run.Status)
	}
	if run.EpicID != "ag-typo" {
		t.Errorf("expected epic ag-typo, got %s", run.EpicID)
	}
}

func TestRPIStatusParseLogOldFormatWithoutStartFirst(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T09:00:00Z] plan: completed in 2m0s
[2026-02-15T09:02:00Z] complete: epic=ag-first verdicts=map[]
[2026-02-15T09:03:00Z] start: goal="second run" from=discovery
[2026-02-15T09:04:00Z] complete: epic=ag-second verdicts=map[]
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	if runs[0].RunID != "anon-1" {
		t.Errorf("expected first run anon-1, got %s", runs[0].RunID)
	}
	if runs[0].Status != "completed" {
		t.Errorf("expected first run completed, got %s", runs[0].Status)
	}
	if runs[1].RunID != "anon-2" {
		t.Errorf("expected second run anon-2, got %s", runs[1].RunID)
	}
	if runs[1].Goal != "second run" {
		t.Errorf("expected second run goal 'second run', got %q", runs[1].Goal)
	}
}

func TestRPIStatusParseLogMultipleRuns(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T10:00:00Z] [run1] start: goal="first goal" from=discovery
[2026-02-15T10:05:00Z] [run1] discovery: completed in 5m0s
[2026-02-15T10:05:00Z] [run2] start: goal="second goal" from=discovery
[2026-02-15T10:10:00Z] [run1] complete: epic=ag-first verdicts=map[]
[2026-02-15T10:12:00Z] [run2] discovery: completed in 7m0s
[2026-02-15T10:15:00Z] [run2] complete: epic=ag-second verdicts=map[vibe:WARN]
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	if runs[0].RunID != "run1" || runs[0].Goal != "first goal" {
		t.Errorf("run1 mismatch: id=%s goal=%q", runs[0].RunID, runs[0].Goal)
	}
	if runs[1].RunID != "run2" || runs[1].Goal != "second goal" {
		t.Errorf("run2 mismatch: id=%s goal=%q", runs[1].RunID, runs[1].Goal)
	}
	if runs[0].Status != "completed" {
		t.Errorf("run1 expected completed, got %s", runs[0].Status)
	}
	if runs[1].Status != "completed" {
		t.Errorf("run2 expected completed, got %s", runs[1].Status)
	}
	if runs[1].Verdicts["vibe"] != "WARN" {
		t.Errorf("run2 expected vibe verdict WARN, got %q", runs[1].Verdicts["vibe"])
	}
}

func TestRPIStatusParseLogRetries(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T10:00:00Z] [abc] start: goal="retry test" from=discovery
[2026-02-15T10:05:00Z] [abc] discovery: completed in 5m0s
[2026-02-15T10:10:00Z] [abc] validation: RETRY attempt 2/3
[2026-02-15T10:15:00Z] [abc] validation: RETRY attempt 3/3
[2026-02-15T10:20:00Z] [abc] validation: completed in 10m0s
[2026-02-15T10:25:00Z] [abc] complete: epic=ag-retry verdicts=map[pre_mortem:PASS]
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	run := runs[0]
	if run.Retries["validation"] != 2 {
		t.Errorf("expected 2 retries for validation, got %d", run.Retries["validation"])
	}
	if run.Status != "completed" {
		t.Errorf("expected status completed, got %s", run.Status)
	}
}

func TestRPIStatusParseLogFailed(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T10:00:00Z] [fail1] start: goal="failing run" from=discovery
[2026-02-15T10:05:00Z] [fail1] discovery: completed in 5m0s
[2026-02-15T10:10:00Z] [fail1] implementation: FAILED: claude exited with code 1
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	if runs[0].Status != "failed" {
		t.Errorf("expected status failed, got %s", runs[0].Status)
	}
}

func TestRPIStatusParseLogFatal(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "phased-orchestration.log")

	logContent := `[2026-02-15T10:00:00Z] [fatal1] start: goal="fatal run" from=discovery
[2026-02-15T10:01:00Z] [fatal1] discovery: FATAL: build prompt failed
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("parseOrchestrationLog failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != "failed" {
		t.Errorf("expected status failed for FATAL details, got %s", runs[0].Status)
	}
}

func TestDiscoverLiveStatuses(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "repo-rpi-1234")

	for _, dir := range []string{
		filepath.Join(cwd, ".agents", "rpi"),
		filepath.Join(sibling, ".agents", "rpi"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	mainPath := filepath.Join(cwd, ".agents", "rpi", "live-status.md")
	if err := os.WriteFile(mainPath, []byte("# Live Status\nmain"), 0644); err != nil {
		t.Fatal(err)
	}
	siblingPath := filepath.Join(sibling, ".agents", "rpi", "live-status.md")
	if err := os.WriteFile(siblingPath, []byte("# Live Status\nsibling"), 0644); err != nil {
		t.Fatal(err)
	}

	got := discoverLiveStatuses(cwd)
	if len(got) != 2 {
		t.Fatalf("expected 2 live status snapshots, got %d", len(got))
	}
}

func TestRPIStatusParseLogMissingFile(t *testing.T) {
	_, err := parseOrchestrationLog("/nonexistent/path/log")
	if err == nil {
		t.Fatal("expected error for missing log file")
	}
}

func TestRPIStatusParseLogEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "empty.log")
	if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs for empty log, got %d", len(runs))
	}
}

func TestRPIStatusParseLogGarbageLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "garbage.log")

	logContent := `this is not a log line
also not valid
[2026-02-15T10:00:00Z] [good] start: goal="valid" from=discovery
random noise here
[2026-02-15T10:05:00Z] [good] complete: epic=ag-good verdicts=map[]
more garbage
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Goal != "valid" {
		t.Errorf("expected goal 'valid', got %q", runs[0].Goal)
	}
}

func TestRPIStatusExtractGoalFromDetails(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`goal="add auth" from=discovery`, "add auth"},
		{`goal="" from=discovery`, ""},
		{`just plain text`, "just plain text"},
	}
	for _, tt := range tests {
		got := extractGoalFromDetails(tt.input)
		if got != tt.expected {
			t.Errorf("extractGoalFromDetails(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRPIStatusExtractEpicFromDetails(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`epic=ag-test verdicts=map[]`, "ag-test"},
		{`epic=ag-1234 verdicts=map[vibe:PASS]`, "ag-1234"},
		{`no epic here`, ""},
	}
	for _, tt := range tests {
		got := extractEpicFromDetails(tt.input)
		if got != tt.expected {
			t.Errorf("extractEpicFromDetails(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRPIStatusExtractVerdictsFromDetails(t *testing.T) {
	verdicts := make(map[string]string)
	extractVerdictsFromDetails("epic=ag-test verdicts=map[pre_mortem:PASS vibe:WARN]", verdicts)
	if verdicts["pre_mortem"] != "PASS" {
		t.Errorf("expected pre_mortem=PASS, got %q", verdicts["pre_mortem"])
	}
	if verdicts["vibe"] != "WARN" {
		t.Errorf("expected vibe=WARN, got %q", verdicts["vibe"])
	}
}

func TestRPIStatusExtractVerdictsEmpty(t *testing.T) {
	verdicts := make(map[string]string)
	extractVerdictsFromDetails("epic=ag-test verdicts=map[]", verdicts)
	if len(verdicts) != 0 {
		t.Errorf("expected 0 verdicts, got %d", len(verdicts))
	}
}

func TestRPIStatusExtractInlineVerdict(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"completed with PASS", "PASS"},
		{"gate FAIL after retry", "FAIL"},
		{"verdict WARN for complexity", "WARN"},
		{"nothing here", ""},
	}
	for _, tt := range tests {
		got := extractInlineVerdict(tt.input)
		if got != tt.expected {
			t.Errorf("extractInlineVerdict(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRPIStatusParseLogRunningStatus(t *testing.T) {
	// A run with start but no complete entry should be "running"
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "running.log")

	logContent := `[2026-02-15T10:00:00Z] [active1] start: goal="in progress" from=discovery
[2026-02-15T10:05:00Z] [active1] discovery: completed in 5m0s
[2026-02-15T10:10:00Z] [active1] implementation: completed in 5m0s
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != "running" {
		t.Errorf("expected status 'running' for incomplete run, got %s", runs[0].Status)
	}
}

func TestRPIStatusParseLogInlineVerdictsConsolidated(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "inline-verdicts.log")

	logContent := `[2026-02-15T10:00:00Z] [v1] start: goal="inline verdicts" from=discovery
[2026-02-15T10:05:00Z] [v1] discovery: pre-mortem verdict: WARN
[2026-02-15T10:10:00Z] [v1] validation: vibe verdict: PASS
[2026-02-15T10:15:00Z] [v1] complete: epic=ag-inline verdicts=map[]
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatal(err)
	}

	runs, err := parseOrchestrationLog(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Verdicts["pre_mortem"] != "WARN" {
		t.Errorf("expected pre_mortem WARN, got %q", runs[0].Verdicts["pre_mortem"])
	}
	if runs[0].Verdicts["vibe"] != "PASS" {
		t.Errorf("expected vibe PASS, got %q", runs[0].Verdicts["vibe"])
	}
}

// --- Registry-first discovery tests ---

// TestDiscoverRPIRuns_RegistryFirst verifies that discoverRPIRunsRegistryFirst
// reads runs from .agents/rpi/runs/ (not just the flat state file).
func TestDiscoverRPIRuns_RegistryFirst(t *testing.T) {
	tmpDir := t.TempDir()

	// Write two runs in the registry; fresh heartbeat makes first "active".
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "run-active",
		phase:  2,
		schema: 1,
		goal:   "active goal",
		hbAge:  1 * time.Minute, // fresh
	})
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "run-hist",
		phase:  3,
		schema: 1,
		goal:   "historical goal",
		hbAge:  0, // no heartbeat => historical (completed phase)
	})

	active, historical := discoverRPIRunsRegistryFirst(tmpDir)

	if len(active) != 1 {
		t.Fatalf("expected 1 active run, got %d", len(active))
	}
	if active[0].RunID != "run-active" {
		t.Errorf("expected active run-active, got %s", active[0].RunID)
	}
	if active[0].Status != "running" {
		t.Errorf("expected active run status 'running', got %s", active[0].Status)
	}

	if len(historical) != 1 {
		t.Fatalf("expected 1 historical run, got %d", len(historical))
	}
	if historical[0].RunID != "run-hist" {
		t.Errorf("expected historical run-hist, got %s", historical[0].RunID)
	}
}

// TestDiscoverRPIRuns_HeartbeatLiveness verifies that heartbeat age correctly
// drives active vs historical classification.
func TestDiscoverRPIRuns_HeartbeatLiveness(t *testing.T) {
	tmpDir := t.TempDir()

	// Fresh heartbeat (1 min old) → active
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "fresh-hb",
		phase:  1,
		schema: 1,
		hbAge:  1 * time.Minute,
	})
	// Stale heartbeat (10 min old) → historical (no tmux in test env)
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "stale-hb",
		phase:  1,
		schema: 1,
		hbAge:  10 * time.Minute,
	})

	active, historical := discoverRPIRunsRegistryFirst(tmpDir)

	foundActive := false
	for _, r := range active {
		if r.RunID == "fresh-hb" {
			foundActive = true
		}
	}
	if !foundActive {
		t.Error("expected fresh-hb to be in active set")
	}

	foundHistorical := false
	for _, r := range historical {
		if r.RunID == "stale-hb" {
			foundHistorical = true
		}
	}
	if !foundHistorical {
		t.Error("expected stale-hb to be in historical set")
	}
}

// TestDiscoverRPIRuns_CompletedPhase verifies that a run at the terminal phase
// is classified as "completed" even without a heartbeat.
func TestDiscoverRPIRuns_CompletedPhase(t *testing.T) {
	tmpDir := t.TempDir()

	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "done-run",
		phase:  3,
		schema: 1,
		hbAge:  0, // no heartbeat
	})

	active, historical := discoverRPIRunsRegistryFirst(tmpDir)

	if len(active) != 0 {
		t.Errorf("completed run should not be active, got %d active", len(active))
	}
	if len(historical) != 1 {
		t.Fatalf("expected 1 historical run, got %d", len(historical))
	}
	if historical[0].Status != "completed" {
		t.Errorf("expected status 'completed', got %s", historical[0].Status)
	}
}

// TestDiscoverRPIRuns_SiblingWorktrees verifies that sibling *-rpi-* worktrees
// are discovered when scanning from cwd.
func TestDiscoverRPIRuns_SiblingWorktrees(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "myrepo")
	sibling := filepath.Join(parent, "myrepo-rpi-abc")

	for _, dir := range []string{cwd, sibling} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	writeRegistryRun(t, cwd, registryRunSpec{
		runID:  "main-run",
		phase:  1,
		schema: 1,
		hbAge:  2 * time.Minute, // active
	})
	writeRegistryRun(t, sibling, registryRunSpec{
		runID:  "side-run",
		phase:  3,
		schema: 1,
		hbAge:  0, // historical/completed
	})

	active, historical := discoverRPIRunsRegistryFirst(cwd)

	foundMain, foundSide := false, false
	for _, r := range active {
		if r.RunID == "main-run" {
			foundMain = true
		}
	}
	for _, r := range historical {
		if r.RunID == "side-run" {
			foundSide = true
		}
	}
	if !foundMain {
		t.Error("expected main-run in active set")
	}
	if !foundSide {
		t.Error("expected side-run in historical set")
	}
}

// TestDiscoverRPIRuns_FallbackFlatState verifies that when the registry is empty,
// discoverRPIRuns falls back to the flat phased-state.json.
func TestDiscoverRPIRuns_FallbackFlatState(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "legacy-run",
		"goal":           "legacy goal",
		"phase":          2,
		"started_at":     time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(stateDir, "phased-state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	runs := discoverRPIRuns(tmpDir)
	if len(runs) == 0 {
		t.Fatal("expected at least 1 run from flat state fallback")
	}
	found := false
	for _, r := range runs {
		if r.RunID == "legacy-run" {
			found = true
		}
	}
	if !found {
		t.Error("expected legacy-run in fallback results")
	}
}

// TestRPIStatusRegistryDiscovery verifies that scanRegistryRuns reads all runs
// in the .agents/rpi/runs/ directory correctly.
func TestRPIStatusRegistryDiscovery(t *testing.T) {
	tmpDir := t.TempDir()

	for _, spec := range []registryRunSpec{
		{runID: "r1", phase: 1, schema: 1, goal: "first", hbAge: 30 * time.Second},
		{runID: "r2", phase: 2, schema: 1, goal: "second", hbAge: 0},
		{runID: "r3", phase: 3, schema: 1, goal: "third", hbAge: 0},
	} {
		writeRegistryRun(t, tmpDir, spec)
	}

	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs from registry, got %d", len(runs))
	}

	found := make(map[string]bool)
	for _, r := range runs {
		found[r.RunID] = true
	}
	for _, id := range []string{"r1", "r2", "r3"} {
		if !found[id] {
			t.Errorf("expected run %s in registry scan results", id)
		}
	}
}

// TestRPIStatusRegistryDiscovery_EmptyDir verifies that scanRegistryRuns returns
// nil when the runs directory does not exist.
func TestRPIStatusRegistryDiscovery_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 0 {
		t.Errorf("expected 0 runs for empty dir, got %d", len(runs))
	}
}

// TestCheckTmuxSessionAlive_Timeout verifies that checkTmuxSessionAlive does not
// block indefinitely when tmux is unavailable or slow. The test measures elapsed
// time and asserts it stays well below 20 seconds (3 phases x 2s timeout = 6s max).
func TestCheckTmuxSessionAlive_Timeout(t *testing.T) {
	start := time.Now()
	alive := checkTmuxSessionAlive("nonexistent-run-id-xyz")
	elapsed := time.Since(start)

	if alive {
		t.Error("expected nonexistent session to not be alive")
	}
	// 3 phases x 2s timeout = 6s theoretical max; give generous headroom.
	if elapsed > 20*time.Second {
		t.Errorf("checkTmuxSessionAlive took too long: %v (expected < 20s)", elapsed)
	}
}

// TestCheckTmuxSessionAlive_EmptyRunID verifies that an empty runID returns false
// immediately without probing tmux.
func TestCheckTmuxSessionAlive_EmptyRunID(t *testing.T) {
	start := time.Now()
	alive := checkTmuxSessionAlive("")
	elapsed := time.Since(start)

	if alive {
		t.Error("empty runID should return false")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("empty runID check was too slow: %v", elapsed)
	}
}

func TestCheckTmuxSessionAlive_UsesConfiguredTmuxCommand(t *testing.T) {
	tmpBin := t.TempDir()
	customTmux := filepath.Join(tmpBin, "tmux-custom")
	script := "#!/usr/bin/env bash\nexit 0\n"
	if err := os.WriteFile(customTmux, []byte(script), 0755); err != nil {
		t.Fatalf("write custom tmux script: %v", err)
	}

	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "auto")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "tmux-custom")
	t.Setenv("PATH", tmpBin+":"+os.Getenv("PATH"))

	if !checkTmuxSessionAlive("run-custom-tmux") {
		t.Fatal("expected run to be considered alive when configured tmux command succeeds")
	}
}

// TestDetermineRunLiveness_FreshHeartbeat verifies that a fresh heartbeat
// marks a run as alive without a tmux probe.
func TestDetermineRunLiveness_FreshHeartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "hb-live-test"

	// Write a heartbeat just now.
	updateRunHeartbeat(tmpDir, runID)

	state := &phasedState{
		SchemaVersion: 1,
		RunID:         runID,
		Phase:         2,
	}

	isActive, lastHB := determineRunLiveness(tmpDir, state)
	if !isActive {
		t.Error("expected run with fresh heartbeat to be active")
	}
	if lastHB.IsZero() {
		t.Error("expected non-zero last heartbeat time")
	}
}

// TestDetermineRunLiveness_NoHeartbeat verifies that without a heartbeat and
// without a matching tmux session, the run is not active.
func TestDetermineRunLiveness_NoHeartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	state := &phasedState{
		SchemaVersion: 1,
		RunID:         "no-hb-no-tmux",
		Phase:         2,
	}

	isActive, lastHB := determineRunLiveness(tmpDir, state)
	if isActive {
		t.Error("expected run without heartbeat or tmux to be inactive")
	}
	if !lastHB.IsZero() {
		t.Errorf("expected zero last heartbeat, got %v", lastHB)
	}
}

// TestLocateRunMetadata_RegistryFirst verifies that locateRunMetadata finds a
// run in the registry directory before checking the flat state file.
func TestLocateRunMetadata_RegistryFirst(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "locate-test"

	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  runID,
		phase:  2,
		schema: 1,
		goal:   "registry goal",
		hbAge:  0,
	})

	state, root, err := locateRunMetadata(tmpDir, runID)
	if err != nil {
		t.Fatalf("locateRunMetadata: %v", err)
	}
	if state.RunID != runID {
		t.Errorf("expected RunID %s, got %s", runID, state.RunID)
	}
	if state.Goal != "registry goal" {
		t.Errorf("expected goal 'registry goal', got %q", state.Goal)
	}
	if root != tmpDir {
		t.Errorf("expected root %s, got %s", tmpDir, root)
	}
}

// TestLocateRunMetadata_FlatFallback verifies that locateRunMetadata falls back
// to the flat phased-state.json when the registry entry is absent.
func TestLocateRunMetadata_FlatFallback(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "flat-fallback"

	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	stateData := map[string]interface{}{
		"schema_version": 1,
		"run_id":         runID,
		"goal":           "flat fallback goal",
		"phase":          1,
	}
	data, _ := json.Marshal(stateData)
	if err := os.WriteFile(filepath.Join(stateDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	state, root, err := locateRunMetadata(tmpDir, runID)
	if err != nil {
		t.Fatalf("locateRunMetadata (flat fallback): %v", err)
	}
	if state.RunID != runID {
		t.Errorf("expected RunID %s, got %s", runID, state.RunID)
	}
	if root != tmpDir {
		t.Errorf("expected root %s, got %s", tmpDir, root)
	}
}

// TestLocateRunMetadata_NotFound verifies that locateRunMetadata returns an error
// when the run is not found in the registry or flat state.
func TestLocateRunMetadata_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := locateRunMetadata(tmpDir, "nonexistent-run")
	if err == nil {
		t.Fatal("expected error for nonexistent run, got nil")
	}
}

// TestLocateRunMetadata_WrongRunIDInFlat verifies that locateRunMetadata rejects
// a flat state file whose run_id does not match the requested run ID.
func TestLocateRunMetadata_WrongRunIDInFlat(t *testing.T) {
	tmpDir := t.TempDir()

	stateDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	stateData := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "different-run",
		"goal":           "some goal",
		"phase":          1,
	}
	data, _ := json.Marshal(stateData)
	if err := os.WriteFile(filepath.Join(stateDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := locateRunMetadata(tmpDir, "requested-run")
	if err == nil {
		t.Fatal("expected error when flat state run_id doesn't match requested run, got nil")
	}
}

// TestRPIStatusActiveHistoricalSeparation verifies the full pipeline:
// registry scan → active/historical separation → combined discoverRPIRuns result.
func TestRPIStatusActiveHistoricalSeparation(t *testing.T) {
	tmpDir := t.TempDir()

	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "running-now",
		phase:  2,
		schema: 1,
		goal:   "in progress",
		hbAge:  90 * time.Second, // fresh (< 5 min)
	})
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "done-already",
		phase:  3,
		schema: 1,
		goal:   "completed work",
		hbAge:  0, // no heartbeat, terminal phase
	})
	writeRegistryRun(t, tmpDir, registryRunSpec{
		runID:  "interrupted",
		phase:  1,
		schema: 1,
		goal:   "was interrupted",
		hbAge:  60 * time.Minute, // very stale heartbeat (> 5 min)
	})

	active, historical := discoverRPIRunsRegistryFirst(tmpDir)

	activeIDs := make(map[string]bool)
	for _, r := range active {
		activeIDs[r.RunID] = true
	}
	histIDs := make(map[string]bool)
	for _, r := range historical {
		histIDs[r.RunID] = true
	}

	if !activeIDs["running-now"] {
		t.Error("expected running-now to be active")
	}
	if activeIDs["done-already"] {
		t.Error("expected done-already NOT to be active (terminal phase)")
	}
	if activeIDs["interrupted"] {
		t.Error("expected interrupted NOT to be active (stale heartbeat)")
	}
	if !histIDs["done-already"] {
		t.Error("expected done-already to be historical")
	}
	if !histIDs["interrupted"] {
		t.Error("expected interrupted to be historical")
	}
}

// TestClassifyRunStatus_StaleWorktree verifies that a run with a worktree_path
// pointing to a nonexistent directory is classified as "stale".
func TestClassifyRunStatus_StaleWorktree(t *testing.T) {
	state := phasedState{
		SchemaVersion: 1,
		RunID:         "stale-wt",
		Phase:         2,
		WorktreePath:  "/nonexistent/worktree/path",
	}
	status := classifyRunStatus(state, false)
	if status != "stale" {
		t.Errorf("expected status 'stale' for missing worktree, got %s", status)
	}
}

// TestClassifyRunStatus_TerminalMetadata verifies that when terminal_status is set,
// classifyRunStatus uses it directly.
func TestClassifyRunStatus_TerminalMetadata(t *testing.T) {
	tests := []struct {
		name           string
		terminalStatus string
		phase          int
		isActive       bool
		expected       string
	}{
		{"interrupted", "interrupted", 2, false, "interrupted"},
		{"failed", "failed", 1, false, "failed"},
		{"stale explicit", "stale", 2, false, "stale"},
		// Terminal status takes precedence even if phase looks completed.
		{"failed at terminal phase", "failed", 3, false, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := phasedState{
				SchemaVersion:  1,
				RunID:          "term-test",
				Phase:          tt.phase,
				TerminalStatus: tt.terminalStatus,
			}
			status := classifyRunStatus(state, tt.isActive)
			if status != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, status)
			}
		})
	}
}

// TestDetermineRunLiveness_MissingWorktree verifies that when a state has a
// worktree_path set but the directory doesn't exist, the run is not active.
func TestDetermineRunLiveness_MissingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "missing-wt"

	// Write a fresh heartbeat (which would normally make it active).
	updateRunHeartbeat(tmpDir, runID)

	state := &phasedState{
		SchemaVersion: 1,
		RunID:         runID,
		Phase:         2,
		WorktreePath:  "/nonexistent/worktree/path",
	}

	isActive, _ := determineRunLiveness(tmpDir, state)
	if isActive {
		t.Error("expected run with missing worktree to NOT be active, even with fresh heartbeat")
	}
}

// TestClassifyRunReason verifies reason generation for various run states.
func TestClassifyRunReason(t *testing.T) {
	tests := []struct {
		name     string
		state    phasedState
		isActive bool
		expected string
	}{
		{
			name: "terminal reason from state",
			state: phasedState{
				TerminalReason: "signal: interrupt",
			},
			isActive: false,
			expected: "signal: interrupt",
		},
		{
			name: "worktree missing",
			state: phasedState{
				WorktreePath: "/nonexistent/path",
			},
			isActive: false,
			expected: "worktree missing",
		},
		{
			name:     "no reason for active run",
			state:    phasedState{},
			isActive: true,
			expected: "",
		},
		{
			name:     "no reason when no worktree",
			state:    phasedState{},
			isActive: false,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := classifyRunReason(tt.state, tt.isActive)
			if reason != tt.expected {
				t.Errorf("expected reason %q, got %q", tt.expected, reason)
			}
		})
	}
}

// TestScanRegistryRuns_StaleWorktreeReason verifies that scanRegistryRuns populates
// the Reason field when a worktree is missing.
func TestScanRegistryRuns_StaleWorktreeReason(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a run with a worktree_path pointing to a nonexistent directory.
	runDir := filepath.Join(tmpDir, ".agents", "rpi", "runs", "stale-wt-run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}
	state := map[string]interface{}{
		"schema_version": 1,
		"run_id":         "stale-wt-run",
		"goal":           "stale worktree test",
		"phase":          2,
		"worktree_path":  "/nonexistent/worktree",
		"started_at":     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
	}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(runDir, phasedStateFile), data, 0644); err != nil {
		t.Fatal(err)
	}

	runs := scanRegistryRuns(tmpDir)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != "stale" {
		t.Errorf("expected status 'stale', got %s", runs[0].Status)
	}
	if runs[0].Reason != "worktree missing" {
		t.Errorf("expected reason 'worktree missing', got %q", runs[0].Reason)
	}
}
