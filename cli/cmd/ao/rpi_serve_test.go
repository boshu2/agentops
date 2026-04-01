package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

func TestParseServeRunTime(t *testing.T) {
	tests := []struct {
		input string
		zero  bool
	}{
		{"", true},
		{"  ", true},
		{"not-a-time", true},
		{"2026-03-09T21:14:06Z", false},
		{"2026-03-09T21:14:06.123456789Z", false},
	}
	for _, tt := range tests {
		got := parseServeRunTime(tt.input)
		if tt.zero && !got.IsZero() {
			t.Errorf("parseServeRunTime(%q) = %v, want zero", tt.input, got)
		}
		if !tt.zero && got.IsZero() {
			t.Errorf("parseServeRunTime(%q) = zero, want non-zero", tt.input)
		}
	}

	got := parseServeRunTime("2026-03-09T21:14:06Z")
	want := time.Date(2026, 3, 9, 21, 14, 6, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseServeRunTime parsed incorrectly: got %v, want %v", got, want)
	}
}

func TestServeRPIIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	serveRPIIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "mission control") {
		t.Errorf("expected 'mission control' in body")
	}
	if !strings.Contains(body, "EventSource") {
		t.Errorf("expected EventSource JS in body")
	}
}

func TestServeRPIRuns_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	rr := httptest.NewRecorder()
	serveRPIRuns(rr, req, dir)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}
}

func TestServeRPIEvents_NoRun(t *testing.T) {
	dir := t.TempDir()

	// Use a cancellable context to simulate client disconnect.
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events?run-id=nonexistent", nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		serveRPIEvents(rr, req, dir, "nonexistent")
		close(done)
	}()

	// Give the handler one poll cycle, then cancel.
	time.Sleep(600 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveRPIEvents did not return after context cancel")
	}

	// SSE headers must be set even with no events.
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
}

func TestWriteSSEEvent(t *testing.T) {
	rr := httptest.NewRecorder()
	ev := RPIC2Event{
		SchemaVersion: 1,
		EventID:       "evt-abc",
		RunID:         "rpi-test",
		Type:          "phase.stream.started",
		Message:       "hello",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := writeSSEEvent(rr, ev); err != nil {
		t.Fatalf("writeSSEEvent: %v", err)
	}
	body := rr.Body.String()
	if !strings.HasPrefix(body, "data: ") {
		t.Errorf("expected SSE 'data: ' prefix, got %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("expected SSE double-newline suffix, got %q", body)
	}
	// Verify payload is valid JSON.
	jsonPart := strings.TrimPrefix(body, "data: ")
	jsonPart = strings.TrimSpace(jsonPart)
	var parsed RPIC2Event
	if err := json.Unmarshal([]byte(jsonPart), &parsed); err != nil {
		t.Errorf("SSE payload is not valid JSON: %v\nbody: %q", err, body)
	}
	if parsed.EventID != ev.EventID {
		t.Errorf("event_id: want %q, got %q", ev.EventID, parsed.EventID)
	}
}

func TestBuildServeMux(t *testing.T) {
	dir := t.TempDir()
	mux := buildServeMux(&serveMuxRoot{path: dir}, "rpi-test")
	if mux == nil {
		t.Fatal("buildServeMux returned nil")
	}

	// Verify / route returns HTML.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/ returned %d, want 200", rr.Code)
	}

	// Verify /runs route returns JSON array.
	req = httptest.NewRequest(http.MethodGet, "/runs", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/runs returned %d, want 200", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/artifacts", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/artifacts returned %d, want 200", rr.Code)
	}
}

func TestSetCORSHeaders(t *testing.T) {
	// Without request, defaults to localhost
	rr := httptest.NewRecorder()
	setCORSHeaders(rr)
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "http://localhost" {
		t.Errorf("CORS origin (no request): want http://localhost, got %q", v)
	}

	// With localhost origin, echoes it back
	rr = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	setCORSHeaders(rr, req)
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "http://localhost:3000" {
		t.Errorf("CORS origin (localhost): want http://localhost:3000, got %q", v)
	}

	// With non-localhost origin, does not set origin header
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	setCORSHeaders(rr, req)
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("CORS origin (external): want empty, got %q", v)
	}
}

