// practices: [distributed-systems-design, event-sourcing-cqrs]
package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Event type constants for Claude Code streaming JSON output.
const (
	EventTypeSystem    = "system"
	EventTypeAssistant = "assistant"
	EventTypeUser      = "user"
	EventTypeResult    = "result"
	EventTypeInit      = "init"
)

// StreamErrorClass categorizes stream-level errors for downstream handling.
// When a Claude Code stream reports an error (IsError=true), this enum
// classifies the failure to enable appropriate retry/escalation behavior.
type StreamErrorClass string

const (
	StreamErrorClassNone             StreamErrorClass = ""
	StreamErrorClassTimeout          StreamErrorClass = "timeout"
	StreamErrorClassRateLimit        StreamErrorClass = "rate_limit"
	StreamErrorClassAuthFailure      StreamErrorClass = "auth_failure"
	StreamErrorClassContextOverflow  StreamErrorClass = "context_overflow"
	StreamErrorClassSandboxViolation StreamErrorClass = "sandbox_violation"
	StreamErrorClassExecutionError   StreamErrorClass = "execution_error"
	StreamErrorClassUnknown          StreamErrorClass = "unknown"
)

// Precompiled patterns for error classification.
// Using word boundaries and contextual patterns to avoid false positives
// on port numbers, line numbers, or benign validation messages.
var (
	// HTTP status codes require surrounding HTTP/error context.
	reHTTP429 = regexp.MustCompile(`(?i)\b(status|http|error|code)\s*:?\s*429\b`)
	reHTTP401 = regexp.MustCompile(`(?i)\b(status|http|error|code)\s*:?\s*401\b`)
	reHTTP403 = regexp.MustCompile(`(?i)\b(status|http|error|code)\s*:?\s*403\b`)

	// Broader error families for execution_error fallback.
	reNetworkError = regexp.MustCompile(`(?i)\b(enoent|econnrefused|econnreset|dial tcp|dns resolve|connection refused|network unreachable)\b`)
	reProcessError = regexp.MustCompile(`(?i)\b(fork failed|exec format error|signal:\s*killed|oom killed|out of memory)\b`)
	reModelError   = regexp.MustCompile(`(?i)\b(model not found|model.+unavailable|invalid model|unsupported model)\b`)
	reQuotaError   = regexp.MustCompile(`(?i)\b(quota exceeded|billing|insufficient.+credits?|payment required)\b`)
)

// StreamEvent is the top-level envelope for every JSON line emitted by
// Claude Code's streaming output (--output-format stream-json).
// The Type field determines which payload fields are populated.
type StreamEvent struct {
	// Type is one of the EventType* constants.
	Type string `json:"type"`

	// Subtype provides further classification within a type
	// (e.g. "tool_use", "tool_result").
	Subtype string `json:"subtype,omitempty"`

	// SessionID is the unique session identifier (present in init events).
	SessionID string `json:"session_id,omitempty"`

	// Tools lists available tool names (present in init events).
	Tools []string `json:"tools,omitempty"`

	// Model is the model identifier (present in init events).
	Model string `json:"model,omitempty"`

	// Message holds the text content for system, assistant, user, and
	// result events.
	Message string `json:"message,omitempty"`

	// ToolName is the tool being invoked (assistant tool_use subtype).
	ToolName string `json:"tool_name,omitempty"`

	// ToolInput holds the raw JSON input for a tool call.
	ToolInput json.RawMessage `json:"tool_input,omitempty"`

	// ToolUseID links a tool_result back to its tool_use request.
	ToolUseID string `json:"tool_use_id,omitempty"`

	// CostUSD is the cumulative cost reported in result events.
	CostUSD float64 `json:"cost_usd,omitempty"`

	// DurationMS is the total duration reported in result events.
	DurationMS float64 `json:"duration_ms,omitempty"`

	// DurationAPIMS is the API-side duration reported in result events.
	DurationAPIMS float64 `json:"duration_api_ms,omitempty"`

	// IsError indicates whether a result event represents an error.
	IsError bool `json:"is_error,omitempty"`

	// ErrorClass provides structured classification of the error when
	// IsError is true. Auto-populated by ParseStreamEvent via
	// ClassifyStreamError when not already set.
	ErrorClass StreamErrorClass `json:"error_class,omitempty"`

	// NumTurns is the number of conversation turns in a result event.
	NumTurns int `json:"num_turns,omitempty"`
}

