// practices: [hexagonal-architecture, design-by-contract]
package mcptransport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// ToolDescriptor is one curated tool exposed through the AO MCP surface.
type ToolDescriptor struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	InputSchema      map[string]any `json:"input_schema"`
	HoldoutSensitive bool           `json:"holdout_sensitive"`
}

// ToolExecutor runs a curated tool by name and returns structured text output.
type ToolExecutor func(name string, args map[string]string) (string, error)

// DenyFunc rejects a tool call before execution when its arguments cross a
// policy boundary.
type DenyFunc func(name string, args map[string]string) (bool, string)

// Options configures the JSON-RPC transport.
type Options struct {
	ToolDescriptors func() []ToolDescriptor
	Deny            DenyFunc
	Exec            ToolExecutor
}

// Request is the JSON-RPC 2.0 request shape consumed by the MCP transport.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// RPCError is the JSON-RPC 2.0 error shape emitted by the MCP transport.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Response is the JSON-RPC 2.0 response shape emitted by the MCP transport.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Serve reads newline-delimited JSON-RPC requests from r, dispatches each, and
// writes one response line per request to w. A request with no id is a
// notification and gets no response. Returns on EOF.
func Serve(r io.Reader, w io.Writer, opts Options) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			if encErr := enc.Encode(ParseErrorResponse()); encErr != nil {
				return encErr
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		result, rpcErr := Dispatch(req, opts)
		resp := Response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// ParseErrorResponse returns the JSON-RPC parse-error response.
func ParseErrorResponse() Response {
	return Response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &RPCError{Code: -32700, Message: "parse error"}}
}

// Dispatch routes one JSON-RPC request to its handler.
func Dispatch(req Request, opts Options) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return HandleInitialize(), nil
	case "tools/list":
		tools := []ToolDescriptor{}
		if opts.ToolDescriptors != nil {
			tools = opts.ToolDescriptors()
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		return HandleToolsCall(req.Params, opts)
	default:
		return nil, &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

// HandleInitialize returns the MCP initialize response payload.
func HandleInitialize() map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "ao-mcp", "version": "1"},
	}
}

// HandleToolsCall enforces denial policy, dispatches to the executor, and wraps
// the output in MCP `content` shape.
func HandleToolsCall(params json.RawMessage, opts Options) (any, *RPCError) {
	var call struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid tools/call params"}
	}
	if opts.Deny != nil {
		if denied, reason := opts.Deny(call.Name, call.Arguments); denied {
			return nil, &RPCError{Code: -32001, Message: reason}
		}
	}
	if opts.Exec == nil {
		return nil, &RPCError{Code: -32603, Message: "no tool executor configured"}
	}
	out, err := opts.Exec(call.Name, call.Arguments)
	if err != nil {
		return nil, &RPCError{Code: -32000, Message: fmt.Sprintf("tool %q failed: %v", call.Name, err)}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": out}},
	}, nil
}
