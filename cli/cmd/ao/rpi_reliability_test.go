package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// ─── gateFailError ────────────────────────────────────────────────────────────

// TestGateFailError_Error verifies that gateFailError.Error() returns a
// human-readable string that includes phase, verdict, and report path.
func TestGateFailError_Error(t *testing.T) {
	err := &gateFailError{
		Phase:   2,
		Verdict: "FAIL",
		Report:  "/tmp/council/report.md",
	}
	msg := err.Error()
	if !strings.Contains(msg, "2") {
		t.Errorf("Error() should contain phase number, got: %q", msg)
	}
	if !strings.Contains(msg, "FAIL") {
		t.Errorf("Error() should contain verdict, got: %q", msg)
	}
	if !strings.Contains(msg, "/tmp/council/report.md") {
		t.Errorf("Error() should contain report path, got: %q", msg)
	}
}

// TestGateFailError_AllPhases verifies gateFailError.Error() at each valid phase.
func TestGateFailError_AllPhases(t *testing.T) {
	for _, phase := range []int{1, 2, 3} {
		err := &gateFailError{
			Phase:   phase,
			Verdict: "FAIL",
			Report:  "report.md",
		}
		msg := err.Error()
		if msg == "" {
			t.Errorf("phase %d: Error() must not be empty", phase)
		}
		if !strings.Contains(msg, fmt.Sprintf("%d", phase)) {
			t.Errorf("phase %d: Error() should mention phase, got: %q", phase, msg)
		}
	}
}

// TestGateFailError_WithFindings ensures findings do not panic the Error() method.
func TestGateFailError_WithFindings(t *testing.T) {
	err := &gateFailError{
		Phase:   3,
		Verdict: "WARN",
		Findings: []finding{
			{Description: "Test is slow", Fix: "Use table-driven tests", Ref: "test.go:42"},
		},
		Report: "vibe-report.md",
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("Error() must not be empty when findings are present")
	}
}

// ─── selectExecutor ───────────────────────────────────────────────────────────

// TestSelectExecutor_ReturnsNonNil verifies that selectExecutor always returns
// a non-nil executor regardless of environment state.
func TestSelectExecutor_ReturnsNonNil(t *testing.T) {
	origLiveStatus := phasedLiveStatus
	origLookPath := lookPath
	defer func() {
		phasedLiveStatus = origLiveStatus
		lookPath = origLookPath
	}()

	phasedLiveStatus = false
	lookPath = func(name string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	exec := selectExecutor("", nil)
	if exec == nil {
		t.Fatal("selectExecutor must return a non-nil executor")
	}
	if exec.Name() == "" {
		t.Fatal("executor Name() must not be empty")
	}
}

// TestSelectExecutor_DirectFallback verifies that direct is selected when
// runtime mode is auto and live-status is disabled.
func TestSelectExecutor_DirectFallback(t *testing.T) {
	origLiveStatus := phasedLiveStatus
	defer func() {
		phasedLiveStatus = origLiveStatus
	}()

	phasedLiveStatus = false

	exec := selectExecutor("", nil)
	if exec.Name() != "direct" {
		t.Errorf("expected direct fallback, got %q", exec.Name())
	}
}

// ─── executor Execute() delegation ────────────────────────────────────────────

// TestDirectExecutor_Execute_PropagatesError verifies that directExecutor.Execute
// returns an error when the claude binary is not available.
func TestDirectExecutor_Execute_PropagatesError(t *testing.T) {
	// Temporarily redirect PATH so claude is not found.
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)
	t.Setenv("PATH", t.TempDir()) // empty dir = no binaries

	d := &directExecutor{}
	err := d.Execute("test prompt", t.TempDir(), "run-1", 1)
	if err == nil {
		t.Fatal("expected error when claude binary is absent")
	}
}

