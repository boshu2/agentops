package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// L1: Unit Tests — gcExecutor fields and simple methods
// =============================================================================

func TestGCExecutor_Name(t *testing.T) {
	e := &gcExecutor{}
	if e.Name() != "gc" {
		t.Errorf("gcExecutor.Name() = %q, want %q", e.Name(), "gc")
	}
}

func TestGCExecutor_DefaultTimeouts(t *testing.T) {
	e := &gcExecutor{}
	// Zero values should be handled by pollSessionCompletion defaults
	if e.phaseTimeout != 0 {
		t.Errorf("default phaseTimeout = %v, want 0 (uses 90m default)", e.phaseTimeout)
	}
	if e.pollInterval != 0 {
		t.Errorf("default pollInterval = %v, want 0 (uses 10s default)", e.pollInterval)
	}
}

func TestGCExecutor_CustomTimeouts(t *testing.T) {
	e := &gcExecutor{
		phaseTimeout: 5 * time.Minute,
		pollInterval: 1 * time.Second,
	}
	if e.phaseTimeout != 5*time.Minute {
		t.Errorf("phaseTimeout = %v, want 5m", e.phaseTimeout)
	}
	if e.pollInterval != 1*time.Second {
		t.Errorf("pollInterval = %v, want 1s", e.pollInterval)
	}
}

func TestGCExecutor_ResolveCityPath_Explicit(t *testing.T) {
	e := &gcExecutor{cityPath: "/explicit/city"}
	got := e.resolveCityPath("/some/cwd")
	if got != "/explicit/city" {
		t.Errorf("resolveCityPath with explicit = %q, want /explicit/city", got)
	}
}

func TestGCExecutor_ResolveCityPath_AutoDiscover(t *testing.T) {
	cityDir := setupCityDir(t, "auto-test")
	e := &gcExecutor{}
	got := e.resolveCityPath(cityDir)
	if got != cityDir {
		t.Errorf("resolveCityPath auto-discover = %q, want %q", got, cityDir)
	}
}

func TestGCExecutor_ResolveCityPath_NotFound(t *testing.T) {
	e := &gcExecutor{}
	got := e.resolveCityPath(t.TempDir())
	if got != "" {
		t.Errorf("resolveCityPath with no city.toml = %q, want empty", got)
	}
}

func TestGCExecutor_ResolveCityPath_Subdirectory(t *testing.T) {
	cityDir := setupCityDir(t, "subdir-test")
	subDir := filepath.Join(cityDir, "deep", "nested")
	os.MkdirAll(subDir, 0755)

	e := &gcExecutor{}
	got := e.resolveCityPath(subDir)
	if got != cityDir {
		t.Errorf("resolveCityPath from subdir = %q, want %q", got, cityDir)
	}
}

// =============================================================================
// L1: Mocked exec tests — checkSessionDone, gcRunCommand, gcExecutorAvailable
// =============================================================================

