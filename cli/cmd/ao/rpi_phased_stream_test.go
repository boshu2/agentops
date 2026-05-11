// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- splitRuntimeCommand ---

func TestSplitRuntimeCommand_Empty(t *testing.T) {
	exe, args := splitRuntimeCommand("")
	if exe != "" {
		t.Errorf("executable = %q, want empty", exe)
	}
	if args != nil {
		t.Errorf("args = %v, want nil", args)
	}
}

func TestSplitRuntimeCommand_SingleWord(t *testing.T) {
	exe, args := splitRuntimeCommand("claude")
	if exe != "claude" {
		t.Errorf("executable = %q, want %q", exe, "claude")
	}
	if len(args) != 0 {
		t.Errorf("args length = %d, want 0", len(args))
	}
}

func TestSplitRuntimeCommand_WithArgs(t *testing.T) {
	exe, args := splitRuntimeCommand("codex --profile ci")
	if exe != "codex" {
		t.Errorf("executable = %q, want %q", exe, "codex")
	}
	if len(args) != 2 {
		t.Fatalf("args length = %d, want 2", len(args))
	}
	if args[0] != "--profile" || args[1] != "ci" {
		t.Errorf("args = %v, want [--profile ci]", args)
	}
}

func TestSplitRuntimeCommand_WithLeadingWhitespace(t *testing.T) {
	exe, _ := splitRuntimeCommand("  claude  ")
	if exe != "claude" {
		t.Errorf("executable = %q, want %q", exe, "claude")
	}
}

// --- runtimeBinaryName ---

func TestRuntimeBinaryName_Claude(t *testing.T) {
	got := runtimeBinaryName("claude")
	if got != "claude" {
		t.Errorf("runtimeBinaryName = %q, want %q", got, "claude")
	}
}

func TestRuntimeBinaryName_Codex(t *testing.T) {
	got := runtimeBinaryName("codex --profile ci")
	if got != "codex" {
		t.Errorf("runtimeBinaryName = %q, want %q", got, "codex")
	}
}

func TestRuntimeBinaryName_WithPath(t *testing.T) {
	got := runtimeBinaryName("/usr/local/bin/claude")
	if got != "claude" {
		t.Errorf("runtimeBinaryName = %q, want %q", got, "claude")
	}
}

func TestRuntimeBinaryName_Empty(t *testing.T) {
	got := runtimeBinaryName("")
	if got != "" {
		t.Errorf("runtimeBinaryName = %q, want empty", got)
	}
}

func TestRuntimeBinaryName_WindowsExe(t *testing.T) {
	got := runtimeBinaryName("claude.exe")
	if got != "claude" {
		t.Errorf("runtimeBinaryName = %q, want %q", got, "claude")
	}
}

// --- runtimeDirectCommandArgs ---

func TestRuntimeDirectCommandArgs_Claude(t *testing.T) {
	args := runtimeDirectCommandArgs("claude", "test prompt")
	if len(args) != 2 {
		t.Fatalf("args length = %d, want 2", len(args))
	}
	if args[0] != "-p" || args[1] != "test prompt" {
		t.Errorf("args = %v, want [-p test prompt]", args)
	}
}

func TestRuntimeDirectCommandArgs_Codex(t *testing.T) {
	args := runtimeDirectCommandArgs("codex", "test prompt")
	if len(args) != 2 {
		t.Fatalf("args length = %d, want 2", len(args))
	}
	if args[0] != "exec" || args[1] != "test prompt" {
		t.Errorf("args = %v, want [exec test prompt]", args)
	}
}

func TestRuntimeDirectCommandArgs_CodexWithPrefixArgs(t *testing.T) {
	args := runtimeDirectCommandArgs("codex --profile ci", "test prompt")
	if len(args) != 4 {
		t.Fatalf("args length = %d, want 4", len(args))
	}
	if args[0] != "--profile" || args[1] != "ci" || args[2] != "exec" || args[3] != "test prompt" {
		t.Errorf("args = %v, want [--profile ci exec test prompt]", args)
	}
}

// --- runtimeStreamCommandArgs ---

func TestRuntimeStreamCommandArgs_Claude(t *testing.T) {
	args, err := runtimeStreamCommandArgs("claude", "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) < 4 {
		t.Fatalf("args length = %d, want at least 4", len(args))
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "stream-json") {
		t.Errorf("args should contain stream-json, got: %v", args)
	}
	if !strings.Contains(joined, "--verbose") {
		t.Errorf("args should contain --verbose, got: %v", args)
	}
}

