// practices: [dora-metrics, continuous-delivery]
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func TestRunRatchetCheckUsesCallerContextAndStreams(t *testing.T) {
	t.Run("pre-canceled context does not launch tracker", func(t *testing.T) {
		resetCommandState(t)
		root := t.TempDir()
		setupAgentsDir(t, root)
		t.Chdir(root)
		marker := filepath.Join(t.TempDir(), "launched")
		installRatchetTracker(t, "br", `printf launched > "$TRACKER_MARKER"`)
		t.Setenv("TRACKER_MARKER", marker)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd := newTraceTestCmd(&bytes.Buffer{})
		cmd.SetContext(ctx)
		err := runRatchetCheck(cmd, []string{"implement"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runRatchetCheck error = %T %v, want context.Canceled", err, err)
		}
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("pre-canceled command launched tracker: %v", err)
		}
	})

	t.Run("stdin and stderr reach tracker while stdout is parsed", func(t *testing.T) {
		resetCommandState(t)
		root := t.TempDir()
		setupAgentsDir(t, root)
		t.Chdir(root)
		installRatchetTracker(t, "br", `
IFS= read -r input
printf 'tracker-stderr:%s\n' "$input" >&2
printf 'epic-stream Stream proof\n'
`)

		var stdout, stderr bytes.Buffer
		cmd := newTraceTestCmd(&stdout)
		cmd.SetIn(strings.NewReader("caller-input\n"))
		cmd.SetErr(&stderr)
		if err := runRatchetCheck(cmd, []string{"implement"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stderr.String(), "tracker-stderr:caller-input") {
			t.Fatalf("tracker stderr = %q", stderr.String())
		}
		if !strings.Contains(stdout.String(), "GATE PASSED") || !strings.Contains(stdout.String(), "epic-stream") {
			t.Fatalf("command output = %q", stdout.String())
		}
	})
}

func installRatchetTracker(t *testing.T, name, body string) {
	t.Helper()
	binDir := t.TempDir()
	tracker := filepath.Join(binDir, name)
	script := "#!/bin/sh\nset -eu\n" + body + "\n"
	if err := os.WriteFile(tracker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOPS_TRACKER", name)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

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
