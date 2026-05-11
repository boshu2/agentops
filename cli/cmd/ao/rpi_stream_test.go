// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFallbackValue(t *testing.T) {
	tests := []struct {
		value, fallback, want string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"  ", "default", "default"},
		{"  trimmed  ", "default", "trimmed"},
	}
	for _, tt := range tests {
		got := fallbackValue(tt.value, tt.fallback)
		if got != tt.want {
			t.Errorf("fallbackValue(%q, %q) = %q, want %q", tt.value, tt.fallback, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// writeStreamEvent
// ---------------------------------------------------------------------------

func TestWriteStreamEvent_NilFile(t *testing.T) {
	event := RPIC2Event{Type: "test.event", Message: "hello"}
	err := writeStreamEvent(nil, event, "human")
	if err == nil {
		t.Fatal("expected error when out is nil, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("error = %q, want it to mention 'nil'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// directExecutor
// ---------------------------------------------------------------------------

func TestWriteStreamEvent_JSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "events.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	event := RPIC2Event{
		EventID:   "evt-001",
		Type:      "phase.start",
		Phase:     1,
		WorkerID:  "w1",
		Message:   "starting discovery",
		Source:    "stream",
		Timestamp: "2026-04-04T10:00:00Z",
	}

	if err := writeStreamEvent(f, event, "json"); err != nil {
		t.Fatalf("writeStreamEvent(json) error: %v", err)
	}
	f.Close()

	data, _ := os.ReadFile(path)
	var parsed RPIC2Event
	if err := json.Unmarshal(bytes.TrimSpace(data), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, data)
	}
	if parsed.EventID != "evt-001" {
		t.Errorf("EventID = %q, want %q", parsed.EventID, "evt-001")
	}
	if parsed.Type != "phase.start" {
		t.Errorf("Type = %q, want %q", parsed.Type, "phase.start")
	}
}

func TestWriteStreamEvent_SSE(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "events.sse")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	event := RPIC2Event{
		EventID:   "evt-002",
		Type:      "phase.complete",
		Phase:     2,
		Message:   "planning done",
		Timestamp: "2026-04-04T11:00:00Z",
	}

	if err := writeStreamEvent(f, event, "sse"); err != nil {
		t.Fatalf("writeStreamEvent(sse) error: %v", err)
	}
	f.Close()

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "id: evt-002") {
		t.Errorf("SSE missing id line, got: %q", content)
	}
	if !strings.Contains(content, "event: phase.complete") {
		t.Errorf("SSE missing event line, got: %q", content)
	}
	if !strings.Contains(content, "data: ") {
		t.Errorf("SSE missing data line, got: %q", content)
	}
}

func TestWriteStreamEvent_Human(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "events.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	event := RPIC2Event{
		EventID:   "evt-003",
		Type:      "worker.log",
		Phase:     3,
		WorkerID:  "worker-A",
		Message:   "implementing feature",
		Source:    "direct",
		Timestamp: "2026-04-04T12:00:00Z",
	}

	if err := writeStreamEvent(f, event, "human"); err != nil {
		t.Fatalf("writeStreamEvent(human) error: %v", err)
	}
	f.Close()

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "phase=3") {
		t.Errorf("human output missing phase, got: %q", content)
	}
	if !strings.Contains(content, "worker=worker-A") {
		t.Errorf("human output missing worker, got: %q", content)
	}
	if !strings.Contains(content, "type=worker.log") {
		t.Errorf("human output missing type, got: %q", content)
	}
	if !strings.Contains(content, "msg=implementing feature") {
		t.Errorf("human output missing message, got: %q", content)
	}
}

