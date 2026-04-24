package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- handleDryRunPhase ---

func TestHandleDryRunPhase_DryRunActive(t *testing.T) {
	origDryRun := dryRun
	dryRun = true
	defer func() { dryRun = origDryRun }()

	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "rpi.log")
	state := newTestPhasedState().WithRunID("dry-test")
	p := phases[0]
	opts := phasedEngineOptions{}

	result := handleDryRunPhase(tmp, state, 1, p, opts, "test prompt", logPath)
	if !result {
		t.Error("handleDryRunPhase should return true in dry-run mode")
	}

	// Verify log was written.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if !strings.Contains(string(data), "dry-run") {
		t.Errorf("log should contain 'dry-run', got: %s", string(data))
	}
}

func TestHandleDryRunPhase_DryRunInactive(t *testing.T) {
	origDryRun := dryRun
	dryRun = false
	defer func() { dryRun = origDryRun }()

	state := newTestPhasedState()
	p := phases[0]
	opts := phasedEngineOptions{}

	result := handleDryRunPhase(t.TempDir(), state, 1, p, opts, "test prompt", "")
	if result {
		t.Error("handleDryRunPhase should return false when dry-run is off")
	}
}

// --- maybeLiveStatus ---

func TestMaybeLiveStatus_Enabled(t *testing.T) {
	tmp := t.TempDir()
	statusPath := filepath.Join(tmp, "status.md")
	allPhases := buildAllPhases(phases)
	opts := phasedEngineOptions{LiveStatus: true}

	// Should not panic and should create/update the status file.
	maybeLiveStatus(opts, statusPath, allPhases, 1, "running", 0, "")

	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Error("status file should be created when LiveStatus is enabled")
	}
}

func TestMaybeLiveStatus_Disabled(t *testing.T) {
	tmp := t.TempDir()
	statusPath := filepath.Join(tmp, "status.md")
	opts := phasedEngineOptions{LiveStatus: false}

	maybeLiveStatus(opts, statusPath, nil, 1, "running", 0, "")

	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Error("status file should NOT be created when LiveStatus is disabled")
	}
}

type blockingPhaseExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingPhaseExecutor) Name() string { return "blocking" }

func (b *blockingPhaseExecutor) Execute(_ context.Context, _, _, _ string, _ int) error {
	close(b.started)
	<-b.release
	return nil
}

func TestExecutePhaseSession_RefreshesHeartbeatWhileRunning(t *testing.T) {
	prevInterval := phaseHeartbeatInterval
	phaseHeartbeatInterval = 10 * time.Millisecond
	defer func() { phaseHeartbeatInterval = prevInterval }()

	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o750); err != nil {
		t.Fatal(err)
	}
	state := newTestPhasedState().WithRunID("heartbeat-refresh")
	executor := &blockingPhaseExecutor{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	done := make(chan error, 1)
	go func() {
		done <- executePhaseSession(context.Background(), tmp, state, phases[0], phasedEngineOptions{}, filepath.Join(rpiDir, "live-status.md"), buildAllPhases(phases), filepath.Join(rpiDir, "phased-orchestration.log"), "prompt", executor)
	}()

	<-executor.started
	heartbeatPath := filepath.Join(tmp, ".agents", "rpi", "runs", "heartbeat-refresh", "heartbeat.txt")
	initial := waitForHeartbeatContent(t, heartbeatPath)
	refreshed := waitForHeartbeatChange(t, heartbeatPath, initial)

	close(executor.release)
	if err := <-done; err != nil {
		t.Fatalf("executePhaseSession returned error: %v", err)
	}
	if refreshed == initial {
		t.Fatal("expected heartbeat content to refresh while phase executor was still running")
	}
}

func waitForHeartbeatContent(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return strings.TrimSpace(string(data))
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("heartbeat %s was not written before timeout", path)
	return ""
}

func waitForHeartbeatChange(t *testing.T, path, initial string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			current := strings.TrimSpace(string(data))
			if current != "" && current != initial {
				return current
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("heartbeat %s did not change before timeout", path)
	return ""
}

// --- writeFinalPhasedReport ---

func TestWriteFinalPhasedReport_BasicOutput(t *testing.T) {
	state := newTestPhasedState().WithGoal("test goal").WithEpicID("ag-99").
		WithVerdicts(map[string]string{"pre_mortem": "PASS", "vibe": "PASS"})

	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "rpi.log")

	// Capture stdout — we just verify no panic.
	writeFinalPhasedReport(state, logPath)

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if !strings.Contains(string(data), "complete") {
		t.Errorf("log should contain 'complete', got: %s", string(data))
	}
}

