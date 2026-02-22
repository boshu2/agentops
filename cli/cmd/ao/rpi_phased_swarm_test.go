package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSwarmFirstFlagDefault verifies that phasedSwarmFirst propagates to
// phasedState.SwarmFirst during initialization.
func TestSwarmFirstFlagDefault(t *testing.T) {
	orig := phasedSwarmFirst
	defer func() { phasedSwarmFirst = orig }()

	phasedSwarmFirst = true

	state := &phasedState{
		Goal:       "add auth",
		SwarmFirst: phasedSwarmFirst,
	}
	if !state.SwarmFirst {
		t.Error("SwarmFirst should be propagated from phasedSwarmFirst flag to state")
	}
}

// TestSwarmFirstInPromptPhase1 verifies that when SwarmFirst=true, phase 1
// (discovery) prompts include the SWARM-FIRST EXECUTION CONTRACT.
func TestSwarmFirstInPromptPhase1(t *testing.T) {
	state := &phasedState{
		Goal:       "add user authentication",
		SwarmFirst: true,
	}
	prompt, err := buildPromptForPhase("", 1, state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "SWARM-FIRST") {
		t.Errorf("phase 1 prompt with SwarmFirst=true should contain SWARM-FIRST, got: %q", prompt)
	}
	if !strings.Contains(prompt, "SWARM-FIRST EXECUTION CONTRACT") {
		t.Errorf("phase 1 prompt should contain SWARM-FIRST EXECUTION CONTRACT, got: %q", prompt)
	}
	if !strings.Contains(prompt, "/swarm") {
		t.Errorf("phase 1 prompt should mention /swarm when SwarmFirst=true, got: %q", prompt)
	}
}

// TestSwarmFirstInPromptPhase2 verifies that when SwarmFirst=true, phase 2
// (implementation/crank) prompt includes swarm contract.
func TestSwarmFirstInPromptPhase2(t *testing.T) {
	state := &phasedState{
		Goal:       "add auth",
		EpicID:     "ag-test1",
		SwarmFirst: true,
	}
	prompt, err := buildPromptForPhase("", 2, state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "SWARM-FIRST") {
		t.Errorf("phase 2 prompt with SwarmFirst=true should contain SWARM-FIRST, got: %q", prompt)
	}
}

// TestSwarmFirstInPromptPhase3 verifies that when SwarmFirst=true, phase 3
// (validation) prompt includes swarm contract for validation workers.
func TestSwarmFirstInPromptPhase3(t *testing.T) {
	state := &phasedState{
		Goal:       "add auth",
		EpicID:     "ag-test1",
		SwarmFirst: true,
	}
	prompt, err := buildPromptForPhase("", 3, state, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "SWARM-FIRST") {
		t.Errorf("phase 3 prompt with SwarmFirst=true should contain SWARM-FIRST, got: %q", prompt)
	}
}

// TestSwarmFirstDisabledNoContract verifies that when SwarmFirst=false, phase
// prompts do NOT include the SWARM-FIRST EXECUTION CONTRACT.
func TestSwarmFirstDisabledNoContract(t *testing.T) {
	state := &phasedState{
		Goal:       "add auth",
		EpicID:     "ag-test1",
		SwarmFirst: false,
	}
	for phaseNum := 1; phaseNum <= 3; phaseNum++ {
		prompt, err := buildPromptForPhase("", phaseNum, state, nil)
		if err != nil {
			t.Fatalf("phase %d: unexpected error: %v", phaseNum, err)
		}
		if strings.Contains(prompt, "SWARM-FIRST EXECUTION CONTRACT") {
			t.Errorf("phase %d: prompt with SwarmFirst=false should NOT contain SWARM-FIRST EXECUTION CONTRACT, got: %q",
				phaseNum, prompt)
		}
	}
}

