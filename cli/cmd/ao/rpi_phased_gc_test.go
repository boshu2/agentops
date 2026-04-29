package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/gascity"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

type fakeRPIGasCityClient struct {
	cityReadinessCalls int
	createCalls        []fakeRPIGasCityCreateCall
	submitCalls        []fakeRPIGasCitySubmitCall
	streamCalls        []fakeRPIGasCityStreamCall
	transcriptCalls    []fakeRPIGasCityTranscriptCall
	session            gascity.Session
	createMeta         gascity.ResponseMeta
	submitMeta         gascity.ResponseMeta
	streamMeta         gascity.ResponseMeta
	transcriptMeta     gascity.ResponseMeta
	transcript         gascity.TranscriptResponse
	stream             *fakeRPIGasCityEventStream
}

type fakeRPIGasCityCreateCall struct {
	cityName string
	req      gascity.SessionCreateRequest
}

type fakeRPIGasCitySubmitCall struct {
	cityName  string
	sessionID string
	req       gascity.SessionSubmitRequest
}

type fakeRPIGasCityStreamCall struct {
	cityName string
	opts     gascity.EventStreamOptions
}

type fakeRPIGasCityTranscriptCall struct {
	cityName  string
	sessionID string
	opts      gascity.TranscriptOptions
}

type fakeRPIGasCityEventStream struct {
	frames    []gascity.EventStreamFrame
	nextCalls int
	closed    bool
}

func (s *fakeRPIGasCityEventStream) NextEvent() (gascity.EventStreamFrame, error) {
	s.nextCalls++
	if len(s.frames) == 0 {
		return gascity.EventStreamFrame{}, io.EOF
	}
	frame := s.frames[0]
	s.frames = s.frames[1:]
	return frame, nil
}

func (s *fakeRPIGasCityEventStream) Close() error {
	s.closed = true
	return nil
}

func (f *fakeRPIGasCityClient) CityReadiness(context.Context, string) (gascity.ReadinessResponse, error) {
	f.cityReadinessCalls++
	return gascity.ReadinessResponse{Ready: true}, nil
}

func (f *fakeRPIGasCityClient) CreateSession(_ context.Context, cityName string, req gascity.SessionCreateRequest) (gascity.Session, gascity.ResponseMeta, error) {
	f.createCalls = append(f.createCalls, fakeRPIGasCityCreateCall{cityName: cityName, req: req})
	session := f.session
	if session.ID == "" {
		session.ID = "sess_fake"
	}
	meta := f.createMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-create"
	}
	return session, meta, nil
}

func (f *fakeRPIGasCityClient) SubmitSession(_ context.Context, cityName string, sessionID string, req gascity.SessionSubmitRequest) (gascity.SessionSubmitResponse, gascity.ResponseMeta, error) {
	f.submitCalls = append(f.submitCalls, fakeRPIGasCitySubmitCall{cityName: cityName, sessionID: sessionID, req: req})
	meta := f.submitMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-submit"
	}
	return gascity.SessionSubmitResponse{Queued: true, Intent: req.Intent}, meta, nil
}

func (f *fakeRPIGasCityClient) GetSession(context.Context, string, string, gascity.SessionGetOptions) (gascity.Session, gascity.ResponseMeta, error) {
	return gascity.Session{}, gascity.ResponseMeta{}, nil
}

func (f *fakeRPIGasCityClient) SessionTranscript(_ context.Context, cityName string, sessionID string, opts gascity.TranscriptOptions) (gascity.TranscriptResponse, gascity.ResponseMeta, error) {
	f.transcriptCalls = append(f.transcriptCalls, fakeRPIGasCityTranscriptCall{cityName: cityName, sessionID: sessionID, opts: opts})
	transcript := f.transcript
	if transcript.ID == "" && transcript.SessionID == "" {
		transcript.ID = sessionID
		transcript.SessionID = sessionID
		transcript.Format = opts.Format
		transcript.Turns = []gascity.TranscriptEntry{{Role: "assistant", Text: "done"}}
	}
	meta := f.transcriptMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-transcript"
	}
	return transcript, meta, nil
}