func TestWriteStreamEvent_HumanDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "events.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	// Event with zero phase, empty worker, empty message, empty source
	event := RPIC2Event{
		EventID:   "evt-004",
		Type:      "info",
		Timestamp: "2026-04-04T13:00:00Z",
	}

	if err := writeStreamEvent(f, event, "human"); err != nil {
		t.Fatalf("writeStreamEvent(human defaults) error: %v", err)
	}
	f.Close()

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "phase=-") {
		t.Errorf("expected phase=-, got: %q", content)
	}
	if !strings.Contains(content, "worker=-") {
		t.Errorf("expected worker=-, got: %q", content)
	}
	if !strings.Contains(content, "msg=-") {
		t.Errorf("expected msg=-, got: %q", content)
	}
	if !strings.Contains(content, "source=-") {
		t.Errorf("expected source=-, got: %q", content)
	}
}

func TestStreamCoverage_DirectExecutorName(t *testing.T) {
	d := &directExecutor{runtimeCommand: "claude", phaseTimeout: 90 * time.Minute}
	if d.Name() != "direct" {
		t.Errorf("Name() = %q, want %q", d.Name(), "direct")
	}
}

// ---------------------------------------------------------------------------
// streamExecutor
// ---------------------------------------------------------------------------

func TestStreamCoverage_StreamExecutorName(t *testing.T) {
	s := &streamExecutor{runtimeCommand: "claude"}
	if s.Name() != "stream" {
		t.Errorf("Name() = %q, want %q", s.Name(), "stream")
	}
}

// ---------------------------------------------------------------------------
// shouldFallbackToDirect
// ---------------------------------------------------------------------------

