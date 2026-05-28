// practices: [distributed-systems-design, event-sourcing-cqrs]
package main

import (
	"strings"
	"testing"
)

func TestParseStreamEvents(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"init","session_id":"sess-1","model":"claude-sonnet-4-20250514","tools":["Bash","Read"]}`,
		`{"type":"assistant","subtype":"tool_use","tool_name":"Bash","message":"running ls"}`,
		`{"type":"assistant","subtype":"tool_use","tool_name":"Read","message":"reading file"}`,
		`{"type":"assistant","message":"Here is my answer"}`,
		`{"type":"result","cost_usd":0.042,"num_turns":3,"duration_ms":12500}`,
	}, "\n")

	var updates int
	progress, err := ParseStreamEvents(strings.NewReader(input), func(_ PhaseProgress) {
		updates++
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if progress.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", progress.SessionID, "sess-1")
	}
	if progress.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", progress.Model, "claude-sonnet-4-20250514")
	}
	if progress.ToolCount != 2 {
		t.Errorf("ToolCount = %d, want 2", progress.ToolCount)
	}
	if progress.LastToolCall != "Read" {
		t.Errorf("LastToolCall = %q, want %q", progress.LastToolCall, "Read")
	}
	if progress.CurrentAction != "result received" {
		t.Errorf("CurrentAction = %q, want %q", progress.CurrentAction, "result received")
	}
	if progress.CostUSD != 0.042 {
		t.Errorf("CostUSD = %f, want 0.042", progress.CostUSD)
	}
	if progress.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want 3", progress.TurnCount)
	}
	if progress.Elapsed.Milliseconds() != 12500 {
		t.Errorf("Elapsed = %v, want 12500ms", progress.Elapsed)
	}
	if progress.LastUpdate.IsZero() {
		t.Error("LastUpdate should be set")
	}
	if updates != 5 {
		t.Errorf("onUpdate called %d times, want 5", updates)
	}
}

func TestParseStreamEvents_SkipsMalformed(t *testing.T) {
	input := "not json\n{\"type\":\"init\",\"session_id\":\"s2\"}\n"

	progress, err := ParseStreamEvents(strings.NewReader(input), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.SessionID != "s2" {
		t.Errorf("SessionID = %q, want %q", progress.SessionID, "s2")
	}
	if progress.CurrentAction != "initialized" {
		t.Errorf("CurrentAction = %q, want %q", progress.CurrentAction, "initialized")
	}
}

func TestParseStreamEvents_NilCallback(t *testing.T) {
	input := `{"type":"init","session_id":"s3"}` + "\n"

	progress, err := ParseStreamEvents(strings.NewReader(input), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.SessionID != "s3" {
		t.Errorf("SessionID = %q, want %q", progress.SessionID, "s3")
	}
}

func TestStreamParseUnknownTypes(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"init","session_id":"s4","model":"claude-sonnet-4-20250514"}`,
		`{"type":"custom_event","message":"something new"}`,
		`{"type":"telemetry","payload":"data"}`,
		`{"type":"result","cost_usd":0.01,"num_turns":1,"duration_ms":500}`,
	}, "\n")

	var updates int
	progress, err := ParseStreamEvents(strings.NewReader(input), func(_ PhaseProgress) {
		updates++
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.SessionID != "s4" {
		t.Errorf("SessionID = %q, want %q", progress.SessionID, "s4")
	}
	if progress.CostUSD != 0.01 {
		t.Errorf("CostUSD = %f, want 0.01", progress.CostUSD)
	}
	// All 4 lines should trigger the callback even if the type is unknown
	if updates != 4 {
		t.Errorf("onUpdate called %d times, want 4", updates)
	}
}

func TestCumulativeProgress(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"init","session_id":"s5","model":"claude-sonnet-4-20250514"}`,
		`{"type":"assistant","subtype":"tool_use","tool_name":"Bash","message":"ls -la"}`,
		`{"type":"assistant","subtype":"tool_use","tool_name":"Read","message":"reading"}`,
		`{"type":"assistant","subtype":"tool_use","tool_name":"Grep","message":"searching"}`,
		`{"type":"assistant","message":"analysis complete"}`,
		`{"type":"result","cost_usd":0.125,"num_turns":5,"duration_ms":30000}`,
	}, "\n")

	snapshots := make([]PhaseProgress, 0, 6)
	progress, err := ParseStreamEvents(strings.NewReader(input), func(p PhaseProgress) {
		snapshots = append(snapshots, p)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tool count accumulates across events
	if progress.ToolCount != 3 {
		t.Errorf("final ToolCount = %d, want 3", progress.ToolCount)
	}
	if progress.LastToolCall != "Grep" {
		t.Errorf("LastToolCall = %q, want %q", progress.LastToolCall, "Grep")
	}
	if progress.CostUSD != 0.125 {
		t.Errorf("CostUSD = %f, want 0.125", progress.CostUSD)
	}
	if progress.TurnCount != 5 {
		t.Errorf("TurnCount = %d, want 5", progress.TurnCount)
	}
	if progress.Elapsed.Milliseconds() != 30000 {
		t.Errorf("Elapsed = %v, want 30000ms", progress.Elapsed)
	}

	// Verify progressive accumulation in snapshots
	if len(snapshots) != 6 {
		t.Fatalf("snapshot count = %d, want 6", len(snapshots))
	}
	// After init: 0 tools
	if snapshots[0].ToolCount != 0 {
		t.Errorf("snapshot[0].ToolCount = %d, want 0", snapshots[0].ToolCount)
	}
	// After first tool_use: 1 tool
	if snapshots[1].ToolCount != 1 {
		t.Errorf("snapshot[1].ToolCount = %d, want 1", snapshots[1].ToolCount)
	}
	// After second tool_use: 2 tools
	if snapshots[2].ToolCount != 2 {
		t.Errorf("snapshot[2].ToolCount = %d, want 2", snapshots[2].ToolCount)
	}
	// After third tool_use: 3 tools
	if snapshots[3].ToolCount != 3 {
		t.Errorf("snapshot[3].ToolCount = %d, want 3", snapshots[3].ToolCount)
	}
	// Plain assistant message doesn't increment tool count
	if snapshots[4].ToolCount != 3 {
		t.Errorf("snapshot[4].ToolCount = %d, want 3 (no tool in plain assistant)", snapshots[4].ToolCount)
	}
}

func TestParseStreamEvents_LargeLine(t *testing.T) {
	// Use a payload under the 1MB max line length limit.
	largeMsg := strings.Repeat("x", 512*1024) // 512KB payload (under 1MB limit)
	input := strings.Join([]string{
		`{"type":"init","session_id":"s-large","model":"claude-sonnet-4-20250514"}`,
		`{"type":"assistant","message":"` + largeMsg + `"}`,
		`{"type":"result","cost_usd":0.001,"num_turns":1,"duration_ms":1000}`,
	}, "\n")

	progress, err := ParseStreamEvents(strings.NewReader(input), nil)
	if err != nil {
		t.Fatalf("unexpected error for large stream line: %v", err)
	}
	if progress.SessionID != "s-large" {
		t.Errorf("SessionID = %q, want %q", progress.SessionID, "s-large")
	}
	if progress.CurrentAction != "result received" {
		t.Errorf("CurrentAction = %q, want %q", progress.CurrentAction, "result received")
	}
}

func TestStreamLineReader_MaxLineLength(t *testing.T) {
	// Feed >1MB without a newline — should return an error containing "max length"
	huge := strings.Repeat("A", (1<<20)+1) // 1MB + 1 byte, no newline
	_, err := ParseStreamEvents(strings.NewReader(huge), nil)
	if err == nil {
		t.Fatal("expected error for line exceeding max length, got nil")
	}
	if !strings.Contains(err.Error(), "max length") {
		t.Errorf("error = %q, want it to contain 'max length'", err.Error())
	}
}

func TestParseStreamEvents_ErrorResult(t *testing.T) {
	t.Run("result with is_error=true sets error fields", func(t *testing.T) {
		input := strings.Join([]string{
			`{"type":"init","session_id":"s-err"}`,
			`{"type":"result","is_error":true,"message":"something went wrong","cost_usd":0.01,"num_turns":1}`,
		}, "\n")

		progress, err := ParseStreamEvents(strings.NewReader(input), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if progress.CurrentAction != "result error" {
			t.Errorf("CurrentAction = %q, want %q", progress.CurrentAction, "result error")
		}
		if progress.LastError == "" {
			t.Error("expected LastError to be set for error result")
		}
	})

	t.Run("result with is_error=true and no message uses default error", func(t *testing.T) {
		input := strings.Join([]string{
			`{"type":"init","session_id":"s-err2"}`,
			`{"type":"result","is_error":true,"cost_usd":0.0,"num_turns":0}`,
		}, "\n")

		progress, err := ParseStreamEvents(strings.NewReader(input), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if progress.LastError != "result event reported error" {
			t.Errorf("LastError = %q, want %q", progress.LastError, "result event reported error")
		}
	})
}
