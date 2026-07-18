package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

func withAgenticHooks(t *testing.T, steps []agenticStep) {
	t.Helper()
	orig := agenticRunnerHooks
	call := 0
	agenticRunnerHooks.runCodex = func(_ context.Context, _ string, _ string, _ bool) (string, int, error) {
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
		ID:        "s-agentic-test",
		Goal:      "create a file named result.txt",
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