func TestStreamCoverage_ShouldFallbackToDirect(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "stream startup timeout", err: errors.New("stream startup timeout: no events received"), want: true},
		{name: "stream parse error", err: errors.New("stream parse error: malformed JSON"), want: true},
		{name: "stall with stream activity", err: errors.New("phase 1 (stall): stall detected: no stream activity for 10m0s"), want: true},
		{name: "unrelated error", err: errors.New("command not found"), want: false},
		{name: "exit code error", err: errors.New("claude exited with code 1 (exit_error): signal killed"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFallbackToDirect(tt.err)
			if got != tt.want {
				t.Errorf("shouldFallbackToDirect(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// probeBackendCapabilities
// ---------------------------------------------------------------------------

func TestStreamCoverage_ProbeBackendCapabilities(t *testing.T) {
	t.Run("live status enabled", func(t *testing.T) {
		caps := probeBackendCapabilities(true, "auto")
		if !caps.LiveStatusEnabled {
			t.Error("expected LiveStatusEnabled=true")
		}
		if caps.RuntimeMode != "auto" {
			t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "auto")
		}
	})

	t.Run("live status disabled", func(t *testing.T) {
		caps := probeBackendCapabilities(false, "stream")
		if caps.LiveStatusEnabled {
			t.Error("expected LiveStatusEnabled=false")
		}
		if caps.RuntimeMode != "stream" {
			t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "stream")
		}
	})

	t.Run("empty mode normalizes to auto", func(t *testing.T) {
		caps := probeBackendCapabilities(false, "")
		if caps.RuntimeMode != "auto" {
			t.Errorf("RuntimeMode = %q, want %q", caps.RuntimeMode, "auto")
		}
	})
}

// ---------------------------------------------------------------------------
// selectExecutorFromCaps
// ---------------------------------------------------------------------------

func TestStreamCoverage_SelectExecutorFromCaps(t *testing.T) {
	opts := defaultPhasedEngineOptions()
	phases := []PhaseProgress{{Name: "discovery"}}

	tests := []struct {
		name       string
		caps       backendCapabilities
		wantName   string
		wantReason string
	}{
		{
			name:     "runtime=stream always selects stream",
			caps:     backendCapabilities{RuntimeMode: "stream", LiveStatusEnabled: false},
			wantName: "stream",
		},
		{
			name:     "runtime=direct always selects direct",
			caps:     backendCapabilities{RuntimeMode: "direct", LiveStatusEnabled: true},
			wantName: "direct",
		},
		{
			name:     "runtime=auto with live status selects stream",
			caps:     backendCapabilities{RuntimeMode: "auto", LiveStatusEnabled: true},
			wantName: "stream",
		},
		{
			name:     "runtime=auto without live status selects stream",
			caps:     backendCapabilities{RuntimeMode: "auto", LiveStatusEnabled: false},
			wantName: "stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, reason := selectExecutorFromCaps(tt.caps, "/tmp/status", phases, opts)
			if executor.Name() != tt.wantName {
				t.Errorf("executor.Name() = %q, want %q (reason=%q)", executor.Name(), tt.wantName, reason)
			}
			if reason == "" {
				t.Error("expected non-empty reason")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildStreamPhaseContext
// ---------------------------------------------------------------------------

func TestStreamCoverage_BuildStreamPhaseContext(t *testing.T) {
	t.Run("with timeout", func(t *testing.T) {
		ctx, cancel := buildStreamPhaseContext(context.Background(), 5*time.Second)
		defer cancel()
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("expected deadline to be set")
		}
		if deadline.Before(time.Now()) {
			t.Error("deadline should be in the future")
		}
	})

	t.Run("without timeout", func(t *testing.T) {
		ctx, cancel := buildStreamPhaseContext(context.Background(), 0)
		defer cancel()
		_, ok := ctx.Deadline()
		if ok {
			t.Error("expected no deadline with 0 timeout")
		}
	})
}

// ---------------------------------------------------------------------------
// normalizeCheckInterval
// ---------------------------------------------------------------------------

func TestStreamCoverage_NormalizeCheckInterval(t *testing.T) {
	t.Run("positive value kept", func(t *testing.T) {
		got := normalizeCheckInterval(5 * time.Second)
		if got != 5*time.Second {
			t.Errorf("got %v, want 5s", got)
		}
	})

	t.Run("zero defaults to 1s", func(t *testing.T) {
		got := normalizeCheckInterval(0)
		if got != 1*time.Second {
			t.Errorf("got %v, want 1s", got)
		}
	})

	t.Run("negative defaults to 1s", func(t *testing.T) {
		got := normalizeCheckInterval(-1 * time.Second)
		if got != 1*time.Second {
			t.Errorf("got %v, want 1s", got)
		}
	})
}

// ---------------------------------------------------------------------------
// mergePhaseProgress
// ---------------------------------------------------------------------------

func TestStreamCoverage_MergePhaseProgress(t *testing.T) {
	t.Run("non-zero fields merged", func(t *testing.T) {
		dst := PhaseProgress{
			Name:          "discovery",
			CurrentAction: "old action",
			RetryCount:    0,
		}
		src := PhaseProgress{
			Name:          "updated",
			CurrentAction: "new action",
			RetryCount:    2,
			LastError:     "some error",
		}
		mergePhaseProgress(&dst, src)
		if dst.Name != "updated" {
			t.Errorf("Name = %q, want %q", dst.Name, "updated")
		}
		if dst.CurrentAction != "new action" {
			t.Errorf("CurrentAction = %q, want %q", dst.CurrentAction, "new action")
		}
		if dst.RetryCount != 2 {
			t.Errorf("RetryCount = %d, want 2", dst.RetryCount)
		}
		if dst.LastError != "some error" {
			t.Errorf("LastError = %q, want %q", dst.LastError, "some error")
		}
	})

	t.Run("zero fields not merged", func(t *testing.T) {
		dst := PhaseProgress{
			Name:          "discovery",
			CurrentAction: "old",
			RetryCount:    3,
			LastError:     "old error",
		}
		src := PhaseProgress{}
		mergePhaseProgress(&dst, src)
		if dst.Name != "discovery" {
			t.Errorf("Name = %q, want %q", dst.Name, "discovery")
		}
		if dst.CurrentAction != "old" {
			t.Errorf("CurrentAction = %q, want %q", dst.CurrentAction, "old")
		}
		if dst.RetryCount != 3 {
			t.Errorf("RetryCount = %d, want 3", dst.RetryCount)
		}
		if dst.LastError != "old error" {
			t.Errorf("LastError = %q, want %q", dst.LastError, "old error")
		}
	})
}

// ---------------------------------------------------------------------------
// classifyStreamResult
// ---------------------------------------------------------------------------

func TestStreamCoverage_ClassifyStreamResult(t *testing.T) {
	t.Run("no errors returns nil", func(t *testing.T) {
		ctx := context.Background()
		stallCtx := context.Background()
		err := classifyStreamResult(ctx, stallCtx, "claude", 1, 90*time.Minute, nil, nil, 5)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("zero events returns error", func(t *testing.T) {
		ctx := context.Background()
		stallCtx := context.Background()
		err := classifyStreamResult(ctx, stallCtx, "claude", 1, 90*time.Minute, nil, nil, 0)
		if err == nil {
			t.Error("expected error for zero events")
		}
	})

	t.Run("context deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		time.Sleep(5 * time.Millisecond) // Ensure deadline is exceeded
		defer cancel()
		stallCtx := context.Background()
		err := classifyStreamResult(ctx, stallCtx, "claude", 1, 5*time.Minute, nil, nil, 0)
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("parse error", func(t *testing.T) {
		ctx := context.Background()
		stallCtx := context.Background()
		parseErr := errors.New("malformed JSON")
		err := classifyStreamResult(ctx, stallCtx, "claude", 1, 90*time.Minute, nil, parseErr, 5)
		if err == nil {
			t.Error("expected error for parse error")
		}
	})

	t.Run("stall context cancelled with cause", func(t *testing.T) {
		ctx := context.Background()
		stallCtx, stallCancel := context.WithCancelCause(ctx)
		stallCancel(errors.New("stall detected: no stream activity for 10m"))
		err := classifyStreamResult(ctx, stallCtx, "claude", 1, 90*time.Minute, nil, nil, 3)
		if err == nil {
			t.Error("expected stall error")
		}
	})
}

// ---------------------------------------------------------------------------
// buildAllPhases
// ---------------------------------------------------------------------------

func TestStreamCoverage_BuildAllPhases(t *testing.T) {
	defs := []phase{
		{Num: 1, Name: "discovery", Step: "discover"},
		{Num: 2, Name: "planning", Step: "plan"},
		{Num: 3, Name: "implementation", Step: "implement"},
	}

	phases := buildAllPhases(defs)
	if len(phases) != 3 {
		t.Fatalf("len = %d, want 3", len(phases))
	}
	for i, p := range phases {
		if p.Name != defs[i].Name {
			t.Errorf("phase[%d].Name = %q, want %q", i, p.Name, defs[i].Name)
		}
		if p.CurrentAction != "pending" {
			t.Errorf("phase[%d].CurrentAction = %q, want %q", i, p.CurrentAction, "pending")
		}
	}
}

// ---------------------------------------------------------------------------
// cleanEnvNoClaude
// ---------------------------------------------------------------------------

func TestStreamCoverage_CleanEnvNoClaude(t *testing.T) {
	// Set some CLAUDE vars to verify they're filtered
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_FOO", "bar")
	t.Setenv("TEST_VAR_KEEP", "yes")

	env := cleanEnvNoClaude()

	for _, e := range env {
		if len(e) >= 10 && e[:10] == "CLAUDECODE" {
			t.Errorf("CLAUDECODE should be filtered, got %q", e)
		}
		if len(e) >= 11 && e[:11] == "CLAUDE_CODE" {
			t.Errorf("CLAUDE_CODE_ should be filtered, got %q", e)
		}
	}

	found := false
	for _, e := range env {
		if len(e) >= 13 && e[:13] == "TEST_VAR_KEEP" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected TEST_VAR_KEEP to be preserved")
	}
}

// ---------------------------------------------------------------------------
// updateLivePhaseStatus
// ---------------------------------------------------------------------------

func TestStreamCoverage_UpdateLivePhaseStatus(t *testing.T) {
	t.Run("out of range phaseNum does nothing", func(t *testing.T) {
		phases := []PhaseProgress{{Name: "test", CurrentAction: "running"}}
		// phaseNum 0 => phaseIdx -1 => out of range, should not panic
		updateLivePhaseStatus("/tmp/nonexistent", phases, 0, "test", 0, "")
		// phaseNum 10 => phaseIdx 9 => out of range
		updateLivePhaseStatus("/tmp/nonexistent", phases, 10, "test", 0, "")
	})

	t.Run("updates phase fields", func(t *testing.T) {
		tmp := t.TempDir()
		statusPath := tmp + "/status.md"
		phases := []PhaseProgress{
			{Name: "discovery", CurrentAction: "pending"},
			{Name: "planning", CurrentAction: "pending"},
		}
		updateLivePhaseStatus(statusPath, phases, 1, "running tests", 2, "timeout")
		// CurrentAction may have been updated by summarizeStatusAction;
		// the important assertion is RetryCount below.
		if phases[0].RetryCount != 2 {
			t.Errorf("RetryCount = %d, want 2", phases[0].RetryCount)
		}
	})
}

// ---------------------------------------------------------------------------
// buildStreamUpdateCallback
// ---------------------------------------------------------------------------

func TestStreamCoverage_BuildStreamUpdateCallback(t *testing.T) {
	tmp := t.TempDir()
	statusPath := tmp + "/status.md"
	phases := []PhaseProgress{
		{Name: "discovery", CurrentAction: "pending"},
		{Name: "planning", CurrentAction: "pending"},
	}

	watchdog := &streamWatchdogState{}

	callback := buildStreamUpdateCallback(watchdog, phases, 1, statusPath)

	// Invoke callback
	callback(PhaseProgress{Name: "discovery", CurrentAction: "running"})

	if watchdog.eventCount.Load() != 1 {
		t.Errorf("eventCount = %d, want 1", watchdog.eventCount.Load())
	}
	if watchdog.lastActivityUnix.Load() == 0 {
		t.Error("expected lastActivityUnix to be set")
	}
	if phases[0].CurrentAction != "running" {
		t.Errorf("phases[0].CurrentAction = %q, want %q", phases[0].CurrentAction, "running")
	}
}

// ---------------------------------------------------------------------------
// streamWatchdogState
// ---------------------------------------------------------------------------

func TestStreamCoverage_StreamWatchdogState(t *testing.T) {
	w := &streamWatchdogState{}
	if w.eventCount.Load() != 0 {
		t.Error("expected initial eventCount=0")
	}

	w.eventCount.Add(5)
	if w.eventCount.Load() != 5 {
		t.Errorf("eventCount = %d, want 5", w.eventCount.Load())
	}

	now := time.Now().UnixNano()
	w.lastActivityUnix.Store(now)
	if w.lastActivityUnix.Load() != now {
		t.Error("lastActivityUnix mismatch")
	}
}

// ---------------------------------------------------------------------------
// startStreamWatchdogs (quick functional test)
// ---------------------------------------------------------------------------

func TestStreamCoverage_StartStreamWatchdogs(t *testing.T) {
	t.Run("startup watchdog cancels on timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		var eventCount atomic.Int64
		startedAt := time.Now()

		// Very short startup timeout
		go runStartupWatchdog(ctx, cancel, &eventCount, startedAt, 10*time.Millisecond, 50*time.Millisecond)

		select {
		case <-ctx.Done():
			// Success — watchdog cancelled the context.
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for startup watchdog to cancel context")
		}
	})

	t.Run("startup watchdog exits on event", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		var eventCount atomic.Int64
		eventCount.Store(1) // Already received an event
		startedAt := time.Now()

		done := make(chan struct{})
		go func() {
			runStartupWatchdog(ctx, cancel, &eventCount, startedAt, 10*time.Millisecond, 50*time.Millisecond)
			close(done)
		}()

		select {
		case <-done:
			// Goroutine exited — verify it did NOT cancel the context.
			if ctx.Err() != nil {
				t.Error("expected context to NOT be cancelled when events exist")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for startup watchdog goroutine to exit")
		}
	})

	t.Run("stall watchdog cancels on inactivity", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		var lastActivity atomic.Int64
		lastActivity.Store(time.Now().Add(-1 * time.Hour).UnixNano()) // Old activity

		go runStallWatchdog(ctx, cancel, &lastActivity, 10*time.Millisecond, 50*time.Millisecond)

		select {
		case <-ctx.Done():
			// Success — stall watchdog cancelled the context.
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for stall watchdog to cancel context")
		}
	})

	t.Run("stall watchdog stays alive with activity", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		var lastActivity atomic.Int64
		lastActivity.Store(time.Now().UnixNano())

		go runStallWatchdog(ctx, cancel, &lastActivity, 10*time.Millisecond, 500*time.Millisecond)

		// Keep updating activity
		for i := 0; i < 5; i++ {
			time.Sleep(20 * time.Millisecond)
			lastActivity.Store(time.Now().UnixNano())
		}

		if ctx.Err() != nil {
			t.Error("expected context to NOT be cancelled with recent activity")
		}
	})
}

// ---------------------------------------------------------------------------
// Exit codes
// ---------------------------------------------------------------------------

func TestStreamCoverage_ExitCodes(t *testing.T) {
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

// ---------------------------------------------------------------------------
// selectExecutor (default path)
// ---------------------------------------------------------------------------

func TestStreamCoverage_SelectExecutor(t *testing.T) {
	phases := []PhaseProgress{{Name: "test"}}
	executor := selectExecutor("/tmp/status", phases)
	if executor == nil {
		t.Fatal("expected non-nil executor")
	}
	// Default should be stream (auto mode always prefers stream)
	if executor.Name() != "stream" {
		t.Errorf("Name() = %q, want %q", executor.Name(), "stream")
	}
}

// ---------------------------------------------------------------------------
// selectExecutorWithLog
// ---------------------------------------------------------------------------

func TestStreamCoverage_SelectExecutorWithLog(t *testing.T) {
	tmp := t.TempDir()
	logPath := tmp + "/orch.log"
	phases := []PhaseProgress{{Name: "test"}}
	opts := defaultPhasedEngineOptions()

	t.Run("with live status selects stream", func(t *testing.T) {
		executor := selectExecutorWithLog("/tmp/status", phases, logPath, "run-1", true, opts)
		if executor.Name() != "stream" {
			t.Errorf("Name() = %q, want %q", executor.Name(), "stream")
		}
	})

	t.Run("without live status selects stream", func(t *testing.T) {
		executor := selectExecutorWithLog("/tmp/status", phases, logPath, "run-2", false, opts)
		if executor.Name() != "stream" {
			t.Errorf("Name() = %q, want %q", executor.Name(), "stream")
		}
	})

	t.Run("empty log path does not crash", func(t *testing.T) {
		executor := selectExecutorWithLog("/tmp/status", phases, "", "", false, opts)
		if executor == nil {
			t.Fatal("expected non-nil executor")
		}
	})

	t.Run("stream mode overrides", func(t *testing.T) {
		streamOpts := opts
		streamOpts.RuntimeMode = "stream"
		executor := selectExecutorWithLog("/tmp/status", phases, logPath, "run-3", false, streamOpts)
		if executor.Name() != "stream" {
			t.Errorf("Name() = %q, want %q", executor.Name(), "stream")
		}
	})
}
