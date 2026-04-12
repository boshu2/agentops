package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/types"
)

// --- legacyGateAction ---

func TestLegacyGateAction_RetryWhenBelowMax(t *testing.T) {
	got := legacyGateAction(1, 3)
	if got != types.MemRLActionRetry {
		t.Errorf("legacyGateAction(1,3) = %q, want %q", got, types.MemRLActionRetry)
	}
}

func TestLegacyGateAction_EscalateWhenAtMax(t *testing.T) {
	got := legacyGateAction(3, 3)
	if got != types.MemRLActionEscalate {
		t.Errorf("legacyGateAction(3,3) = %q, want %q", got, types.MemRLActionEscalate)
	}
}

func TestLegacyGateAction_EscalateWhenAboveMax(t *testing.T) {
	got := legacyGateAction(5, 3)
	if got != types.MemRLActionEscalate {
		t.Errorf("legacyGateAction(5,3) = %q, want %q", got, types.MemRLActionEscalate)
	}
}

// --- classifyGateFailureClass ---

func TestClassifyGateFailureClass_NilError(t *testing.T) {
	got := classifyGateFailureClass(1, nil)
	if got != "" {
		t.Errorf("classifyGateFailureClass(1, nil) = %q, want empty", got)
	}
}

func TestClassifyGateFailureClass_PreMortemFail(t *testing.T) {
	gateErr := &gateFailError{Phase: 1, Verdict: "FAIL"}
	got := classifyGateFailureClass(1, gateErr)
	if got != types.MemRLFailureClassPreMortemFail {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassPreMortemFail)
	}
}

func TestClassifyGateFailureClass_CrankBlocked(t *testing.T) {
	gateErr := &gateFailError{Phase: 2, Verdict: "BLOCKED"}
	got := classifyGateFailureClass(2, gateErr)
	if got != types.MemRLFailureClassCrankBlocked {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassCrankBlocked)
	}
}

func TestClassifyGateFailureClass_CrankPartial(t *testing.T) {
	gateErr := &gateFailError{Phase: 2, Verdict: "PARTIAL"}
	got := classifyGateFailureClass(2, gateErr)
	if got != types.MemRLFailureClassCrankPartial {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassCrankPartial)
	}
}

func TestClassifyGateFailureClass_VibeFail(t *testing.T) {
	gateErr := &gateFailError{Phase: 3, Verdict: "FAIL"}
	got := classifyGateFailureClass(3, gateErr)
	if got != types.MemRLFailureClassVibeFail {
		t.Errorf("got %q, want %q", got, types.MemRLFailureClassVibeFail)
	}
}

func TestClassifyGateFailureClass_WhitespaceHandling(t *testing.T) {
	gateErr := &gateFailError{Phase: 1, Verdict: "  FAIL  "}
	got := classifyGateFailureClass(1, gateErr)
	if got != types.MemRLFailureClassPreMortemFail {
		t.Errorf("got %q (whitespace should be trimmed), want %q", got, types.MemRLFailureClassPreMortemFail)
	}
}

// --- resolveGateRetryAction ---

func TestResolveGateRetryAction_ModeOffUsesLegacy(t *testing.T) {
	// When memrl mode is off, the legacy action should be returned.
	t.Setenv("MEMRL_MODE", "off")
	state := newTestPhasedState().WithMaxRetries(3)
	gateErr := &gateFailError{Phase: 1, Verdict: "FAIL"}

	action, _ := resolveGateRetryAction(state, 1, gateErr, 1)
	// Legacy: attempt 1 < maxRetries 3 => retry
	if action != types.MemRLActionRetry {
		t.Errorf("action = %q, want %q", action, types.MemRLActionRetry)
	}
}

func TestResolveGateRetryAction_ModeOffEscalatesAtMax(t *testing.T) {
	t.Setenv("MEMRL_MODE", "off")
	state := newTestPhasedState().WithMaxRetries(2)
	gateErr := &gateFailError{Phase: 1, Verdict: "FAIL"}

	action, _ := resolveGateRetryAction(state, 1, gateErr, 2)
	// Legacy: attempt 2 >= maxRetries 2 => escalate
	if action != types.MemRLActionEscalate {
		t.Errorf("action = %q, want %q", action, types.MemRLActionEscalate)
	}
}

// --- logGateRetryMemRL ---