// TestStreamExecutor_Execute_PropagatesError verifies streamExecutor.Execute
// returns an error when claude is not found.
func TestStreamExecutor_Execute_PropagatesError(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)
	t.Setenv("PATH", t.TempDir()) // no binaries

	s := &streamExecutor{statusPath: filepath.Join(t.TempDir(), "status.md"), allPhases: nil}
	err := s.Execute("prompt", t.TempDir(), "run-stream-err", 3)
	if err == nil {
		t.Fatal("expected error from stream executor when claude is absent")
	}
}

// ─── validatePriorPhaseResult ─────────────────────────────────────────────────

// TestValidatePriorPhaseResult_Missing verifies that a missing result file
// returns an error mentioning the expected phase.
func TestValidatePriorPhaseResult_Missing(t *testing.T) {
	dir := t.TempDir()
	err := validatePriorPhaseResult(dir, 1)
	if err == nil {
		t.Fatal("expected error for missing phase result file")
	}
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("error should mention phase 1, got: %v", err)
	}
}

// TestValidatePriorPhaseResult_Malformed verifies that a non-JSON result file
// returns a parse error.
func TestValidatePriorPhaseResult_Malformed(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, 1))
	if err := os.WriteFile(path, []byte("not-json{{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	err := validatePriorPhaseResult(dir, 1)
	if err == nil {
		t.Fatal("expected error for malformed result file")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("error should mention 'malformed', got: %v", err)
	}
}

// TestValidatePriorPhaseResult_WrongStatus verifies that a result file with
// status other than "completed" returns an error.
func TestValidatePriorPhaseResult_WrongStatus(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, status := range []string{"failed", "running", "partial", ""} {
		result := phaseResult{
			SchemaVersion: 1,
			Phase:         2,
			PhaseName:     "implementation",
			Status:        status,
			RunID:         "test-run",
			StartedAt:     "2026-02-19T00:00:00Z",
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		path := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, 2))
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}

		err := validatePriorPhaseResult(dir, 2)
		if err == nil {
			t.Errorf("status=%q: expected error, got nil", status)
		}
		if !strings.Contains(err.Error(), status) && status != "" {
			t.Errorf("status=%q: error should mention status, got: %v", status, err)
		}
	}
}

// TestValidatePriorPhaseResult_Completed verifies the happy path: a result with
// status "completed" returns nil.
func TestValidatePriorPhaseResult_Completed(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	result := phaseResult{
		SchemaVersion: 1,
		Phase:         1,
		PhaseName:     "discovery",
		Status:        "completed",
		RunID:         "abc123",
		StartedAt:     "2026-02-19T00:00:00Z",
		CompletedAt:   "2026-02-19T01:00:00Z",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	path := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, 1))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := validatePriorPhaseResult(dir, 1); err != nil {
		t.Errorf("expected nil for completed result, got: %v", err)
	}
}

// ─── writePhaseResult ─────────────────────────────────────────────────────────

// TestWritePhaseResult_AtomicWrite verifies that writePhaseResult creates the
// result file and leaves no .tmp artifacts.
func TestWritePhaseResult_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	result := &phaseResult{
		SchemaVersion: 1,
		RunID:         "write-test",
		Phase:         2,
		PhaseName:     "implementation",
		Status:        "completed",
		StartedAt:     "2026-02-19T00:00:00Z",
	}

	if err := writePhaseResult(dir, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	// Verify the file exists with correct content.
	path := filepath.Join(dir, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, 2))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("result file not written: %v", err)
	}

	var loaded phaseResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("result file is not valid JSON: %v", err)
	}
	if loaded.Status != "completed" {
		t.Errorf("Status: got %q, want completed", loaded.Status)
	}
	if loaded.RunID != "write-test" {
		t.Errorf("RunID: got %q, want write-test", loaded.RunID)
	}

	// No .tmp files should remain.
	stateDir := filepath.Join(dir, ".agents", "rpi")
	entries, _ := os.ReadDir(stateDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

// TestWritePhaseResult_RoundTrip verifies that phaseResult fields survive a
// full write→read→parse cycle.
func TestWritePhaseResult_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &phaseResult{
		SchemaVersion:   1,
		RunID:           "roundtrip-run",
		Phase:           3,
		PhaseName:       "validation",
		Status:          "failed",
		Retries:         2,
		Error:           "vibe FAIL after max retries",
		Backend:         "direct",
		StartedAt:       "2026-02-19T00:00:00Z",
		CompletedAt:     "2026-02-19T02:00:00Z",
		DurationSeconds: 3600.0,
		Verdicts:        map[string]string{"vibe": "FAIL"},
		Artifacts:       map[string]string{"vibe_report": ".agents/council/vibe.md"},
	}

	if err := writePhaseResult(dir, original); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	path := filepath.Join(dir, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, 3))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	var loaded phaseResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"RunID", loaded.RunID, original.RunID},
		{"PhaseName", loaded.PhaseName, original.PhaseName},
		{"Status", loaded.Status, original.Status},
		{"Error", loaded.Error, original.Error},
		{"Backend", loaded.Backend, original.Backend},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
	if loaded.Retries != 2 {
		t.Errorf("Retries: got %d, want 2", loaded.Retries)
	}
	if loaded.Verdicts["vibe"] != "FAIL" {
		t.Errorf("Verdicts[vibe]: got %q, want FAIL", loaded.Verdicts["vibe"])
	}
}

