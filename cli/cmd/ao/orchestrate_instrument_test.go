//go:build legacy

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/boshu2/agentops/cli/internal/orchestration"
)

func TestOrchestrateToolsCommand_Registered(t *testing.T) {
	if orchestrateToolsCmd.Use != "tools" {
		t.Fatalf("Use = %q", orchestrateToolsCmd.Use)
	}
}

func TestOrchestrateRoute_TriVendorJSON(t *testing.T) {
	result := orchestration.RunRoute(orchestration.RouteOptions{
		Writers: 3,
		Models:  []string{"opus", "codex", "agy"},
	})
	if result.Route == nil || result.Route.Profile != "tri-vendor" {
		t.Fatalf("route = %+v", result.Route)
	}
}

func TestFinishInstrumentCommand_FailExit(t *testing.T) {
	cmd := orchestrateToolsCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) }) // age-ztf8: shared command; don't leak the writer
	result := orchestration.InstrumentResult{
		SchemaVersion: orchestration.InstrumentSchemaVersionV1,
		Command:       orchestration.InstrumentCommandTools,
		Verdict:       orchestration.Verdict{Status: orchestration.VerdictStatusFail, Confidence: orchestration.VerdictConfidenceHigh},
	}
	if err := finishInstrumentCommand(cmd, result); err == nil {
		t.Fatal("expected error on FAIL")
	}
}

func TestInstrumentResult_JSONRoundTrip(t *testing.T) {
	result := orchestration.InstrumentResult{
		SchemaVersion: 1,
		Command:       orchestration.InstrumentCommandPreflight,
		Profile:       "dual-pane",
		Verdict:       orchestration.Verdict{Status: orchestration.VerdictStatusPass, Confidence: orchestration.VerdictConfidenceHigh},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded orchestration.InstrumentResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Profile != "dual-pane" {
		t.Fatalf("profile = %q", decoded.Profile)
	}
}