func TestClassifyServeArg_12HexRunID(t *testing.T) {
	tests := []struct {
		name      string
		flagRunID string
		args      []string
		wantGoal  string
		wantRunID string
	}{
		{"12-hex via flag", "760fc86f0c0f", nil, "", "760fc86f0c0f"},
		{"12-hex via arg", "", []string{"760fc86f0c0f"}, "", "760fc86f0c0f"},
		{"8-hex rpi prefix via flag", "rpi-a1b2c3d4", nil, "", "rpi-a1b2c3d4"},
		{"12-hex rpi prefix via flag", "rpi-760fc86f0c0f", nil, "", "rpi-760fc86f0c0f"},
		{"bare 8-hex via flag treated as goal", "0aa420a9", nil, "0aa420a9", ""},
		{"bare 8-hex via arg treated as goal", "", []string{"4c538e8a"}, "4c538e8a", ""},
		{"goal string", "improve-coverage", nil, "improve-coverage", ""},
		{"empty", "", nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goal, runID := classifyServeArg(tt.flagRunID, tt.args)
			if goal != tt.wantGoal || runID != tt.wantRunID {
				t.Errorf("classifyServeArg(%q, %v) = (%q, %q), want (%q, %q)",
					tt.flagRunID, tt.args, goal, runID, tt.wantGoal, tt.wantRunID)
			}
		})
	}
}

func TestClassifyServeArg_RunID(t *testing.T) {
	// Flag --run-id with valid run ID returns ("", runID)
	goal, runID := classifyServeArg("rpi-abcdef01", nil)
	if goal != "" {
		t.Errorf("expected empty goal, got %q", goal)
	}
	if runID != "rpi-abcdef01" {
		t.Errorf("expected run ID rpi-abcdef01, got %q", runID)
	}
}

func TestClassifyServeArg_Goal(t *testing.T) {
	// Non-run-ID string returns (goal, "")
	goal, runID := classifyServeArg("add user authentication", nil)
	if goal != "add user authentication" {
		t.Errorf("expected goal 'add user authentication', got %q", goal)
	}
	if runID != "" {
		t.Errorf("expected empty run ID, got %q", runID)
	}
}

func TestClassifyServeArg_EmptyArgs(t *testing.T) {
	// No args returns ("", "")
	goal, runID := classifyServeArg("", nil)
	if goal != "" {
		t.Errorf("expected empty goal, got %q", goal)
	}
	if runID != "" {
		t.Errorf("expected empty run ID, got %q", runID)
	}

	// Also with empty slice
	goal2, runID2 := classifyServeArg("", []string{})
	if goal2 != "" {
		t.Errorf("expected empty goal with empty slice, got %q", goal2)
	}
	if runID2 != "" {
		t.Errorf("expected empty run ID with empty slice, got %q", runID2)
	}
}

func TestBuildServeEngineOptions(t *testing.T) {
	cwd := t.TempDir()
	toolchain := cliRPI.Toolchain{
		RuntimeMode:    "direct",
		RuntimeCommand: "codex",
		AOCommand:      "/tmp/ao",
		BDCommand:      "/tmp/bd",
		TmuxCommand:    "/tmp/tmux",
	}
	opts := buildServeEngineOptions(cwd, "run-1234", toolchain)

	if opts.WorkingDir != cwd {
		t.Errorf("WorkingDir = %q, want %q", opts.WorkingDir, cwd)
	}
	if opts.RunID != "run-1234" {
		t.Errorf("RunID = %q, want %q", opts.RunID, "run-1234")
	}
	if !opts.NoDashboard {
		t.Error("expected NoDashboard=true")
	}
	if opts.RuntimeMode != "direct" {
		t.Errorf("RuntimeMode = %q, want %q", opts.RuntimeMode, "direct")
	}
	if opts.RuntimeCommand != "codex" {
		t.Errorf("RuntimeCommand = %q, want %q", opts.RuntimeCommand, "codex")
	}
	if opts.AOCommand != "/tmp/ao" {
		t.Errorf("AOCommand = %q, want %q", opts.AOCommand, "/tmp/ao")
	}
	if opts.BDCommand != "/tmp/bd" {
		t.Errorf("BDCommand = %q, want %q", opts.BDCommand, "/tmp/bd")
	}
	if opts.TmuxCommand != "/tmp/tmux" {
		t.Errorf("TmuxCommand = %q, want %q", opts.TmuxCommand, "/tmp/tmux")
	}
}

