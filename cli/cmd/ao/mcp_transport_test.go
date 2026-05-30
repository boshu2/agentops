package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// driveMCP feeds newline-delimited JSON-RPC requests through serveMCP with the
// given executor and returns the decoded responses, one per request line.
func driveMCP(t *testing.T, exec mcpToolExecutor, requests ...string) []mcpResponse {
	t.Helper()
	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := serveMCP(in, &out, exec); err != nil {
		t.Fatalf("serveMCP: %v", err)
	}
	var resps []mcpResponse
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var r mcpResponse
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("decoding response %q: %v", line, err)
		}
		resps = append(resps, r)
	}
	return resps
}

func okExec(name string, _ map[string]string) (string, error) {
	return "RESULT:" + name, nil
}

func TestServeMCP_Initialize(t *testing.T) {
	resps := driveMCP(t, okExec, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(resps) != 1 {
		t.Fatalf("got %d responses, want 1", len(resps))
	}
	if resps[0].Error != nil {
		t.Fatalf("initialize errored: %v", resps[0].Error)
	}
	res, _ := json.Marshal(resps[0].Result)
	if !strings.Contains(string(res), "capabilities") || !strings.Contains(string(res), "serverInfo") {
		t.Errorf("initialize result missing capabilities/serverInfo: %s", res)
	}
}

func TestServeMCP_ToolsList(t *testing.T) {
	resps := driveMCP(t, okExec, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	res, _ := json.Marshal(resps[0].Result)
	var doc struct {
		Tools []mcpToolDescriptor `json:"tools"`
	}
	if err := json.Unmarshal(res, &doc); err != nil {
		t.Fatalf("tools/list result not decodable: %v", err)
	}
	if len(doc.Tools) != 6 {
		t.Errorf("tools/list returned %d tools, want 6", len(doc.Tools))
	}
}

func TestServeMCP_ToolsCall_DispatchesToExecutor(t *testing.T) {
	var calledWith string
	exec := func(name string, args map[string]string) (string, error) {
		calledWith = name + ":" + args["target"]
		return "PASS", nil
	}
	resps := driveMCP(t, exec,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"validate","arguments":{"target":"plan.md"}}}`)
	if resps[0].Error != nil {
		t.Fatalf("tools/call errored: %v", resps[0].Error)
	}
	if calledWith != "validate:plan.md" {
		t.Errorf("executor called with %q, want validate:plan.md", calledWith)
	}
	res, _ := json.Marshal(resps[0].Result)
	if !strings.Contains(string(res), "PASS") {
		t.Errorf("tools/call result should carry executor output, got %s", res)
	}
}

func TestServeMCP_ToolsCall_HoldoutDenied(t *testing.T) {
	called := false
	exec := func(string, map[string]string) (string, error) { called = true; return "", nil }
	resps := driveMCP(t, exec,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"corpus_inject","arguments":{"query":"the holdout ground_truth"}}}`)
	if resps[0].Error == nil {
		t.Fatal("a holdout-surfacing tool call must return a JSON-RPC error")
	}
	if called {
		t.Error("executor must NOT run for a denied holdout call (NOT-ZDR)")
	}
	low := strings.ToLower(resps[0].Error.Message)
	if !strings.Contains(low, "holdout") && !strings.Contains(low, "zdr") {
		t.Errorf("denial message must name the holdout/ZDR boundary, got %q", resps[0].Error.Message)
	}
}

func TestServeMCP_UnknownMethod(t *testing.T) {
	resps := driveMCP(t, okExec, `{"jsonrpc":"2.0","id":5,"method":"does/not/exist"}`)
	if resps[0].Error == nil || resps[0].Error.Code != -32601 {
		t.Errorf("unknown method must return JSON-RPC -32601, got %+v", resps[0].Error)
	}
}

func TestServeMCP_MultiRequestRoundTrip(t *testing.T) {
	// The acceptance: a session round-trips initialize + tool calls over the
	// transport and gets structured responses back.
	resps := driveMCP(t, okExec,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"session_bootstrap","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"validate","arguments":{"target":"x"}}}`,
	)
	if len(resps) != 3 {
		t.Fatalf("got %d responses, want 3", len(resps))
	}
	for i, r := range resps {
		if r.Error != nil {
			t.Errorf("response %d errored: %v", i, r.Error)
		}
	}
}
