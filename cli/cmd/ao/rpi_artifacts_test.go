package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRPIPhaseResultArtifactsIncludeEvaluatorProof(t *testing.T) {
	root := t.TempDir()
	rpiDir := filepath.Join(root, ".agents", "rpi")
	runID := "rpi-proof"
	if err := os.MkdirAll(filepath.Join(rpiDir, "runs", runID), 0o755); err != nil {
		t.Fatal(err)
	}

	result := &phaseResult{
		SchemaVersion: 1,
		RunID:         runID,
		Phase:         2,
		PhaseName:     "implementation",
		Status:        "completed",
	}
	if err := writePhaseResult(root, result); err != nil {
		t.Fatalf("writePhaseResult: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-summary.md"), []byte("summary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-handoff.json"), []byte(`{"phase":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-evaluator.json"), []byte(`{"verdict":"PASS"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "execution-packet.json"), []byte(`{"objective":"proof path"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, phasedStateFile), []byte(`{"run_id":"`+runID+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "runs", runID, rpiC2EventsFileName), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithRunID(runID).WithPhase(2)
	if err := updatePhaseResultArtifacts(root, state, 2, nil); err != nil {
		t.Fatalf("updatePhaseResultArtifacts: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rpiDir, "phase-2-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var updated phaseResult
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"summary", "handoff", "evaluator", "execution_packet", "events"} {
		if strings.TrimSpace(updated.Artifacts[key]) == "" {
			t.Fatalf("artifacts[%s] missing from updated phase result: %#v", key, updated.Artifacts)
		}
	}
}

func TestRPIExecutionPacketProofPreservesExistingFields(t *testing.T) {
	root := t.TempDir()
	rpiDir := filepath.Join(root, ".agents", "rpi")
	runID := "rpi-proof"
	if err := os.MkdirAll(filepath.Join(rpiDir, "runs", runID), 0o755); err != nil {
		t.Fatal(err)
	}

	packet := `{
  "objective": "proof packet",
  "custom_field": "keep-me",
  "related_issue_ids": ["ag-7t6"]
}
`
	if err := os.WriteFile(filepath.Join(rpiDir, "execution-packet.json"), []byte(packet), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-evaluator.json"), []byte(`{"verdict":"WARN"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-result.json"), []byte(`{"phase":2,"status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "runs", runID, rpiC2EventsFileName), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := newTestPhasedState().WithRunID(runID).WithPhase(2)
	if err := updateExecutionPacketProof(root, state); err != nil {
		t.Fatalf("updateExecutionPacketProof: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rpiDir, "execution-packet.json"))
	if err != nil {
		t.Fatal(err)
	}
	archivedData, err := os.ReadFile(filepath.Join(rpiDir, "runs", runID, executionPacketFile))
	if err != nil {
		t.Fatalf("read archived execution packet: %v", err)
	}
	if string(archivedData) != string(data) {
		t.Fatalf("archived execution packet does not match latest alias:\nlatest:\n%s\narchived:\n%s", data, archivedData)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["custom_field"] != "keep-me" {
		t.Fatalf("custom_field lost: %#v", parsed)
	}
	if parsed["run_id"] != runID {
		t.Fatalf("run_id = %v, want %q", parsed["run_id"], runID)
	}
	proof, ok := parsed["proof_artifacts"].([]any)
	if !ok || len(proof) == 0 {
		t.Fatalf("proof_artifacts missing: %#v", parsed["proof_artifacts"])
	}
	evaluators, ok := parsed["evaluator_artifacts"].(map[string]any)
	if !ok {
		t.Fatalf("evaluator_artifacts missing: %#v", parsed["evaluator_artifacts"])
	}
	if evaluators["phase_2"] != ".agents/rpi/phase-2-evaluator.json" {
		t.Fatalf("phase_2 evaluator path = %#v", evaluators["phase_2"])
	}
}

func TestRPIExecutionPacketProofPopulatesMixedModeProvenance(t *testing.T) {
	root := t.TempDir()
	runID := "mixed-proof"
	state := newTestPhasedState().
		WithGoal("mixed provenance").
		WithRunID(runID).
		WithOpts(phasedEngineOptions{Mixed: true, RuntimeCommand: "claude"})

	if err := writeExecutionPacketSeed(root, state); err != nil {
		t.Fatalf("writeExecutionPacketSeed: %v", err)
	}
	handoff := buildPhaseHandoffFromState(state, 1, root)
	if err := writePhaseHandoff(root, handoff); err != nil {
		t.Fatalf("writePhaseHandoff: %v", err)
	}
	if err := updateExecutionPacketProof(root, state); err != nil {
		t.Fatalf("updateExecutionPacketProof: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".agents", "rpi", "execution-packet.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["mixed_mode_requested"] != true {
		t.Fatalf("mixed_mode_requested = %#v, want true", parsed["mixed_mode_requested"])
	}
	if parsed["mixed_mode_effective"] != true {
		t.Fatalf("mixed_mode_effective = %#v, want true", parsed["mixed_mode_effective"])
	}
	if parsed["planner_vendor"] != "claude" {
		t.Fatalf("planner_vendor = %#v, want claude", parsed["planner_vendor"])
	}
	if parsed["reviewer_vendor"] != "codex" {
		t.Fatalf("reviewer_vendor = %#v, want codex", parsed["reviewer_vendor"])
	}
	if _, ok := parsed["mixed_mode_degraded_reason"]; ok {
		t.Fatalf("mixed_mode_degraded_reason unexpectedly set: %#v", parsed["mixed_mode_degraded_reason"])
	}
}

func TestRPIExecutionPacketProofUsesPacketRunIDWhenStateNil(t *testing.T) {
	root := t.TempDir()
	rpiDir := filepath.Join(root, ".agents", "rpi")
	runID := "rpi-packet"
	if err := os.MkdirAll(filepath.Join(rpiDir, "runs", runID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "execution-packet.json"), []byte(`{"objective":"packet","run_id":"`+runID+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "runs", runID, rpiC2EventsFileName), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := updateExecutionPacketProof(root, nil); err != nil {
		t.Fatalf("updateExecutionPacketProof: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(rpiDir, "execution-packet.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["run_id"] != runID {
		t.Fatalf("run_id = %v, want %q", parsed["run_id"], runID)
	}
	if !packetProofArtifactsContain(parsed, filepath.ToSlash(filepath.Join(".agents", "rpi", "runs", runID, rpiC2EventsFileName))) {
		t.Fatalf("proof_artifacts did not include run events: %#v", parsed["proof_artifacts"])
	}
}

func packetProofArtifactsContain(packet map[string]any, want string) bool {
	proof, ok := packet["proof_artifacts"].([]any)
	if !ok {
		return false
	}
	for _, raw := range proof {
		if path, ok := raw.(string); ok && path == want {
			return true
		}
	}
	return false
}
