// practices: [dora-metrics, continuous-delivery]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetCheck_UnknownStep(t *testing.T) {
	err := checkStepParse("nonexistent-step")
	if err == nil {
		t.Fatal("expected error for unknown step, got nil")
	}
	if want := "unknown step: nonexistent-step"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunRatchetCheck_ParseStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    string
		wantErr bool
	}{
		{"valid canonical research", "research", false},
		{"valid canonical plan", "plan", false},
		{"valid alias premortem", "premortem", false},
		{"valid alias postmortem", "postmortem", false},
		{"valid alias autopilot", "autopilot", false},
		{"valid alias validate", "validate", false},
		{"valid alias review", "review", false},
		{"unknown step", "bogus", true},
		{"empty step", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkStepParse(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkStepParse(%q) error = %v, wantErr = %v", tt.step, err, tt.wantErr)
			}
		})
	}
}

func TestRunRatchetCheck_GateChecker_ResearchAlwaysPasses(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	checker, err := ratchet.NewGateChecker(tmp)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepResearch)
	if err != nil {
		t.Fatalf("Check(research): %v", err)
	}
	if !result.Passed {
		t.Errorf("research gate should always pass, got Passed=false")
	}
}

func TestRunRatchetCheck_GateChecker_GateResultFields(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	checker, err := ratchet.NewGateChecker(tmp)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepResearch)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if result.Step != ratchet.StepResearch {
		t.Errorf("Step = %q, want %q", result.Step, ratchet.StepResearch)
	}
	if result.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestRunRatchetCheck_GateChecker_PreMortemWithResearch(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	// Write a research artifact (setupAgentsDir creates .agents/research/)
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "research", "topic.md"), []byte("# Research\n\nFindings here."), 0644); err != nil {
		t.Fatalf("write research file: %v", err)
	}

	checker, err := ratchet.NewGateChecker(tmp)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepPreMortem)
	if err != nil {
		t.Fatalf("Check(pre-mortem): %v", err)
	}
	if !result.Passed {
		t.Errorf("pre-mortem gate should pass with research artifact, msg: %s", result.Message)
	}
	if result.Input == "" {
		t.Error("expected non-empty Input path for passed pre-mortem gate")
	}
}

func TestRunRatchetCheck_GateChecker_VibeAlwaysPasses(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	checker, err := ratchet.NewGateChecker(tmp)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepVibe)
	if err != nil {
		t.Fatalf("Check(vibe): %v", err)
	}
	if !result.Passed {
		t.Errorf("vibe gate should always pass (soft gate)")
	}
}

func TestRunRatchetCheck_GateChecker_PostMortemSoftGate(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	checker, err := ratchet.NewGateChecker(tmp)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	result, err := checker.Check(ratchet.StepPostMortem)
	if err != nil {
		t.Fatalf("Check(post-mortem): %v", err)
	}
	if !result.Passed {
		t.Errorf("post-mortem gate should always pass (soft gate)")
	}
}

func TestRunRatchetCheck_GateChecker_AllStepsHaveResults(t *testing.T) {
	tmp := t.TempDir()
	setupAgentsDir(t, tmp)

	checker, err := ratchet.NewGateChecker(tmp)
	if err != nil {
		t.Fatalf("NewGateChecker: %v", err)
	}

	for _, step := range ratchet.AllSteps() {
		t.Run(string(step), func(t *testing.T) {
			result, err := checker.Check(step)
			if err != nil {
				t.Fatalf("Check(%s): %v", step, err)
			}
			if result == nil {
				t.Fatalf("Check(%s) returned nil result", step)
			}
			if result.Step != step {
				t.Errorf("result.Step = %q, want %q", result.Step, step)
			}
			if result.Message == "" {
				t.Errorf("result.Message should not be empty for step %s", step)
			}
		})
	}
}

// checkStepParse mirrors the step-parsing logic from runRatchetCheck.
func checkStepParse(stepName string) error {
	step := ratchet.ParseStep(stepName)
	if step == "" {
		return fmt.Errorf("unknown step: %s", stepName)
	}
	return nil
}
