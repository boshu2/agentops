// practices: [microservices, sre]
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// =============================================================================
// L1: Unit Tests — constants and arg formatting
// =============================================================================

func TestGCEventConstants(t *testing.T) {
	expected := map[string]string{
		"GCEventAOPhase":   "ao:phase",
		"GCEventAOGate":    "ao:gate",
		"GCEventAOFailure": "ao:failure",
		"GCEventAOMetric":  "ao:metric",
	}
	actuals := map[string]string{
		"GCEventAOPhase":   GCEventAOPhase,
		"GCEventAOGate":    GCEventAOGate,
		"GCEventAOFailure": GCEventAOFailure,
		"GCEventAOMetric":  GCEventAOMetric,
	}
	for name, want := range expected {
		got := actuals[name]
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestGCEmitPhaseEvent_ArgsFormat(t *testing.T) {
	data := map[string]any{
		"phase":  1,
		"status": "complete",
		"run_id": "test-123",
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	args := gcEventEmitArgs(GCEventAOPhase, string(dataJSON))
	if args[0] != "event" || args[1] != "emit" || args[2] != "ao:phase" {
		t.Errorf("unexpected args prefix: %v", args[:3])
	}
	if args[3] != "--payload" {
		t.Errorf("args[3] = %q, want --data", args[3])
	}
	// Verify payload is valid JSON
	var decoded map[string]any
	if err := json.Unmarshal([]byte(args[4]), &decoded); err != nil {
		t.Errorf("payload is not valid JSON: %v", err)
	}
}

func TestGCEmitGateEvent_ArgsFormat(t *testing.T) {
	data := map[string]any{
		"gate":    "pre-mortem",
		"verdict": "PASS",
		"run_id":  "test-456",
	}
	dataJSON, _ := json.Marshal(data)
	args := gcEventEmitArgs(GCEventAOGate, string(dataJSON))
	if args[2] != "ao:gate" {
		t.Errorf("event type = %q, want %q", args[2], "ao:gate")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(args[4]), &decoded); err != nil {
		t.Errorf("payload not valid JSON: %v", err)
	}
	if decoded["gate"] != "pre-mortem" {
		t.Errorf("gate = %v, want pre-mortem", decoded["gate"])
	}
}

func TestGCEmitFailureEvent_ArgsFormat(t *testing.T) {
	data := map[string]any{
		"error":  "phase timeout",
		"phase":  2,
		"run_id": "test-789",
	}
	dataJSON, _ := json.Marshal(data)
	args := gcEventEmitArgs(GCEventAOFailure, string(dataJSON))
	if args[2] != "ao:failure" {
		t.Errorf("event type = %q, want %q", args[2], "ao:failure")
	}
}

func TestGCEmitMetricEvent_ArgsFormat(t *testing.T) {
	data := map[string]any{
		"metric": "phase_duration_s",
		"value":  42.5,
		"run_id": "test-metric",
	}
	dataJSON, _ := json.Marshal(data)
	args := gcEventEmitArgs(GCEventAOMetric, string(dataJSON))
	if args[2] != "ao:metric" {
		t.Errorf("event type = %q, want %q", args[2], "ao:metric")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(args[4]), &decoded); err != nil {
		t.Errorf("payload not valid JSON: %v", err)
	}
	if decoded["value"] != 42.5 {
		t.Errorf("value = %v, want 42.5", decoded["value"])
	}
}

// =============================================================================
// L1: Mocked exec tests — gcEmitEvent and gcEventsAvailable
// =============================================================================

func TestGCEmitEvent_NoBinary_SilentNoop(t *testing.T) {
	mock := newGCMock()
	mock.binaryAvailable = false
	mock.install(t)

	err := gcEmitEvent("", "ao:test", map[string]any{"test": true}, mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitEvent should return nil when gc not available, got: %v", err)
	}
	if mock.callCount() != 0 {
		t.Errorf("expected no gc calls when binary unavailable, got %d", mock.callCount())
	}
}

func TestGCEmitEvent_Mocked_Success(t *testing.T) {
	mock := newGCMock()
	mock.on("event emit ao:phase --payload", gcMockHandler{Stdout: ""})
	mock.install(t)

	err := gcEmitEvent("", GCEventAOPhase, map[string]any{
		"phase":  1,
		"status": "started",
	}, mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitEvent should succeed, got: %v", err)
	}
	calls := mock.callsMatching("event emit")
	if len(calls) != 1 {
		t.Errorf("expected 1 event emit call, got %d", len(calls))
	}
}

func TestGCEmitEvent_Mocked_WithCityPath(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcEmitEvent("/my/city", GCEventAOGate, map[string]any{
		"gate":    "vibe",
		"verdict": "PASS",
	}, mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitEvent with city path error: %v", err)
	}
	calls := mock.callsMatching("--city")
	if len(calls) == 0 {
		t.Error("expected --city flag in event emit call")
	}
}

func TestGCEmitEvent_Mocked_CommandFails(t *testing.T) {
	mock := newGCMock()
	mock.on("event emit ao:failure", gcMockHandler{ExitCode: 1, Stderr: "event bus error"})
	mock.install(t)

	err := gcEmitEvent("", GCEventAOFailure, map[string]any{"error": "test"}, mock.execCommand, mock.lookPathFn)
	if err == nil {
		t.Fatal("gcEmitEvent should return error when command fails")
	}
	if !strings.Contains(err.Error(), "gc event emit") {
		t.Errorf("error should mention gc event emit, got: %v", err)
	}
}

func TestGCEventsAvailable_NoBinary(t *testing.T) {
	mock := newGCMock()
	mock.binaryAvailable = false
	mock.install(t)

	if gcEventsAvailable("", mock.execCommand, mock.lookPathFn) {
		t.Error("gcEventsAvailable should return false when gc not available")
	}
}

func TestGCEventsAvailable_Mocked_Success(t *testing.T) {
	mock := newGCMock()
	mock.on("events --help", gcMockHandler{ExitCode: 0})
	mock.install(t)

	if !gcEventsAvailable("", mock.execCommand, mock.lookPathFn) {
		t.Error("gcEventsAvailable should return true when events --help succeeds")
	}
}

func TestGCEventsAvailable_Mocked_WithCityPath(t *testing.T) {
	mock := newGCMock()
	mock.on("events --help", gcMockHandler{ExitCode: 0})
	mock.install(t)

	if !gcEventsAvailable("/my/city", mock.execCommand, mock.lookPathFn) {
		t.Error("gcEventsAvailable with city path should return true")
	}
	calls := mock.callsMatching("--city")
	if len(calls) == 0 {
		t.Error("expected --city flag in events --help call")
	}
}

func TestGCEventsAvailable_Mocked_CommandFails(t *testing.T) {
	mock := newGCMock()
	mock.on("events --help", gcMockHandler{ExitCode: 1})
	mock.install(t)

	if gcEventsAvailable("", mock.execCommand, mock.lookPathFn) {
		t.Error("gcEventsAvailable should return false when events --help fails")
	}
}

// =============================================================================
// L2: Integration Tests — event emitters through gcEmitEvent
// =============================================================================

func TestGCEmitPhaseEvent_Mocked_FullChain(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcEmitPhaseEvent("/city", 3, "complete", "run-abc", mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitPhaseEvent error: %v", err)
	}
	calls := mock.callsMatching("event emit")
	if len(calls) != 1 {
		t.Fatalf("expected 1 event emit call, got %d", len(calls))
	}
	full := strings.Join(calls[0].Args, " ")
	if !strings.Contains(full, "ao:phase") {
		t.Errorf("call should contain ao:phase, got: %s", full)
	}
}

func TestGCEmitGateEvent_Mocked_FullChain(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcEmitGateEvent("/city", "pre-mortem", "PASS", "run-def", mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitGateEvent error: %v", err)
	}
	calls := mock.callsMatching("ao:gate")
	if len(calls) != 1 {
		t.Errorf("expected 1 ao:gate call, got %d", len(calls))
	}
}

func TestGCEmitFailureEvent_Mocked_FullChain(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcEmitFailureEvent("/city", "timeout", "run-ghi", 2, mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitFailureEvent error: %v", err)
	}
	calls := mock.callsMatching("ao:failure")
	if len(calls) != 1 {
		t.Errorf("expected 1 ao:failure call, got %d", len(calls))
	}
}

func TestGCEmitMetricEvent_Mocked_FullChain(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	err := gcEmitMetricEvent("/city", "duration_s", 123.45, "run-jkl", mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Errorf("gcEmitMetricEvent error: %v", err)
	}
	calls := mock.callsMatching("ao:metric")
	if len(calls) != 1 {
		t.Errorf("expected 1 ao:metric call, got %d", len(calls))
	}
}

func TestGCEmitEvent_Mocked_PayloadContainsTimestamp(t *testing.T) {
	mock := newGCMock()
	mock.install(t)

	// gcEmitPhaseEvent adds a timestamp — verify the JSON payload
	// is correctly structured through the chain
	err := gcEmitPhaseEvent("", 1, "started", "ts-test", mock.execCommand, mock.lookPathFn)
	if err != nil {
		t.Fatalf("gcEmitPhaseEvent error: %v", err)
	}
	calls := mock.callsMatching("event emit")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	// Find the --data arg and verify it contains timestamp
	full := strings.Join(calls[0].Args, " ")
	if !strings.Contains(full, "timestamp") {
		t.Errorf("event payload should contain timestamp field, got: %s", full)
	}
}

// =============================================================================
// L3: Live Integration Tests — real gc binary
// =============================================================================

func TestGCEmitEvent_Live_SilentDegradation(t *testing.T) {
	if _, err := exec.LookPath("gc"); err != nil {
		// gc not installed: emitEvent should silently no-op
		err := gcEmitEvent("", "ao:test", map[string]any{"test": true}, nil, nil)
		if err != nil {
			t.Errorf("gcEmitEvent should silently no-op when gc unavailable, got: %v", err)
		}
		return
	}
	// gc IS installed — emit should either succeed or fail gracefully
	cwd, _ := os.Getwd()
	cityPath := gcBridgeCityPath(cwd)
	if cityPath == "" {
		t.Skip("no city.toml found")
	}

	ready, reason := gcBridgeReady(cityPath, nil, nil)
	if !ready {
		t.Skipf("gc controller not running: %s", reason)
	}

	// Controller running — emit a real event
	err := gcEmitPhaseEvent(cityPath, 0, "test", "live-test-run", nil, nil)
	if err != nil {
		t.Errorf("gcEmitPhaseEvent with live controller: %v", err)
	}
}

func TestGCEmitAllEventTypes_Live(t *testing.T) {
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

	runID := "live-all-events"

	if err := gcEmitPhaseEvent(cityPath, 1, "test-started", runID, nil, nil); err != nil {
		t.Errorf("phase event: %v", err)
	}
	if err := gcEmitGateEvent(cityPath, "test-gate", "PASS", runID, nil, nil); err != nil {
		t.Errorf("gate event: %v", err)
	}
	if err := gcEmitFailureEvent(cityPath, "test-error", runID, 1, nil, nil); err != nil {
		t.Errorf("failure event: %v", err)
	}
	if err := gcEmitMetricEvent(cityPath, "test_metric", 99.9, runID, nil, nil); err != nil {
		t.Errorf("metric event: %v", err)
	}
}
