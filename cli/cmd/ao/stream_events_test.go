// practices: [distributed-systems-design, event-sourcing-cqrs]
package main

import (
	"encoding/json"
	"testing"
)

func TestStreamEvents_ParseStreamEvent_Init(t *testing.T) {
	data := `{"type":"init","session_id":"sess-abc","model":"claude-sonnet-4-20250514","tools":["Bash","Read","Grep"]}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.Type != EventTypeInit {
		t.Errorf("Type = %q, want %q", ev.Type, EventTypeInit)
	}
	if ev.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", ev.SessionID, "sess-abc")
	}
	if ev.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", ev.Model, "claude-sonnet-4-20250514")
	}
	if len(ev.Tools) != 3 {
		t.Errorf("Tools count = %d, want 3", len(ev.Tools))
	}
}

func TestStreamEvents_ParseStreamEvent_Assistant(t *testing.T) {
	data := `{"type":"assistant","subtype":"tool_use","tool_name":"Bash","message":"ls -la"}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.Type != EventTypeAssistant {
		t.Errorf("Type = %q, want %q", ev.Type, EventTypeAssistant)
	}
	if ev.Subtype != "tool_use" {
		t.Errorf("Subtype = %q, want %q", ev.Subtype, "tool_use")
	}
	if ev.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", ev.ToolName, "Bash")
	}
}

func TestStreamEvents_ParseStreamEvent_Result(t *testing.T) {
	data := `{"type":"result","cost_usd":0.042,"duration_ms":12500,"duration_api_ms":10000,"is_error":false,"num_turns":3}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.Type != EventTypeResult {
		t.Errorf("Type = %q, want %q", ev.Type, EventTypeResult)
	}
	if ev.CostUSD != 0.042 {
		t.Errorf("CostUSD = %f, want 0.042", ev.CostUSD)
	}
	if ev.DurationMS != 12500 {
		t.Errorf("DurationMS = %f, want 12500", ev.DurationMS)
	}
	if ev.DurationAPIMS != 10000 {
		t.Errorf("DurationAPIMS = %f, want 10000", ev.DurationAPIMS)
	}
	if ev.IsError {
		t.Error("IsError should be false")
	}
	if ev.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", ev.NumTurns)
	}
}

func TestStreamEvents_ParseStreamEvent_ErrorResult(t *testing.T) {
	data := `{"type":"result","is_error":true,"message":"something went wrong"}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if !ev.IsError {
		t.Error("IsError should be true")
	}
	if ev.Message != "something went wrong" {
		t.Errorf("Message = %q, want %q", ev.Message, "something went wrong")
	}
}

func TestStreamEvents_ParseStreamEvent_AutoClassifiesError(t *testing.T) {
	// Finding #3: ParseStreamEvent should auto-populate ErrorClass
	data := `{"type":"result","is_error":true,"message":"request timed out after 120s"}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.ErrorClass != StreamErrorClassTimeout {
		t.Errorf("ErrorClass = %q, want %q (auto-classified)", ev.ErrorClass, StreamErrorClassTimeout)
	}
}

func TestStreamEvents_ParseStreamEvent_PreservesExplicitErrorClass(t *testing.T) {
	// When ErrorClass is already set in JSON to a valid enum, preserve it.
	data := `{"type":"result","is_error":true,"message":"timed out","error_class":"auth_failure"}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.ErrorClass != StreamErrorClassAuthFailure {
		t.Errorf("ErrorClass = %q, want %q (preserved from JSON)", ev.ErrorClass, StreamErrorClassAuthFailure)
	}
}

func TestStreamEvents_ParseStreamEvent_InvalidWireEnum(t *testing.T) {
	// Invalid error_class values must be reclassified, not trusted.
	data := `{"type":"result","is_error":true,"message":"timed out","error_class":"bogus"}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.ErrorClass != StreamErrorClassTimeout {
		t.Errorf("ErrorClass = %q, want %q (reclassified from invalid wire value)", ev.ErrorClass, StreamErrorClassTimeout)
	}
}

func TestStreamEvents_ParseStreamEvent_ClearsErrorClassWhenNotError(t *testing.T) {
	// is_error=false with error_class set → must clear to none.
	data := `{"type":"result","is_error":false,"error_class":"timeout"}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.ErrorClass != StreamErrorClassNone {
		t.Errorf("ErrorClass = %q, want %q (cleared for non-error)", ev.ErrorClass, StreamErrorClassNone)
	}
}