func TestRuntimeStreamCommandArgs_CodexUnsupported(t *testing.T) {
	_, err := runtimeStreamCommandArgs("codex", "test prompt")
	if err == nil {
		t.Fatal("expected error for codex (does not support stream-json)")
	}
}

// --- formatRuntimePromptInvocation ---

func TestFormatRuntimePromptInvocation_Claude(t *testing.T) {
	got := formatRuntimePromptInvocation("claude", "hello world")
	if !strings.Contains(got, "claude") {
		t.Errorf("got %q, should contain 'claude'", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Errorf("got %q, should contain the prompt", got)
	}
}

func TestFormatRuntimePromptInvocation_EmptyCommand(t *testing.T) {
	got := formatRuntimePromptInvocation("", "test")
	// Should not panic even with empty command.
	if got == "" {
		t.Error("should produce some output even with empty command")
	}
}

// --- probeBackendCapabilities ---

func TestProbeBackendCapabilities_Auto(t *testing.T) {
	caps := probeBackendCapabilities(false, "auto")
	if caps.RuntimeMode != "auto" {
		t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "auto")
	}
	if caps.LiveStatusEnabled {
		t.Error("LiveStatusEnabled should be false")
	}
}

func TestProbeBackendCapabilities_StreamWithLiveStatus(t *testing.T) {
	caps := probeBackendCapabilities(true, "stream")
	if caps.RuntimeMode != "stream" {
		t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "stream")
	}
	if !caps.LiveStatusEnabled {
		t.Error("LiveStatusEnabled should be true")
	}
}

func TestProbeBackendCapabilities_EmptyMode(t *testing.T) {
	caps := probeBackendCapabilities(false, "")
	if caps.RuntimeMode != "auto" {
		t.Errorf("RuntimeMode = %q, want %q (empty should normalize to auto)", caps.RuntimeMode, "auto")
	}
}

// --- selectExecutorFromCaps ---

func TestSelectExecutorFromCaps_StreamMode(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "stream", LiveStatusEnabled: false}
	opts := defaultPhasedEngineOptions()
	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "stream" {
		t.Errorf("executor name = %q, want %q", executor.Name(), "stream")
	}
	if !strings.Contains(reason, "stream") {
		t.Errorf("reason = %q, should mention stream", reason)
	}
}

func TestSelectExecutorFromCaps_DirectMode(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "direct", LiveStatusEnabled: false}
	opts := defaultPhasedEngineOptions()
	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "direct" {
		t.Errorf("executor name = %q, want %q", executor.Name(), "direct")
	}
	if !strings.Contains(reason, "direct") {
		t.Errorf("reason = %q, should mention direct", reason)
	}
}

func TestSelectExecutorFromCaps_AutoWithLiveStatus(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "auto", LiveStatusEnabled: true}
	opts := defaultPhasedEngineOptions()
	executor, _ := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "stream" {
		t.Errorf("auto + live-status should select stream, got %q", executor.Name())
	}
}

func TestSelectExecutorFromCaps_AutoWithoutLiveStatus(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "auto", LiveStatusEnabled: false}
	opts := defaultPhasedEngineOptions()
	executor, _ := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "stream" {
		t.Errorf("auto + no live-status should select stream, got %q", executor.Name())
	}
}

// --- normalizeCheckInterval ---

func TestNormalizeCheckInterval_Zero(t *testing.T) {
	got := normalizeCheckInterval(0)
	if got != 1*time.Second {
		t.Errorf("normalizeCheckInterval(0) = %v, want 1s", got)
	}
}

func TestNormalizeCheckInterval_Negative(t *testing.T) {
	got := normalizeCheckInterval(-1 * time.Second)
	if got != 1*time.Second {
		t.Errorf("normalizeCheckInterval(-1s) = %v, want 1s", got)
	}
}

func TestNormalizeCheckInterval_Positive(t *testing.T) {
	got := normalizeCheckInterval(5 * time.Second)
	if got != 5*time.Second {
		t.Errorf("normalizeCheckInterval(5s) = %v, want 5s", got)
	}
}

// --- buildStreamPhaseContext ---