// TestWritePhaseResult_AllPhases verifies writePhaseResult creates correctly
// named files for each of the 3 phases.
func TestWritePhaseResult_AllPhases(t *testing.T) {
	dir := t.TempDir()

	for _, phaseNum := range []int{1, 2, 3} {
		result := &phaseResult{
			SchemaVersion: 1,
			RunID:         fmt.Sprintf("run-phase%d", phaseNum),
			Phase:         phaseNum,
			PhaseName:     phases[phaseNum-1].Name,
			Status:        "completed",
			StartedAt:     "2026-02-19T00:00:00Z",
		}
		if err := writePhaseResult(dir, result); err != nil {
			t.Errorf("phase %d: writePhaseResult: %v", phaseNum, err)
		}

		expected := filepath.Join(dir, ".agents", "rpi", fmt.Sprintf(phaseResultFileFmt, phaseNum))
		if _, err := os.Stat(expected); err != nil {
			t.Errorf("phase %d: result file not created at %s", phaseNum, expected)
		}
	}
}

// ─── writePhasedStateAtomic failure paths ────────────────────────────────────

// TestWritePhasedStateAtomic_NonWritableDir verifies that writePhasedStateAtomic
// returns an error when the target directory is not writable.
func TestWritePhasedStateAtomic_NonWritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write to any directory")
	}

	dir := t.TempDir()
	// Remove write permission from the directory.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0755) //nolint:errcheck

	path := filepath.Join(dir, "state.json")
	err := writePhasedStateAtomic(path, []byte(`{"test":true}`))
	if err == nil {
		t.Fatal("expected error writing to non-writable directory")
	}
}

// TestWritePhasedStateAtomic_LeavesNoTmpOnSuccess verifies the tmp-cleanup
// defer fires correctly on the rename-success path.
func TestWritePhasedStateAtomic_LeavesNoTmpOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	data := []byte(`{"schema_version":1,"goal":"atomic","phase":1}`)
	if err := writePhasedStateAtomic(path, data); err != nil {
		t.Fatalf("writePhasedStateAtomic: %v", err)
	}

	// Verify the final file exists.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("final file not created: %v", err)
	}

	// No leftover .tmp files in the directory.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

// ─── recordRatchetCheckpoint ─────────────────────────────────────────────────

// TestRecordRatchetCheckpoint_FailSilently verifies that recordRatchetCheckpoint
// does not panic or return an error when ao ratchet is not on PATH.
func TestRecordRatchetCheckpoint_FailSilently(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)
	// Use an empty dir so no binaries are found.
	t.Setenv("PATH", t.TempDir())

	// Should not panic.
	recordRatchetCheckpoint("research", "")
	recordRatchetCheckpoint("implement", "")
	recordRatchetCheckpoint("validate", "")
}

