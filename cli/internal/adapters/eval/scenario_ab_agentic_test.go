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
	agenticRunnerHooks.runCmd = func(_ context.Context, workDir, command string, _ bool) (string, int, error) {
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

// TestAgenticRunnerRedactsCommandOutputBeforePrompt (age-6j9ee.3): raw worker-command
// stdout is model-bound — it accumulates into `history`, which is fed into the NEXT
// turn's prompt AND (on done with an empty model summary) into the arm output the judge
// grades. Secrets emitted by a workspace command must be scrubbed at the egress choke
// point, so neither the next-turn prompt nor the graded output carries them.
func TestAgenticRunnerRedactsCommandOutputBeforePrompt(t *testing.T) {
	orig := agenticRunnerHooks
	t.Cleanup(func() { agenticRunnerHooks = orig })

	const awsKey = "AKIA1234567890ABCDEF"
	const ghToken = "ghp_0123456789012345678901234567890123456789"

	var prompts []string
	call := 0
	agenticRunnerHooks.runCodex = func(_ context.Context, prompt string, _ string, _ bool) (string, int, error) {
		prompts = append(prompts, prompt)
		defer func() { call++ }()
		if call == 0 {
			// Turn 1: emit a command whose (faked) output carries secrets.
			return `{"commands":["leak-secrets"],"done":false,"summary":""}`, 1, nil
		}
		// Turn 2: finish with an empty summary so the arm output falls back to history.
		return `{"commands":[],"done":true,"summary":""}`, 1, nil
	}
	agenticRunnerHooks.runCmd = func(_ context.Context, _ string, _ string, _ bool) (string, int, error) {
		return "leaked AWS_SECRET " + awsKey + " and github " + ghToken, 0, nil
	}

	sc := scenario.Scenario{ID: "s-redact", Goal: "do work", Narrative: "n"}
	out, err := (agenticScenarioRunner{}).RunArm(context.Background(), sc, false)
	if err != nil {
		t.Fatalf("RunArm: %v", err)
	}

	if len(prompts) < 2 {
		t.Fatalf("expected >=2 turns, got %d", len(prompts))
	}
	turn2 := prompts[1]
	if strings.Contains(turn2, awsKey) || strings.Contains(turn2, ghToken) {
		t.Fatalf("next-turn prompt leaked raw secrets:\n%s", turn2)
	}
	if !strings.Contains(turn2, "[REDACTED]") {
		t.Fatalf("next-turn prompt must contain [REDACTED]; got:\n%s", turn2)
	}
	if strings.Contains(out.Output, awsKey) || strings.Contains(out.Output, ghToken) {
		t.Fatalf("graded arm output leaked raw secrets: %q", out.Output)
	}
	if !strings.Contains(out.Output, "[REDACTED]") {
		t.Fatalf("graded arm output must carry redacted history [REDACTED]; got: %q", out.Output)
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