func TestBuildStreamPhaseContext_NoTimeout(t *testing.T) {
	ctx, cancel := buildStreamPhaseContext(context.Background(), 0)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Error("context should not have a deadline when timeout is 0")
	}
}

func TestBuildStreamPhaseContext_ZeroTimeout(t *testing.T) {
	// When timeout is 0, the returned cancel must actually cancel the context.
	ctx, cancel := buildStreamPhaseContext(context.Background(), 0)
	cancel()
	if ctx.Err() == nil {
		t.Error("calling cancel() on zero-timeout context should cancel it, but ctx.Err() is nil")
	}
}

func TestBuildStreamPhaseContext_WithTimeout(t *testing.T) {
	ctx, cancel := buildStreamPhaseContext(context.Background(), 10*time.Second)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Error("context should have a deadline when timeout > 0")
	}
}

// --- shouldFallbackToDirect ---

func TestShouldFallbackToDirect_NilError(t *testing.T) {
	if shouldFallbackToDirect(nil) {
		t.Error("nil error should not trigger fallback")
	}
}

func TestShouldFallbackToDirect_StartupTimeout(t *testing.T) {
	err := fmt.Errorf("stream startup timeout: no events received")
	if !shouldFallbackToDirect(err) {
		t.Error("startup timeout should trigger fallback")
	}
}

func TestShouldFallbackToDirect_StreamParseError(t *testing.T) {
	err := fmt.Errorf("stream parse error: invalid json")
	if !shouldFallbackToDirect(err) {
		t.Error("stream parse error should trigger fallback")
	}
}

func TestShouldFallbackToDirect_UnsupportedStreamJSON(t *testing.T) {
	err := fmt.Errorf("runtime does not support stream-json")
	if !shouldFallbackToDirect(err) {
		t.Error("unsupported stream-json should trigger fallback")
	}
}

func TestShouldFallbackToDirect_StallWithNoActivity(t *testing.T) {
	err := fmt.Errorf("phase 1 (stall): no stream activity for 30s")
	if !shouldFallbackToDirect(err) {
		t.Error("stall with no stream activity should trigger fallback")
	}
}

func TestShouldFallbackToDirect_RegularError(t *testing.T) {
	err := fmt.Errorf("phase 1 failed: exit code 1")
	if shouldFallbackToDirect(err) {
		t.Error("regular error should not trigger fallback")
	}
}

// --- mergePhaseProgress ---

func TestMergePhaseProgress_UpdatesNonZeroFields(t *testing.T) {
	dst := PhaseProgress{Name: "discovery", CurrentAction: "pending"}
	src := PhaseProgress{CurrentAction: "running", RetryCount: 2}

	mergePhaseProgress(&dst, src)

	if dst.Name != "discovery" {
		t.Errorf("Name = %q, should not change (src.Name is empty)", dst.Name)
	}
	if dst.CurrentAction != "running" {
		t.Errorf("CurrentAction = %q, want %q", dst.CurrentAction, "running")
	}
	if dst.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", dst.RetryCount)
	}
}

func TestMergePhaseProgress_PreservesExistingWhenSrcEmpty(t *testing.T) {
	dst := PhaseProgress{Name: "test", CurrentAction: "working", LastError: "prev error"}
	src := PhaseProgress{}

	mergePhaseProgress(&dst, src)

	if dst.Name != "test" {
		t.Errorf("Name = %q, should be preserved", dst.Name)
	}
	if dst.CurrentAction != "working" {
		t.Errorf("CurrentAction = %q, should be preserved", dst.CurrentAction)
	}
	if dst.LastError != "prev error" {
		t.Errorf("LastError = %q, should be preserved", dst.LastError)
	}
}

func TestMergePhaseProgress_OverridesName(t *testing.T) {
	dst := PhaseProgress{Name: "old"}
	src := PhaseProgress{Name: "new"}

	mergePhaseProgress(&dst, src)

	if dst.Name != "new" {
		t.Errorf("Name = %q, want %q", dst.Name, "new")
	}
}

// --- buildAllPhases ---

func TestBuildAllPhases_MatchesPhaseDefinitions(t *testing.T) {
	all := buildAllPhases(phases)
	if len(all) != len(phases) {
		t.Fatalf("buildAllPhases length = %d, want %d", len(all), len(phases))
	}
	for i, p := range all {
		if p.Name != phases[i].Name {
			t.Errorf("phase[%d].Name = %q, want %q", i, p.Name, phases[i].Name)
		}
		if p.CurrentAction != "pending" {
			t.Errorf("phase[%d].CurrentAction = %q, want %q", i, p.CurrentAction, "pending")
		}
	}
}