// TestRecordRatchetCheckpoint_StepsDoNotPanic verifies multiple checkpoint steps
// can be called without panicking regardless of ao availability.
func TestRecordRatchetCheckpoint_StepsDoNotPanic(t *testing.T) {
	steps := []string{"research", "implement", "validate", "", "nonexistent-step"}
	for _, step := range steps {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("step=%q: recordRatchetCheckpoint panicked: %v", step, r)
				}
			}()
			recordRatchetCheckpoint(step, "")
		}()
	}
}

// ─── ledgerActionFromDetails ─────────────────────────────────────────────────

// TestLedgerActionFromDetails covers all branches of the switch/case.
func TestLedgerActionFromDetails(t *testing.T) {
	tests := []struct {
		details string
		want    string
	}{
		{"", "event"},
		{"  ", "event"},
		{"started phase 2", "started"},
		{"completed: all done", "completed"},
		{"failed: claude exited 1", "failed"},
		{"fatal: disk full", "fatal"},
		{"retry attempt 2/3 verdict=FAIL", "retry"},
		{"dry-run: would spawn", "dry-run"},
		{"handoff: vibe failed", "handoff"},
		{"epic=ag-abc1 verdicts=map[]", "summary"},
		{"backend=direct reason=\"runtime=auto live-status disabled\"", "backend=direct"},
		{"discovery: pre-mortem verdict: PASS", "discovery"},
	}

	for _, tt := range tests {
		t.Run(tt.details, func(t *testing.T) {
			got := ledgerActionFromDetails(tt.details)
			if got != tt.want {
				t.Errorf("ledgerActionFromDetails(%q) = %q, want %q", tt.details, got, tt.want)
			}
		})
	}
}

// ─── deriveRepoRootFromRPIOrchestrationLog ───────────────────────────────────

// TestDeriveRepoRootFromRPIOrchestrationLog_ValidPath verifies the happy path
// where logPath lives at <root>/.agents/rpi/phased-orchestration.log.
func TestDeriveRepoRootFromRPIOrchestrationLog_ValidPath(t *testing.T) {
	root := "/tmp/myrepo"
	logPath := filepath.Join(root, ".agents", "rpi", "phased-orchestration.log")

	got, ok := deriveRepoRootFromRPIOrchestrationLog(logPath)
	if !ok {
		t.Fatal("expected ok=true for valid RPI log path")
	}
	if got != root {
		t.Errorf("got root %q, want %q", got, root)
	}
}

// TestDeriveRepoRootFromRPIOrchestrationLog_InvalidPaths verifies that paths
// not matching the <root>/.agents/rpi/ pattern return ok=false.
func TestDeriveRepoRootFromRPIOrchestrationLog_InvalidPaths(t *testing.T) {
	badPaths := []string{
		"/tmp/myrepo/.agents/other/phased.log",
		"/tmp/myrepo/rpi/phased.log",
		"/tmp/myrepo/.config/rpi/phased.log",
		"/tmp/phased.log",
		"",
	}
	for _, p := range badPaths {
		_, ok := deriveRepoRootFromRPIOrchestrationLog(p)
		if ok {
			t.Errorf("expected ok=false for path %q", p)
		}
	}
}

// ─── updateRunHeartbeat failure path ─────────────────────────────────────────

// TestUpdateRunHeartbeat_UnwritableDir verifies updateRunHeartbeat does not
// panic when the run directory cannot be created.
func TestUpdateRunHeartbeat_UnwritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write to any directory")
	}

	dir := t.TempDir()
	// Make the .agents directory non-writable so MkdirAll fails.
	agentsDir := filepath.Join(dir, ".agents")
	if err := os.MkdirAll(agentsDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(agentsDir, 0755) //nolint:errcheck

	// Should not panic.
	updateRunHeartbeat(dir, "test-run-id")
}