func (f *fakeRPIGasCityClient) ListCityEvents(context.Context, string, gascity.EventListParams) (gascity.EventListResponse, gascity.ResponseMeta, error) {
	return gascity.EventListResponse{}, gascity.ResponseMeta{}, nil
}

func (f *fakeRPIGasCityClient) StreamCityEvents(_ context.Context, cityName string, opts gascity.EventStreamOptions) (rpiGasCityEventStream, gascity.ResponseMeta, error) {
	f.streamCalls = append(f.streamCalls, fakeRPIGasCityStreamCall{cityName: cityName, opts: opts})
	stream := f.stream
	if stream == nil {
		sessionID := f.session.ID
		if sessionID == "" {
			sessionID = "sess_fake"
		}
		stream = &fakeRPIGasCityEventStream{frames: []gascity.EventStreamFrame{{
			ID: "1",
			CityEvent: &gascity.EventStreamEnvelope{
				Seq:     1,
				Type:    "session.completed",
				Subject: sessionID,
				Payload: map[string]any{"status": "completed", "session_id": sessionID},
			},
		}}}
		f.stream = stream
	}
	meta := f.streamMeta
	if meta.RequestID == "" {
		meta.RequestID = "req-stream"
	}
	return stream, meta, nil
}

func (f *fakeRPIGasCityClient) EmitCityEvent(context.Context, string, gascity.EventEmitRequest) (gascity.EventEmitResponse, gascity.ResponseMeta, error) {
	return gascity.EventEmitResponse{}, gascity.ResponseMeta{}, nil
}

// =============================================================================
// L1: Unit Tests — gcExecutor fields and simple methods
// =============================================================================

func TestGCExecutor_Name(t *testing.T) {
	e := &gcExecutor{}
	if e.Name() != "gc" {
		t.Errorf("gcExecutor.Name() = %q, want %q", e.Name(), "gc")
	}
}

