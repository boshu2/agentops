package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// CodexUsage retains native inclusive input and nullable counters. Reasoning is
// a subset of output; cached input is a subset of input. Neither is added twice.
type CodexUsage struct {
	Input     *int64 `json:"input_tokens"`
	Cached    *int64 `json:"cached_input_tokens"`
	Fresh     *int64 `json:"fresh_input_tokens"`
	Output    *int64 `json:"output_tokens"`
	Reasoning *int64 `json:"reasoning_output_tokens"`
	Total     *int64 `json:"total_tokens"`
}

// AccountingDiagnostic identifies evidence that could not be interpreted.
type AccountingDiagnostic struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// CodexProviderError is an HTTP error embedded by the runtime in a terminal
// event. It is not inferred from an assistant's prose or tool output.
type CodexProviderError struct {
	Status  int    `json:"status"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// CodexAccounting is a read-only view of one native rollout. Metadata and the
// last cumulative payload are retained verbatim, including unrecognized fields.
// Raw content for other events remains in the evidence file at the stated line.
type CodexAccounting struct {
	ID                string                 `json:"id"`
	ParentID          string                 `json:"parent_id"`
	Metadata          json.RawMessage        `json:"metadata"`
	FirstTimestamp    string                 `json:"first_timestamp"`
	LastTimestamp     string                 `json:"last_timestamp"`
	LatestTurnState   string                 `json:"latest_turn_state"`
	LatestTurnPayload json.RawMessage        `json:"latest_turn_payload"`
	ProviderError     *CodexProviderError    `json:"provider_error"`
	Usage             *CodexUsage            `json:"usage"`
	UsagePayload      json.RawMessage        `json:"usage_payload"`
	UsageLine         int                    `json:"usage_line"`
	UsageTimestamp    string                 `json:"usage_timestamp"`
	UsageEvents       int                    `json:"usage_events"`
	Lines             int                    `json:"lines"`
	Uninterpreted     map[string]int         `json:"uninterpreted_event_counts"`
	Diagnostics       []AccountingDiagnostic `json:"diagnostics"`
}

// ParseCodexAccounting never sums cumulative events. Missing usage remains nil;
// malformed records are diagnosed without hiding the rest of an interrupted file.
func ParseCodexAccounting(r io.Reader) (*CodexAccounting, error) {
	a := &CodexAccounting{Uninterpreted: map[string]int{}, Diagnostics: []AccountingDiagnostic{}}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		a.Lines++
		if strings.TrimSpace(scanner.Text()) != "" {
			a.readEvent(scanner.Bytes())
		}
	}
	if err := scanner.Err(); err != nil {
		return a, fmt.Errorf("read Codex accounting at line %d: %w", a.Lines+1, err)
	}
	if a.ID == "" {
		a.diagnose("missing session_meta identity")
	}
	return a, nil
}

func (a *CodexAccounting) diagnose(message string) {
	a.Diagnostics = append(a.Diagnostics, AccountingDiagnostic{Line: a.Lines, Message: message})
}

func (a *CodexAccounting) readEvent(line []byte) {
	var event struct {
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		a.diagnose("malformed event: " + err.Error())
		return
	}
	if a.FirstTimestamp == "" {
		a.FirstTimestamp = event.Timestamp
	}
	if event.Timestamp != "" {
		a.LastTimestamp = event.Timestamp
	}
	switch event.Type {
	case "session_meta":
		a.readMetadata(event.Payload)
	case "event_msg":
		a.readPayload(event.Payload, event.Timestamp)
	default:
		a.Uninterpreted[event.Type]++
	}
}

func (a *CodexAccounting) readMetadata(raw json.RawMessage) {
	var meta struct {
		ID        string `json:"id"`
		SessionID string `json:"session_id"`
		ParentID  string `json:"parent_thread_id"`
		Source    struct {
			Subagent struct {
				ThreadSpawn struct {
					ParentID string `json:"parent_thread_id"`
				} `json:"thread_spawn"`
			} `json:"subagent"`
		} `json:"source"`
	}
	// source is either a string (CLI) or an object (subagent). Decode the common
	// identity separately so the valid string variant cannot erase identity.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		a.diagnose("malformed session_meta object")
		return
	}
	source := fields["source"]
	delete(fields, "source")
	identity, err := json.Marshal(fields)
	if err != nil {
		a.diagnose("encode session_meta identity: " + err.Error())
		return
	}
	if err := json.Unmarshal(identity, &meta); err != nil {
		a.diagnose("malformed session_meta identity: " + err.Error())
		return
	}
	if len(source) > 0 && source[0] == '{' {
		if err := json.Unmarshal(source, &meta.Source); err != nil {
			a.diagnose("unrecognized session_meta source: " + err.Error())
		}
	}
	id := coalesce(meta.ID, meta.SessionID)
	if a.ID != "" && a.ID != id {
		a.diagnose("conflicting session_meta identities")
		return
	}
	a.ID = id
	a.ParentID = coalesce(meta.ParentID, meta.Source.Subagent.ThreadSpawn.ParentID)
	a.Metadata = append(json.RawMessage(nil), raw...)
}

func (a *CodexAccounting) readPayload(raw json.RawMessage, timestamp string) {
	var payload struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
		Info  *struct {
			Total json.RawMessage `json:"total_token_usage"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		a.diagnose("malformed event_msg payload: " + err.Error())
		return
	}
	switch payload.Type {
	case "task_started", "task_complete", "task_aborted":
		a.LatestTurnState = payload.Type
		a.LatestTurnPayload = append(json.RawMessage(nil), raw...)
		a.ProviderError = readProviderError(payload.Error)
	case "token_count":
		a.UsageEvents++
		if payload.Info != nil && len(payload.Info.Total) > 0 && string(payload.Info.Total) != "null" {
			a.readUsage(payload.Info.Total, raw, timestamp)
		}
	default:
		a.Uninterpreted["event_msg/"+payload.Type]++
	}
}

