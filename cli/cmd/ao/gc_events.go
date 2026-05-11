// practices: [microservices, sre]
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// gcEventType constants for ao events in the gc event bus.
const (
	GCEventAOPhase   = "ao:phase"
	GCEventAOGate    = "ao:gate"
	GCEventAOFailure = "ao:failure"
	GCEventAOMetric  = "ao:metric"
)

// gcEmitPhaseEvent emits an ao:phase event to the gc event bus.
func gcEmitPhaseEvent(cityPath string, phase int, status, runID string, execCommand gcExecFn, lookPath gcLookFn) error {
	data := map[string]any{
		"phase":     phase,
		"status":    status,
		"run_id":    runID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return gcEmitEvent(cityPath, GCEventAOPhase, data, execCommand, lookPath)
}

// gcEmitGateEvent emits an ao:gate event (pre-mortem, vibe, etc.) to the gc event bus.
func gcEmitGateEvent(cityPath string, gate, verdict, runID string, execCommand gcExecFn, lookPath gcLookFn) error {
	data := map[string]any{
		"gate":      gate,
		"verdict":   verdict,
		"run_id":    runID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return gcEmitEvent(cityPath, GCEventAOGate, data, execCommand, lookPath)
}

// gcEmitFailureEvent emits an ao:failure event to the gc event bus.
func gcEmitFailureEvent(cityPath string, errMsg, runID string, phase int, execCommand gcExecFn, lookPath gcLookFn) error {
	data := map[string]any{
		"error":     errMsg,
		"phase":     phase,
		"run_id":    runID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return gcEmitEvent(cityPath, GCEventAOFailure, data, execCommand, lookPath)
}

// gcEmitMetricEvent emits an ao:metric event to the gc event bus.
func gcEmitMetricEvent(cityPath string, metric string, value float64, runID string, execCommand gcExecFn, lookPath gcLookFn) error {
	data := map[string]any{
		"metric":    metric,
		"value":     value,
		"run_id":    runID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return gcEmitEvent(cityPath, GCEventAOMetric, data, execCommand, lookPath)
}

// gcEmitEvent is the low-level event emitter to the gc event bus.
func gcEmitEvent(cityPath, eventType string, data map[string]any, execCommand gcExecFn, lookPath gcLookFn) error {
	if !gcBridgeAvailable(lookPath) {
		return nil // silently skip if gc not available
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal event data: %w", err)
	}
	args := gcEventEmitArgs(eventType, string(dataJSON))
	if cityPath != "" {
		args = append([]string{"--city", cityPath}, args...)
	}
	cmd := gcDefaultExec(execCommand)("gc", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gc event emit %s: %w (output: %s)", eventType, err, string(out))
	}
	return nil
}

// gcEventsAvailable returns true if gc event system is accessible.
func gcEventsAvailable(cityPath string, execCommand gcExecFn, lookPath gcLookFn) bool {
	if !gcBridgeAvailable(lookPath) {
		return false
	}
	args := []string{"events", "--help"}
	if cityPath != "" {
		args = append([]string{"--city", cityPath}, args...)
	}
	return gcDefaultExec(execCommand)("gc", args...).Run() == nil
}
