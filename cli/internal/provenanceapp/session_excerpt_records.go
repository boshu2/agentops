package provenanceapp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/boshu2/agentops/cli/internal/verdictcheck"
)

type excerptField struct {
	Pointer string `json:"pointer"`
	Text    string `json:"text"`
	Tool    string `json:"tool,omitempty"`
	CallID  string `json:"call_id,omitempty"`
}

type excerptDiagnostic struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
}

type excerptTool struct {
	Pointer string `json:"pointer"`
	Name    string `json:"name,omitempty"`
	CallID  string `json:"call_id,omitempty"`
}

type excerptRecord struct {
	Span        excerptSpan         `json:"span"`
	Status      string              `json:"status"`
	NativeType  string              `json:"native_type,omitempty"`
	PayloadType string              `json:"payload_type,omitempty"`
	Role        string              `json:"role,omitempty"`
	ID          string              `json:"id,omitempty"`
	UUID        string              `json:"uuid,omitempty"`
	RequestID   string              `json:"request_id,omitempty"`
	SessionID   string              `json:"session_id,omitempty"`
	Tool        string              `json:"tool,omitempty"`
	CallID      string              `json:"call_id,omitempty"`
	Fields      []excerptField      `json:"fields"`
	Tools       []excerptTool       `json:"tools,omitempty"`
	Diagnostics []excerptDiagnostic `json:"diagnostics"`
}

func decodeExcerptRecord(raw []byte) excerptRecord {
	r := excerptRecord{Status: "supported", Fields: []excerptField{}, Diagnostics: []excerptDiagnostic{}}
	obj, err := verdictcheck.DecodeObject(raw)
	if err != nil {
		r.Status = "malformed"
		r.note("", "invalid_json_object") // Never echo decoder errors containing source bytes.
		return r
	}
	r.NativeType, r.Role = excerptString(obj, "type"), excerptString(obj, "role")
	r.ID, r.SessionID = excerptString(obj, "uuid"), excerptString(obj, "sessionId")
	r.UUID, r.RequestID = excerptString(obj, "uuid"), excerptString(obj, "requestId")
	switch r.NativeType {
	case "user", "assistant":
		r.claudeMessage(obj)
	case "tool_use":
		r.Tool, r.CallID = excerptString(obj, "tool_name"), excerptString(obj, "id")
		if r.Tool == "" {
			r.Tool = excerptString(obj, "name")
		}
		if _, ok := obj["tool_input"]; ok {
			r.Tools = append(r.Tools, excerptTool{Pointer: "", Name: r.Tool, CallID: r.CallID})
			r.input(obj["tool_input"], "/tool_input", r.Tool, r.CallID)
		} else {
			r.toolUse(obj, "")
		}
	case "tool_result":
		r.Tool, r.CallID = excerptString(obj, "tool_name"), excerptString(obj, "tool_use_id")
		key := firstExcerptKey(obj, "tool_output", "toolUseResult", "content")
		if key == "" {
			r.note("", "missing_tool_output")
		} else {
			r.text(obj[key], "/"+key, r.Tool, r.CallID)
		}
	case "event_msg", "response_item":
		payload, ok := obj["payload"].(map[string]any)
		if !ok {
			r.note("/payload", "missing_or_unsupported_payload")
		} else {
			r.codexPayload(payload)
		}
	default:
		r.note("/type", "unsupported_native_type")
	}
	if len(r.Diagnostics) > 0 {
		r.Status = "partial"
		if len(r.Fields) == 0 {
			r.Status = "unsupported"
		}
	}
	return r
}

func (r *excerptRecord) claudeMessage(obj map[string]any) {
	found := false
	if nested, present := obj["message"]; present {
		message, ok := nested.(map[string]any)
		if !ok {
			r.note("/message", "unsupported_message")
		} else {
			if role := excerptString(message, "role"); role != "" {
				r.Role = role
			}
			if id := excerptString(message, "id"); id != "" {
				r.ID = id
			}
			r.content(message["content"], "/message/content", "", "")
		}
		found = true
	}
	if content, present := obj["content"]; present {
		r.content(content, "/content", "", "")
		found = true
	}
	if !found {
		r.note("/content", "missing_content")
	}
}

