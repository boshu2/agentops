// practices: [dora-metrics, continuous-delivery]
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestOutputRatchetStatusTable(t *testing.T) {
	data := &ratchetStatusOutput{
		ChainID: "test-chain-123",
		Started: "2025-01-15T10:00:00Z",
		EpicID:  "ag-test",
		Path:    "/tmp/test/.agents/ao/chain.jsonl",
		Steps: []ratchetStepInfo{
			{Step: ratchet.StepResearch, Status: ratchet.StatusLocked, Output: "findings.md"},
			{Step: ratchet.StepPreMortem, Status: ratchet.StatusPending},
		},
	}

	// Set output to table (default)
	origOutput := output
	output = "table"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	err := outputRatchetStatus(&buf, data)
	if err != nil {
		t.Fatalf("outputRatchetStatus() error = %v", err)
	}

	out := buf.String()

	// Verify key content is captured in the buffer
	checks := []string{
		"Ratchet Chain Status",
		"test-chain-123",
		"ag-test",
		"STEP",
		"STATUS",
		"findings.md",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestOutputRatchetStatusJSON(t *testing.T) {
	data := &ratchetStatusOutput{
		ChainID: "json-chain",
		Started: "2025-01-15T10:00:00Z",
		Steps:   []ratchetStepInfo{},
		Path:    "/tmp/chain.jsonl",
	}

	origOutput := output
	output = "json"
	defer func() { output = origOutput }()

	var buf bytes.Buffer
	err := outputRatchetStatus(&buf, data)
	if err != nil {
		t.Fatalf("outputRatchetStatus() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "json-chain") {
		t.Errorf("JSON output missing chain ID\nGot:\n%s", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON output, got:\n%s", out)
	}
}