func TestGCExecutor_AcceptsInjectedGasCityClient(t *testing.T) {
	fake := &fakeRPIGasCityClient{}
	e := &gcExecutor{apiClient: fake, apiCityName: "agentops"}

	client, ok := e.gasCityClient()
	if !ok {
		t.Fatal("gasCityClient should report an injected client")
	}
	if client != fake {
		t.Fatal("gasCityClient should return the injected fake client")
	}

	ready, err := client.CityReadiness(context.Background(), "agentops")
	if err != nil {
		t.Fatalf("CityReadiness: %v", err)
	}
	if !ready.Ready {
		t.Fatal("fake readiness should be true")
	}
	if fake.cityReadinessCalls != 1 {
		t.Fatalf("cityReadinessCalls = %d, want 1", fake.cityReadinessCalls)
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

func TestGCExecutor_CheckSessionDone_Mocked_GCV1DormantState(t *testing.T) {
	mock := newGCMock()
	sessionsJSON := `[{"ID":"s1","Alias":"rpi-run1-p1","State":"asleep","Template":"worker","Closed":false}]`
	mock.on("session list --json", gcMockHandler{Stdout: sessionsJSON})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err != nil {
		t.Fatalf("checkSessionDone error: %v", err)
	}
	if !done {
		t.Error("checkSessionDone should return true for gc v1 dormant sessions")
	}
}

func TestGCSessionDoneStates(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"creating", false},
		{"active", false},
		{"awake", false},
		{"draining", false},
		{"closed", true},
		{"completed", true},
		{"asleep", true},
		{"suspended", true},
		{"drained", true},
		{"archived", true},
		{"stopped", true},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := gcSessionDone(GCSession{State: tt.state})
			if got != tt.want {
				t.Fatalf("gcSessionDone(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
	if !gcSessionDone(GCSession{State: "active", Closed: true}) {
		t.Fatal("gcSessionDone should honor Closed=true")
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
	if err == nil {
		t.Fatal("checkSessionDone should return lost error when session is not found")
	}
	if done {
		t.Error("checkSessionDone should not return done when session is lost")
	}
	var lostErr *cliRPI.ProviderSessionLostError
	if !errors.As(err, &lostErr) {
		t.Fatalf("err = %T %v, want ProviderSessionLostError", err, err)
	}
}

func TestGCExecutor_CheckSessionDone_Mocked_EmptyList(t *testing.T) {
	mock := newGCMock()
	mock.on("session list --json", gcMockHandler{Stdout: "[]"})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-any-p1")
	if err == nil {
		t.Fatal("checkSessionDone should return lost error when session list is empty")
	}
	if done {
		t.Error("checkSessionDone should not return done when session list is empty")
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

func TestGCExecutor_CheckSessionDone_Mocked_SchemaDriftMissingAlias(t *testing.T) {
	mock := newGCMock()
	mock.on("session list --json", gcMockHandler{Stdout: `[{"id":"s1","state":"active","template":"worker"}]`})
	mock.install(t)

	e := &gcExecutor{execCommand: mock.execCommand, lookPath: mock.lookPathFn}
	done, err := e.checkSessionDone("/city", "rpi-run1-p1")
	if err == nil {
		t.Fatal("checkSessionDone should return error when session JSON is missing alias")
	}
	if done {
		t.Fatal("checkSessionDone should not treat schema drift as complete")
	}
	if !strings.Contains(err.Error(), `missing required field "alias"`) {
		t.Errorf("error should mention missing alias field, got: %v", err)
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
	if reason != "runtime=gc backend=gc-cli-fallback" {
		t.Errorf("reason = %q, want %q", reason, "runtime=gc backend=gc-cli-fallback")
	}
}

func TestSelectExecutorFromCaps_GCAPIBackendReason(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "gc"}
	opts := defaultPhasedEngineOptions()
	opts.GasCityClient = &fakeRPIGasCityClient{}
	opts.GCCityName = "agentops"

	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	gcExec, ok := executor.(*gcExecutor)
	if !ok {
		t.Fatal("executor is not *gcExecutor")
	}
	if gcExec.backendMode() != "gc-api" {
		t.Fatalf("backendMode = %q, want gc-api", gcExec.backendMode())
	}
	if reason != "runtime=gc backend=gc-api" {
		t.Fatalf("reason = %q, want runtime=gc backend=gc-api", reason)
	}
}

func TestSelectExecutorFromCaps_AutoLogsGCDegradedReason(t *testing.T) {
	caps := backendCapabilities{RuntimeMode: "auto"}
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = t.TempDir()

	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() == "gc" {
		t.Fatal("auto mode should not select gc when no city is available")
	}
	if !strings.Contains(reason, "gc-degraded=") || !strings.Contains(reason, "city.toml") {
		t.Fatalf("reason = %q, want degraded city.toml detail", reason)
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

func TestPreflightOpts_GCRuntimeSkipsRuntimeCommand(t *testing.T) {
	cityDir := setupCityDir(t, "gc-preflight")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"gc-preflight","controller":{"running":true,"pid":42},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cityDir
	opts.RuntimeMode = "gc"
	opts.RuntimeCommand = "missing-runtime-command"
	opts.ExecCommand = mock.execCommand
	opts.LookPath = mock.lookPathFn

	if err := preflightOpts(&opts); err != nil {
		t.Fatalf("preflightOpts(runtime=gc) error = %v, want nil", err)
	}
	if len(mock.callsMatching("version")) == 0 {
		t.Fatal("expected gc version check during preflight")
	}
}

func TestPreflightOpts_AutoRuntimeSkipsRuntimeCommandWhenGCReady(t *testing.T) {
	cityDir := setupCityDir(t, "gc-auto-preflight")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"gc-auto-preflight","controller":{"running":true,"pid":42},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cityDir
	opts.RuntimeMode = "auto"
	opts.RuntimeCommand = "missing-runtime-command"
	opts.ExecCommand = mock.execCommand
	opts.LookPath = mock.lookPathFn

	if err := preflightOpts(&opts); err != nil {
		t.Fatalf("preflightOpts(runtime=auto with ready gc) error = %v, want nil", err)
	}
}

func TestPreparePhasedRun_GCPreflightUsesResolvedWorkingDir(t *testing.T) {
	cityDir := setupCityDir(t, "gc-prepare-preflight")
	t.Chdir(cityDir)

	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "1.0.0"})
	statusJSON := `{"city":null,"controller":{"running":true,"pid":42,"mode":"standalone"},"agents":[],"summary":{"total_agents":0,"running_agents":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	opts := defaultPhasedEngineOptions()
	opts.RuntimeMode = "gc"
	opts.NoWorktree = true
	opts.ExecCommand = mock.execCommand
	opts.LookPath = mock.lookPathFn

	run, err := preparePhasedRun(&opts, []string{"validate gc preflight"})
	if err != nil {
		t.Fatalf("preparePhasedRun(runtime=gc) error = %v, want nil", err)
	}
	if run.cwd != cityDir {
		t.Fatalf("run.cwd = %q, want %q", run.cwd, cityDir)
	}
	if opts.WorkingDir != cityDir {
		t.Fatalf("opts.WorkingDir = %q, want %q", opts.WorkingDir, cityDir)
	}
	if len(mock.callsMatching("--city "+cityDir+" status --json")) == 0 {
		t.Fatalf("expected gc status preflight to use resolved city %q, calls: %#v", cityDir, mock.calls)
	}
}

func TestPreflightOpts_GCRuntimeRequiresCity(t *testing.T) {
	mock := newGCMock()
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = t.TempDir()
	opts.RuntimeMode = "gc"
	opts.ExecCommand = mock.execCommand
	opts.LookPath = mock.lookPathFn

	err := preflightOpts(&opts)
	if err == nil {
		t.Fatal("expected error when runtime=gc has no city.toml")
	}
	if !strings.Contains(err.Error(), "city.toml") {
		t.Fatalf("error = %q, want city.toml detail", err.Error())
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

func TestGCExecutor_Execute_API_CreatesAndSubmitsSession(t *testing.T) {
	cityDir := setupCityDir(t, "api-create-submit")
	fake := &fakeRPIGasCityClient{
		session:    gascity.Session{ID: "sess_rpi_123"},
		createMeta: gascity.ResponseMeta{RequestID: "req-create-123"},
		submitMeta: gascity.ResponseMeta{RequestID: "req-submit-456"},
	}
	e := &gcExecutor{
		cityPath:    cityDir,
		apiClient:   fake,
		apiCityName: "agentops",
	}

	err := e.Execute(context.Background(), "phase prompt", cityDir, "run-api", 3)
	if err != nil {
		t.Fatalf("Execute with API client: %v", err)
	}

	if len(fake.createCalls) != 1 {
		t.Fatalf("create calls = %d, want 1", len(fake.createCalls))
	}
	create := fake.createCalls[0]
	if create.cityName != "agentops" {
		t.Fatalf("create cityName = %q, want agentops", create.cityName)
	}
	if create.req.Alias != "rpi-run-api-p3" {
		t.Fatalf("create alias = %q, want rpi-run-api-p3", create.req.Alias)
	}
	if create.req.Kind != "agent" || create.req.Name != "worker" || !create.req.Async {
		t.Fatalf("create request = %#v, want async agent worker", create.req)
	}

	if len(fake.submitCalls) != 1 {
		t.Fatalf("submit calls = %d, want 1", len(fake.submitCalls))
	}
	submit := fake.submitCalls[0]
	if submit.cityName != "agentops" || submit.sessionID != "sess_rpi_123" {
		t.Fatalf("submit target = %s/%s, want agentops/sess_rpi_123", submit.cityName, submit.sessionID)
	}
	if submit.req.Message != "phase prompt" || submit.req.Intent != "follow_up" {
		t.Fatalf("submit request = %#v, want prompt follow_up", submit.req)
	}

	if e.apiLastPhase.SessionID != "sess_rpi_123" ||
		e.apiLastPhase.CreateRequestID != "req-create-123" ||
		e.apiLastPhase.SubmitRequestID != "req-submit-456" {
		t.Fatalf("apiLastPhase = %#v, want session and request IDs recorded", e.apiLastPhase)
	}
}

func TestGCExecutor_Execute_API_WaitsForStartedAndCompleteEvents(t *testing.T) {
	cityDir := setupCityDir(t, "api-event-wait")
	stream := &fakeRPIGasCityEventStream{frames: []gascity.EventStreamFrame{
		{
			ID: "evt-1",
			CityEvent: &gascity.EventStreamEnvelope{
				Seq:     1,
				Type:    "session.started",
				Subject: "sess_events",
				Payload: map[string]any{"session_id": "sess_events", "status": "running"},
			},
		},
		{
			ID: "evt-2",
			CityEvent: &gascity.EventStreamEnvelope{
				Seq:     2,
				Type:    "session.completed",
				Subject: "sess_events",
				Payload: map[string]any{"session_id": "sess_events", "status": "completed"},
			},
		},
	}}
	fake := &fakeRPIGasCityClient{
		session:    gascity.Session{ID: "sess_events"},
		streamMeta: gascity.ResponseMeta{RequestID: "req-stream-events"},
		stream:     stream,
	}
	e := &gcExecutor{
		cityPath:     cityDir,
		apiClient:    fake,
		apiCityName:  "agentops",
		phaseTimeout: time.Second,
	}

	err := e.Execute(context.Background(), "event prompt", cityDir, "run-events", 1)
	if err != nil {
		t.Fatalf("Execute with API events: %v", err)
	}
	if len(fake.streamCalls) != 1 || fake.streamCalls[0].cityName != "agentops" {
		t.Fatalf("stream calls = %#v, want one agentops stream call", fake.streamCalls)
	}
	if stream.nextCalls != 2 {
		t.Fatalf("stream nextCalls = %d, want 2", stream.nextCalls)
	}
	if !stream.closed {
		t.Fatal("event stream should be closed")
	}
	if !e.apiLastPhase.StartedEventSeen {
		t.Fatal("started event should be recorded")
	}
	if e.apiLastPhase.TerminalStatus != gascity.TerminalStatusCompleted {
		t.Fatalf("terminal status = %q, want completed", e.apiLastPhase.TerminalStatus)
	}
	if e.apiLastPhase.LastEventID != "evt-2" {
		t.Fatalf("last event ID = %q, want evt-2", e.apiLastPhase.LastEventID)
	}
	if e.apiLastPhase.EventStreamRequestID != "req-stream-events" {
		t.Fatalf("stream request ID = %q, want req-stream-events", e.apiLastPhase.EventStreamRequestID)
	}
}

func TestGCExecutor_Execute_API_WritesTranscriptEvidenceArtifact(t *testing.T) {
	cityDir := setupCityDir(t, "api-transcript-evidence")
	fake := &fakeRPIGasCityClient{
		session:        gascity.Session{ID: "sess_evidence"},
		createMeta:     gascity.ResponseMeta{RequestID: "req-create-evidence"},
		submitMeta:     gascity.ResponseMeta{RequestID: "req-submit-evidence"},
		streamMeta:     gascity.ResponseMeta{RequestID: "req-stream-evidence"},
		transcriptMeta: gascity.ResponseMeta{RequestID: "req-transcript-evidence"},
		transcript: gascity.TranscriptResponse{
			ID:       "transcript-evidence",
			Format:   "conversation",
			Turns:    []gascity.TranscriptEntry{{Role: "assistant", Text: "finished"}},
			Messages: []map[string]any{{"role": "assistant", "content": "finished"}},
			Artifacts: []gascity.TranscriptArtifact{{
				Path: ".agents/rpi/phase-2-summary.md",
				Kind: "summary",
			}},
		},
		stream: &fakeRPIGasCityEventStream{frames: []gascity.EventStreamFrame{{
			ID: "evidence-cursor",
			CityEvent: &gascity.EventStreamEnvelope{
				Seq:     7,
				Type:    "session.completed",
				Subject: "sess_evidence",
				Payload: map[string]any{"session_id": "sess_evidence", "status": "completed"},
			},
		}}},
	}
	e := &gcExecutor{
		cityPath:     cityDir,
		apiClient:    fake,
		apiCityName:  "agentops",
		phaseTimeout: time.Second,
	}

	err := e.Execute(context.Background(), "capture evidence", cityDir, "run-evidence", 2)
	if err != nil {
		t.Fatalf("Execute with transcript evidence: %v", err)
	}
	if len(fake.transcriptCalls) != 1 {
		t.Fatalf("transcript calls = %d, want 1", len(fake.transcriptCalls))
	}
	if fake.transcriptCalls[0].opts.Format != "conversation" {
		t.Fatalf("transcript format = %q, want conversation", fake.transcriptCalls[0].opts.Format)
	}
	if e.apiLastPhase.EvidencePath == "" {
		t.Fatal("EvidencePath should be recorded")
	}

	data, err := os.ReadFile(e.apiLastPhase.EvidencePath)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	var evidence cliRPI.GasCityPhaseEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if evidence.RunID != "run-evidence" || evidence.Phase != 2 || evidence.SessionID != "sess_evidence" {
		t.Fatalf("unexpected evidence identity: %#v", evidence)
	}
	if evidence.EventCursor != "evidence-cursor" || evidence.RequestIDs["transcript"] != "req-transcript-evidence" {
		t.Fatalf("unexpected evidence metadata: %#v", evidence)
	}
	if evidence.TranscriptID != "transcript-evidence" || evidence.TranscriptTurnCount != 1 || evidence.TranscriptMsgCount != 1 {
		t.Fatalf("unexpected transcript evidence: %#v", evidence)
	}
	if len(evidence.TranscriptArtifacts) != 1 || evidence.TranscriptArtifacts[0].Kind != "summary" {
		t.Fatalf("unexpected transcript artifacts: %#v", evidence.TranscriptArtifacts)
	}
}

func TestGCExecutor_Execute_API_HTTPFixtureEndToEnd(t *testing.T) {
	cityDir := setupCityDir(t, "api-http-fixture")
	var sawCreate, sawSubmit, sawStream, sawTranscript bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(gascity.RequestIDHeader, "req-"+strings.Trim(strings.ReplaceAll(r.URL.Path, "/", "-"), "-"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/readiness":
			_ = json.NewEncoder(w).Encode(gascity.ReadinessResponse{Ready: true, Status: "ready"})
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city/agentops/sessions":
			sawCreate = true
			if got := r.Header.Get(gascity.MutationHeader); got == "" {
				t.Fatalf("create missing %s", gascity.MutationHeader)
			}
			var req gascity.SessionCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			if req.Alias != "rpi-run-http-p2" || req.Name != "worker" || !req.Async {
				t.Fatalf("create request = %#v", req)
			}
			_ = json.NewEncoder(w).Encode(gascity.Session{ID: "sess_http", Alias: req.Alias, Running: true})
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city/agentops/session/sess_http/submit":
			sawSubmit = true
			if got := r.Header.Get(gascity.MutationHeader); got == "" {
				t.Fatalf("submit missing %s", gascity.MutationHeader)
			}
			var req gascity.SessionSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode submit: %v", err)
			}
			if req.Message != "http fixture prompt" || req.Intent != "follow_up" {
				t.Fatalf("submit request = %#v", req)
			}
			_ = json.NewEncoder(w).Encode(gascity.SessionSubmitResponse{Queued: true, Intent: req.Intent})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/events/stream":
			sawStream = true
			w.Header().Set("Content-Type", "text/event-stream")
			writeSSEFrame(t, w, "http-1", gascity.EventStreamEnvelope{
				Seq:     1,
				Type:    "session.started",
				Subject: "sess_http",
				Payload: map[string]any{"session_id": "sess_http", "status": "running"},
			})
			writeSSEFrame(t, w, "http-2", gascity.EventStreamEnvelope{
				Seq:     2,
				Type:    "session.completed",
				Subject: "sess_http",
				Payload: map[string]any{"session_id": "sess_http", "status": "completed"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/session/sess_http/transcript":
			sawTranscript = true
			if got := r.URL.Query().Get("format"); got != "conversation" {
				t.Fatalf("transcript format = %q, want conversation", got)
			}
			_ = json.NewEncoder(w).Encode(gascity.TranscriptResponse{
				ID:       "transcript-http",
				Format:   "conversation",
				Turns:    []gascity.TranscriptEntry{{Role: "assistant", Text: "done"}},
				Messages: []map[string]any{{"role": "assistant", "content": "done"}},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client, err := gascity.NewClient(gascity.Config{Endpoint: server.URL, MutationToken: "agentops-test"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	e := &gcExecutor{
		cityPath:     cityDir,
		apiClient:    &rpiGasCityAPIAdapter{client: client},
		apiCityName:  "agentops",
		phaseTimeout: time.Second,
	}

	if err := e.Execute(context.Background(), "http fixture prompt", cityDir, "run-http", 2); err != nil {
		t.Fatalf("Execute HTTP fixture: %v", err)
	}
	for name, saw := range map[string]bool{
		"create":     sawCreate,
		"submit":     sawSubmit,
		"stream":     sawStream,
		"transcript": sawTranscript,
	} {
		if !saw {
			t.Fatalf("server did not see %s request", name)
		}
	}
	if e.apiLastPhase.TerminalStatus != gascity.TerminalStatusCompleted ||
		e.apiLastPhase.LastEventID != "http-2" ||
		e.apiLastPhase.EvidencePath == "" {
		t.Fatalf("apiLastPhase = %#v", e.apiLastPhase)
	}
	data, err := os.ReadFile(e.apiLastPhase.EvidencePath)
	if err != nil {
		t.Fatalf("read HTTP fixture evidence: %v", err)
	}
	if !strings.Contains(string(data), `"transcript_id": "transcript-http"`) ||
		!strings.Contains(string(data), `"event_cursor": "http-2"`) {
		t.Fatalf("evidence missing transcript/cursor:\n%s", string(data))
	}
}

func writeSSEFrame(t *testing.T, w http.ResponseWriter, id string, event gascity.EventStreamEnvelope) {
	t.Helper()
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal SSE event: %v", err)
	}
	fmt.Fprintf(w, "id: %s\n", id)
	fmt.Fprintln(w, "event: event")
	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func TestGCExecutor_Execute_Mocked_SessionCreateFails(t *testing.T) {
	cityDir := setupCityDir(t, "create-fail")
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"test","controller":{"running":true,"pid":1},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})
	mock.on("session new worker --alias rpi-run-3-p1 --no-attach", gcMockHandler{ExitCode: 1, Stderr: "cannot create"})
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

func TestGCExecutor_PollSessionCompletion_Mocked_LostSessionReturnsError(t *testing.T) {
	cityDir := setupCityDir(t, "poll-lost")
	mock := newGCMock()
	mock.on("session list --json", gcMockHandler{Stdout: `[]`})
	mock.install(t)

	e := &gcExecutor{
		cityPath:     cityDir,
		pollInterval: 10 * time.Millisecond,
		phaseTimeout: time.Second,
		execCommand:  mock.execCommand,
		lookPath:     mock.lookPathFn,
	}

	err := e.pollSessionCompletion(context.Background(), cityDir, "rpi-lost-p1", "run-lost", 1)
	if err == nil {
		t.Fatal("pollSessionCompletion should return lost error")
	}
	var lostErr *cliRPI.ProviderSessionLostError
	if !errors.As(err, &lostErr) {
		t.Fatalf("err = %T %v, want ProviderSessionLostError", err, err)
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
	if err == nil {
		t.Fatal("checkSessionDone should return lost error for nonexistent session")
	}
	if done {
		t.Fatal("nonexistent session should not be treated as done")
	}
	var lostErr *cliRPI.ProviderSessionLostError
	if !errors.As(err, &lostErr) {
		t.Fatalf("err = %T %v, want ProviderSessionLostError", err, err)
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
