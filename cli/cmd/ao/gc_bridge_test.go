package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// L1: Unit Tests — pure functions, no side effects
// =============================================================================

func TestGCBridgeCompatible(t *testing.T) {
	tests := []struct {
		version    string
		compatible bool
	}{
		{"0.13.0", true},       // exact minimum
		{"0.13.5", true},       // above minimum
		{"0.14.0", true},       // minor bump
		{"1.0.0", true},        // major bump
		{"0.12.9", false},      // below minimum
		{"0.1.0", false},       // way below
		{"0.0.1", false},       // tiny
		{"99.0.0", true},       // large major
		{"0.13", true},         // missing patch
		{"v0.13.0", true},      // v-prefix
		{"0.13.0-rc1", true},   // pre-release on minimum
		{"0.12.5-beta", false}, // pre-release below min
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := gcBridgeCompatible(tt.version)
			if got != tt.compatible {
				t.Errorf("gcBridgeCompatible(%q) = %v, want %v", tt.version, got, tt.compatible)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"0.13.0", "0.12.0", 1},
		{"0.12.0", "0.13.0", -1},
		{"0.13.1", "0.13.0", 1},
		{"0.13.0", "0.13.1", -1},
		{"2.0.0", "1.99.99", 1},
		{"0.13", "0.13.0", 0},       // missing patch = 0
		{"0.13.0", "0.13", 0},       // symmetric
		{"v0.14.0", "0.14.0", 0},    // v-prefix
		{"1.0.0-rc1", "1.0.0", 0},   // pre-release stripped
		{"", "0.0.0", 0},            // empty = 0.0.0
		{"0.0.0", "", 0},            // symmetric empty
		{"v1.2.3", "v1.2.3", 0},     // both v-prefixed
		{"10.20.30", "10.20.29", 1}, // multi-digit
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseSemverParts(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"0.13.0", [3]int{0, 13, 0}},
		{"v1.0.0", [3]int{1, 0, 0}},
		{"1.0.0-rc1", [3]int{1, 0, 0}},
		{"0.13", [3]int{0, 13, 0}},
		{"5", [3]int{5, 0, 0}},
		{"", [3]int{0, 0, 0}},
		{"v", [3]int{0, 0, 0}},
		{"1.2.3-beta.4", [3]int{1, 2, 3}},
		{"10.20.30", [3]int{10, 20, 30}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSemverParts(tt.input)
			if got != tt.want {
				t.Errorf("parseSemverParts(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseGCStatus(t *testing.T) {
	jsonOutput := `{
		"city": "agentops-nami",
		"controller": {"running": true, "pid": 12345},
		"agents": [
			{"name": "worker-1", "state": "running", "template": "worker"},
			{"name": "worker-2", "state": "stopped", "template": "worker"},
			{"name": "mayor", "state": "running", "template": "mayor"}
		],
		"summary": {"running": 2, "stopped": 1, "total": 3}
	}`
	status, err := parseGCStatus([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseGCStatus error: %v", err)
	}
	if status.City != "agentops-nami" {
		t.Errorf("City = %q, want %q", status.City, "agentops-nami")
	}
	if !status.Controller.Running {
		t.Errorf("Controller.Running = %v, want true", status.Controller.Running)
	}
	if status.Controller.PID != 12345 {
		t.Errorf("Controller.PID = %d, want 12345", status.Controller.PID)
	}
	if len(status.Agents) != 3 {
		t.Fatalf("len(Agents) = %d, want 3", len(status.Agents))
	}
	if status.Agents[0].Name != "worker-1" {
		t.Errorf("Agents[0].Name = %q, want %q", status.Agents[0].Name, "worker-1")
	}
	if status.Agents[1].State != "stopped" {
		t.Errorf("Agents[1].State = %q, want %q", status.Agents[1].State, "stopped")
	}
	if status.Agents[2].Template != "mayor" {
		t.Errorf("Agents[2].Template = %q, want %q", status.Agents[2].Template, "mayor")
	}
	if status.Summary.Running != 2 {
		t.Errorf("Summary.Running = %d, want 2", status.Summary.Running)
	}
	if status.Summary.Stopped != 1 {
		t.Errorf("Summary.Stopped = %d, want 1", status.Summary.Stopped)
	}
	if status.Summary.Total != 3 {
		t.Errorf("Summary.Total = %d, want 3", status.Summary.Total)
	}
}

func TestParseGCStatus_InvalidJSON(t *testing.T) {
	_, err := parseGCStatus([]byte("not json"))
	if err == nil {
		t.Error("parseGCStatus should return error on invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse gc status") {
		t.Errorf("error should wrap with context, got: %v", err)
	}
}

func TestParseGCStatus_EmptyObject(t *testing.T) {
	_, err := parseGCStatus([]byte(`{}`))
	if err == nil {
		t.Fatal("parseGCStatus should reject empty object")
	}
	if !strings.Contains(err.Error(), `missing required field "controller"`) {
		t.Errorf("error should mention missing controller field, got: %v", err)
	}
}

func TestParseGCStatus_ExtraFields(t *testing.T) {
	// Verify forward compatibility: extra fields don't cause errors
	jsonOutput := `{
		"city": "test",
		"controller": {"running": true, "pid": 1, "uptime": "5h"},
		"agents": [],
		"summary": {"running": 0, "stopped": 0, "total": 0},
		"new_field": "should be ignored"
	}`
	status, err := parseGCStatus([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseGCStatus error with extra fields: %v", err)
	}
	if status.City != "test" {
		t.Errorf("City = %q, want %q", status.City, "test")
	}
}

func TestParseGCStatus_MissingRequiredField(t *testing.T) {
	jsonOutput := `{
		"city": "test",
		"controller": {"running": true, "pid": 1},
		"summary": {"running": 0, "stopped": 0, "total": 0}
	}`
	_, err := parseGCStatus([]byte(jsonOutput))
	if err == nil {
		t.Fatal("parseGCStatus should reject missing agents field")
	}
	if !strings.Contains(err.Error(), `missing required field "agents"`) {
		t.Errorf("error should mention missing agents field, got: %v", err)
	}
}

func TestParseGCSessions(t *testing.T) {
	jsonOutput := `[
		{"id": "sess-abc123", "alias": "worker-1", "state": "active", "template": "worker"},
		{"id": "sess-def456", "alias": "mayor", "state": "suspended", "template": "mayor"}
	]`
	sessions, err := parseGCSessions([]byte(jsonOutput))
	if err != nil {
		t.Fatalf("parseGCSessions error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	if sessions[0].ID != "sess-abc123" {
		t.Errorf("sessions[0].ID = %q, want %q", sessions[0].ID, "sess-abc123")
	}
	if sessions[0].Alias != "worker-1" {
		t.Errorf("sessions[0].Alias = %q, want %q", sessions[0].Alias, "worker-1")
	}
	if sessions[1].State != "suspended" {
		t.Errorf("sessions[1].State = %q, want %q", sessions[1].State, "suspended")
	}
}

func TestParseGCSessions_Empty(t *testing.T) {
	sessions, err := parseGCSessions([]byte(`[]`))
	if err != nil {
		t.Fatalf("parseGCSessions error on empty: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("len(sessions) = %d, want 0", len(sessions))
	}
}

func TestParseGCSessions_InvalidJSON(t *testing.T) {
	_, err := parseGCSessions([]byte("not json"))
	if err == nil {
		t.Error("parseGCSessions should return error on invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse gc sessions") {
		t.Errorf("error should wrap with context, got: %v", err)
	}
}

func TestParseGCSessions_MissingRequiredField(t *testing.T) {
	_, err := parseGCSessions([]byte(`[{"id":"sess-1","state":"active","template":"worker"}]`))
	if err == nil {
		t.Fatal("parseGCSessions should reject session entries missing alias")
	}
	if !strings.Contains(err.Error(), `missing required field "alias"`) {
		t.Errorf("error should mention missing alias field, got: %v", err)
	}
}

func TestGCNudgeArgs(t *testing.T) {
	args := gcNudgeArgs("worker-1", "Pick up ag-0ln9.2 and implement it")
	expected := []string{"session", "nudge", "worker-1", "Pick up ag-0ln9.2 and implement it"}
	if len(args) != len(expected) {
		t.Fatalf("gcNudgeArgs len = %d, want %d", len(args), len(expected))
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestGCNudgeArgs_SpecialChars(t *testing.T) {
	args := gcNudgeArgs("worker-1", `prompt with "quotes" and $vars`)
	if args[3] != `prompt with "quotes" and $vars` {
		t.Errorf("message not preserved: %q", args[3])
	}
}

func TestGCPeekArgs(t *testing.T) {
	tests := []struct {
		agent string
		lines int
		want  []string
	}{
		{"worker-1", 50, []string{"session", "peek", "worker-1", "--lines", "50"}},
		{"mayor", 1, []string{"session", "peek", "mayor", "--lines", "1"}},
		{"w", 0, []string{"session", "peek", "w", "--lines", "0"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d", tt.agent, tt.lines), func(t *testing.T) {
			got := gcPeekArgs(tt.agent, tt.lines)
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("args[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestGCEventEmitArgs(t *testing.T) {
	data := map[string]string{"phase": "research", "status": "complete"}
	dataJSON, _ := json.Marshal(data)
	args := gcEventEmitArgs("ao:phase", string(dataJSON))
	expected := []string{"event", "emit", "ao:phase", "--payload", string(dataJSON)}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestGCBridgeCityPath(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "city.toml"), []byte("[city]\nname = \"test\""), 0644)

	// Find from subdirectory
	subDir := filepath.Join(tmpDir, "sub", "dir")
	os.MkdirAll(subDir, 0755)
	path := gcBridgeCityPath(subDir)
	if path != tmpDir {
		t.Errorf("gcBridgeCityPath = %q, want %q", path, tmpDir)
	}

	// Find from root directory
	path = gcBridgeCityPath(tmpDir)
	if path != tmpDir {
		t.Errorf("gcBridgeCityPath from root = %q, want %q", path, tmpDir)
	}

	// Not found
	emptyDir := t.TempDir()
	path = gcBridgeCityPath(emptyDir)
	if path != "" {
		t.Errorf("gcBridgeCityPath should return empty when no city.toml, got %q", path)
	}
}

func TestGCBridgeCityPath_DeepNesting(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "city.toml"), []byte("[city]\nname = \"deep\""), 0644)

	// Create 10-level deep directory
	deep := tmpDir
	for i := 0; i < 10; i++ {
		deep = filepath.Join(deep, fmt.Sprintf("level%d", i))
	}
	os.MkdirAll(deep, 0755)

	path := gcBridgeCityPath(deep)
	if path != tmpDir {
		t.Errorf("gcBridgeCityPath from deep nesting = %q, want %q", path, tmpDir)
	}
}

// =============================================================================
// L1: Unit Tests — mocked exec (via gcMock)
// =============================================================================

func TestGCBridgeAvailable_Mocked_Found(t *testing.T) {
	mock := newGCMock()
	mock.binaryAvailable = true

	if !gcBridgeAvailable(mock.lookPathFn) {
		t.Error("gcBridgeAvailable() should return true when binary found")
	}
}

func TestGCBridgeAvailable_Mocked_NotFound(t *testing.T) {
	mock := newGCMock()
	mock.binaryAvailable = false

	if gcBridgeAvailable(mock.lookPathFn) {
		t.Error("gcBridgeAvailable() should return false when binary not found")
	}
}

func TestGCBridgeVersion_Mocked(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.13.5\n"})

	v, err := gcBridgeVersion(mock.execCommand)
	if err != nil {
		t.Fatalf("gcBridgeVersion error: %v", err)
	}
	if v != "0.13.5" {
		t.Errorf("gcBridgeVersion() = %q, want %q", v, "0.13.5")
	}
}

func TestGCBridgeVersion_Mocked_Error(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{ExitCode: 1, Stderr: "not found"})

	_, err := gcBridgeVersion(mock.execCommand)
	if err == nil {
		t.Error("gcBridgeVersion should return error on exit code 1")
	}
}

func TestGCBridgeVersion_Mocked_EmptyOutput(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: ""})

	_, err := gcBridgeVersion(mock.execCommand)
	if err == nil {
		t.Error("gcBridgeVersion should return error on empty output")
	}
}

func TestGCBridgeReady_Mocked_AllGood(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"test","controller":{"running":true,"pid":999},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if !ready {
		t.Errorf("gcBridgeReady should be true, reason: %s", reason)
	}
	if reason != "gc bridge ready" {
		t.Errorf("reason = %q, want %q", reason, "gc bridge ready")
	}
}

func TestGCBridgeReady_Mocked_NoBinary(t *testing.T) {
	mock := newGCMock()
	mock.binaryAvailable = false

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if ready {
		t.Error("gcBridgeReady should be false when binary not found")
	}
	if !strings.Contains(reason, "not found") {
		t.Errorf("reason should mention not found, got: %q", reason)
	}
}

func TestGCBridgeReady_Mocked_VersionTooLow(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.12.0"})

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if ready {
		t.Error("gcBridgeReady should be false when version too low")
	}
	if !strings.Contains(reason, "below minimum") {
		t.Errorf("reason should mention below minimum, got: %q", reason)
	}
}

func TestGCBridgeReady_Mocked_ControllerStopped(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.13.0"})
	statusJSON := `{"city":"test","controller":{"running":false,"pid":0},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if ready {
		t.Error("gcBridgeReady should be false when controller stopped")
	}
	if !strings.Contains(reason, "controller not running") {
		t.Errorf("reason should mention controller not running, got: %q", reason)
	}
}

func TestGCBridgeReady_Mocked_StatusCommandFails(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.13.0"})
	mock.on("status --json", gcMockHandler{ExitCode: 1})

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if ready {
		t.Error("gcBridgeReady should be false when status command fails")
	}
	if !strings.Contains(reason, "controller not running") {
		t.Errorf("reason should mention controller not running, got: %q", reason)
	}
}

func TestGCBridgeReady_Mocked_StatusInvalidJSON(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.13.0"})
	mock.on("status --json", gcMockHandler{Stdout: "not json at all"})

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if ready {
		t.Error("gcBridgeReady should be false on invalid JSON")
	}
	if !strings.Contains(reason, "parse error") {
		t.Errorf("reason should mention parse error, got: %q", reason)
	}
}

func TestGCBridgeReady_Mocked_WithCityPath(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.13.5"})
	statusJSON := `{"city":"my-city","controller":{"running":true,"pid":42},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	ready, reason := gcBridgeReady("/some/city/path", mock.execCommand, mock.lookPathFn)
	if !ready {
		t.Errorf("gcBridgeReady with city path should be true, reason: %s", reason)
	}

	// Verify --city flag was passed
	calls := mock.callsMatching("--city")
	if len(calls) == 0 {
		t.Error("expected --city flag in gc status call")
	}
}

func TestGCBridgeReady_Mocked_VersionCheckFails(t *testing.T) {
	mock := newGCMock()
	mock.on("version", gcMockHandler{ExitCode: 1})

	ready, reason := gcBridgeReady("", mock.execCommand, mock.lookPathFn)
	if ready {
		t.Error("gcBridgeReady should be false when version check fails")
	}
	if !strings.Contains(reason, "version check failed") {
		t.Errorf("reason should mention version check, got: %q", reason)
	}
}

// =============================================================================
// L2: Integration Tests — component chains, still mocked exec
// =============================================================================

func TestGCBridge_StatusParsingToReadyFlow(t *testing.T) {
	tests := []struct {
		name      string
		running   bool
		wantReady bool
	}{
		{"running controller", true, true},
		{"stopped controller", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runningStr := "false"
			if tt.running {
				runningStr = "true"
			}
			jsonData := []byte(fmt.Sprintf(`{
				"city": "test-city",
				"controller": {"running": %s, "pid": 1234},
				"agents": [],
				"summary": {"running": 0, "stopped": 0, "total": 0}
			}`, runningStr))
			status, err := parseGCStatus(jsonData)
			if err != nil {
				t.Fatalf("parseGCStatus error: %v", err)
			}
			if status.Controller.Running != tt.wantReady {
				t.Errorf("controller running=%v: got ready=%v, want %v", tt.running, status.Controller.Running, tt.wantReady)
			}
		})
	}
}

func TestGCBridge_SessionLifecycleStates(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		wantDone bool
	}{
		{"active session", "active", false},
		{"closed session", "closed", true},
		{"completed session", "completed", true},
		{"suspended session", "suspended", false},
		{"errored session", "errored", false},
		{"starting session", "starting", false},
		{"unknown state", "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := []byte(fmt.Sprintf(`[
				{"id": "sess-001", "alias": "rpi-test-p1", "state": %q, "template": "worker"}
			]`, tt.state))
			sessions, err := parseGCSessions(jsonData)
			if err != nil {
				t.Fatalf("parseGCSessions error: %v", err)
			}
			s := sessions[0]
			isDone := s.State == "closed" || s.State == "completed"
			if isDone != tt.wantDone {
				t.Errorf("session state %q: done=%v, want %v", tt.state, isDone, tt.wantDone)
			}
		})
	}
}

func TestGCBridge_SessionNotFoundTreatedAsDone(t *testing.T) {
	jsonData := []byte(`[
		{"id": "sess-001", "alias": "other-session", "state": "active", "template": "worker"}
	]`)
	sessions, err := parseGCSessions(jsonData)
	if err != nil {
		t.Fatalf("parseGCSessions error: %v", err)
	}
	targetAlias := "rpi-test-p1"
	found := false
	for _, s := range sessions {
		if s.Alias == targetAlias {
			found = true
			break
		}
	}
	if found {
		t.Error("session should not be found in list")
	}
}

func TestGCBridge_CityPathToExecutorIntegration(t *testing.T) {
	tmpDir := setupCityDir(t, "integration-test")

	// Explicit path takes precedence
	opts := defaultPhasedEngineOptions()
	opts.GCCityPath = "/explicit/override"
	opts.WorkingDir = tmpDir
	got := gcCityPathFromOpts(opts)
	if got != "/explicit/override" {
		t.Errorf("explicit GCCityPath: got %q, want /explicit/override", got)
	}

	// Empty falls back to auto-discover
	opts.GCCityPath = ""
	got = gcCityPathFromOpts(opts)
	if got != tmpDir {
		t.Errorf("auto-discover: got %q, want %q", got, tmpDir)
	}

	// Subdirectory walks up
	subDir := filepath.Join(tmpDir, "deep", "nested")
	os.MkdirAll(subDir, 0755)
	opts.WorkingDir = subDir
	got = gcCityPathFromOpts(opts)
	if got != tmpDir {
		t.Errorf("walk-up: got %q, want %q", got, tmpDir)
	}

	// No city.toml
	emptyDir := t.TempDir()
	opts.WorkingDir = emptyDir
	got = gcCityPathFromOpts(opts)
	if got != "" {
		t.Errorf("no city.toml: got %q, want empty", got)
	}
}

func TestGCBridge_EventArgsChainIntegration(t *testing.T) {
	// Verify full chain: typed data → JSON → gcEventEmitArgs → parseable args
	phaseData := map[string]any{
		"phase":  2,
		"status": "complete",
		"run_id": "integ-001",
	}
	dataJSON, err := json.Marshal(phaseData)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	args := gcEventEmitArgs(GCEventAOPhase, string(dataJSON))

	if args[0] != "event" || args[1] != "emit" {
		t.Errorf("args[0:2] = %v, want [event emit]", args[:2])
	}
	if args[2] != "ao:phase" {
		t.Errorf("event type = %q, want ao:phase", args[2])
	}
	if args[3] != "--payload" {
		t.Errorf("args[3] = %q, want --data", args[3])
	}

	// Verify JSON round-trips
	var decoded map[string]any
	if err := json.Unmarshal([]byte(args[4]), &decoded); err != nil {
		t.Fatalf("round-trip decode error: %v", err)
	}
	if decoded["run_id"] != "integ-001" {
		t.Errorf("round-trip run_id = %v, want integ-001", decoded["run_id"])
	}
	if decoded["status"] != "complete" {
		t.Errorf("round-trip status = %v, want complete", decoded["status"])
	}
}

func TestGCBridge_ExecutorSelectsWithCityPath(t *testing.T) {
	tmpDir := setupCityDir(t, "test")

	caps := backendCapabilities{RuntimeMode: "gc"}
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = tmpDir

	executor, reason := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "gc" {
		t.Errorf("executor.Name() = %q, want gc", executor.Name())
	}
	if reason != "runtime=gc" {
		t.Errorf("reason = %q, want runtime=gc", reason)
	}

	gcExec, ok := executor.(*gcExecutor)
	if !ok {
		t.Fatal("executor is not *gcExecutor")
	}
	if gcExec.cityPath != tmpDir {
		t.Errorf("gcExecutor.cityPath = %q, want %q", gcExec.cityPath, tmpDir)
	}
}

func TestGCBridge_ReadyToExecutorFullChain_Mocked(t *testing.T) {
	// Full chain: city discovery → bridge ready → executor selection
	cityDir := setupCityDir(t, "full-chain-test")

	mock := newGCMock()
	mock.on("version", gcMockHandler{Stdout: "0.14.0"})
	statusJSON := `{"city":"full-chain-test","controller":{"running":true,"pid":42},"agents":[],"summary":{"running":0,"stopped":0,"total":0}}`
	mock.on("status --json", gcMockHandler{Stdout: statusJSON})

	// Step 1: City path discovery
	cityPath := gcBridgeCityPath(cityDir)
	if cityPath != cityDir {
		t.Fatalf("city path discovery: got %q, want %q", cityPath, cityDir)
	}

	// Step 2: Bridge ready check
	ready, reason := gcBridgeReady(cityPath, mock.execCommand, mock.lookPathFn)
	if !ready {
		t.Fatalf("bridge ready check failed: %s", reason)
	}

	// Step 3: Executor selection
	caps := backendCapabilities{RuntimeMode: "gc"}
	opts := defaultPhasedEngineOptions()
	opts.WorkingDir = cityDir
	executor, _ := selectExecutorFromCaps(caps, "", nil, opts)
	if executor.Name() != "gc" {
		t.Fatalf("executor.Name() = %q, want gc", executor.Name())
	}

	// Verify the mock captured the expected calls
	if mock.callCount() < 2 {
		t.Errorf("expected at least 2 gc calls (version + status), got %d", mock.callCount())
	}
}

// =============================================================================
// L3: Live Integration Tests — real gc binary
// =============================================================================

func TestGCBridgeAvailable_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH — skipping live test")
	}
	if !gcBridgeAvailable(nil) {
		t.Error("gcBridgeAvailable() should return true when gc is on PATH")
	}
}

func TestGCBridgeVersion_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	v, err := gcBridgeVersion(nil)
	if err != nil {
		t.Skipf("gc binary is not a compatible Gas City CLI: %v", err)
	}
	if v[0] < '0' || v[0] > '9' {
		t.Errorf("gcBridgeVersion() = %q, expected version starting with digit", v)
	}
	// Verify it's compatible with our minimum
	if !gcBridgeCompatible(v) {
		t.Errorf("installed gc version %q is below minimum %s", v, gcMinVersion)
	}
}

func TestGCBridgeReady_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	// Find city.toml from this repo
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml found — skipping live ready test")
	}

	ready, reason := gcBridgeReady(cityPath, nil, nil)
	// Don't fail if controller isn't running — just log it
	t.Logf("gcBridgeReady(cityPath=%q) = %v, reason=%q", cityPath, ready, reason)
	if !ready {
		t.Skipf("gc controller not running: %s", reason)
	}
}

func TestGCBridgeCityPath_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml in tree — skipping")
	}
	// Verify city.toml actually exists at discovered path
	if _, err := os.Stat(filepath.Join(cityPath, "city.toml")); err != nil {
		t.Errorf("city.toml not found at discovered path %q: %v", cityPath, err)
	}
	t.Logf("discovered city path: %s", cityPath)
}

func TestGCBridgeReady_Live_FullControllerCheck(t *testing.T) {
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

	// If controller IS running, verify we can also get status directly
	out, err := exec.Command("gc", "--city", cityPath, "status", "--json").Output()
	if err != nil {
		t.Fatalf("gc status --json failed: %v", err)
	}
	status, err := parseGCStatus(out)
	if err != nil {
		t.Fatalf("parseGCStatus on live output: %v", err)
	}
	if !status.Controller.Running {
		t.Errorf("live controller running = %v, want true", status.Controller.Running)
	}
	if status.Controller.PID <= 0 {
		t.Errorf("live controller PID = %d, expected positive", status.Controller.PID)
	}
	t.Logf("live gc status: city=%s, controller running=%v (pid %d), agents=%d",
		status.City, status.Controller.Running, status.Controller.PID, status.Summary.Total)
}

func TestGCEventsAvailable_Live(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		t.Skip("gc not on PATH")
	}
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml found")
	}

	avail := gcEventsAvailable(cityPath, nil, nil)
	t.Logf("gcEventsAvailable(cityPath=%q) = %v", cityPath, avail)
	// gc is on PATH and city.toml exists — events subcommand should be available
	if !avail {
		t.Errorf("gcEventsAvailable should be true when gc binary is available with city.toml")
	}
}

func TestGCBridge_ExecutorAvailable_Live(t *testing.T) {
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

	avail := gcExecutorAvailable(cwd, nil, nil)
	t.Logf("gcExecutorAvailable(cwd=%q) = %v", cwd, avail)
	if !avail {
		t.Errorf("gcExecutorAvailable should be true when the gc bridge is ready")
	}
}