func (r *excerptRecord) codexPayload(p map[string]any) {
	r.PayloadType, r.Role = excerptString(p, "type"), excerptString(p, "role")
	r.ID, r.Tool, r.CallID = excerptString(p, "id"), excerptString(p, "name"), excerptString(p, "call_id")
	if r.NativeType == "event_msg" {
		if r.PayloadType == "user_message" || r.PayloadType == "agent_message" {
			r.text(p["message"], "/payload/message", "", "")
		} else {
			r.note("/payload/type", "unsupported_event_type")
		}
		return
	}
	switch r.PayloadType {
	case "message":
		r.content(p["content"], "/payload/content", "", "")
	case "function_call", "custom_tool_call":
		key := firstExcerptKey(p, "arguments", "input")
		if key == "" {
			r.note("/payload", "missing_tool_input")
		} else {
			r.text(p[key], "/payload/"+key, r.Tool, r.CallID)
		}
	case "function_call_output", "custom_tool_call_output":
		r.text(p["output"], "/payload/output", r.Tool, r.CallID)
	default:
		r.note("/payload/type", "unsupported_response_type")
	}
}

func (r *excerptRecord) content(value any, pointer, tool, callID string) {
	if _, ok := value.(string); ok {
		r.text(value, pointer, tool, callID)
		return
	}
	blocks, ok := value.([]any)
	if !ok || len(blocks) == 0 {
		r.note(pointer, "missing_or_unsupported_content")
		return
	}
	for i, value := range blocks {
		base := fmt.Sprintf("%s/%d", pointer, i)
		block, ok := value.(map[string]any)
		if !ok {
			r.note(base, "unsupported_content_block")
			continue
		}
		switch excerptString(block, "type") {
		case "text", "input_text", "output_text":
			r.text(block["text"], base+"/text", tool, callID)
		case "tool_use":
			r.toolUse(block, base)
		case "tool_result":
			r.Tools = append(r.Tools, excerptTool{Pointer: base, CallID: excerptString(block, "tool_use_id")})
			r.content(block["content"], base+"/content", "", excerptString(block, "tool_use_id"))
		default:
			r.note(base+"/type", "unsupported_content_type")
		}
	}
}

func (r *excerptRecord) toolUse(block map[string]any, base string) {
	r.Tools = append(r.Tools, excerptTool{Pointer: base, Name: excerptString(block, "name"), CallID: excerptString(block, "id")})
	r.input(block["input"], base+"/input", excerptString(block, "name"), excerptString(block, "id"))
}

// Tool input objects expose only their literal string fields. Other values
// remain diagnosed at their exact pointers; they are never reserialized as quotes.
func (r *excerptRecord) input(value any, pointer, tool, callID string) {
	input, ok := value.(map[string]any)
	if !ok {
		r.text(value, pointer, tool, callID)
		return
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if len(keys) == 0 {
		r.note(pointer, "empty_input_object")
	}
	for _, key := range keys {
		escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
		r.text(input[key], pointer+"/"+escaped, tool, callID)
	}
}

func (r *excerptRecord) text(value any, pointer, tool, callID string) {
	text, ok := value.(string)
	if !ok {
		r.note(pointer, "missing_or_unsupported_text; objects_and_arrays_are_not_quotes")
		return
	}
	r.Fields = append(r.Fields, excerptField{Pointer: pointer, Text: text, Tool: tool, CallID: callID})
}

func (r *excerptRecord) note(pointer, code string) {
	r.Diagnostics = append(r.Diagnostics, excerptDiagnostic{Pointer: pointer, Code: code})
}

func excerptString(obj map[string]any, key string) string {
	value, _ := obj[key].(string)
	return value
}

func firstExcerptKey(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if _, ok := obj[key]; ok {
			return key
		}
	}
	return ""
}