func TestBuildServeEngineOptions_DefaultsWithoutToolchainOverrides(t *testing.T) {
	cwd := t.TempDir()
	opts := buildServeEngineOptions(cwd, "run-default", cliRPI.Toolchain{})

	if opts.RuntimeMode != "auto" {
		t.Errorf("RuntimeMode = %q, want %q", opts.RuntimeMode, "auto")
	}
	if opts.RuntimeCommand != "claude" {
		t.Errorf("RuntimeCommand = %q, want %q", opts.RuntimeCommand, "claude")
	}
}

func TestClassifyServeArg_PositionalRunID(t *testing.T) {
	// Positional arg matching run ID pattern
	goal, runID := classifyServeArg("", []string{"rpi-a1b2c3d4"})
	if goal != "" {
		t.Errorf("expected empty goal, got %q", goal)
	}
	if runID != "rpi-a1b2c3d4" {
		t.Errorf("expected run ID rpi-a1b2c3d4, got %q", runID)
	}

	// Bare 8-hex positional — now treated as goal (not run ID)
	goal2, runID2 := classifyServeArg("", []string{"abcdef01"})
	if goal2 != "abcdef01" {
		t.Errorf("expected bare 8-hex as goal, got %q", goal2)
	}
	if runID2 != "" {
		t.Errorf("expected empty run ID for bare 8-hex, got %q", runID2)
	}
}

func TestClassifyServeArg_PositionalGoal(t *testing.T) {
	// Positional arg not matching run ID pattern
	goal, runID := classifyServeArg("", []string{"fix the cache bug"})
	if goal != "fix the cache bug" {
		t.Errorf("expected goal 'fix the cache bug', got %q", goal)
	}
	if runID != "" {
		t.Errorf("expected empty run ID, got %q", runID)
	}
}

func TestClassifyServeArg_FlagOverridesPositional(t *testing.T) {
	// Flag --run-id wins over positional arg
	goal, runID := classifyServeArg("rpi-deadbeef", []string{"some-goal"})
	if goal != "" {
		t.Errorf("expected empty goal when flag run-id is set, got %q", goal)
	}
	if runID != "rpi-deadbeef" {
		t.Errorf("expected run ID from flag, got %q", runID)
	}
}