func TestWriteFinalPhasedReport_PlanFileEpic(t *testing.T) {
	state := newTestPhasedState().WithGoal("test").WithEpicID("plan:.agents/plans/feature.md")

	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "rpi.log")

	writeFinalPhasedReport(state, logPath)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "complete") {
		t.Errorf("log missing 'complete' marker, got: %s", string(data))
	}
	if !strings.Contains(string(data), "feature.md") {
		t.Errorf("log missing plan file reference, got: %s", string(data))
	}
}

func TestWriteFinalPhasedReport_EmptyEpicID(t *testing.T) {
	state := newTestPhasedState().WithGoal("test")

	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "rpi.log")

	writeFinalPhasedReport(state, logPath)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "complete") {
		t.Errorf("log missing 'complete' marker, got: %s", string(data))
	}
}

// --- logAndFailPhase ---

func TestLogAndFailPhase_SetsTerminalFields(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(stateDir, "rpi.log")

	state := newTestPhasedState().WithRunID("fail-test")

	origErr := logAndFailPhase(state, "implementation", logPath, tmp, errFakeExecFailure)
	if origErr == nil {
		t.Fatal("logAndFailPhase should return the original error")
	}
	if state.TerminalStatus != "failed" {
		t.Errorf("TerminalStatus = %q, want %q", state.TerminalStatus, "failed")
	}
	if state.TerminalReason == "" {
		t.Error("TerminalReason should be set")
	}
	if !strings.Contains(state.TerminalReason, "implementation") {
		t.Errorf("TerminalReason = %q, should mention phase name", state.TerminalReason)
	}
	if state.TerminatedAt == "" {
		t.Error("TerminatedAt should be set")
	}
}

// --- rescuePhaseOnTimeout helpers ---