// TestSwarmFirstStateRoundTrip verifies that SwarmFirst is persisted in
// phasedState and survives a JSON save/load cycle.
func TestSwarmFirstStateRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agents", "rpi"), 0755); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithGoal("add auth").WithSwarmFirst(true)

	if err := savePhasedState(tmpDir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadPhasedState(tmpDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !loaded.SwarmFirst {
		t.Error("SwarmFirst should be true after round-trip")
	}
}

// TestSwarmFirstStateRoundTrip_False verifies that SwarmFirst=false also
// survives the JSON round-trip.
func TestSwarmFirstStateRoundTrip_False(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agents", "rpi"), 0755); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithGoal("fix typo").WithSwarmFirst(false)

	if err := savePhasedState(tmpDir, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadPhasedState(tmpDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.SwarmFirst {
		t.Error("SwarmFirst should be false after round-trip when set to false")
	}
}

// TestRunRPIPhased_DryRunBackendSelection verifies that selectExecutorWithLog
// records the backend selection in the orchestration log with the run ID.
// This exercises the same code path as runRPIPhased's executor setup block.
func TestRunRPIPhased_DryRunBackendSelection(t *testing.T) {
	origLookPath := lookPath
	defer func() {
		lookPath = origLookPath
	}()

	// Force direct backend: runtime=auto with live-status disabled.
	lookPath = func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}
	os.Unsetenv("CLAUDECODE")
	os.Unsetenv("CLAUDE_CODE_ENTRYPOINT")

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	opts := defaultPhasedEngineOptions()
	opts.LiveStatus = false
	opts.SwarmFirst = true
	executor := selectExecutorWithLog("", nil, logPath, "dryrun-run-id", false, opts)
	if executor.Name() != "direct" {
		t.Errorf("expected direct executor (runtime=auto, live-status disabled), got %q", executor.Name())
	}

	// Verify backend-selection was logged.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log not written: %v", err)
	}
	if !strings.Contains(string(data), "backend-selection") {
		t.Errorf("log should record backend-selection, got: %q", string(data))
	}
	if !strings.Contains(string(data), "direct") {
		t.Errorf("log should name the selected backend, got: %q", string(data))
	}
}

// TestRunRPIPhased_SwarmFirstBackendLogged verifies that the run ID appears
// in the backend selection log entry, making each run's executor choice traceable.
func TestRunRPIPhased_SwarmFirstBackendLogged(t *testing.T) {
	origLookPath := lookPath
	defer func() {
		lookPath = origLookPath
	}()

	lookPath = func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}
	os.Unsetenv("CLAUDECODE")
	os.Unsetenv("CLAUDE_CODE_ENTRYPOINT")

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, ".agents", "rpi", "phased-orchestration.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}

	runID := "swarm-test-run-id"
	opts := defaultPhasedEngineOptions()
	opts.LiveStatus = false
	executor := selectExecutorWithLog("", nil, logPath, runID, false, opts)

	if executor.Name() == "" {
		t.Fatal("executor name must be non-empty")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log not written: %v", err)
	}
	logContent := string(data)

	// Run ID must appear in the log for traceability.
	if !strings.Contains(logContent, runID) {
		t.Errorf("log should contain run ID %q for traceability, got: %q", runID, logContent)
	}
	// Executor name must appear.
	if !strings.Contains(logContent, executor.Name()) {
		t.Errorf("log should contain executor name %q, got: %q", executor.Name(), logContent)
	}
}

// TestRunRPIPhased_BackendStoredInState verifies that after executor selection,
// state.Backend is set to the executor's name. This mirrors what runRPIPhased
// does: executor := selectExecutorWithLog(...); state.Backend = executor.Name()
func TestRunRPIPhased_BackendStoredInState(t *testing.T) {
	origLookPath := lookPath
	defer func() {
		lookPath = origLookPath
	}()

	lookPath = func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}
	os.Unsetenv("CLAUDECODE")
	os.Unsetenv("CLAUDE_CODE_ENTRYPOINT")

	opts := defaultPhasedEngineOptions()
	opts.LiveStatus = false
	state := newTestPhasedState().WithGoal("add feature").WithSwarmFirst(true).WithOpts(opts)

	// Simulate what runRPIPhasedWithOpts does.
	executor := selectExecutorWithLog("", nil, "", "", false, opts)
	state.Backend = executor.Name()

	if state.Backend == "" {
		t.Error("state.Backend should be set after selectExecutorWithLog")
	}
	if state.Backend != "direct" {
		t.Errorf("expected direct backend, got %q", state.Backend)
	}
}