func TestLogGateRetryMemRL_OffModeSkipsLog(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "memrl.log")
	decision := types.MemRLPolicyDecision{Mode: types.MemRLModeOff}
	logGateRetryMemRL(tmp, logPath, "test", decision, types.MemRLActionRetry)
	// Off mode should not create any log file
	if _, err := os.Stat(logPath); err == nil {
		t.Error("off mode should not create log file")
	}
}

// --- executeWithStatus ---

func TestExecuteWithStatus_SuccessPath(t *testing.T) {
	executor := &fakeExecutor{err: nil}
	state := newTestPhasedState()
	state.Opts.LiveStatus = false
	allPhases := buildAllPhases(phases)
	statusPath := ""

	err := executeWithStatus(context.Background(), executor, state, statusPath, allPhases, 1, 0, "test prompt", t.TempDir(), "running", "failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteWithStatus_FailurePath(t *testing.T) {
	executor := &fakeExecutor{err: errFakeExecFailure}
	state := newTestPhasedState()
	state.Opts.LiveStatus = false
	allPhases := buildAllPhases(phases)

	err := executeWithStatus(context.Background(), executor, state, "", allPhases, 1, 0, "test prompt", t.TempDir(), "running", "failed")
	if err == nil {
		t.Fatal("expected error from failing executor")
	}
}

// --- maybeUpdateLiveStatus ---

func TestMaybeUpdateLiveStatus_DisabledDoesNotPanic(t *testing.T) {
	state := newTestPhasedState()
	state.Opts.LiveStatus = false
	allPhases := buildAllPhases(phases)
	maybeUpdateLiveStatus(state, "", allPhases, 1, "test", 0, "")
	// When disabled, phases should remain in pending state
	if allPhases[0].CurrentAction != "pending" {
		t.Errorf("disabled status should not update phases, got %q", allPhases[0].CurrentAction)
	}
}

// --- postPhaseProcessing ---

func TestPostPhaseProcessing_UnknownPhase(t *testing.T) {
	state := newTestPhasedState()
	err := postPhaseProcessing(t.TempDir(), state, 99, "")
	if err != nil {
		t.Errorf("unknown phase should return nil, got: %v", err)
	}
}

// --- gateFailError ---

func TestGateFailError_ErrorString(t *testing.T) {
	err := &gateFailError{Phase: 2, Verdict: "BLOCKED", Report: "/path/to/report.md"}
	got := err.Error()
	want := "gate FAIL at phase 2: BLOCKED (report: /path/to/report.md)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestGateFailError_WithFindingsSlice(t *testing.T) {
	err := &gateFailError{
		Phase:   3,
		Verdict: "FAIL",
		Findings: []finding{
			{Description: "test error", Fix: "fix it", Ref: "file.go:10"},
		},
		Report: "report.md",
	}
	if len(err.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(err.Findings))
	}
}

// --- shouldForceEscalation ---

func TestShouldForceEscalation_BelowCeiling(t *testing.T) {
	for i := 1; i <= maxGateRetryDepth; i++ {
		if shouldForceEscalation(i) {
			t.Errorf("shouldForceEscalation(%d) = true, want false (maxGateRetryDepth=%d)", i, maxGateRetryDepth)
		}
	}
}

func TestShouldForceEscalation_AboveCeiling(t *testing.T) {
	for _, attempt := range []int{maxGateRetryDepth + 1, maxGateRetryDepth + 10, 100} {
		if !shouldForceEscalation(attempt) {
			t.Errorf("shouldForceEscalation(%d) = false, want true", attempt)
		}
	}
}

func TestDiscoveryCouncilEpicIDSkipsPlanFileEpics(t *testing.T) {
	if got := discoveryCouncilEpicID(&phasedState{EpicID: "plan:/tmp/plan.md"}); got != "" {
		t.Fatalf("discoveryCouncilEpicID(plan file) = %q, want empty", got)
	}
	if got := discoveryCouncilEpicID(&phasedState{EpicID: "na-123"}); got != "na-123" {
		t.Fatalf("discoveryCouncilEpicID(normal epic) = %q, want na-123", got)
	}
}

// --- C2 event emission tests ---

