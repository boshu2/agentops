package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

// RuntimeSpan identifies exact bytes in the supplied native JSONL span. Offsets
// are zero-based, end-exclusive; newlines are included, never normalized.
type RuntimeSpan struct {
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	SHA256 string `json:"sha256"`
	Kind   string `json:"kind"`
}

// RuntimeMetadata reports native envelope facts only, never assistant text.
// A model absent from runtime reporting stays unknown even when launch options,
// Claude's init event or Codex turn_context repeat the requested model.
type RuntimeMetadata struct {
	Model       string        `json:"model"`
	ContextID   string        `json:"context_id"`
	Provider    string        `json:"provider"`
	Effort      string        `json:"effort"`
	Completed   bool          `json:"completed"`
	Termination string        `json:"termination"`
	Spans       []RuntimeSpan `json:"spans"`
}

type runtimeEnvelope struct {
	Type            string `json:"type"`
	Subtype         string `json:"subtype"`
	SessionID       string `json:"sessionId"`
	NativeSessionID string `json:"session_id"`
	IsError         *bool  `json:"is_error"`
	Message         *struct {
		Role  string `json:"role"`
		Model string `json:"model"`
	} `json:"message"`
	Payload struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		Model    string `json:"model"`
		Provider string `json:"model_provider"`
		Effort   string `json:"reasoning_effort"`
	} `json:"payload"`
}

// ParseRuntime is the strict metadata seam beside Parse's tolerant content
// reader. It rejects malformed/duplicate JSON and conflicting native identities
// across the complete supplied invocation span. It does not parse nested text
// as an envelope or infer actual model identity from requested configuration.
func (p *Parser) ParseRuntime(payload []byte, runtime string) (*RuntimeMetadata, error) {
	if runtime != "codex" && runtime != "claude" {
		return nil, fmt.Errorf("unsupported native runtime %q", runtime)
	}
	result := &RuntimeMetadata{Spans: []RuntimeSpan{}}
	offset := int64(0)
	for _, line := range bytes.SplitAfter(payload, []byte("\n")) {
		end := offset + int64(len(line))
		if len(bytes.TrimSpace(line)) != 0 {
			observed, kind, err := nativeObservation(line, runtime)
			if err != nil {
				return nil, fmt.Errorf("native transcript at byte %d: %w", offset, err)
			}
			if kind != "" {
				if err = mergeRuntime(result, observed); err != nil {
					return nil, err
				}
				sum := sha256.Sum256(line)
				result.Spans = append(result.Spans, RuntimeSpan{offset, end, hex.EncodeToString(sum[:]), kind})
			}
		}
		offset = end
	}
	return result, nil
}

func nativeObservation(line []byte, runtime string) (RuntimeMetadata, string, error) {
	if _, err := verdictcheck.DecodeObject(line); err != nil {
		return RuntimeMetadata{}, "", err
	}
	var row runtimeEnvelope
	if err := json.Unmarshal(line, &row); err != nil {
		return RuntimeMetadata{}, "", err
	}
	if runtime == "claude" {
		return claudeObservation(row)
	}
	return codexObservation(row)
}

func claudeObservation(row runtimeEnvelope) (RuntimeMetadata, string, error) {
	obs := RuntimeMetadata{}
	if row.SessionID != "" && row.NativeSessionID != "" && row.SessionID != row.NativeSessionID {
		return obs, "", fmt.Errorf("conflicting native session IDs")
	}
	obs.ContextID = coalesce(row.NativeSessionID, row.SessionID)
	switch {
	case row.Type == "assistant" && row.Message != nil && row.Message.Role == "assistant":
		obs.Model = row.Message.Model
		obs.Provider = "anthropic"
		return obs, "assistant.message.model", nil
	case row.Type == "system" && row.Subtype == "init":
		return obs, "system.init.session_id", nil
	case row.Type == "result":
		obs.Completed = row.Subtype == "success" && row.IsError != nil && !*row.IsError
		obs.Termination = row.Subtype
		if obs.Termination == "" {
			obs.Termination = "unknown"
		}
		return obs, "result", nil
	default:
		return RuntimeMetadata{}, "", nil
	}
}

func codexObservation(row runtimeEnvelope) (RuntimeMetadata, string, error) {
	obs := RuntimeMetadata{}
	switch {
	case row.Type == "session_meta":
		obs.ContextID = row.Payload.ID
		obs.Model = row.Payload.Model
		obs.Provider = row.Payload.Provider
		obs.Effort = row.Payload.Effort
		return obs, "session_meta.payload", nil
	case row.Type == "event_msg" && row.Payload.Type == "task_complete":
		obs.Completed = true
		obs.Termination = "task_complete"
		return obs, "event_msg.task_complete", nil
	case row.Type == "event_msg" && (row.Payload.Type == "turn_aborted" || row.Payload.Type == "error"):
		obs.Termination = row.Payload.Type
		return obs, "event_msg.termination", nil
	default:
		return obs, "", nil
	}
}

func mergeRuntime(dst *RuntimeMetadata, src RuntimeMetadata) error {
	fields := []struct {
		target      *string
		value, name string
	}{
		{&dst.Model, src.Model, "model"}, {&dst.ContextID, src.ContextID, "context"},
		{&dst.Provider, src.Provider, "provider"}, {&dst.Effort, src.Effort, "effort"},
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		if *field.target != "" && *field.target != field.value {
			return fmt.Errorf("conflicting native %s identity", field.name)
		}
		*field.target = field.value
	}
	if src.Termination != "" {
		if dst.Termination != "" {
			return fmt.Errorf("multiple native terminations in invocation span")
		}
		dst.Termination = src.Termination
		dst.Completed = src.Completed
	}
	return nil
}