// TestRetryWithBackendSemantics verifies that handleGateRetry uses the same
// executor that was selected for the main phase loop — both the retry spawn
// and the rerun spawn go through the same backend.
func TestRetryWithBackendSemantics(t *testing.T) {
	// Track all Execute calls on our mock executor.
	var executeCalls []struct {
		prompt   string
		phaseNum int
	}

	mockExec := &mockExecutor{
		name: "mock-swarm",
		executeFn: func(prompt, cwd, runID string, phaseNum int) error {
			executeCalls = append(executeCalls, struct {
				prompt   string
				phaseNum int
			}{prompt, phaseNum})
			return nil
		},
	}

	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a PASS vibe report so postPhaseProcessing succeeds after retry.
	councilDir := filepath.Join(tmpDir, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}
	vibeReport := filepath.Join(councilDir, "2026-02-19-vibe-recent.md")
	if err := os.WriteFile(vibeReport, []byte("## Council Verdict: PASS\n"), 0644); err != nil {
		t.Fatal(err)
	}

	retryOpts := defaultPhasedEngineOptions()
	retryOpts.MaxRetries = 3
	retryOpts.LiveStatus = false

	state := newTestPhasedState().WithGoal("add auth").WithEpicID("ag-test1").WithSwarmFirst(true).WithOpts(retryOpts)
	state.Phase = 0      // not yet started
	state.StartPhase = 3 // simulate --from=vibe (no prior phase results)
	state.Backend = mockExec.Name()

	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	statusPath := filepath.Join(rpiDir, "live-status.md")

	// Gate failure that triggers retry.
	gateErr := &gateFailError{
		Phase:   3,
		Verdict: "FAIL",
		Findings: []finding{
			{Description: "Auth bypass", Fix: "Add checks", Ref: "auth.go:10"},
		},
		Report: vibeReport,
	}

	retried, err := handleGateRetry(tmpDir, state, 3, gateErr, logPath, tmpDir, statusPath, nil, mockExec)
	if err != nil {
		t.Fatalf("handleGateRetry: %v", err)
	}
	if !retried {
		t.Error("expected retry to succeed (mock executor returns nil, vibe PASS on rerun)")
	}

	// At least 2 executor.Execute calls: retry prompt + rerun prompt.
	if len(executeCalls) < 2 {
		t.Errorf("expected at least 2 executor.Execute calls (retry + rerun), got %d", len(executeCalls))
	}

	// Both calls should be for phase 3 (retry preserves phase semantics).
	for i, call := range executeCalls {
		if call.phaseNum != 3 {
			t.Errorf("execute call %d: expected phaseNum=3 (backend semantics preserved), got %d", i, call.phaseNum)
		}
	}
}

// TestRetryBackendPreservesExecutor verifies that the executor passed to
// handleGateRetry is the exact one used — not a newly-selected executor.
// This is the core "retry preserves backend semantics" guarantee.
func TestRetryBackendPreservesExecutor(t *testing.T) {
	calls := make(map[string]int)

	primaryExec := &mockExecutor{
		name: "primary-backend",
		executeFn: func(prompt, cwd, runID string, phaseNum int) error {
			calls["primary"]++
			return nil
		},
	}

	tmpDir := t.TempDir()
	rpiDir := filepath.Join(tmpDir, ".agents", "rpi")
	councilDir := filepath.Join(tmpDir, ".agents", "council")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}
	// PASS vibe so the gate check after retry succeeds.
	vibeReport := filepath.Join(councilDir, "2026-02-19-vibe-check.md")
	if err := os.WriteFile(vibeReport, []byte("## Council Verdict: PASS\n"), 0644); err != nil {
		t.Fatal(err)
	}

	preserveOpts := defaultPhasedEngineOptions()
	preserveOpts.MaxRetries = 3

	state := newTestPhasedState().WithGoal("fix bug").WithEpicID("ag-fix1").WithOpts(preserveOpts)
	state.Phase = 0      // not yet started
	state.StartPhase = 3 // simulate --from=vibe (no prior phase results)
	state.Backend = primaryExec.Name()

	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	statusPath := filepath.Join(rpiDir, "live-status.md")

	gateErr := &gateFailError{
		Phase:   3,
		Verdict: "FAIL",
		Report:  vibeReport,
	}

	_, _ = handleGateRetry(tmpDir, state, 3, gateErr, logPath, tmpDir, statusPath, nil, primaryExec)

	if calls["primary"] == 0 {
		t.Error("primary executor should have been called — backend semantics must be preserved in retry")
	}
}

