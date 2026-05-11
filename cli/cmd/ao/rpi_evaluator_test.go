// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRPIPhaseEvaluatorUsesTranscriptOutcome(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	transcriptDir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := `{"type":"user","sessionId":"sess-eval","message":{"content":"run tests"}}
{"type":"tool_result","content":"PASSED 3 tests"}
{"type":"tool_result","content":"[main abc123] feat: evaluator proof"}
{"type":"tool_result","content":"Enumerating objects: 2, done.\nWriting objects: 100% (2/2), done."}
`
	if err := os.WriteFile(filepath.Join(transcriptDir, "session.jsonl"), []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}

	rpiDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(filepath.Join(rpiDir, "runs", "rpi-eval"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-result.json"), []byte(`{"phase":2,"status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rpiDir, "phase-2-summary.md"), []byte("implementation summary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := appendRPIC2Event(root, rpiC2EventInput{
		RunID: "rpi-eval",
		Phase: 2,
		Type:  "stream.result",
		Details: map[string]any{
			"session_id": "sess-eval",
			"num_turns":  4,
		},
	}); err != nil {
		t.Fatalf("appendRPIC2Event: %v", err)
	}

	state := newTestPhasedState().WithRunID("rpi-eval").WithPhase(2)
	artifact, err := emitPhaseEvaluatorArtifact(root, state, 2, "PASS", nil)
	if err != nil {
		t.Fatalf("emitPhaseEvaluatorArtifact: %v", err)
	}
	if artifact.SessionOutcome == nil {
		t.Fatal("expected session outcome evidence")
	}
	if artifact.SessionOutcome.Reward <= 0 {
		t.Fatalf("reward = %.2f, want > 0", artifact.SessionOutcome.Reward)
	}
	if artifact.Verdict != "PASS" {
		t.Fatalf("verdict = %q, want PASS", artifact.Verdict)
	}
	if state.Verdicts["implementation_evaluator"] != "PASS" {
		t.Fatalf("state verdict not updated: %#v", state.Verdicts)
	}
	if _, err := os.Stat(filepath.Join(rpiDir, "phase-2-evaluator.json")); err != nil {
		t.Fatalf("expected evaluator artifact on disk: %v", err)
	}
}