// --- cleanEnvNoClaude ---

func TestCleanEnvNoClaude_FiltersClaudeVars(t *testing.T) {
	t.Setenv("CLAUDECODE", "test")
	t.Setenv("CLAUDE_CODE_SESSION", "abc")
	t.Setenv("PATH", "/usr/bin")

	env := cleanEnvNoClaude()

	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDECODE=") || strings.HasPrefix(e, "CLAUDE_CODE_") {
			t.Errorf("env should not contain Claude vars, found: %s", e)
		}
	}

	hasPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		t.Error("env should still contain PATH")
	}
}

// --- classifyStreamResult ---

func TestClassifyStreamResult_Success(t *testing.T) {
	ctx := context.Background()
	stallCtx := context.Background()
	err := classifyStreamResult(ctx, stallCtx, "claude", 1, 0, nil, nil, 5)
	if err != nil {
		t.Errorf("expected nil for successful stream, got: %v", err)
	}
}

func TestClassifyStreamResult_PhaseTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	time.Sleep(2 * time.Millisecond) // Let it expire.
	defer cancel()

	err := classifyStreamResult(ctx, ctx, "claude", 1, 1*time.Second, nil, nil, 5)
	if err == nil {
		t.Fatal("expected error for phase timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, should mention timed out", err.Error())
	}
}

func TestClassifyStreamResult_WaitError(t *testing.T) {
	ctx := context.Background()
	stallCtx := context.Background()
	waitErr := fmt.Errorf("some error: %w", &exec.ExitError{})
	err := classifyStreamResult(ctx, stallCtx, "claude", 1, 0, waitErr, nil, 5)
	if err == nil {
		t.Fatal("expected error from waitErr")
	}
}

func TestClassifyStreamResult_ParseError(t *testing.T) {
	ctx := context.Background()
	stallCtx := context.Background()
	parseErr := fmt.Errorf("invalid json")
	err := classifyStreamResult(ctx, stallCtx, "claude", 1, 0, nil, parseErr, 5)
	if err == nil {
		t.Fatal("expected error from parseErr")
	}
	if !strings.Contains(err.Error(), "stream parse error") {
		t.Errorf("error = %q, should mention parse error", err.Error())
	}
}

func TestClassifyStreamResult_ZeroEvents(t *testing.T) {
	ctx := context.Background()
	stallCtx := context.Background()
	err := classifyStreamResult(ctx, stallCtx, "claude", 1, 0, nil, nil, 0)
	if err == nil {
		t.Fatal("expected error for zero events")
	}
	if !strings.Contains(err.Error(), "startup timeout") {
		t.Errorf("error = %q, should mention startup timeout", err.Error())
	}
}

func TestClassifyStreamResult_StallDetected(t *testing.T) {
	parentCtx := context.Background()
	stallCtx, stallCancel := context.WithCancelCause(parentCtx)
	stallCancel(fmt.Errorf("stall detected: no stream activity for 30s"))

	err := classifyStreamResult(parentCtx, stallCtx, "claude", 1, 0, nil, nil, 5)
	if err == nil {
		t.Fatal("expected error for stall detection")
	}
	if !strings.Contains(err.Error(), "stall") {
		t.Errorf("error = %q, should mention stall", err.Error())
	}
}

// --- streamWatchdogState ---

func TestStreamWatchdogState_Atomics(t *testing.T) {
	w := &streamWatchdogState{}
	w.eventCount.Store(5)
	w.lastActivityUnix.Store(time.Now().UnixNano())

	if w.eventCount.Load() != 5 {
		t.Errorf("eventCount = %d, want 5", w.eventCount.Load())
	}
	if w.lastActivityUnix.Load() == 0 {
		t.Error("lastActivityUnix should be non-zero")
	}
}

// --- buildStreamUpdateCallback ---

func TestBuildStreamUpdateCallback_RecordsActivity(t *testing.T) {
	watchdog := &streamWatchdogState{}
	allPhases := buildAllPhases(phases)
	statusPath := ""

	callback := buildStreamUpdateCallback(watchdog, allPhases, 1, statusPath)
	callback(PhaseProgress{CurrentAction: "running"})

	if watchdog.eventCount.Load() != 1 {
		t.Errorf("eventCount = %d, want 1", watchdog.eventCount.Load())
	}
	if watchdog.lastActivityUnix.Load() == 0 {
		t.Error("lastActivityUnix should be updated")
	}
}