// TestReadRunHeartbeat_MalformedTimestamp verifies readRunHeartbeat returns
// zero time when the heartbeat file contains a non-parseable timestamp.
func TestReadRunHeartbeat_MalformedTimestamp(t *testing.T) {
	dir := t.TempDir()
	runID := "malformed-hb"
	runDir := rpiRunRegistryDir(dir, runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a malformed timestamp.
	heartbeatPath := filepath.Join(runDir, "heartbeat.txt")
	if err := os.WriteFile(heartbeatPath, []byte("not-a-timestamp\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ts := readRunHeartbeat(dir, runID)
	if !ts.IsZero() {
		t.Errorf("expected zero time for malformed timestamp, got %v", ts)
	}
}

// ─── handleGateRetry exhaustion path ─────────────────────────────────────────

// TestHandleGateRetry_ExhaustsRetries verifies that handleGateRetry returns
// (false, nil) when the attempt count reaches phasedMaxRetries, without
// spawning another session.
func TestHandleGateRetry_ExhaustsRetries(t *testing.T) {
	dir := t.TempDir()

	origMaxRetries := phasedMaxRetries
	origLiveStatus := phasedLiveStatus
	defer func() {
		phasedMaxRetries = origMaxRetries
		phasedLiveStatus = origLiveStatus
	}()

	phasedMaxRetries = 2
	phasedLiveStatus = false

	state := &phasedState{
		SchemaVersion: 1,
		Goal:          "test goal",
		EpicID:        "ag-test1",
		Phase:         1,
		Cycle:         1,
		RunID:         "retry-exhaust",
		Verdicts:      make(map[string]string),
		Attempts:      map[string]int{"phase_1": 2}, // already at max
	}

	gateErr := &gateFailError{
		Phase:   1,
		Verdict: "FAIL",
		Report:  "some-report.md",
	}

	// Use a fake executor that should NOT be called when retries are exhausted.
	executor := &mockPhaseExecutor{name: "mock", execErr: nil}

	logPath := filepath.Join(dir, ".agents", "rpi", "orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	shouldRetry, err := handleGateRetry(dir, state, 1, gateErr, logPath, dir, "", nil, executor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shouldRetry {
		t.Error("expected shouldRetry=false when max retries exhausted")
	}
	if executor.execCount > 0 {
		t.Errorf("executor should not be called when retries exhausted, called %d times", executor.execCount)
	}
}

func TestResolveGateRetryAction_ModeOffParity(t *testing.T) {
	t.Setenv(types.MemRLModeEnvVar, string(types.MemRLModeOff))

	state := &phasedState{
		Opts: phasedEngineOptions{
			MaxRetries: 3,
		},
	}
	gateErr := &gateFailError{
		Phase:   3,
		Verdict: "FAIL",
		Report:  "vibe.md",
	}

	action1, decision1 := resolveGateRetryAction(state, 3, gateErr, 1)
	if action1 != types.MemRLActionRetry {
		t.Fatalf("mode=off attempt=1 action=%q, want retry", action1)
	}
	if decision1.Mode != types.MemRLModeOff {
		t.Fatalf("mode=off decision mode=%q, want off", decision1.Mode)
	}

	action3, _ := resolveGateRetryAction(state, 3, gateErr, 3)
	if action3 != types.MemRLActionEscalate {
		t.Fatalf("mode=off attempt=max action=%q, want escalate", action3)
	}
}

func TestResolveGateRetryAction_EnforceCrankBlockedEscalatesEarly(t *testing.T) {
	t.Setenv(types.MemRLModeEnvVar, string(types.MemRLModeEnforce))

	state := &phasedState{
		Opts: phasedEngineOptions{
			MaxRetries: 3,
		},
	}
	gateErr := &gateFailError{
		Phase:   2,
		Verdict: "BLOCKED",
		Report:  "crank.md",
	}

	action, decision := resolveGateRetryAction(state, 2, gateErr, 1)
	if action != types.MemRLActionEscalate {
		t.Fatalf("mode=enforce crank BLOCKED attempt=1 action=%q, want escalate", action)
	}
	if decision.RuleID == "" {
		t.Fatal("expected enforce decision to include matching rule_id")
	}
}

// mockPhaseExecutor is a test double for PhaseExecutor.
type mockPhaseExecutor struct {
	name      string
	execErr   error
	execCount int
}

func (m *mockPhaseExecutor) Name() string { return m.name }
func (m *mockPhaseExecutor) Execute(prompt, cwd, runID string, phaseNum int) error {
	m.execCount++
	return m.execErr
}

// ─── Registry resume: loadLatestRunRegistryState ──────────────────────────────

// TestLoadLatestRunRegistryState_NoRunsDir verifies the function returns
// os.ErrNotExist when the runs directory does not exist.
func TestLoadLatestRunRegistryState_NoRunsDir(t *testing.T) {
	dir := t.TempDir()
	state, err := loadLatestRunRegistryState(dir)
	if err == nil {
		t.Fatal("expected error when runs dir does not exist")
	}
	if state != nil {
		t.Error("expected nil state when runs dir does not exist")
	}
}

// TestLoadLatestRunRegistryState_EmptyRunsDir verifies the function returns
// os.ErrNotExist when the runs directory exists but has no run subdirectories.
func TestLoadLatestRunRegistryState_EmptyRunsDir(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatal(err)
	}

	state, err := loadLatestRunRegistryState(dir)
	if err == nil {
		t.Fatal("expected error for empty runs dir")
	}
	if state != nil {
		t.Error("expected nil state for empty runs dir")
	}
}

// TestLoadLatestRunRegistryState_PicksMostRecent verifies that when multiple
// run directories exist, the most recently modified state is returned.
func TestLoadLatestRunRegistryState_PicksMostRecent(t *testing.T) {
	dir := t.TempDir()

	// Write three run states with increasing phases.
	runs := []struct {
		id    string
		phase int
		goal  string
	}{
		{"run-001", 1, "oldest goal"},
		{"run-002", 2, "middle goal"},
		{"run-003", 3, "newest goal"},
	}

	for i, r := range runs {
		state := &phasedState{
			SchemaVersion: 1,
			Goal:          r.goal,
			Phase:         r.phase,
			Cycle:         1,
			RunID:         r.id,
			Verdicts:      make(map[string]string),
			Attempts:      make(map[string]int),
		}
		if err := savePhasedState(dir, state); err != nil {
			t.Fatalf("savePhasedState(%s): %v", r.id, err)
		}
		// Ensure file mtimes differ.
		if i < len(runs)-1 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Touch the newest state file to ensure it has the latest mtime.
	newest := filepath.Join(rpiRunRegistryDir(dir, "run-003"), phasedStateFile)
	now := time.Now().Add(100 * time.Millisecond)
	if err := os.Chtimes(newest, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	loaded, err := loadLatestRunRegistryState(dir)
	if err != nil {
		t.Fatalf("loadLatestRunRegistryState: %v", err)
	}
	if loaded.Goal != "newest goal" {
		t.Errorf("expected newest goal, got %q", loaded.Goal)
	}
	if loaded.Phase != 3 {
		t.Errorf("expected phase 3, got %d", loaded.Phase)
	}
}

// TestLoadLatestRunRegistryState_SkipsMalformedEntries verifies that run
// directories with missing or malformed state files are skipped.
func TestLoadLatestRunRegistryState_SkipsMalformedEntries(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".agents", "rpi", "runs")

	// Create a run dir without a state file (should be skipped).
	emptyRunDir := filepath.Join(runsDir, "empty-run")
	if err := os.MkdirAll(emptyRunDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a run dir with a malformed state file (should be skipped).
	badRunDir := filepath.Join(runsDir, "bad-run")
	if err := os.MkdirAll(badRunDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badRunDir, phasedStateFile), []byte("{{not-json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a good run dir.
	goodState := &phasedState{
		SchemaVersion: 1,
		Goal:          "good goal",
		Phase:         2,
		Cycle:         1,
		RunID:         "good-run",
		Verdicts:      make(map[string]string),
		Attempts:      make(map[string]int),
	}
	if err := savePhasedState(dir, goodState); err != nil {
		t.Fatalf("savePhasedState: %v", err)
	}

	// Touch good-run state to ensure it has a newer mtime.
	goodPath := filepath.Join(rpiRunRegistryDir(dir, "good-run"), phasedStateFile)
	now := time.Now().Add(100 * time.Millisecond)
	if err := os.Chtimes(goodPath, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	loaded, err := loadLatestRunRegistryState(dir)
	if err != nil {
		t.Fatalf("loadLatestRunRegistryState: %v", err)
	}
	if loaded.Goal != "good goal" {
		t.Errorf("expected good goal, got %q", loaded.Goal)
	}
}

// ─── loop queue semantics ─────────────────────────────────────────────────────

// TestMarkEntryConsumed_WritesEntry verifies that markEntryConsumed sets
// Consumed=true and populates ConsumedAt/ConsumedBy fields, removing the
// entry from the unconsumed set.
func TestMarkEntryConsumed_WritesEntry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	// Seed with one unconsumed entry.
	entry := nextWorkEntry{
		SourceEpic: "ag-test",
		Timestamp:  "2026-02-19T00:00:00Z",
		Items: []nextWorkItem{
			{Title: "Do the thing", Severity: "high"},
		},
		Consumed: false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Mark consumed (index 0 = the only entry).
	runID := "test-run-consume"
	if err := markEntryConsumed(path, 0, runID); err != nil {
		t.Fatalf("markEntryConsumed: %v", err)
	}

	// Re-read the file — after marking consumed the entry should be gone from
	// unconsumed items.
	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("readUnconsumedItems after consume: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 unconsumed items after mark, got %d", len(items))
	}
}

// TestMarkEntryConsumed_MissingFile verifies markEntryConsumed returns an error
// for a missing file (it does a stat pre-check so callers can distinguish a
// missing-queue situation from a successful no-op).
func TestMarkEntryConsumed_MissingFile(t *testing.T) {
	err := markEntryConsumed("/nonexistent/next-work.jsonl", 0, "run-id")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// TestMarkEntryConsumed_WrongIndex verifies that marking an entry at a
// non-existent index is a safe no-op — the existing entries are unchanged.
func TestMarkEntryConsumed_WrongIndex(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-real-epic",
		Items:      []nextWorkItem{{Title: "Item", Severity: "low"}},
		Consumed:   false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// Index 99 does not exist — should not error, just no-op.
	if err := markEntryConsumed(path, 99, "run-id"); err != nil {
		t.Fatalf("markEntryConsumed with wrong index: %v", err)
	}

	// Original entry should still be unconsumed.
	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Error("original entry should still be unconsumed after marking wrong index")
	}
}

// TestMarkEntryFailed_SetsFailedAt verifies that markEntryFailed records a
// FailedAt timestamp but leaves Consumed=false, so the entry can be retried.
func TestMarkEntryFailed_SetsFailedAt(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entry := nextWorkEntry{
		SourceEpic: "ag-fail-test",
		Items:      []nextWorkItem{{Title: "Failing item", Severity: "medium"}},
		Consumed:   false,
	}
	data, _ := json.Marshal(entry)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	if err := markEntryFailed(path, 0); err != nil {
		t.Fatalf("markEntryFailed: %v", err)
	}

	// After marking failed, the entry should NOT appear in unconsumed items
	// (failed entries are excluded by readQueueEntries).
	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatalf("readUnconsumedItems after fail: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items after marking failed, got %d", len(items))
	}
}

// TestMarkEntryConsumed_MultipleEntries verifies that marking one entry
// consumed does not affect adjacent entries.
func TestMarkEntryConsumed_MultipleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	lines := []nextWorkEntry{
		{SourceEpic: "ag-1", Items: []nextWorkItem{{Title: "Item 1", Severity: "high"}}, Consumed: false},
		{SourceEpic: "ag-2", Items: []nextWorkItem{{Title: "Item 2", Severity: "medium"}}, Consumed: false},
		{SourceEpic: "ag-3", Items: []nextWorkItem{{Title: "Item 3", Severity: "low"}}, Consumed: false},
	}
	var content []string
	for _, e := range lines {
		d, _ := json.Marshal(e)
		content = append(content, string(d))
	}
	if err := os.WriteFile(path, []byte(strings.Join(content, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mark only the middle entry (index 1) consumed.
	if err := markEntryConsumed(path, 1, "run-middle"); err != nil {
		t.Fatalf("markEntryConsumed: %v", err)
	}

	// Items from entries 0 and 2 should still be unconsumed.
	items, err := readUnconsumedItems(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 remaining unconsumed items, got %d", len(items))
	}
	for _, item := range items {
		if item.Title == "Item 2" {
			t.Error("Item 2 (from consumed entry) should not be in unconsumed list")
		}
	}
}

// TestReadUnconsumedItems_FilterByRepo verifies that targetRepo filtering
// works correctly when a non-empty filter is provided.
func TestReadUnconsumedItems_FilterByRepo(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "next-work.jsonl")

	entries := []nextWorkEntry{
		{
			SourceEpic: "ag-1",
			Items:      []nextWorkItem{{Title: "For repo-A", TargetRepo: "repo-A", Severity: "high"}},
			Consumed:   false,
		},
		{
			SourceEpic: "ag-2",
			Items:      []nextWorkItem{{Title: "For repo-B", TargetRepo: "repo-B", Severity: "high"}},
			Consumed:   false,
		},
		{
			SourceEpic: "ag-3",
			Items:      []nextWorkItem{{Title: "No target repo", Severity: "high"}},
			Consumed:   false,
		},
	}

	var lines []string
	for _, e := range entries {
		d, _ := json.Marshal(e)
		lines = append(lines, string(d))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Filter for repo-A: should return only items from ag-1.
	items, err := readUnconsumedItems(path, "repo-A")
	if err != nil {
		t.Fatalf("readUnconsumedItems: %v", err)
	}
	for _, item := range items {
		if item.TargetRepo != "" && item.TargetRepo != "repo-A" {
			t.Errorf("filter leak: got item with TargetRepo=%q", item.TargetRepo)
		}
	}
}

// ─── logPhaseTransition ───────────────────────────────────────────────────────

// TestLogPhaseTransition_WithRunID verifies that logPhaseTransition writes a
// log entry containing the runID and phase details.
func TestLogPhaseTransition_WithRunID(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	logPhaseTransition(logPath, "run-abc123", "discovery", "started research phase")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "run-abc123") {
		t.Errorf("log should contain runID, got: %q", content)
	}
	if !strings.Contains(content, "discovery") {
		t.Errorf("log should contain phase, got: %q", content)
	}
	if !strings.Contains(content, "started research phase") {
		t.Errorf("log should contain details, got: %q", content)
	}
}

// TestLogPhaseTransition_WithoutRunID verifies that logPhaseTransition works
// correctly when runID is empty (pre-run or legacy path).
func TestLogPhaseTransition_WithoutRunID(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	logPhaseTransition(logPath, "", "backend-selection", "backend=direct reason=\"runtime=auto live-status disabled\"")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "backend-selection") {
		t.Errorf("log should contain phase, got: %q", content)
	}
}

// TestLogPhaseTransition_Appends verifies that multiple calls append entries
// rather than overwriting the file.
func TestLogPhaseTransition_Appends(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 5; i++ {
		logPhaseTransition(logPath, "run-append", fmt.Sprintf("phase-%d", i), fmt.Sprintf("detail %d", i))
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 5 {
		t.Errorf("expected at least 5 log lines, got %d:\n%s", len(lines), string(data))
	}
}

// TestLogPhaseTransition_NonWritablePath verifies that logPhaseTransition does
// not panic when the log file cannot be opened (e.g., bad path).
func TestLogPhaseTransition_NonWritablePath(t *testing.T) {
	// Pass a path in a non-existent directory — should fail silently.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("logPhaseTransition panicked: %v", r)
		}
	}()
	logPhaseTransition("/nonexistent/dir/log.txt", "run-x", "test", "details")
}
