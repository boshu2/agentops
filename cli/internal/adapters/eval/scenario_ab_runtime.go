package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/scenario"
)

func (Runtime) LoadScenario(path string) (*scenario.Scenario, error) { return scenario.Load(path) }
func (Runtime) Runner(spec scenario.Scenario) aoeval.ScenarioRunner {
	return selectScenarioRunner(spec)
}
func (Runtime) Judge(scenario.Scenario) aoeval.ScenarioJudge { return codexScenarioJudge{} }
func (Runtime) WriteScenarioCard(card aoeval.ScenarioDeltaScorecard, path string) error {
	return aoeval.WriteScenarioDeltaScorecard(card, path)
}
func (Runtime) LoadScenarioCard(path string) (aoeval.ScenarioDeltaScorecard, error) {
	return aoeval.LoadScenarioDeltaScorecard(path)
}
func (Runtime) WriteMoatResult(path string, result aoeval.MoatClaimResult) error {
	return aoeval.WriteMoatClaimResult(path, result)
}

type codexScenarioRunner struct{}

func (codexScenarioRunner) RunArm(ctx context.Context, spec scenario.Scenario, withGold bool) (aoeval.ArmOutcome, error) {
	var prompt strings.Builder
	if withGold {
		if pointers := goldPointers(ctx, spec.Goal); pointers != "" {
			prompt.WriteString("Relevant prior knowledge (gold corpus pointers):\n")
			prompt.WriteString(pointers)
			prompt.WriteString("\n\n")
		}
	}
	prompt.WriteString(spec.Narrative)
	prompt.WriteString("\n\nGoal: ")
	prompt.WriteString(spec.Goal)
	prompt.WriteString("\n\nProduce your best attempt. Output only the result, no preamble.")
	output, tokens, err := runCodexExec(ctx, prompt.String(), "")
	if err != nil {
		return aoeval.ArmOutcome{}, err
	}
	return aoeval.ArmOutcome{Output: output, TokenCost: tokens}, nil
}

type codexScenarioJudge struct{}

func (codexScenarioJudge) Judge(ctx context.Context, spec scenario.Scenario, arm aoeval.ScenarioArm, outcome aoeval.ArmOutcome) (aoeval.JudgeVerdict, error) {
	schema, cleanup, err := writeJudgeSchema()
	if err != nil {
		return aoeval.JudgeVerdict{}, err
	}
	defer cleanup()
	output, _, err := runCodexExec(ctx, taskSuccessJudgePrompt(spec, outcome.Output), schema)
	if err != nil {
		return aoeval.JudgeVerdict{}, err
	}
	verdict, err := parseJudgeJSON(output)
	if err != nil {
		return aoeval.JudgeVerdict{}, fmt.Errorf("judge (%s) returned unparseable output: %w", arm, err)
	}
	return verdict, nil
}

func taskSuccessJudgePrompt(spec scenario.Scenario, output string) string {
	var prompt strings.Builder
	prompt.WriteString("You are a strict, fair judge. Grade ONLY whether the OUTPUT below accomplishes the task — score 0.0 to 1.0 (1.0 = the goal is fully achieved and the expected outcome met; 0.0 = not at all). Judge this OUTPUT on its own merits; do NOT compare it to any other answer.\n\n")
	prompt.WriteString("Goal: " + spec.Goal + "\n")
	if strings.TrimSpace(spec.ExpectedOutcome) != "" {
		prompt.WriteString("Expected outcome: " + spec.ExpectedOutcome + "\n")
	}
	prompt.WriteString("\nOUTPUT:\n" + output + "\n\n")
	prompt.WriteString(`Return ONLY strict JSON, no prose: {"vectors":[{"dimension":"task-success","pass":true,"score":0.0}],"aggregate_score":0.0}. Set pass=true iff score>=0.5; aggregate_score in [0,1] is your task-success grade.`)
	return prompt.String()
}

const judgeOutputSchema = `{"type":"object","additionalProperties":false,"properties":{"vectors":{"type":"array","items":{"type":"object","additionalProperties":false,"properties":{"dimension":{"type":"string"},"pass":{"type":"boolean"},"score":{"type":"number"}},"required":["dimension","pass","score"]}},"aggregate_score":{"type":"number"}},"required":["vectors","aggregate_score"]}`

func writeJudgeSchema() (string, func(), error) {
	file, err := os.CreateTemp("", "scenario-ab-judge-schema-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("judge schema temp: %w", err)
	}
	name := file.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := file.WriteString(judgeOutputSchema); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write judge schema: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close judge schema: %w", err)
	}
	return name, cleanup, nil
}

func goldPointers(ctx context.Context, query string) string {
	if _, err := os.Stat(".ao/wiki"); err != nil {
		return ""
	}
	command := exec.CommandContext(ctx, "ao", "lookup", "--query", query, "--gold", "--pointers", "--limit", "3")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

var codexTokensRe = regexp.MustCompile(`(?i)tokens used[^\d]*([\d,]+)`)

func runCodexExec(ctx context.Context, prompt, outputSchemaPath string) (string, int, error) {
	messageFile, err := os.CreateTemp("", "scenario-ab-codex-msg-*.txt")
	if err != nil {
		return "", 0, fmt.Errorf("codex last-message temp: %w", err)
	}
	messagePath := messageFile.Name()
	_ = messageFile.Close()
	defer func() { _ = os.Remove(messagePath) }()
	args := codexExecArgs(messagePath, outputSchemaPath, prompt)
	cwd, err := os.Getwd()
	if err != nil {
		return "", 0, fmt.Errorf("resolve cwd for arm isolation: %w", err)
	}
	command, err := sandboxedCodexCmd(ctx, corpusDenyPaths(cwd), args)
	if err != nil {
		return "", 0, err
	}
	command.Stdin = strings.NewReader("")
	combined, err := command.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("codex exec: %w", err)
	}
	tokens := parseCodexTokens(string(combined))
	if message, readErr := os.ReadFile(messagePath); readErr == nil && strings.TrimSpace(string(message)) != "" {
		return strings.TrimSpace(string(message)), tokens, nil
	}
	return string(combined), tokens, nil
}
func codexExecArgs(messagePath, outputSchemaPath, prompt string) []string {
	args := []string{"exec", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "--output-last-message", messagePath}
	if outputSchemaPath != "" {
		args = append(args, "--output-schema", outputSchemaPath)
	}
	return append(args, prompt)
}
func parseJudgeJSON(text string) (aoeval.JudgeVerdict, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return aoeval.JudgeVerdict{}, fmt.Errorf("no JSON object found")
	}
	var verdict aoeval.JudgeVerdict
	if err := json.Unmarshal([]byte(text[start:end+1]), &verdict); err != nil {
		return aoeval.JudgeVerdict{}, err
	}
	return verdict, nil
}
func parseCodexTokens(text string) int {
	if match := codexTokensRe.FindStringSubmatch(text); match != nil {
		if tokens, err := strconv.Atoi(strings.ReplaceAll(match[1], ",", "")); err == nil {
			return tokens
		}
	}
	return len(text) / 4
}