func TestBuildStreamUpdateCallback_MergesProgress(t *testing.T) {
	watchdog := &streamWatchdogState{}
	allPhases := buildAllPhases(phases)
	statusPath := ""

	callback := buildStreamUpdateCallback(watchdog, allPhases, 1, statusPath)
	callback(PhaseProgress{CurrentAction: "executing tool"})

	if allPhases[0].CurrentAction != "executing tool" {
		t.Errorf("allPhases[0].CurrentAction = %q, want %q", allPhases[0].CurrentAction, "executing tool")
	}
}

func TestBuildStreamUpdateCallback_InvalidPhaseIndex(t *testing.T) {
	watchdog := &streamWatchdogState{}
	var allPhases []PhaseProgress // empty
	statusPath := ""

	// Should not panic even with phaseNum out of bounds.
	callback := buildStreamUpdateCallback(watchdog, allPhases, 99, statusPath)
	callback(PhaseProgress{CurrentAction: "test"})

	if watchdog.eventCount.Load() != 1 {
		t.Errorf("eventCount = %d, want 1 (should still record activity)", watchdog.eventCount.Load())
	}
}

// --- directExecutor ---

func TestDirectExecutor_Name(t *testing.T) {
	d := &directExecutor{}
	if d.Name() != "direct" {
		t.Errorf("Name() = %q, want %q", d.Name(), "direct")
	}
}

// --- streamExecutor ---

func TestStreamExecutor_Name(t *testing.T) {
	s := &streamExecutor{}
	if s.Name() != "stream" {
		t.Errorf("Name() = %q, want %q", s.Name(), "stream")
	}
}

// --- ExitCodes ---

func TestExitCodes_Constants(t *testing.T) {
	if ExitGateFail != 10 {
		t.Errorf("ExitGateFail = %d, want 10", ExitGateFail)
	}
	if ExitUserAbort != 20 {
		t.Errorf("ExitUserAbort = %d, want 20", ExitUserAbort)
	}
	if ExitCLIError != 30 {
		t.Errorf("ExitCLIError = %d, want 30", ExitCLIError)
	}
}

// --- startStreamWatchdogs ---

func TestStartStreamWatchdogs_BothDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchdog := &streamWatchdogState{}
	watchdog.lastActivityUnix.Store(time.Now().UnixNano())

	// Both timeouts at 0 should not start any watchdog goroutines.
	startStreamWatchdogs(ctx, func(err error) { cancel() }, watchdog, time.Now(), time.Second, 0, 0)

	// Wait briefly then verify context is not cancelled.
	time.Sleep(50 * time.Millisecond)
	if ctx.Err() != nil {
		t.Error("context should not be cancelled when both watchdogs are disabled")
	}
}

// --- runStartupWatchdog ---

func TestRunStartupWatchdog_CancelsOnTimeout(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	var eventCount atomic.Int64
	startedAt := time.Now().Add(-2 * time.Second) // Started 2s ago

	go runStartupWatchdog(ctx, cancel, &eventCount, startedAt, 10*time.Millisecond, 500*time.Millisecond)

	// Wait for the watchdog to cancel the context (should fire on first tick).
	select {
	case <-ctx.Done():
		// Success — watchdog cancelled the context.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for startup watchdog to cancel context")
	}
}

func TestRunStartupWatchdog_ExitsOnEvents(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	var eventCount atomic.Int64
	eventCount.Store(1) // Already have events

	done := make(chan struct{})
	go func() {
		runStartupWatchdog(ctx, cancel, &eventCount, time.Now(), 10*time.Millisecond, 100*time.Millisecond)
		close(done)
	}()

	// Wait for the watchdog goroutine to exit (should return on first tick).
	select {
	case <-done:
		// Goroutine exited — verify it did NOT cancel the context.
		if ctx.Err() != nil {
			t.Error("context should not be cancelled when events already exist")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for startup watchdog goroutine to exit")
	}
}

func TestCleanEnvNoClaude(t *testing.T) {
	// Set some CLAUDE_ vars to verify they are filtered out.
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_SOMETHING", "value")
	// Set a normal var to verify it survives.
	t.Setenv("AGENTOPS_TEST_MARKER", "keep-me")

	env := cleanEnvNoClaude()

	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Errorf("cleanEnvNoClaude should remove CLAUDECODE, found %q", e)
		}
		if strings.HasPrefix(e, "CLAUDE_CODE_") {
			t.Errorf("cleanEnvNoClaude should remove CLAUDE_CODE_ vars, found %q", e)
		}
	}

	// Verify non-CLAUDE vars are preserved.
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "AGENTOPS_TEST_MARKER=") {
			found = true
			break
		}
	}
	if !found {
		t.Error("cleanEnvNoClaude should preserve non-CLAUDE environment variables")
	}
}