func readProviderError(raw json.RawMessage) *CodexProviderError {
	var native struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		return nil
	}
	var provider struct {
		Status int `json:"status"`
		Error  struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(native.Message), &provider); err != nil {
		return nil
	}
	if provider.Status < 400 || provider.Status > 599 || provider.Error.Type == "" {
		return nil
	}
	return &CodexProviderError{Status: provider.Status, Type: provider.Error.Type, Message: provider.Error.Message}
}

func (a *CodexAccounting) readUsage(raw, payload json.RawMessage, timestamp string) {
	a.UsagePayload = append(json.RawMessage(nil), payload...)
	a.UsageLine, a.UsageTimestamp = a.Lines, timestamp
	var usage CodexUsage
	if err := json.Unmarshal(raw, &usage); err != nil {
		a.Usage = nil
		a.diagnose("malformed cumulative usage: " + err.Error())
		return
	}
	// Fresh is derived, never accepted from a worker-authored extra field.
	usage.Fresh = nil
	if err := validateCodexUsage(usage); err != nil {
		a.Usage = nil
		a.diagnose(err.Error())
		return
	}
	if usage.Input != nil && usage.Cached != nil {
		fresh := *usage.Input - *usage.Cached
		usage.Fresh = &fresh
	}
	if a.Usage != nil && countersDecrease(*a.Usage, usage) {
		a.diagnose("cumulative counters decreased; last native snapshot retained, aggregation requires review")
	}
	a.Usage = &usage
}

func validateCodexUsage(u CodexUsage) error {
	for _, value := range []*int64{u.Input, u.Cached, u.Output, u.Reasoning, u.Total} {
		if value != nil && *value < 0 {
			return fmt.Errorf("negative cumulative token counter")
		}
	}
	if u.Input != nil && u.Cached != nil && *u.Cached > *u.Input {
		return fmt.Errorf("cached input exceeds inclusive input")
	}
	if u.Output != nil && u.Reasoning != nil && *u.Reasoning > *u.Output {
		return fmt.Errorf("reasoning tokens exceed inclusive output")
	}
	if u.Input != nil && u.Output != nil && u.Total != nil && (*u.Total < *u.Input || *u.Total-*u.Input != *u.Output) {
		return fmt.Errorf("total tokens disagree with input plus output")
	}
	return nil
}

func countersDecrease(old, next CodexUsage) bool {
	return (old.Input != nil && next.Input != nil && *next.Input < *old.Input) ||
		(old.Output != nil && next.Output != nil && *next.Output < *old.Output)
}