func TestIsPhaseTimeoutError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"timeout direct", fmt.Errorf("phase 2 timed out after 1h30m0s (set --phase-timeout to increase)"), true},
		{"timeout stream", fmt.Errorf("phase 2 (timeout) timed out after 45m0s (set --phase-timeout to increase)"), true},
		{"non-timeout error", fmt.Errorf("phase 2 failed: exit code 1"), false},
		{"stall error", fmt.Errorf("phase 2 (stall): stall detected: no stream activity for 10m"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPhaseTimeoutError(tc.err); got != tc.want {
				t.Errorf("isPhaseTimeoutError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestPhaseSummaryExists(t *testing.T) {
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		t.Fatal(err)
	}

	if phaseSummaryExists(tmp, 2) {
		t.Error("should return false when no summary file exists")
	}

	summaryPath := filepath.Join(rpiDir, "phase-2-summary.md")
	if err := os.WriteFile(summaryPath, []byte("# Phase 2 Summary\nDone."), 0600); err != nil {
		t.Fatal(err)
	}

	if !phaseSummaryExists(tmp, 2) {
		t.Error("should return true when phase-2-summary.md exists")
	}
	if phaseSummaryExists(tmp, 1) {
		t.Error("should return false for phase 1 when only phase 2 summary exists")
	}
}

func TestRescuePhaseOnTimeout_NoSummary(t *testing.T) {
	tmp := t.TempDir()
	p := phases[1] // phase 2: implementation
	timeoutErr := fmt.Errorf("phase 2 timed out after 1h30m0s (set --phase-timeout to increase)")

	if rescuePhaseOnTimeout(tmp, p, timeoutErr) {
		t.Error("rescue should fail when no phase summary exists")
	}
}

func TestRescuePhaseOnTimeout_WithSummary(t *testing.T) {
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		t.Fatal(err)
	}
	summaryPath := filepath.Join(rpiDir, "phase-2-summary.md")
	if err := os.WriteFile(summaryPath, []byte("# Phase 2 Summary\nCrank completed all 29 issues."), 0600); err != nil {
		t.Fatal(err)
	}

	p := phases[1] // phase 2: implementation
	timeoutErr := fmt.Errorf("phase 2 timed out after 1h30m0s (set --phase-timeout to increase)")

	if !rescuePhaseOnTimeout(tmp, p, timeoutErr) {
		t.Error("rescue should succeed when phase summary exists and error is timeout")
	}
}

func TestRescuePhaseOnTimeout_NonTimeoutError(t *testing.T) {
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Write a summary — rescue should still fail because error is not a timeout.
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-summary.md"), []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}

	p := phases[1]
	execErr := fmt.Errorf("claude exited with code 1")

	if rescuePhaseOnTimeout(tmp, p, execErr) {
		t.Error("rescue should not trigger for non-timeout errors, even with summary present")
	}
}

// --- runPhaseLoop ---

// --- handlePostPhaseGate ---

func TestHandlePostPhaseGate_Pass(t *testing.T) {
	// Use phase 99 (unknown) so postPhaseProcessing returns nil,
	// simulating a gate that passes without error.
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(stateDir, "phased-orchestration.log")
	statusPath := filepath.Join(stateDir, "live-status.md")

	state := newTestPhasedState().WithRunID("gate-pass").WithMaxRetries(3)
	allPhases := buildAllPhases(phases)
	executor := &fakeExecutor{}
	p := phase{Num: 99, Name: "unknown-phase", Step: "none"}

	err := handlePostPhaseGate(context.Background(), tmp, state, p, logPath, statusPath, allPhases, executor)
	if err != nil {
		t.Fatalf("handlePostPhaseGate should return nil for passing gate, got: %v", err)
	}
}

func TestHandlePostPhaseGate_Fail(t *testing.T) {
	// Set up validation phase (3) with a FAIL council report. postPhaseProcessing
	// returns a gateFailError, which triggers retry logic. With attempts pre-loaded
	// to maxGateRetryDepth, handleGateRetry hits forced escalation and returns
	// (false, nil), causing handlePostPhaseGate to return the "gate failed after
	// max retries" error.
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create prior phase result for prerequisite check.
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-result.json"), []byte(`{"status":"ok"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a council report with FAIL verdict.
	councilDir := filepath.Join(tmp, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}
	reportContent := "# Vibe\n## Council Verdict: FAIL\nCritical issues found.\n"
	if err := os.WriteFile(filepath.Join(councilDir, "2026-03-05-vibe-gate-fail.md"), []byte(reportContent), 0644); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	statusPath := filepath.Join(rpiDir, "live-status.md")

	state := newTestPhasedState().WithRunID("gate-fail").WithMaxRetries(3).WithPhase(3).WithStartPhase(3)
	// Pre-load attempts so escalation triggers immediately.
	state.Attempts["phase_3"] = maxGateRetryDepth
	allPhases := buildAllPhases(phases)
	executor := &fakeExecutor{}
	p := phases[2] // phase 3: validation

	t.Setenv("MEMRL_MODE", "off")

	err := handlePostPhaseGate(context.Background(), tmp, state, p, logPath, statusPath, allPhases, executor)
	if err == nil {
		t.Fatal("handlePostPhaseGate should return error when gate fails after max retries")
	}
	if !strings.Contains(err.Error(), "gate failed after max retries") {
		t.Errorf("error = %q, should contain 'gate failed after max retries'", err.Error())
	}
}

func TestRunPhaseLoop_FastPathRunsPhase3(t *testing.T) {
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(stateDir, "rpi.log")
	statusPath := filepath.Join(stateDir, "status.md")

	origDryRun := dryRun
	dryRun = true
	defer func() { dryRun = origDryRun }()

	state := newTestPhasedState().
		WithGoal("trivial fix").
		WithRunID("fast-test").
		WithFastPath(true).
		WithEpicID("ag-fast-1")
	state.Complexity = ComplexityFast

	allPhases := buildAllPhases(phases)
	opts := phasedEngineOptions{NoWorktree: true}
	executor := &fakeExecutor{}

	// Run from phase 1 through all phases in dry-run mode.
	err := runPhaseLoop(context.Background(), tmp, tmp, state, 1, opts, statusPath, allPhases, logPath, executor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify phase 3 ran in dry-run mode instead of being skipped.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	log := string(data)
	if !strings.Contains(log, "validation: started") {
		t.Errorf("log should show phase 3 validation starting, got: %s", log)
	}
	if !strings.Contains(log, "validation: dry-run") {
		t.Errorf("log should show phase 3 validation dry-run, got: %s", log)
	}
	if strings.Contains(log, "validation: skipped") {
		t.Errorf("phase 3 validation should not be skipped, got: %s", log)
	}
}