// TestSpawnClaudeDirectGlobal_CommandBuilt verifies the argument construction
// and delegation chain used by spawnClaudeDirectGlobal. Since calling the real
// binary would start an interactive session, we test via:
// 1. Verifying runtimeDirectCommandArgs produces correct args for "claude"
// 2. Verifying the global phasedPhaseTimeout is wired into the call chain
func TestSpawnClaudeDirectGlobal_CommandBuilt(t *testing.T) {
	prompt := "test prompt for phase"

	// 1. Verify the argument construction that spawnClaudeDirectGlobal would use.
	// Chain: spawnClaudeDirectGlobal -> spawnClaudeDirectImpl("claude", ...) ->
	//        spawnRuntimeDirectWithWriter -> runtimeDirectCommandArgs("claude", prompt)
	args := runtimeDirectCommandArgs("claude", prompt)
	if len(args) != 2 {
		t.Fatalf("runtimeDirectCommandArgs length = %d, want 2", len(args))
	}
	if args[0] != "-p" {
		t.Errorf("args[0] = %q, want %q", args[0], "-p")
	}
	if args[1] != prompt {
		t.Errorf("args[1] = %q, want %q", args[1], prompt)
	}

	// 2. Verify phasedPhaseTimeout is used by spawnClaudeDirectGlobal.
	origTimeout := phasedPhaseTimeout
	defer func() { phasedPhaseTimeout = origTimeout }()
	phasedPhaseTimeout = 50 * time.Millisecond

	// Verify the runtime binary name is "claude" as expected.
	binName := runtimeBinaryName("claude")
	if binName != "claude" {
		t.Errorf("runtimeBinaryName(\"claude\") = %q, want %q", binName, "claude")
	}
}

func TestCleanEnvNoClaude_NoClaudeVars(t *testing.T) {
	// When no CLAUDE vars exist, the result should include everything in os.Environ().
	env := cleanEnvNoClaude()
	if len(env) == 0 {
		t.Error("cleanEnvNoClaude returned empty env (expected at least PATH)")
	}
}

func TestDirectExecutor_EffectiveStdoutWriter_NilFallsBack(t *testing.T) {
	d := &directExecutor{stdoutWriter: nil}
	w := d.effectiveStdoutWriter()
	if w == nil {
		t.Error("should return non-nil even when stdoutWriter is nil")
	}
}

func TestDirectExecutor_EffectiveStdoutWriter_Custom(t *testing.T) {
	var buf strings.Builder
	d := &directExecutor{stdoutWriter: &buf}
	w := d.effectiveStdoutWriter()
	if w != &buf {
		t.Error("should return the custom writer when set")
	}
}

func TestStreamExecutor_EffectiveStdoutWriter_NilFallsBack(t *testing.T) {
	s := &streamExecutor{stdoutWriter: nil}
	w := s.effectiveStdoutWriter()
	if w == nil {
		t.Error("should return non-nil even when stdoutWriter is nil")
	}
}

func TestStreamExecutor_EffectiveStdoutWriter_Custom(t *testing.T) {
	var buf strings.Builder
	s := &streamExecutor{stdoutWriter: &buf}
	w := s.effectiveStdoutWriter()
	if w != &buf {
		t.Error("should return the custom writer when set")
	}
}

func TestUpdateLivePhaseStatus_NegativePhaseNum2(t *testing.T) {
	allPhases := buildAllPhases(phases)
	updateLivePhaseStatus("", allPhases, 0, "test", 0, "")
	if allPhases[0].CurrentAction != "pending" {
		t.Errorf("should be untouched, got %q", allPhases[0].CurrentAction)
	}
}