// ParseStreamEvent unmarshals a single JSON line into a StreamEvent.
// Unknown fields are silently ignored (permissive parsing).
// When IsError is true and ErrorClass is empty, it auto-fills ErrorClass
// via ClassifyStreamError.
// validErrorClasses is the allowlist of recognized StreamErrorClass values.
var validErrorClasses = map[StreamErrorClass]bool{
	StreamErrorClassNone:             true,
	StreamErrorClassTimeout:          true,
	StreamErrorClassRateLimit:        true,
	StreamErrorClassAuthFailure:      true,
	StreamErrorClassContextOverflow:  true,
	StreamErrorClassSandboxViolation: true,
	StreamErrorClassExecutionError:   true,
	StreamErrorClassUnknown:          true,
}

// classifierRule defines a single error classification pattern.
type classifierRule struct {
	class      StreamErrorClass
	substrings []string          // any match on lowered msg → hit
	regexes    []*regexp.Regexp  // any match on original msg → hit
	compound   func(string) bool // optional complex condition
}

// classifierRules is the ordered list of error classification patterns.
// First match wins.
var classifierRules = []classifierRule{
	{class: StreamErrorClassTimeout, substrings: []string{"timeout", "timed out"}},
	{class: StreamErrorClassRateLimit, substrings: []string{"rate limit", "too many requests"}, regexes: []*regexp.Regexp{reHTTP429}},
	{class: StreamErrorClassAuthFailure, substrings: []string{"unauthorized", "authentication", "forbidden", "invalid api key", "invalid_api_key"}, regexes: []*regexp.Regexp{reHTTP401, reHTTP403}},
	{class: StreamErrorClassContextOverflow, compound: func(msg string) bool {
		return strings.Contains(msg, "context") &&
			(strings.Contains(msg, "limit") || strings.Contains(msg, "overflow") || strings.Contains(msg, "too long"))
	}},
	{class: StreamErrorClassSandboxViolation, compound: func(msg string) bool {
		return strings.Contains(msg, "permission denied") ||
			(strings.Contains(msg, "sandbox") && (strings.Contains(msg, "not allowed") || strings.Contains(msg, "not permitted") || strings.Contains(msg, "violation") || strings.Contains(msg, "denied"))) ||
			(strings.Contains(msg, "not allowed") && (strings.Contains(msg, "permission") || strings.Contains(msg, "operation")))
	}},
	{class: StreamErrorClassRateLimit, regexes: []*regexp.Regexp{reQuotaError}},
	{class: StreamErrorClassExecutionError, regexes: []*regexp.Regexp{reNetworkError}},
	{class: StreamErrorClassExecutionError, regexes: []*regexp.Regexp{reProcessError}},
	{class: StreamErrorClassExecutionError, regexes: []*regexp.Regexp{reModelError}},
}

// matchesRule checks whether a message matches a classifier rule.
func matchesRule(r classifierRule, lowMsg, origMsg string) bool {
	for _, s := range r.substrings {
		if strings.Contains(lowMsg, s) {
			return true
		}
	}
	for _, re := range r.regexes {
		if re.MatchString(origMsg) {
			return true
		}
	}
	if r.compound != nil {
		return r.compound(lowMsg)
	}
	return false
}

func ParseStreamEvent(data []byte) (StreamEvent, error) {
	var ev StreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return StreamEvent{}, err
	}
	// Normalize ErrorClass: clear when not an error, validate against allowlist,
	// and auto-classify when empty.
	if !ev.IsError {
		ev.ErrorClass = StreamErrorClassNone
	} else if ev.ErrorClass != StreamErrorClassNone && !validErrorClasses[ev.ErrorClass] {
		// Invalid wire value — reclassify from message content.
		ev.ErrorClass = ClassifyStreamError(ev)
	} else if ev.ErrorClass == StreamErrorClassNone {
		ev.ErrorClass = ClassifyStreamError(ev)
	}
	return ev, nil
}

// ClassifyStreamError assigns a StreamErrorClass to a StreamEvent based on
// pattern matching against the error message. Returns StreamErrorClassNone
// when IsError is false. Uses first-match-wins on the pattern list.
//
// The classifier uses contextual regex patterns instead of bare substring
// matches to avoid false positives on port numbers, line numbers, or
// benign validation messages.
//
// Unrecognized errors with a non-empty message are classified as
// execution_error. Only empty/whitespace-only messages return unknown.
func ClassifyStreamError(ev StreamEvent) StreamErrorClass {
	if !ev.IsError {
		return StreamErrorClassNone
	}

	msg := strings.ToLower(ev.Message)

	for _, rule := range classifierRules {
		if matchesRule(rule, msg, ev.Message) {
			return rule.class
		}
	}

	if strings.TrimSpace(msg) == "" {
		return StreamErrorClassUnknown
	}
	return StreamErrorClassExecutionError
}