func TestValidateExplicitServeRunID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty allowed", input: "", want: ""},
		{name: "bare 12-hex", input: "760fc86f0c0f", want: "760fc86f0c0f"},
		{name: "prefixed 8-hex", input: "rpi-a1b2c3d4", want: "rpi-a1b2c3d4"},
		{name: "bare 8-hex rejected", input: "3f0d90bd", wantErr: true},
		{name: "goal rejected", input: "add auth", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateExplicitServeRunID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateExplicitServeRunID(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldOpenBrowser_Default(t *testing.T) {
	origOpen, origNoOpen := rpiServeOpen, rpiServeNoOpen
	defer func() {
		rpiServeOpen = origOpen
		rpiServeNoOpen = origNoOpen
	}()

	rpiServeOpen = true
	rpiServeNoOpen = false
	if !shouldOpenBrowser() {
		t.Error("expected shouldOpenBrowser() to return true with defaults")
	}
}

func TestShouldOpenBrowser_NoOpen(t *testing.T) {
	origOpen, origNoOpen := rpiServeOpen, rpiServeNoOpen
	defer func() {
		rpiServeOpen = origOpen
		rpiServeNoOpen = origNoOpen
	}()

	rpiServeOpen = true
	rpiServeNoOpen = true
	if shouldOpenBrowser() {
		t.Error("expected shouldOpenBrowser() to return false with --no-open")
	}
}

func TestServeRPIRuns_ReturnsJSONArray(t *testing.T) {
	dir := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	rr := httptest.NewRecorder()
	serveRPIRuns(rr, req, dir)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON Content-Type, got %q", ct)
	}

	// Body should be a valid JSON array (possibly null or empty)
	body := strings.TrimSpace(rr.Body.String())
	if body != "null" && !strings.HasPrefix(body, "[") {
		t.Errorf("expected JSON array or null, got %q", body)
	}
}

func TestWriteSSEEvent_Format(t *testing.T) {
	rr := httptest.NewRecorder()
	ev := RPIC2Event{
		SchemaVersion: 1,
		EventID:       "evt-format-test",
		RunID:         "rpi-test",
		Type:          "phase.stream.started",
		Message:       "format check",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := writeSSEEvent(rr, ev); err != nil {
		t.Fatalf("writeSSEEvent: %v", err)
	}

	body := rr.Body.String()

	// SSE format: "data: <json>\n\n"
	if !strings.HasPrefix(body, "data: ") {
		t.Errorf("expected SSE 'data: ' prefix, got %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("expected SSE double-newline suffix, got %q", body)
	}

	// Verify payload is valid JSON
	jsonPart := strings.TrimPrefix(body, "data: ")
	jsonPart = strings.TrimSpace(jsonPart)
	var parsed RPIC2Event
	if err := json.Unmarshal([]byte(jsonPart), &parsed); err != nil {
		t.Errorf("SSE payload is not valid JSON: %v\nbody: %q", err, body)
	}
	if parsed.EventID != "evt-format-test" {
		t.Errorf("event_id: want %q, got %q", "evt-format-test", parsed.EventID)
	}
	if parsed.Message != "format check" {
		t.Errorf("message: want %q, got %q", "format check", parsed.Message)
	}
}

func TestShouldOpenBrowser(t *testing.T) {
	// Save and restore globals.
	origOpen, origNoOpen := rpiServeOpen, rpiServeNoOpen
	defer func() {
		rpiServeOpen = origOpen
		rpiServeNoOpen = origNoOpen
	}()

	tests := []struct {
		name   string
		open   bool
		noOpen bool
		want   bool
	}{
		{"defaults", true, false, true},
		{"--no-open", true, true, false},
		{"--open=false", false, false, false},
		{"both false", false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rpiServeOpen = tt.open
			rpiServeNoOpen = tt.noOpen
			if got := shouldOpenBrowser(); got != tt.want {
				t.Errorf("shouldOpenBrowser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDashboardServer_Config(t *testing.T) {
	handler := http.NewServeMux()
	addr := "localhost:9999"
	srv := newDashboardServer(addr, handler)

	if srv.Addr != addr {
		t.Errorf("Addr = %q, want %q", srv.Addr, addr)
	}
	if srv.Handler != handler {
		t.Error("Handler does not match the provided handler")
	}
	if srv.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 10s", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 8192 {
		t.Errorf("MaxHeaderBytes = %d, want 8192", srv.MaxHeaderBytes)
	}
}

func TestBuildServeMux_EmptyRunID(t *testing.T) {
	dir := t.TempDir()
	// An empty runID simulates watch mode when no run exists yet.
	mux := buildServeMux(&serveMuxRoot{path: dir}, "")
	if mux == nil {
		t.Fatal("buildServeMux returned nil with empty runID")
	}

	// / should still serve HTML.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/ returned %d, want 200", rr.Code)
	}

	// /runs should return a JSON array (possibly empty).
	req = httptest.NewRequest(http.MethodGet, "/runs", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("/runs returned %d, want 200", rr.Code)
	}
}

// TestBuildServeMux_DynamicRoot verifies that updating the serveMuxRoot after
// mux construction causes handlers to read from the new root.
func TestBuildServeMux_DynamicRoot(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	runID := "rpi-dynroot"

	// Seed events only in dir2.
	ev, err := appendRPIC2Event(dir2, rpiC2EventInput{
		RunID:   runID,
		Type:    "phase.stream.started",
		Message: "dynamic root event",
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}

	root := &serveMuxRoot{path: dir1}
	mux := buildServeMux(root, runID)

	// Before root switch: /events should return no events (dir1 is empty).
	ctx1, cancel1 := context.WithCancel(context.Background())
	req1 := httptest.NewRequest(http.MethodGet, "/events?run-id="+runID, nil)
	req1 = req1.WithContext(ctx1)
	rr1 := httptest.NewRecorder()
	done1 := make(chan struct{})
	go func() { mux.ServeHTTP(rr1, req1); close(done1) }()
	time.Sleep(700 * time.Millisecond)
	cancel1()
	<-done1

	if strings.Contains(rr1.Body.String(), ev.EventID) {
		t.Errorf("event should NOT appear before root switch")
	}

	// Switch root to dir2.
	root.set(dir2)

	// After root switch: /events should return the seeded event.
	ctx2, cancel2 := context.WithCancel(context.Background())
	req2 := httptest.NewRequest(http.MethodGet, "/events?run-id="+runID, nil)
	req2 = req2.WithContext(ctx2)
	rr2 := httptest.NewRecorder()
	done2 := make(chan struct{})
	go func() { mux.ServeHTTP(rr2, req2); close(done2) }()
	time.Sleep(700 * time.Millisecond)
	cancel2()
	<-done2

	if !strings.Contains(rr2.Body.String(), ev.EventID) {
		t.Errorf("event %q should appear after root switch, body: %q", ev.EventID, rr2.Body.String())
	}
}

// TestServeRPIEvents_StreamsPreExistingEvents verifies that events seeded before
// the handler starts are delivered as SSE data lines within one poll cycle.
func TestServeRPIEvents_StreamsPreExistingEvents(t *testing.T) {
	dir := t.TempDir()
	runID := "rpi-ssetest"

	// Seed two events before the handler starts.
	ev1, err := appendRPIC2Event(dir, rpiC2EventInput{
		RunID:   runID,
		Type:    "phase.stream.started",
		Message: "event one",
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event (1): %v", err)
	}
	ev2, err := appendRPIC2Event(dir, rpiC2EventInput{
		RunID:   runID,
		Type:    "phase.stream.done",
		Message: "event two",
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event (2): %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events?run-id="+runID, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		serveRPIEvents(rr, req, dir, runID)
		close(done)
	}()

	// Allow at least one full poll cycle (500ms ticker) plus CI scheduling margin, then cancel.
	time.Sleep(1200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveRPIEvents did not return after context cancel")
	}

	body := rr.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Errorf("expected SSE 'data: ' lines in body, got: %q", body)
	}
	if !strings.Contains(body, ev1.EventID) {
		t.Errorf("event 1 ID %q not found in SSE body", ev1.EventID)
	}
	if !strings.Contains(body, ev2.EventID) {
		t.Errorf("event 2 ID %q not found in SSE body", ev2.EventID)
	}
}

func TestServeRPIEvents_InitialFlush(t *testing.T) {
	dir := t.TempDir()
	runID := "rpi-flushtest"

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events?run-id="+runID, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		serveRPIEvents(rr, req, dir, runID)
		close(done)
	}()

	// The initial flush fires synchronously before the ticker loop.
	// Sleep briefly to let the goroutine start and write the comment.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveRPIEvents did not return after context cancel")
	}

	body := rr.Body.String()
	if !strings.HasPrefix(body, ": connected\n\n") {
		t.Errorf("expected SSE body to start with ': connected\\n\\n', got prefix: %q", body[:min(len(body), 40)])
	}
}

func TestServeRPIRuns_DiscoversSiblingWorktreeRuns(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "repo-rpi-serve")
	for _, dir := range []string{cwd, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	runID := "rpi-sibling-run"
	writeRegistryRun(t, sibling, registryRunSpec{
		runID:  runID,
		phase:  2,
		schema: 1,
		goal:   "follow sibling run",
		hbAge:  time.Minute,
	})

	req := httptest.NewRequest(http.MethodGet, "/runs", nil)
	rr := httptest.NewRecorder()
	serveRPIRuns(rr, req, cwd)

	var runs []rpiRunInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode /runs response: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 discovered run, got %d", len(runs))
	}
	if runs[0].RunID != runID {
		t.Fatalf("run_id = %q, want %q", runs[0].RunID, runID)
	}
	if runs[0].Worktree != sibling {
		t.Fatalf("worktree = %q, want %q", runs[0].Worktree, sibling)
	}
}

func TestServeRPIState_UsesRequestedRunRoot(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "repo-rpi-serve")
	for _, dir := range []string{cwd, sibling} {
		if err := os.MkdirAll(filepath.Join(dir, ".agents", "rpi"), 0o755); err != nil {
			t.Fatalf("mkdir rpi dir for %s: %v", dir, err)
		}
	}

	runID := "rpi-state-root"
	writeRegistryRun(t, sibling, registryRunSpec{
		runID:  runID,
		phase:  3,
		schema: 1,
		goal:   "state should resolve sibling root",
		hbAge:  time.Minute,
	})

	resultPath := filepath.Join(sibling, ".agents", "rpi", "phase-3-result.json")
	if err := os.WriteFile(resultPath, []byte(`{"phase":3,"status":"completed","phase_name":"validation"}`), 0o644); err != nil {
		t.Fatalf("write phase result: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/state?run-id="+runID, nil)
	rr := httptest.NewRecorder()
	serveRPIState(rr, req, cwd, "")

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode /state response: %v", err)
	}

	if got := resp["root"]; got != sibling {
		t.Fatalf("root = %v, want %q", got, sibling)
	}

	phasedState, ok := resp["phased_state"].(map[string]any)
	if !ok {
		t.Fatalf("phased_state missing or wrong type: %#v", resp["phased_state"])
	}
	if got := phasedState["run_id"]; got != runID {
		t.Fatalf("phased_state.run_id = %v, want %q", got, runID)
	}

	phaseResults, ok := resp["phase_results"].(map[string]any)
	if !ok {
		t.Fatalf("phase_results missing or wrong type: %#v", resp["phase_results"])
	}
	phase3, ok := phaseResults["phase_3"].(map[string]any)
	if !ok {
		t.Fatalf("phase_3 result missing: %#v", phaseResults)
	}
	if got := phase3["status"]; got != "completed" {
		t.Fatalf("phase_3.status = %v, want %q", got, "completed")
	}
	if artifacts, ok := resp["artifacts"].([]any); !ok || len(artifacts) == 0 {
		t.Fatalf("artifacts missing or empty: %#v", resp["artifacts"])
	}
}

func TestServeRPIEvents_ResolvesRequestedRunAcrossRoots(t *testing.T) {
	parent := t.TempDir()
	cwd := filepath.Join(parent, "repo")
	sibling := filepath.Join(parent, "repo-rpi-events")
	for _, dir := range []string{cwd, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	runID := "rpi-cross-root-events"
	writeRegistryRun(t, sibling, registryRunSpec{
		runID:  runID,
		phase:  2,
		schema: 1,
		goal:   "stream sibling events",
		hbAge:  time.Minute,
	})

	ev, err := appendRPIC2Event(sibling, rpiC2EventInput{
		RunID:   runID,
		Type:    "phase.stream.started",
		Message: "cross-root event",
	})
	if err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events?run-id="+runID, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		serveRPIEvents(rr, req, cwd, "")
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveRPIEvents did not return after context cancel")
	}

	body := rr.Body.String()
	if !strings.Contains(body, ev.EventID) {
		t.Fatalf("expected SSE body to contain event %q, got %q", ev.EventID, body)
	}
}

func TestServeRPIArtifacts_ListAndRead(t *testing.T) {
	root := t.TempDir()
	rpiDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(filepath.Join(rpiDir, "runs", "rpi-artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(rpiDir, "execution-packet.json")
	if err := os.WriteFile(packetPath, []byte("{\"objective\":\"artifact test\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evaluatorPath := filepath.Join(rpiDir, "phase-2-evaluator.json")
	if err := os.WriteFile(evaluatorPath, []byte("{\"verdict\":\"PASS\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artifacts?run-id=rpi-artifacts", nil)
	rr := httptest.NewRecorder()
	serveRPIArtifacts(rr, req, root, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("/artifacts returned %d", rr.Code)
	}

	var refs []rpiArtifactRef
	if err := json.Unmarshal(rr.Body.Bytes(), &refs); err != nil {
		t.Fatalf("decode /artifacts response: %v", err)
	}
	if len(refs) < 2 {
		t.Fatalf("expected at least 2 artifact refs, got %d", len(refs))
	}

	req = httptest.NewRequest(http.MethodGet, "/artifact?run-id=rpi-artifacts&path=.agents/rpi/execution-packet.json", nil)
	rr = httptest.NewRecorder()
	serveRPIArtifact(rr, req, root, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("/artifact returned %d", rr.Code)
	}

	var content rpiArtifactContent
	if err := json.Unmarshal(rr.Body.Bytes(), &content); err != nil {
		t.Fatalf("decode /artifact response: %v", err)
	}
	if content.Kind != "execution_packet" {
		t.Fatalf("kind = %q, want execution_packet", content.Kind)
	}
	if !strings.Contains(content.Body, "artifact test") {
		t.Fatalf("preview body missing packet content: %q", content.Body)
	}
}

func TestServeRPIArtifact_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	req := httptest.NewRequest(http.MethodGet, "/artifact?path=../../etc/passwd", nil)
	rr := httptest.NewRecorder()
	serveRPIArtifact(rr, req, root, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}
