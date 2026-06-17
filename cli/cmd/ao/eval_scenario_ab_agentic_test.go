package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/scenario"
)

func withAgenticHooks(t *testing.T, steps []agenticStep) {
	t.Helper()
	orig := agenticRunnerHooks
	call := 0
	agenticRunnerHooks.runCodex = func(_ context.Context, _ string, _ string) (string, int, error) {
		idx := call
		if len(steps) == 0 {
			return "", 0, context.Canceled
		}
		if idx >= len(steps) {
			idx = len(steps) - 1
		}
		step := steps[idx]
		call++
		data, err := json.Marshal(step)
		if err != nil {
			return "", 0, err
		}
		return string(data), 10, nil
	}
	agenticRunnerHooks.runCmd = func(_ context.Context, workDir, command string) (string, int, error) {
		if strings.Contains(command, "touch") {
			name := strings.TrimSpace(strings.TrimPrefix(command, "touch"))
			if err := os.WriteFile(filepath.Join(workDir, name), []byte("ok\n"), 0o644); err != nil {
				return "", 1, err
			}
			return "", 0, nil
		}
		return "", 0, nil
	}
	t.Cleanup(func() {
		agenticRunnerHooks = orig
	})
}

func TestAgenticScenarioRunnerProducesWorkspaceArtifact(t *testing.T) {
	withAgenticHooks(t, []agenticStep{
		{Commands: []string{"touch result.txt"}, Done: true, Summary: "created result.txt in workspace"},
	})
	sc := scenario.Scenario{
		ID:      "s-agentic-test",
		Goal:    "create a file named result.txt",
		Narrative: "Ship a file in the workspace.",
	}
	out, err := agenticScenarioRunner{}.RunArm(context.Background(), sc, false)
	if err != nil {
		t.Fatalf("RunArm: %v", err)
	}
	if !strings.Contains(out.Output, "result.txt") {
		t.Errorf("output = %q, want result.txt mention", out.Output)
	}
}

func TestSelectScenarioRunnerAgentic(t *testing.T) {
	sc := scenario.Scenario{RunnerMode: "agentic"}
	if _, ok := selectScenarioRunner(sc).(agenticScenarioRunner); !ok {
		t.Fatalf("selectScenarioRunner(agentic) = %T, want agenticScenarioRunner", selectScenarioRunner(sc))
	}
}

func TestSelectScenarioRunnerOneShotDefault(t *testing.T) {
	sc := scenario.Scenario{}
	if _, ok := selectScenarioRunner(sc).(codexScenarioRunner); !ok {
		t.Fatalf("selectScenarioRunner(default) = %T, want codexScenarioRunner", selectScenarioRunner(sc))
	}
}

func TestScenarioEffectiveRunnerMode(t *testing.T) {
	if got := (scenario.Scenario{}).EffectiveRunnerMode(); got != scenario.RunnerModeOneShot {
		t.Errorf("default = %q", got)
	}
	if got := (scenario.Scenario{RunnerMode: "agentic"}).EffectiveRunnerMode(); got != scenario.RunnerModeAgentic {
		t.Errorf("agentic = %q", got)
	}
}

func TestAgenticRunnerUsedInScenarioABHarness(t *testing.T) {
	withAgenticHooks(t, []agenticStep{
		{Commands: []string{"touch gate.txt"}, Done: true, Summary: "gate artifact ready"},
	})
	withFakes(t,
		agenticScenarioRunner{},
		fakeCmdJudge{
			with:    aoeval.JudgeVerdict{AggregateScore: 0.9},
			without: aoeval.JudgeVerdict{AggregateScore: 0.3},
		},
	)
	dir := t.TempDir()
	path := filepath.Join(dir, "agentic.json")
	body := `{"id":"s-agentic-001","version":1,"date":"2026-06-18","goal":"create gate.txt","narrative":"execution required","expected_outcome":"gate.txt exists","acceptance_vectors":[{"dimension":"artifact","threshold":0.8}],"satisfaction_threshold":0.8,"runner_mode":"agentic","status":"active"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "card.json")
	stdout, err := runScenarioABCmd(t, path, out, 200000)
	if err != nil {
		t.Fatalf("scenario-ab with agentic runner: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "gate=PASS") {
		t.Errorf("stdout = %q", stdout)
	}
}