func TestGCExecutor_CheckSessionDone_Mocked_Closed(t *testing.T) {
	mock := newGCMock()
	sessionsJSON := `[{"id":"s1","alias":"rpi-run1-p1","state":"closed","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if !done {
		t.Error("checkSessionDone should return true for closed session")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_Completed(t *testing.T) {
	mock := newGCMock()
	sessionsJSON := `[{"id":"s1","alias":"rpi-run1-p1","state":"completed","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if !done {
		t.Error("checkSessionDone should return true for completed session")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_Active(t *testing.T) {
	mock := newGCMock()
	sessionsJSON := `[{"id":"s1","alias":"rpi-run1-p1","state":"active","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if done {
		t.Error("checkSessionDone should return false for active session")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_NotFound(t *testing.T) {
	mock := newGCMock()
	sessionsJSON := `[{"id":"s1","alias":"other-session","state":"active","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-missing-p1")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if !done {
		t.Error("checkSessionDone should return true when session not found (treated as complete)")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_EmptyList(t *testing.T) {
	mock := newGCMock()
	mock.on("session list --json", gcMockHandler{Stdout: "[]"})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-any-p1")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if !done {
		t.Error("checkSessionDone should return true when session list is empty")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_CommandFails(t *testing.T) {
	mock := newGCMock()
	mock.on("session list --json", gcMockHandler{ExitCode: 1})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	_, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err == nil {
		t.Error("checkSessionDone should return error when command fails")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_InvalidJSON(t *testing.T) {
	mock := newGCMock()
	mock.on("session list --json", gcMockHandler{Stdout: "not json"})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	_, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err == nil {
		t.Error("checkSessionDone should return error on invalid JSON")
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_MultipleSessions(t *testing.T) {
	mock := newGCMock()
	sessionsJSON := `[
		{"id":"s1","alias":"rpi-run1-p1","state":"active","template":"worker"},
		{"id":"s2","alias":"rpi-run1-p2","state":"closed","template":"worker"},
		{"id":"s3","alias":"rpi-run1-p3","state":"completed","template":"worker"}
	]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}

	// p1 is active
	done, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err != nil {
		t.Fatalf("p1 error: %v", err)
	}
	if done {
		t.Error("p1 (active) should not be done")
	}

	// p2 is closed
	done, err = e.checkSessionDone("/city", "rpi-run1-p2")
	if err != nil {
		t.Fatalf("p2 error: %v", err)
	}
	if !done {
		t.Error("p2 (closed) should be done")
	}

	// p3 is completed
	done, err = e.checkSessionDone("/city", "rpi-run1-p3")
	if err != nil {
		t.Fatalf("p3 error: %v", err)
	}
	if !done {
		t.Error("p3 (completed) should be done")
	}
}

func TestGCRunCommand_Mocked_WithCityPath(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcRunCommand(mock.execCommand, "/my/city", "session", "new", "--alias", "test-worker")
	if err != nil {
		t.Errorf("gcRunCommand error: %v", err)
	}
	calls := mock.callsMatching("session new")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	full := strings.Join(calls[0].Args, " ")
	if !strings.Contains(full, "--city /my/city") {
		t.Errorf("expected --city flag, got: %s", full)
	}
}

func TestGCRunCommand_Mocked_EmptyCityPath(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcRunCommand(mock.execCommand, "", "session", "list")
	if err != nil {
		t.Errorf("gcRunCommand error: %v", err)
	}
	calls := mock.callsMatching("session list")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	full := strings.Join(calls[0].Args, " ")
	if strings.Contains(full, "--city") {
		t.Errorf("should not have --city flag when empty, got: %s", full)
	}
}

func TestGCRunCommand_Mocked_NoDuplicateCity(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	// If args already contain --city, don't add it again
	err := gcRunCommand(mock.execCommand, "/my/city", "--city", "/other/city", "session", "list")
	if err != nil {
		t.Errorf("gcRunCommand error: %v", err)
	}
	calls := mock.callsMatching("session list")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	full := strings.Join(calls[0].Args, " ")
	// Should only have one --city (the one already in args)
	count := strings.Count(full, "--city")
	if count != 1 {
		t.Errorf("expected exactly 1 --city flag, got %d in: %s", count, full)
	}
}

func TestGCRunCommand_Mocked_Failure(t *testing.T) {
	mock := newGCMock()
	mock.on("session new --alias bad", gcMockHandler{ExitCode: 1, Stderr: "session error"})
	mock.install(t)

	err := gcRunCommand(mock.execCommand, "", "session", "new", "--alias", "bad")
	if err == nil {
		t.Error("gcRunCommand should return error on exit code 1")
	}
}

func TestGCExecutorAvailable_Mocked_AllGood(t *testing.T) {
	cityDir := setupCityDir(t, "avail-test")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"avail-test","controller":{"running":true,"pid":7},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})
	mock.install(t)

	if !gcExecutorAvailable(cityDir, mock.execCommand, mock.lookPathFn) {
		t.Error("gcExecutorAvailable should be true when the bridge is ready")
	}
}

func TestGCExecutorAvailable_Mocked_NoBinary(t *testing.T) {
	mock := newGCMock()
	mock.binaryAvailable = false
	mock.install(t)

	if gcExecutorAvailable(t.TempDir(), mock.execCommand, mock.lookPathFn) {
		t.Error("gcExecutorAvailable should be false when binary not found")
	}
}

func TestGCExecutorAvailable_Mocked_NoCityToml(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	if gcExecutorAvailable(t.TempDir(), mock.execCommand, mock.lookPathFn) {
		t.Error("gcExecutorAvailable should be false when no city.toml")
	}
}

func TestGCExecutorAvailable_Mocked_VersionTooLow(t *testing.T) {
	cityDir := setupCityDir(t, "old-version")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.12.0"})
	mock.install(t)

	if gcExecutorAvailable(cityDir, mock.execCommand, mock.lookPathFn) {
		t.Error("gcExecutorAvailable should be false when version too low")
	}
}

func TestGCExecutorAvailable_Mocked_VersionCheckFails(t *testing.T) {
	cityDir := setupCityDir(t, "version-fail")
	mock := newGCMock()
	mock.on("version", gcMockHandler{ExitCode: 1})
	mock.install(t)

	if gcExecutorAvailable(cityDir, mock.execCommand, mock.lookPathFn) {
		t.Error("gcExecutorAvailable should be false when version check fails")
	}
}

func TestGCExecutorAvailable_Mocked_ControllerStopped(t *testing.T) {
	cityDir := setupCityDir(t, "controller-stopped")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"controller-stopped","controller":{"running":false,"pid":0},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})
	mock.install(t)

	if gcExecutorAvailable(cityDir, mock.execCommand, mock.lookPathFn) {
		t.Error("gcExecutorAvailable should be false when the controller is stopped")
	}
}

// =============================================================================
// L1: Executor selection tests
// =============================================================================

func TestSelectExecutorFromCaps_GCBackend(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "gc"}
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = t.TempDir()

	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "gc" {
		t.Errorf("executor.Name() = %q, want %q", executor.Name(), "gc")
	}
	if reason != "runtime=gc" {
		t.Errorf("reason = %q, want %q", reason, "runtime=gc")
	}
}

func TestSelectExecutorFromCaps_GCFallbackToAuto(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "auto"}
	opts := defaultPhasedEngineOptions()

	executor, _ := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() == "gc" {
		t.Error("auto mode should not select gc executor")
	}
}

func TestSelectExecutorFromCaps_GCWithExplicitCityPath(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "gc"}
	opts := defaultPhasedEngineOptions()
	opts.GCCityPath = "/explicit/path"

	executor, _ := selectExecutorFromCaps(caps, "", nil, opts)
	gcExec, ok := executor.(*gcExecutor)
	if !ok {
		t.Fatal("executor is not *gcExecutor")
	}
	if gcExec.cityPath != "/explicit/path" {
		t.Errorf("cityPath = %q, want /explicit/path", gcExec.cityPath)
	}
}

func TestValidateRuntimeMode_GC(t *testing.T) {
	if err := validateRuntimeMode("gc"); err != nil {
		t.Errorf("validateRuntimeMode(\"gc\") should succeed, got: %v", err)
	}
}

func TestGCCityPathFromOpts(t *testing.T) {
	tests := []struct {
		name        string
		gcPath      string
		workingDir  string
		hasCityToml bool
		want        string
	}{
		{"explicit path", "/explicit/path", "", false, "/explicit/path"},
		{"whitespace-only explicit", "  ", "", false, ""},
		{"auto-discover with city.toml", "", "CITY_DIR", true, "CITY_DIR"},
		{"auto-discover no city.toml", "", "EMPTY_DIR", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaultPhasedEngineOptions()
			opts.GCCityPath = tt.gcPath

			if tt.hasCityToml {
				dir := setupCityDir(t, "opts-test")
				opts.WorkingDir = dir
				tt.want = dir
			} else if tt.workingDir == "EMPTY_DIR" {
				opts.WorkingDir = t.TempDir()
			}

			got := gcCityPathFromOpts(opts)
			if got != tt.want {
				t.Errorf("gcCityPathFromOpts = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// L2: Integration Tests — Execute + pollSessionCompletion with mocked exec
// =============================================================================

func TestGCExecutor_Execute_Mocked_NoCityToml(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	err := e.Execute(context.Background(), "test prompt", t.TempDir(), "run-1", 1)
	if err == nil {
		t.Error("Execute should fail when no city.toml found")
	}
	if !strings.Contains(err.Error(), "no city.toml found") {
		t.Errorf("error should mention no city.toml, got: %v", err)
	}
}

func TestGCExecutor_Execute_Mocked_BridgeNotReady(t *testing.T) {
	cityDir := setupCityDir(t, "not-ready")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.12.0"}) // too low
	mock.install(t)

	e := &gcExecutor{cityPath: cityDir, execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	err := e.Execute(context.Background(), "test prompt", cityDir, "run-2", 1)
	if err == nil {
		t.Error("Execute should fail when bridge not ready")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("error should mention not ready, got: %v", err)
	}
}

func TestGCExecutor_Execute_Mocked_SessionCreateFails(t *testing.T) {
	cityDir := setupCityDir(t, "create-fail")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"test","controller":{"running":true,"pid":1},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})
	mock.on("session new --alias rpi-run-3-p1 --template worker", gcMockHandler{ExitCode: 1, Stderr: "cannot create"})
	mock.install(t)

	e := &gcExecutor{cityPath: cityDir, execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	err := e.Execute(context.Background(), "test prompt", cityDir, "run-3", 1)
	if err == nil {
		t.Error("Execute should fail when session creation fails")
	}
	if !strings.Contains(err.Error(), "create session") {
		t.Errorf("error should mention create session, got: %v", err)
	}
}

func TestGCExecutor_PollSessionCompletion_Mocked_ImmediateComplete(t *testing.T) {
	cityDir := setupCityDir(t, "poll-test")
	mock := newGCMock()
	sessionsJSON := `[{"id":"s1","alias":"rpi-poll-p1","state":"completed","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{
		cityPath:     cityDir,
		pollInterval: 10 * time.Millisecond,
		phaseTimeout: 5 * time.Second,
		execCommand:  mock.execCommand,
		lookPath:     mock.lookPathFn,
	}

	err := e.pollSessionCompletion(context.Background(), cityDir, "rpi-poll-p1", "run-poll", 1)
	if err != nil {
		t.Errorf("pollSessionCompletion error: %v", err)
	}
}

func TestGCExecutor_PollSessionCompletion_Mocked_ContextCancelled(t *testing.T) {
	cityDir := setupCityDir(t, "cancel-test")
	mock := newGCMock()
	sessionsJSON := `[{"id":"s1","alias":"rpi-cancel-p1","state":"active","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancel()

	e := &gcExecutor{
		cityPath:     cityDir,
		pollInterval: 10 * time.Millisecond,
		phaseTimeout: 5 * time.Second,
		execCommand:  mock.execCommand,
		lookPath:     mock.lookPathFn,
	}

	err := e.pollSessionCompletion(ctx, cityDir, "rpi-cancel-p1", "run-cancel", 1)
	if err == nil {
		t.Error("pollSessionCompletion should return error on context cancellation")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGCExecutor_PollSessionCompletion_Mocked_Timeout(t *testing.T) {
	cityDir := setupCityDir(t, "timeout-test")
	mock := newGCMock()
	// Session stays active forever
	sessionsJSON := `[{"id":"s1","alias":"rpi-timeout-p1","state":"active","template":"worker"}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{
		cityPath:     cityDir,
		pollInterval: 10 * time.Millisecond,
		phaseTimeout: 50 * time.Millisecond, // very short timeout
		execCommand:  mock.execCommand,
		lookPath:     mock.lookPathFn,
	}

	err := e.pollSessionCompletion(context.Background(), cityDir, "rpi-timeout-p1", "run-timeout", 1)
	if err == nil {
		t.Error("pollSessionCompletion should return error on timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

func TestGCExecutor_PollSessionCompletion_Mocked_TransientError(t *testing.T) {
	cityDir := setupCityDir(t, "transient-test")
	mock := newGCMock()
	// First call fails, but pollSessionCompletion continues on transient errors
	// We can't easily simulate "first fail then succeed" with the simple mock,
	// so we test that a session list that fails doesn't crash the poller
	mock.on("session list --json", gcMockHandler{ExitCode: 1})
	mock.install(t)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	e := &gcExecutor{
		cityPath:     cityDir,
		pollInterval: 10 * time.Millisecond,
		phaseTimeout: 200 * time.Millisecond,
		execCommand:  mock.execCommand,
		lookPath:     mock.lookPathFn,
	}

	// Should timeout (not crash) because transient errors are retried
	err := e.pollSessionCompletion(ctx, cityDir, "rpi-transient-p1", "run-transient", 1)
	if err == nil {
		t.Error("expected error (timeout or cancelled)")
	}
}

// =============================================================================
// L3: Live Integration Tests — real gc binary and controller
// =============================================================================

func TestGCExecutorAvailable_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml found")
	}
	if ready, reason := gcBridgeReady(cityPath, nil, nil); !ready {
		t.Skipf("gc bridge not ready: %s", reason)
	}

	if !gcExecutorAvailable(cwd, nil, nil) {
		t.Errorf("gcExecutorAvailable should be true when the gc bridge is ready")
	}
}

func TestGCExecutor_Execute_Live_NoCityToml(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	e := &gcExecutor{}
	err := e.Execute(context.Background(), "test", t.TempDir(), "live-no-city", 1)
	if err == nil {
		t.Error("Execute should fail with no city.toml")
	}
	if !strings.Contains(err.Error(), "no city.toml found") {
		t.Errorf("error should mention no city.toml, got: %v", err)
	}
}

func TestGCExecutor_CheckSessionDone_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml found")
	}
	ready, reason := gcBridgeReady(cityPath, nil, nil)
	if !ready {
		t.Skipf("gc controller not running: %s", reason)
	}

	e := &gcExecutor{cityPath: cityPath}
	// Check for a session that almost certainly doesn't exist
	done, err := e.checkSessionDone(cityPath, "nonexistent-session-xyz")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if !done {
		t.Error("nonexistent session should be treated as done")
	}
}

func TestGCRunCommand_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml found")
	}
	ready, reason := gcBridgeReady(cityPath, nil, nil)
	if !ready {
		t.Skipf("gc controller not running: %s", reason)
	}

	// Run a read-only gc command (session list) to verify gcRunCommand works
	err := gcRunCommand(nil, cityPath, "session", "list", "--json")
	if err != nil {
		t.Errorf("gcRunCommand(session list) error: %v", err)
	}
}
