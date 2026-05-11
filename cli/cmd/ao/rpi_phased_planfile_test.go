// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsPlanFileEpic(t *testing.T) {
	tests := []struct {
		epicID string
		want   bool
	}{
		{"plan:.agents/plans/my-plan.md", true},
		{"plan:foo", true},
		{"ag-abc", false},
		{"", false},
		{"planner-123", false},
	}
	for _, tt := range tests {
		t.Run(tt.epicID, func(t *testing.T) {
			if got := isPlanFileEpic(tt.epicID); got != tt.want {
				t.Errorf("isPlanFileEpic(%q) = %v, want %v", tt.epicID, got, tt.want)
			}
		})
	}
}

func TestPlanFileFromEpic(t *testing.T) {
	tests := []struct {
		epicID string
		want   string
	}{
		{"plan:.agents/plans/my-plan.md", ".agents/plans/my-plan.md"},
		{"plan:foo", "foo"},
		{"ag-abc", "ag-abc"}, // no prefix → returns full string (TrimPrefix is no-op)
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.epicID, func(t *testing.T) {
			if got := planFileFromEpic(tt.epicID); got != tt.want {
				t.Errorf("planFileFromEpic(%q) = %q, want %q", tt.epicID, got, tt.want)
			}
		})
	}
}

func TestDiscoverPlanFile(t *testing.T) {
	t.Run("finds latest md file", func(t *testing.T) {
		tmp := t.TempDir()
		plansDir := filepath.Join(tmp, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write two plan files with different mod times
		older := filepath.Join(plansDir, "old-plan.md")
		if err := os.WriteFile(older, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		// Ensure different mod time
		oldTime := time.Now().Add(-1 * time.Hour)
		if err := os.Chtimes(older, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}

		newer := filepath.Join(plansDir, "new-plan.md")
		if err := os.WriteFile(newer, []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}

		got, err := discoverPlanFile(tmp)
		if err != nil {
			t.Fatalf("discoverPlanFile() error = %v", err)
		}
		want := filepath.Join(".agents", "plans", "new-plan.md")
		if got != want {
			t.Errorf("discoverPlanFile() = %q, want %q", got, want)
		}
	})

	t.Run("empty dir returns error", func(t *testing.T) {
		tmp := t.TempDir()
		plansDir := filepath.Join(tmp, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}

		_, err := discoverPlanFile(tmp)
		if err == nil {
			t.Error("expected error for empty plans dir")
		}
	})

	t.Run("missing dir returns error", func(t *testing.T) {
		tmp := t.TempDir()
		_, err := discoverPlanFile(tmp)
		if err == nil {
			t.Error("expected error for missing plans dir")
		}
	})

	t.Run("ignores non-md files", func(t *testing.T) {
		tmp := t.TempDir()
		plansDir := filepath.Join(tmp, ".agents", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Write a non-md file only
		if err := os.WriteFile(filepath.Join(plansDir, "notes.txt"), []byte("txt"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := discoverPlanFile(tmp)
		if err == nil {
			t.Error("expected error when only non-md files present")
		}
	})

	t.Run("ignores subdirectories", func(t *testing.T) {
		tmp := t.TempDir()
		plansDir := filepath.Join(tmp, ".agents", "plans")
		if err := os.MkdirAll(filepath.Join(plansDir, "subdir.md"), 0755); err != nil {
			t.Fatal(err)
		}

		_, err := discoverPlanFile(tmp)
		if err == nil {
			t.Error("expected error when only subdirectory named .md present")
		}
	})
}

func TestBuildPromptForPhase_PlanFileMode(t *testing.T) {
	state := &phasedState{
		Goal:   "test goal",
		EpicID: "plan:.agents/plans/my-plan.md",
		Opts:   defaultPhasedEngineOptions(),
	}

	t.Run("phase 2 contains plan file path", func(t *testing.T) {
		prompt, err := buildPromptForPhase("", 2, state, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase() error = %v", err)
		}
		if !strings.Contains(prompt, "PLAN-FILE MODE") {
			t.Error("phase 2 prompt missing PLAN-FILE MODE header")
		}
		if !strings.Contains(prompt, "/crank .agents/plans/my-plan.md") {
			t.Errorf("phase 2 prompt missing plan path in /crank, got:\n%s", prompt)
		}
		// Should NOT contain the raw sentinel
		if strings.Contains(prompt, "/crank plan:") {
			t.Error("phase 2 prompt should not contain raw plan: sentinel in /crank")
		}
	})

	t.Run("phase 3 uses --quick recent for post-mortem", func(t *testing.T) {
		prompt, err := buildPromptForPhase("", 3, state, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase() error = %v", err)
		}
		if !strings.Contains(prompt, "/post-mortem --quick recent") {
			t.Errorf("phase 3 prompt missing '/post-mortem --quick recent', got:\n%s", prompt)
		}
	})

	t.Run("non-plan-file epic uses epic ID directly", func(t *testing.T) {
		normalState := &phasedState{
			Goal:   "test goal",
			EpicID: "ag-abc",
			Opts:   defaultPhasedEngineOptions(),
		}
		prompt, err := buildPromptForPhase("", 2, normalState, nil)
		if err != nil {
			t.Fatalf("buildPromptForPhase() error = %v", err)
		}
		if strings.Contains(prompt, "PLAN-FILE MODE") {
			t.Error("non-plan-file epic should not have PLAN-FILE MODE")
		}
		if !strings.Contains(prompt, "/crank ag-abc") {
			t.Errorf("expected /crank ag-abc in prompt, got:\n%s", prompt)
		}
	})
}

func TestProcessImplementationPhase_PlanFileSkipsBdCheck(t *testing.T) {
	tmp := t.TempDir()
	state := &phasedState{
		EpicID:     "plan:.agents/plans/my-plan.md",
		StartPhase: 2, // skip validatePriorPhaseResult
		Opts:       defaultPhasedEngineOptions(),
		Verdicts:   make(map[string]string),
		Attempts:   make(map[string]int),
	}

	logPath := filepath.Join(tmp, "orchestration.log")
	err := processImplementationPhase(tmp, state, 2, logPath)
	if err != nil {
		t.Errorf("processImplementationPhase() with plan-file epic should return nil, got: %v", err)
	}
}

func TestBuildRetryPrompt_PlanFileMode(t *testing.T) {
	state := &phasedState{
		Goal:   "test goal",
		EpicID: "plan:.agents/plans/my-plan.md",
		Opts:   defaultPhasedEngineOptions(),
	}
	retryCtx := &retryContext{
		Attempt: 1,
		Verdict: "FAIL",
		Findings: []finding{
			{Description: "test finding", Fix: "fix it", Ref: "file.go:1"},
		},
	}

	prompt, err := buildRetryPrompt("", 3, state, retryCtx)
	if err != nil {
		t.Fatalf("buildRetryPrompt() error = %v", err)
	}
	if !strings.Contains(prompt, "/crank .agents/plans/my-plan.md") {
		t.Errorf("retry prompt missing plan path in /crank, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "/crank plan:") {
		t.Error("retry prompt should not contain raw plan: sentinel in /crank")
	}
}