func TestGateDiscoveryVerdictC2Event(t *testing.T) {
	root := t.TempDir()
	runID := "run-disc-c2"

	// Test event shape directly — processDiscoveryPhase requires bd CLI.
	// This mirrors the event emitted at rpi_phased_gates.go:104-110.
	verdict, report := "PASS", "2026-03-01-pre-mortem-test.md"
	ev, err := appendRPIC2Event(root, rpiC2EventInput{
		RunID:   runID,
		Phase:   1,
		Type:    "gate.discovery.verdict",
		Message: fmt.Sprintf("Pre-mortem verdict: %s", verdict),
		Details: map[string]any{"verdict": verdict, "report": report},
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}
	if ev.Type != "gate.discovery.verdict" {
		t.Errorf("event type = %q, want %q", ev.Type, "gate.discovery.verdict")
	}
	if ev.Phase != 1 {
		t.Errorf("event phase = %d, want 1", ev.Phase)
	}
	if ev.RunID != runID {
		t.Errorf("event run_id = %q, want %q", ev.RunID, runID)
	}

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Type == "gate.discovery.verdict" {
			found = true
		}
	}
	if !found {
		t.Errorf("gate.discovery.verdict event not found among %d events", len(events))
	}
}

func TestGateValidationVerdictC2Event(t *testing.T) {
	root := t.TempDir()
	runID := "run-val-c2"

	// Create vibe council report with PASS verdict
	councilDir := filepath.Join(root, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(councilDir, "2026-03-01-vibe-test.md")
	if err := os.WriteFile(reportPath, []byte("# Vibe\n## Council Verdict: PASS\nAll good.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create prior phase result to pass validation
	rpiDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-result.json"), []byte(`{"status":"ok"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithRunID(runID).WithPhase(3).WithStartPhase(3)
	state.Verdicts = make(map[string]string)

	logPath := filepath.Join(rpiDir, "phased-orchestration.log")

	err := processValidationPhase(root, state, 3, logPath)
	if err != nil {
		t.Fatalf("processValidationPhase: %v", err)
	}

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "gate.validation.verdict" {
			found = true
			if ev.Phase != 3 {
				t.Errorf("event phase = %d, want 3", ev.Phase)
			}
			if ev.RunID != runID {
				t.Errorf("event run_id = %q, want %q", ev.RunID, runID)
			}
		}
	}
	if !found {
		t.Errorf("gate.validation.verdict event not found among %d events", len(events))
	}
	evaluatorPath := filepath.Join(root, ".agents", "rpi", "phase-3-evaluator.json")
	if _, err := os.Stat(evaluatorPath); err != nil {
		t.Fatalf("expected evaluator artifact at %s: %v", evaluatorPath, err)
	}
}

func TestGateEscalationC2Event(t *testing.T) {
	root := t.TempDir()
	runID := "run-esc-c2"

	// Set up logPath at root/.agents/rpi/phased-orchestration.log
	rpiDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	statusPath := filepath.Join(rpiDir, "live-status.md")

	state := newTestPhasedState().WithRunID(runID).WithMaxRetries(3)
	gateErr := &gateFailError{Phase: 1, Verdict: "FAIL", Report: "report.md"}
	allPhases := buildAllPhases(phases)

	performGateEscalation(state, 1, 3, gateErr,
		types.MemRLPolicyDecision{}, types.MemRLActionEscalate,
		"discovery", logPath, statusPath, allPhases)

	// escalationRoot = filepath.Dir(filepath.Dir(filepath.Dir(logPath))) = root
	escalationRoot := filepath.Dir(filepath.Dir(filepath.Dir(logPath)))
	events, err := loadRPIC2Events(escalationRoot, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "gate.escalation" {
			found = true
			if ev.Phase != 1 {
				t.Errorf("event phase = %d, want 1", ev.Phase)
			}
			if ev.RunID != runID {
				t.Errorf("event run_id = %q, want %q", ev.RunID, runID)
			}
		}
	}
	if !found {
		t.Errorf("gate.escalation event not found among %d events", len(events))
	}
}

func TestGateRetryAttemptC2Event(t *testing.T) {
	root := t.TempDir()
	runID := "run-retry-c2"

	rpiDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	statusPath := filepath.Join(rpiDir, "live-status.md")

	state := newTestPhasedState().WithRunID(runID).WithMaxRetries(3)
	state.Attempts = map[string]int{}
	gateErr := &gateFailError{Phase: 1, Verdict: "FAIL", Report: "report.md"}
	allPhases := buildAllPhases(phases)

	// Enable dry-run to avoid actually spawning
	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	t.Setenv("MEMRL_MODE", "off")

	_, _ = handleGateRetry(context.Background(), root, state, 1, gateErr, logPath, root, statusPath, allPhases, &fakeExecutor{})

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Type == "gate.retry.attempt" {
			found = true
			if ev.Phase != 1 {
				t.Errorf("event phase = %d, want 1", ev.Phase)
			}
			if ev.RunID != runID {
				t.Errorf("event run_id = %q, want %q", ev.RunID, runID)
			}
		}
	}
	if !found {
		t.Errorf("gate.retry.attempt event not found among %d events", len(events))
	}
}

func TestGateImplementationVerdictC2Event(t *testing.T) {
	root := t.TempDir()
	runID := "run-impl-c2"

	// Directly test the event shape by calling appendRPIC2Event with the same input
	// pattern used in processImplementationPhase. Testing processImplementationPhase
	// end-to-end requires bd CLI which is not available in unit tests.
	status := "COMPLETE"
	epicID := "ag-test"
	_, err := appendRPIC2Event(root, rpiC2EventInput{
		RunID:   runID,
		Phase:   2,
		Type:    "gate.implementation.verdict",
		Message: fmt.Sprintf("Crank status: %s", status),
		Details: map[string]any{"status": status, "epic_id": epicID},
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}

	events, err := loadRPIC2Events(root, runID)
	if err != nil {
		t.Fatalf("loadRPIC2Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Type != "gate.implementation.verdict" {
		t.Errorf("event type = %q, want gate.implementation.verdict", events[0].Type)
	}
	if events[0].Phase != 2 {
		t.Errorf("event phase = %d, want 2", events[0].Phase)
	}
}

// --- verifyGateAfterRetry ---

func TestVerifyGateAfterRetry_Succeeds(t *testing.T) {
	// Use phase 99 (unknown) so postPhaseProcessing returns nil immediately,
	// simulating a gate that passes after retry.
	tmp := t.TempDir()
	stateDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(stateDir, "phased-orchestration.log")
	statusPath := filepath.Join(stateDir, "live-status.md")

	state := newTestPhasedState().WithRunID("verify-ok").WithMaxRetries(3)
	allPhases := buildAllPhases(phases)
	executor := &fakeExecutor{}

	retried, err := verifyGateAfterRetry(
		context.Background(), tmp, state, 99,
		logPath, tmp, statusPath, allPhases, executor, 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !retried {
		t.Error("verifyGateAfterRetry should return true when gate passes")
	}
}

func TestVerifyGateAfterRetry_Exhausted(t *testing.T) {
	// Set up a validation phase (3) with a FAIL council report so
	// postPhaseProcessing returns a gateFailError. Pre-load attempts to
	// maxGateRetryDepth so the recursive handleGateRetry hits forced escalation.
	tmp := t.TempDir()
	rpiDir := filepath.Join(tmp, ".agents", "rpi")
	if err := os.MkdirAll(rpiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a prior phase result so processValidationPhase doesn't fail on prerequisite check.
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-result.json"), []byte(`{"status":"ok"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a council report with FAIL verdict.
	councilDir := filepath.Join(tmp, ".agents", "council")
	if err := os.MkdirAll(councilDir, 0755); err != nil {
		t.Fatal(err)
	}
	reportContent := "# Vibe\n## Council Verdict: FAIL\nCritical issues found.\n"
	if err := os.WriteFile(filepath.Join(councilDir, "2026-03-05-vibe-exhaust.md"), []byte(reportContent), 0644); err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(rpiDir, "phased-orchestration.log")
	statusPath := filepath.Join(rpiDir, "live-status.md")

	state := newTestPhasedState().WithRunID("verify-exhaust").WithMaxRetries(3).WithPhase(3).WithStartPhase(3)
	// Pre-load attempts so next increment exceeds maxGateRetryDepth.
	state.Attempts["phase_3"] = maxGateRetryDepth
	allPhases := buildAllPhases(phases)
	executor := &fakeExecutor{}

	t.Setenv("MEMRL_MODE", "off")

	retried, err := verifyGateAfterRetry(
		context.Background(), tmp, state, 3,
		logPath, tmp, statusPath, allPhases, executor, maxGateRetryDepth,
	)
	if err != nil {
		t.Fatalf("forced escalation should not return error, got: %v", err)
	}
	if retried {
		t.Error("verifyGateAfterRetry should return false when retries are exhausted (escalation)")
	}
}

// --- fakeExecutor for tests ---

type fakeExecutor struct {
	err      error
	executed bool
}

var errFakeExecFailure = fmt.Errorf("fake execution failure")

func (f *fakeExecutor) Name() string { return "fake" }
func (f *fakeExecutor) Execute(_ context.Context, prompt, cwd, runID string, phaseNum int) error {
	f.executed = true
	return f.err
}