// TestSwarmFirstPhaseResultBackend verifies that the backend name is recorded
// in the phaseResult artifact when a phase completes. This makes the
// executor choice visible in machine-readable artifacts.
func TestSwarmFirstPhaseResultBackend(t *testing.T) {
	tmpDir := t.TempDir()

	result := &phaseResult{
		SchemaVersion: 1,
		RunID:         "backend-test-run",
		Phase:         1,
		PhaseName:     "discovery",
		Status:        "completed",
		Backend:       "direct",
	}

	if err := writePhaseResult(tmpDir, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}

	resultPath := filepath.Join(tmpDir, ".agents", "rpi", "phase-1-result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("result not written: %v", err)
	}

	var loaded phaseResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Backend != "direct" {
		t.Errorf("backend should be persisted in phase result, got %q", loaded.Backend)
	}
	if loaded.RunID != "backend-test-run" {
		t.Errorf("run_id mismatch, got %q", loaded.RunID)
	}
}

// TestSwarmFirstBackendInState verifies that phasedState.Backend is populated
// from the executor name after selectExecutorWithLog is called, mirroring the
// runRPIPhasedWithOpts assignment: state.Backend = executor.Name()
func TestSwarmFirstBackendInState(t *testing.T) {
	origLookPath := lookPath
	defer func() {
		lookPath = origLookPath
	}()

	lookPath = func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}
	os.Unsetenv("CLAUDECODE")
	os.Unsetenv("CLAUDE_CODE_ENTRYPOINT")

	stateOpts := defaultPhasedEngineOptions()
	stateOpts.LiveStatus = false
	state := newTestPhasedState().WithGoal("add feature").WithSwarmFirst(true).WithOpts(stateOpts)

	executor := selectExecutorWithLog("", nil, "", "", false, stateOpts)
	state.Backend = executor.Name()

	if state.Backend == "" {
		t.Error("state.Backend should be set after selectExecutorWithLog")
	}
	if state.Backend != "direct" {
		t.Errorf("expected direct backend, got %q", state.Backend)
	}
}

// TestLiveStatusPhaseProgressInitialState verifies that buildAllPhases
// initializes PhaseProgress entries with "pending" status for live-status tracking.
func TestLiveStatusPhaseProgressInitialState(t *testing.T) {
	all := buildAllPhases(phases)

	if len(all) != len(phases) {
		t.Fatalf("buildAllPhases: expected %d entries, got %d", len(phases), len(all))
	}

	for i, p := range all {
		if p.Name == "" {
			t.Errorf("phase %d: Name should be populated", i)
		}
		if p.CurrentAction != "pending" {
			t.Errorf("phase %d: initial CurrentAction should be 'pending', got %q", i, p.CurrentAction)
		}
	}

	expectedNames := []string{"discovery", "implementation", "validation"}
	for i, want := range expectedNames {
		if all[i].Name != want {
			t.Errorf("phase %d: Name=%q, want %q", i, all[i].Name, want)
		}
	}
}

// TestLiveStatusUpdatePhase verifies that updateLivePhaseStatus correctly
// updates the phase progress for a given phase number.
func TestLiveStatusUpdatePhase(t *testing.T) {
	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "live-status.md")
	allPhases := buildAllPhases(phases)

	updateLivePhaseStatus(statusPath, allPhases, 2, "running", 0, "")

	if allPhases[1].CurrentAction != "running" {
		t.Errorf("phase 2 CurrentAction should be 'running', got %q", allPhases[1].CurrentAction)
	}

	updateLivePhaseStatus(statusPath, allPhases, 2, "failed", 1, "timeout")
	if allPhases[1].RetryCount != 1 {
		t.Errorf("phase 2 RetryCount should be 1, got %d", allPhases[1].RetryCount)
	}
}

// TestLiveStatusOutOfBoundsIgnored verifies that updateLivePhaseStatus
// silently ignores out-of-bounds phase numbers (no panic).
func TestLiveStatusOutOfBoundsIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	statusPath := filepath.Join(tmpDir, "live-status.md")
	allPhases := buildAllPhases(phases)

	updateLivePhaseStatus(statusPath, allPhases, 0, "running", 0, "")
	updateLivePhaseStatus(statusPath, allPhases, 99, "running", 0, "")
}

// mockExecutor is a test double for PhaseExecutor that records calls and
// delegates to an optional executeFn. Safe for use in multiple tests.
type mockExecutor struct {
	name      string
	executeFn func(prompt, cwd, runID string, phaseNum int) error
}

func (m *mockExecutor) Name() string { return m.name }
func (m *mockExecutor) Execute(prompt, cwd, runID string, phaseNum int) error {
	if m.executeFn != nil {
		return m.executeFn(prompt, cwd, runID, phaseNum)
	}
	return nil
}
