// practices: [llm-eval-harness]
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/scenario"
)

const (
	agenticMaxTurns       = 8
	agenticStepSchemaJSON = `{"type":"object","additionalProperties":false,"properties":{"commands":{"type":"array","items":{"type":"string"}},"done":{"type":"boolean"},"summary":{"type":"string"}},"required":["commands","done","summary"]}`
)

// agenticStep is one structured turn from the worker model.
type agenticStep struct {
	Commands []string `json:"commands"`
	Done     bool     `json:"done"`
	Summary  string   `json:"summary"`
}

// workspaceCommandRunner executes shell commands inside the isolated workspace.
type workspaceCommandRunner func(ctx context.Context, workDir, command string) (stdout string, exitCode int, err error)

// agenticRunnerHooks is the injectable seam for tests (no live codex).
var agenticRunnerHooks = struct {
	runCodex func(ctx context.Context, prompt, schemaPath string) (string, int, error)
	runCmd   workspaceCommandRunner
}{
	runCodex: runCodexExec,
	runCmd:   defaultWorkspaceCommandRunner,
}

type agenticScenarioRunner struct{}

func (agenticScenarioRunner) RunArm(ctx context.Context, sc scenario.Scenario, withGold bool) (aoeval.ArmOutcome, error) {
	workDir, err := os.MkdirTemp("", "scenario-ab-agentic-*")
	if err != nil {
		return aoeval.ArmOutcome{}, fmt.Errorf("agentic workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	schemaPath, cleanup, err := writeAgenticStepSchema()
	if err != nil {
		return aoeval.ArmOutcome{}, err
	}
	defer cleanup()

	var history strings.Builder
	totalTokens := 0
	for turn := 0; turn < agenticMaxTurns; turn++ {
		prompt := buildAgenticPrompt(ctx, sc, withGold, workDir, history.String())
		out, tokens, err := agenticRunnerHooks.runCodex(ctx, prompt, schemaPath)
		totalTokens += tokens
		if err != nil {
			return aoeval.ArmOutcome{}, fmt.Errorf("agentic turn %d: %w", turn+1, err)
		}
		step, err := parseAgenticStep(out)
		if err != nil {
			return aoeval.ArmOutcome{}, fmt.Errorf("agentic turn %d parse: %w", turn+1, err)
		}
		for _, command := range step.Commands {
			command = strings.TrimSpace(command)
			if command == "" {
				continue
			}
			stdout, exitCode, runErr := agenticRunnerHooks.runCmd(ctx, workDir, command)
			if runErr != nil {
				return aoeval.ArmOutcome{}, fmt.Errorf("agentic command %q: %w", command, runErr)
			}
			fmt.Fprintf(&history, "$ %s\nexit=%d\n%s\n\n", command, exitCode, strings.TrimSpace(stdout))
		}
		if step.Done {
			summary := strings.TrimSpace(step.Summary)
			if summary == "" {
				summary = strings.TrimSpace(history.String())
			}
			artifactSummary, _ := summarizeWorkspace(workDir)
			if artifactSummary != "" {
				summary = summary + "\n\nWorkspace artifacts:\n" + artifactSummary
			}
			return aoeval.ArmOutcome{Output: summary, TokenCost: totalTokens}, nil
		}
	}
	return aoeval.ArmOutcome{}, fmt.Errorf("agentic runner exceeded %d turns without done=true", agenticMaxTurns)
}

func buildAgenticPrompt(ctx context.Context, sc scenario.Scenario, withGold bool, workDir, history string) string {
	var p strings.Builder
	p.WriteString("You are an agentic worker in an isolated workspace. Execute the task by running shell commands in the workspace directory.\n")
	p.WriteString("Return ONLY strict JSON matching the schema: commands (shell lines to run now), done (true when finished), summary (final result for grading when done=true).\n\n")
	if withGold {
		if pointers := goldPointers(ctx, sc.Goal); pointers != "" {
			p.WriteString("Relevant prior knowledge (gold corpus pointers):\n")
			p.WriteString(pointers)
			p.WriteString("\n\n")
		}
	}
	p.WriteString(sc.Narrative)
	p.WriteString("\n\nGoal: ")
	p.WriteString(sc.Goal)
	if strings.TrimSpace(sc.ExpectedOutcome) != "" {
		p.WriteString("\nExpected outcome: ")
		p.WriteString(sc.ExpectedOutcome)
	}
	p.WriteString("\n\nWorkspace: ")
	p.WriteString(workDir)
	p.WriteString("\nRun commands relative to this directory. Do not read outside the workspace.\n")
	if history != "" {
		p.WriteString("\nPrior command history:\n")
		p.WriteString(history)
	}
	return p.String()
}

func writeAgenticStepSchema() (string, func(), error) {
	f, err := os.CreateTemp("", "scenario-ab-agentic-schema-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("agentic schema temp: %w", err)
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := f.WriteString(agenticStepSchemaJSON); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write agentic schema: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close agentic schema: %w", err)
	}
	return name, cleanup, nil
}

func parseAgenticStep(text string) (agenticStep, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return agenticStep{}, fmt.Errorf("no JSON object found")
	}
	var step agenticStep
	if err := json.Unmarshal([]byte(text[start:end+1]), &step); err != nil {
		return agenticStep{}, err
	}
	return step, nil
}

func defaultWorkspaceCommandRunner(ctx context.Context, workDir, command string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return "", 0, err
		}
	}
	return string(out), exitCode, nil
}

func summarizeWorkspace(workDir string) (string, error) {
	var lines []string
	_ = filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(workDir, path)
		if relErr != nil || strings.HasPrefix(rel, ".") {
			return nil
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil
		}
		lines = append(lines, fmt.Sprintf("- %s (%d bytes)", rel, info.Size()))
		return nil
	})
	return strings.Join(lines, "\n"), nil
}

func selectScenarioRunner(sc scenario.Scenario) aoeval.ScenarioRunner {
	if sc.EffectiveRunnerMode() == scenario.RunnerModeAgentic {
		return agenticScenarioRunner{}
	}
	return codexScenarioRunner{}
}
