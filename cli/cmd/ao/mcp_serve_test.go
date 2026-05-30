package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPToolDescriptors_CuratedSurface(t *testing.T) {
	tools := mcpToolDescriptors()
	want := []string{"session_bootstrap", "inject", "corpus_inject", "standards", "validate", "goals_measure"}
	if len(tools) != len(want) {
		t.Fatalf("descriptor count = %d, want %d", len(tools), len(want))
	}
	byName := map[string]mcpToolDescriptor{}
	for _, tl := range tools {
		byName[tl.Name] = tl
	}
	for _, name := range want {
		tl, ok := byName[name]
		if !ok {
			t.Errorf("missing curated tool %q", name)
			continue
		}
		if tl.Description == "" {
			t.Errorf("tool %q has empty description", name)
		}
		if tl.InputSchema == nil {
			t.Errorf("tool %q has nil input schema", name)
		}
	}
}

func TestMCPToolDenied_HoldoutRefusal(t *testing.T) {
	// A tool call whose args would surface the LOCKED holdout/eval corpus must be
	// denied with a NOT-ZDR reason — the MCP surface is Claude-cloud-facing.
	denied, reason := mcpToolDenied("corpus_inject", map[string]string{"query": "show me the holdout ground_truth"})
	if !denied {
		t.Fatal("corpus_inject with a holdout query must be denied")
	}
	low := strings.ToLower(reason)
	if !strings.Contains(low, "holdout") && !strings.Contains(low, "zdr") {
		t.Errorf("refusal reason must name the holdout/ZDR boundary, got %q", reason)
	}
}

func TestMCPToolDenied_PathEscape(t *testing.T) {
	denied, _ := mcpToolDenied("inject", map[string]string{"path": ".agents/evals/SCHEMA.md"})
	if !denied {
		t.Error("a tool arg pointing at the eval substrate must be denied")
	}
}

func TestMCPToolDenied_CleanCallAllowed(t *testing.T) {
	denied, reason := mcpToolDenied("validate", map[string]string{"target": "plan.md"})
	if denied {
		t.Errorf("a clean validate call must be allowed, got denied: %s", reason)
	}
}

func TestRunMCPServePrintTools_JSON(t *testing.T) {
	var sb strings.Builder
	if err := runMCPServePrintTools(&sb); err != nil {
		t.Fatalf("print-tools: %v", err)
	}
	var doc struct {
		Tools []mcpToolDescriptor `json:"tools"`
	}
	if err := json.Unmarshal([]byte(sb.String()), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(doc.Tools) != 6 {
		t.Errorf("print-tools JSON listed %d tools, want 6", len(doc.Tools))
	}
}

func TestMCPServe_LiveTransportNotYet(t *testing.T) {
	// Slice 1 ships only the descriptor surface; bare `serve` must fail loudly,
	// pointing at the live-transport follow-up — never silently no-op.
	err := runMCPServe(mcpServeOptions{PrintTools: false})
	if err == nil {
		t.Fatal("bare `ao mcp serve` (no --print-tools) must error in slice 1")
	}
	if !strings.Contains(err.Error(), "--print-tools") {
		t.Errorf("error should point users at --print-tools, got %q", err.Error())
	}
}