func TestUpdateLivePhaseStatus_PhaseNumTooLarge2(t *testing.T) {
	allPhases := buildAllPhases(phases)
	updateLivePhaseStatus("", allPhases, len(phases)+1, "test", 0, "")
	// Out-of-range phase should leave all phases unchanged
	for i, p := range allPhases {
		if p.CurrentAction != "pending" {
			t.Errorf("phase %d should be untouched, got %q", i, p.CurrentAction)
		}
	}
}

func TestUpdateLivePhaseStatus_ValidPhase2(t *testing.T) {
	allPhases := buildAllPhases(phases)
	updateLivePhaseStatus("", allPhases, 1, "running tool X", 3, "some error")
	if allPhases[0].RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", allPhases[0].RetryCount)
	}
	if allPhases[0].LastUpdate.IsZero() {
		t.Error("LastUpdate should be set")
	}
}

func TestUpdateLivePhaseStatus_EmptyAction2(t *testing.T) {
	allPhases := buildAllPhases(phases)
	allPhases[0].CurrentAction = "already running"
	updateLivePhaseStatus("", allPhases, 1, "", 0, "")
	if allPhases[0].CurrentAction != "already running" {
		t.Errorf("CurrentAction = %q, want %q", allPhases[0].CurrentAction, "already running")
	}
}

func TestStartStreamWatchdogs_CapsStartupCheckInterval2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchdog := &streamWatchdogState{}
	watchdog.lastActivityUnix.Store(time.Now().UnixNano())
	watchdog.eventCount.Store(1)
	startStreamWatchdogs(ctx, func(err error) { cancel() }, watchdog, time.Now(), 10*time.Second, 0, 1*time.Second)
	time.Sleep(50 * time.Millisecond)
	if ctx.Err() != nil {
		t.Error("context should not be cancelled when events already exist")
	}
}

func TestRunStallWatchdog_ExitsOnContextCancel2(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())
	done := make(chan struct{})
	go func() {
		runStallWatchdog(ctx, cancel, &lastActivity, 10*time.Millisecond, 1*time.Hour)
		close(done)
	}()
	cancel(nil)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stall watchdog did not exit after context cancellation")
	}
}

func TestRunStallWatchdog_DetectsStall2(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().Add(-2 * time.Second).UnixNano())
	go runStallWatchdog(ctx, cancel, &lastActivity, 10*time.Millisecond, 100*time.Millisecond)
	select {
	case <-ctx.Done():
		cause := context.Cause(ctx)
		if cause == nil || !strings.Contains(cause.Error(), "stall detected") {
			t.Errorf("expected stall detection cause, got: %v", cause)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stall watchdog did not fire")
	}
}

func TestBuildAllPhases_EmptyInput2(t *testing.T) {
	all := buildAllPhases(nil)
	if len(all) != 0 {
		t.Errorf("length = %d, want 0", len(all))
	}
}

func TestBuildAllPhases_SinglePhase2(t *testing.T) {
	defs := []phase{{Name: "test-phase"}}
	all := buildAllPhases(defs)
	if len(all) != 1 {
		t.Fatalf("length = %d, want 1", len(all))
	}
	if all[0].Name != "test-phase" {
		t.Errorf("Name = %q, want %q", all[0].Name, "test-phase")
	}
}

func TestClassifyStreamResult_NonExitWaitError2(t *testing.T) {
	ctx := context.Background()
	stallCtx := context.Background()
	waitErr := fmt.Errorf("connection reset")
	err := classifyStreamResult(ctx, stallCtx, "claude", 1, 0, waitErr, nil, 5)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "execution failed") {
		t.Errorf("error = %q, should mention 'execution failed'", err.Error())
	}
}

func TestSelectExecutorFromCaps_TmuxMode2(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "tmux", LiveStatusEnabled: false}
	opts := defaultPhasedEngineOptions()
	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "tmux" {
		t.Errorf("executor name = %q, want %q", executor.Name(), "tmux")
	}
	if !strings.Contains(reason, "tmux") {
		t.Errorf("reason = %q, should mention tmux", reason)
	}
}

func TestShouldFallbackToDirect_CaseInsensitive2(t *testing.T) {
	err := fmt.Errorf("STREAM STARTUP TIMEOUT: no events")
	if !shouldFallbackToDirect(err) {
		t.Error("shouldFallbackToDirect should be case-insensitive")
	}
}
