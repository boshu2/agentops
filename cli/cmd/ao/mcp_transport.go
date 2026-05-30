// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// MCP JSON-RPC 2.0 stdio transport (ag-3ucpd). Newline-delimited JSON: one
// request object per line, one response object per line. Serves the curated tool
// surface from mcp_serve.go (mcpToolDescriptors / mcpToolDenied) so a hosted /
// SDK Claude loop can orient and self-check.

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

// mcpToolExecutor runs a curated tool by name and returns its structured text
// output. The production executor shells the wrapped `ao` subcommand; tests
// inject a deterministic fake (the protocol/dispatch/refusal is what we gate).
type mcpToolExecutor func(name string, args map[string]string) (string, error)

// serveMCP reads newline-delimited JSON-RPC requests from r, dispatches each,
// and writes one response line per request to w. A request with no id is a
// notification and gets no response. Returns on EOF.
func serveMCP(r io.Reader, w io.Writer, exec mcpToolExecutor) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal(line, &req); err != nil {
			if encErr := enc.Encode(parseErrorResponse()); encErr != nil {
				return encErr
			}
			continue
		}
		if len(req.ID) == 0 {
			continue // notification — no response
		}
		result, rpcErr := dispatchMCP(req, exec)
		resp := mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func parseErrorResponse() mcpResponse {
	return mcpResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &mcpRPCError{Code: -32700, Message: "parse error"}}
}

// dispatchMCP routes one request to its handler.
func dispatchMCP(req mcpRequest, exec mcpToolExecutor) (any, *mcpRPCError) {
	switch req.Method {
	case "initialize":
		return handleInitialize(), nil
	case "tools/list":
		return map[string]any{"tools": mcpToolDescriptors()}, nil
	case "tools/call":
		return handleToolsCall(req.Params, exec)
	default:
		return nil, &mcpRPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

func handleInitialize() map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "ao-mcp", "version": "1"},
	}
}

// handleToolsCall enforces the NOT-ZDR holdout refusal, then dispatches to the
// executor and wraps the output in MCP `content` shape.
func handleToolsCall(params json.RawMessage, exec mcpToolExecutor) (any, *mcpRPCError) {
	var call struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &mcpRPCError{Code: -32602, Message: "invalid tools/call params"}
	}
	if denied, reason := mcpToolDenied(call.Name, call.Arguments); denied {
		return nil, &mcpRPCError{Code: -32001, Message: reason}
	}
	if exec == nil {
		return nil, &mcpRPCError{Code: -32603, Message: "no tool executor configured"}
	}
	out, err := exec(call.Name, call.Arguments)
	if err != nil {
		return nil, &mcpRPCError{Code: -32000, Message: fmt.Sprintf("tool %q failed: %v", call.Name, err)}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": out}},
	}, nil
}