func TestStreamEvents_ParseStreamEvent_ToolInput(t *testing.T) {
	data := `{"type":"assistant","subtype":"tool_use","tool_name":"Read","tool_use_id":"tu-1","tool_input":{"path":"/tmp/test.go"}}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.ToolUseID != "tu-1" {
		t.Errorf("ToolUseID = %q, want %q", ev.ToolUseID, "tu-1")
	}
	if ev.ToolInput == nil {
		t.Fatal("ToolInput should not be nil")
	}
	var input map[string]string
	if err := json.Unmarshal(ev.ToolInput, &input); err != nil {
		t.Fatalf("unmarshal ToolInput: %v", err)
	}
	if input["path"] != "/tmp/test.go" {
		t.Errorf("ToolInput.path = %q, want %q", input["path"], "/tmp/test.go")
	}
}

func TestStreamEvents_ParseStreamEvent_InvalidJSON(t *testing.T) {
	_, err := ParseStreamEvent([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestStreamEvents_ParseStreamEvent_EmptyObject(t *testing.T) {
	ev, err := ParseStreamEvent([]byte("{}"))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.Type != "" {
		t.Errorf("Type = %q, want empty", ev.Type)
	}
}

func TestStreamEvents_ParseStreamEvent_UnknownFields(t *testing.T) {
	// Permissive parsing: unknown fields should be silently ignored.
	data := `{"type":"system","message":"hello","unknown_field":"value","another":42}`
	ev, err := ParseStreamEvent([]byte(data))
	if err != nil {
		t.Fatalf("ParseStreamEvent error: %v", err)
	}
	if ev.Type != EventTypeSystem {
		t.Errorf("Type = %q, want %q", ev.Type, EventTypeSystem)
	}
	if ev.Message != "hello" {
		t.Errorf("Message = %q, want %q", ev.Message, "hello")
	}
}

func TestStreamEvents_EventTypeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "system", value: EventTypeSystem, want: "system"},
		{name: "assistant", value: EventTypeAssistant, want: "assistant"},
		{name: "user", value: EventTypeUser, want: "user"},
		{name: "result", value: EventTypeResult, want: "result"},
		{name: "init", value: EventTypeInit, want: "init"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.value, tc.want)
			}
		})
	}
}

func TestStreamEvents_ParseStreamEvent_RoundTrip(t *testing.T) {
	original := StreamEvent{
		Type:      EventTypeResult,
		CostUSD:   0.05,
		NumTurns:  7,
		IsError:   false,
		Message:   "done",
		SessionID: "sess-rt",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("ParseStreamEvent: %v", err)
	}
	if parsed.Type != original.Type {
		t.Errorf("Type = %q, want %q", parsed.Type, original.Type)
	}
	if parsed.CostUSD != original.CostUSD {
		t.Errorf("CostUSD = %f, want %f", parsed.CostUSD, original.CostUSD)
	}
	if parsed.NumTurns != original.NumTurns {
		t.Errorf("NumTurns = %d, want %d", parsed.NumTurns, original.NumTurns)
	}
	if parsed.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", parsed.SessionID, original.SessionID)
	}
}

// --- ClassifyStreamError tests ---

func TestClassifyStreamError_Timeout(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "request timed out after 120s"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassTimeout {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassTimeout)
	}
}

func TestClassifyStreamError_RateLimit(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "rate limit exceeded, please retry"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassRateLimit {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassRateLimit)
	}
}

func TestClassifyStreamError_RateLimit_TooManyRequests(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "too many requests, slow down"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassRateLimit {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassRateLimit)
	}
}

func TestClassifyStreamError_RateLimit_HTTP429_WithContext(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "HTTP error code: 429 rate limited"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassRateLimit {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassRateLimit)
	}
}

func TestClassifyStreamError_NoFalsePositive_429InPort(t *testing.T) {
	// Finding #1: bare "429" in a port number must NOT match rate_limit.
	ev := StreamEvent{IsError: true, Message: "connection failed to localhost:4290"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("port number 4290 should classify as execution_error, got %q", got)
	}
}

func TestClassifyStreamError_AuthFailure(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "unauthorized: invalid API key"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassAuthFailure {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassAuthFailure)
	}
}

func TestClassifyStreamError_AuthFailure_Forbidden(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "forbidden: access denied to resource"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassAuthFailure {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassAuthFailure)
	}
}

func TestClassifyStreamError_AuthFailure_HTTP401_WithContext(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "status: 401 unauthorized"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassAuthFailure {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassAuthFailure)
	}
}

func TestClassifyStreamError_NoFalsePositive_401InLineNumber(t *testing.T) {
	// Finding #1: bare "401" in a line reference must NOT match auth_failure.
	ev := StreamEvent{IsError: true, Message: "error at line 401: syntax error"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("line number 401 should classify as execution_error, got %q", got)
	}
}

func TestClassifyStreamError_ContextOverflow(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "context limit exceeded: input too long"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassContextOverflow {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassContextOverflow)
	}
}

func TestClassifyStreamError_SandboxViolation(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "permission denied: operation not allowed in sandbox"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassSandboxViolation {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassSandboxViolation)
	}
}

func TestClassifyStreamError_SandboxViolation_PermissionDenied(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "permission denied: /usr/bin/rm"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassSandboxViolation {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassSandboxViolation)
	}
}

func TestClassifyStreamError_NoFalsePositive_NotAllowed_Benign(t *testing.T) {
	// Finding #1: "not allowed" without sandbox/permission context is NOT sandbox_violation.
	ev := StreamEvent{IsError: true, Message: "value not allowed for this field"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("benign 'not allowed' should classify as execution_error, got %q", got)
	}
}

func TestClassifyStreamError_NoFalsePositive_SandboxCrash(t *testing.T) {
	// Bare "sandbox" without permission/violation context must NOT be sandbox_violation.
	ev := StreamEvent{IsError: true, Message: "sandbox startup failed: resource unavailable"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("sandbox crash should classify as execution_error, got %q", got)
	}
}

func TestClassifyStreamError_ExecutionError_NetworkError(t *testing.T) {
	// Finding #2: network errors should classify as execution_error, not unknown.
	tests := []struct {
		name string
		msg  string
	}{
		{"ENOENT", "ENOENT: no such file or directory"},
		{"dial_tcp", "dial tcp 127.0.0.1:8080: connection refused"},
		{"ECONNREFUSED", "ECONNREFUSED: connection refused"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := StreamEvent{IsError: true, Message: tc.msg}
			got := ClassifyStreamError(ev)
			if got != StreamErrorClassExecutionError {
				t.Errorf("ClassifyStreamError(%q) = %q, want %q", tc.msg, got, StreamErrorClassExecutionError)
			}
		})
	}
}

func TestClassifyStreamError_ExecutionError_ProcessError(t *testing.T) {
	// Finding #2: process errors should classify as execution_error.
	ev := StreamEvent{IsError: true, Message: "fork failed: resource temporarily unavailable"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassExecutionError)
	}
}

func TestClassifyStreamError_ExecutionError_ModelError(t *testing.T) {
	// Finding #2: model errors should classify as execution_error.
	ev := StreamEvent{IsError: true, Message: "model not found: claude-nonexistent"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassExecutionError)
	}
}

func TestClassifyStreamError_ExecutionError_Fallback(t *testing.T) {
	// Finding #4: unrecognized non-empty messages → execution_error (not unknown).
	ev := StreamEvent{IsError: true, Message: "something went completely wrong"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassExecutionError {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassExecutionError)
	}
}

func TestClassifyStreamError_Unknown_EmptyMessage(t *testing.T) {
	// Finding #4: only truly empty/whitespace messages → unknown.
	ev := StreamEvent{IsError: true, Message: ""}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassUnknown {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassUnknown)
	}
}

func TestClassifyStreamError_Unknown_WhitespaceMessage(t *testing.T) {
	ev := StreamEvent{IsError: true, Message: "   "}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassUnknown {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassUnknown)
	}
}

func TestClassifyStreamError_QuotaExceeded(t *testing.T) {
	// Finding #2: quota/billing errors should be classified.
	ev := StreamEvent{IsError: true, Message: "quota exceeded for this billing period"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassRateLimit {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassRateLimit)
	}
}

func TestClassifyStreamError_NotError(t *testing.T) {
	ev := StreamEvent{IsError: false, Message: "all good"}
	got := ClassifyStreamError(ev)
	if got != StreamErrorClassNone {
		t.Errorf("ClassifyStreamError = %q, want %q", got, StreamErrorClassNone)
	}
}

func TestStreamEvent_ErrorClassField(t *testing.T) {
	original := StreamEvent{
		Type:       EventTypeResult,
		IsError:    true,
		ErrorClass: StreamErrorClassTimeout,
		Message:    "timed out",
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("ParseStreamEvent: %v", err)
	}
	if parsed.ErrorClass != StreamErrorClassTimeout {
		t.Errorf("ErrorClass = %q, want %q", parsed.ErrorClass, StreamErrorClassTimeout)
	}
}
