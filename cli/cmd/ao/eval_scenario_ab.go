// practices: [llm-eval-harness]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
	"github.com/boshu2/agentops/cli/internal/scenario"
)

var (
	evalScenarioABScenario string
	evalScenarioABOutput   string
	evalScenarioABBudget   int
	evalScenarioABTimeout  time.Duration
)

// scenarioABRunnerFactory / scenarioABJudgeFactory are the injectable seam:
// production builds codex-exec-backed implementations; tests override these
// package vars with deterministic fakes (no live models, no hangs).
var (
	scenarioABRunnerFactory = func() aoeval.ScenarioRunner { return codexScenarioRunner{} }
	scenarioABJudgeFactory  = func() aoeval.ScenarioJudge { return codexScenarioJudge{} }
)

var evalScenarioABCmd = &cobra.Command{
	Use:   "scenario-ab",
	Short: "Run a knowledge-reuse holdout scenario with vs. without the gold pull (the discriminating A/B)",
	Long: `Run one holdout scenario (scenario.v1) twice — a control arm WITHOUT the gold
pull and a treatment arm WITH it — grade each arm with a cross-family judge,
and emit a ScenarioDeltaScorecard.

The gate is DETERMINISTIC over the (stochastic) judge output: it FAILS LOUDLY
(non-zero exit) when the treatment did not beat the control (delta <= 0 — the
ADR-0002 spray returning), when the treatment misses the scenario's
satisfaction threshold, or when the summed arm token cost exceeds the budget.`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(evalScenarioABScenario) == "" {
			return fmt.Errorf("--scenario <path> is required")
		}
		sc, err := scenario.Load(evalScenarioABScenario)
		if err != nil {
			return err
		}
		card, err := aoeval.RunScenarioAB(cmd.Context(), aoeval.ScenarioABOptions{
			Scenario:     *sc,
			ScenarioPath: evalScenarioABScenario,
			Runner:       scenarioABRunnerFactory(),
			Judge:        scenarioABJudgeFactory(),
			Timeout:      evalScenarioABTimeout,
			TokenBudget:  evalScenarioABBudget,
		})
		if err != nil {
			return err
		}
		if err := aoeval.WriteScenarioDeltaScorecard(card, evalScenarioABOutput); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"scenario-ab %s: delta=%.4f (with=%.4f without=%.4f) tokens=%d gate=%s\n",
			card.ScenarioID, card.AggregateDelta, card.With.Score, card.Without.Score,
			card.With.TokenCost+card.Without.TokenCost, gateLabel(card.Gate.Pass))
		if !card.Gate.Pass {
			for _, r := range card.Gate.Reasons {
				fmt.Fprintf(cmd.ErrOrStderr(), "  FAIL: %s\n", r)
			}
			return fmt.Errorf("scenario-ab gate failed for %s", card.ScenarioID)
		}
		return nil
	},
}

func gateLabel(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func init() {
	evalScenarioABCmd.Flags().StringVar(&evalScenarioABScenario, "scenario", "", "Path to the scenario.v1 JSON file (required)")
	evalScenarioABCmd.Flags().StringVar(&evalScenarioABOutput, "output", "", "Write the ScenarioDeltaScorecard JSON to this path")
	evalScenarioABCmd.Flags().IntVar(&evalScenarioABBudget, "token-budget", 0, "Fail the gate if summed arm token cost exceeds this (0 = default 200000)")
	evalScenarioABCmd.Flags().DurationVar(&evalScenarioABTimeout, "timeout", 0, "Per-arm timeout (0 = default 5m)")
	evalCmd.AddCommand(evalScenarioABCmd)
}

// --- codex-exec-backed defaults (Law 0: codex exec only; no Claude print-mode) -
// These are the live path; they are exercised end-to-end in KF-C, not in this
// slice's tests (which inject fakes via the factory seam above).

type codexScenarioRunner struct{}

func (codexScenarioRunner) RunArm(ctx context.Context, sc scenario.Scenario, withGold bool) (aoeval.ArmOutcome, error) {
	var prompt strings.Builder
	if withGold {
		// The treatment arm consumes the KF-A decision-point gold pull.
		if pointers := goldPointers(ctx, sc.Goal); pointers != "" {
			prompt.WriteString("Relevant prior knowledge (gold corpus pointers):\n")
			prompt.WriteString(pointers)
			prompt.WriteString("\n\n")
		}
	}
	prompt.WriteString(sc.Narrative)
	prompt.WriteString("\n\nGoal: ")
	prompt.WriteString(sc.Goal)
	prompt.WriteString("\n\nProduce your best attempt. Output only the result, no preamble.")
	out, tokens, err := runCodexExec(ctx, prompt.String())
	if err != nil {
		return aoeval.ArmOutcome{}, err
	}
	return aoeval.ArmOutcome{Output: out, TokenCost: tokens}, nil
}

type codexScenarioJudge struct{}

func (codexScenarioJudge) Judge(ctx context.Context, sc scenario.Scenario, arm aoeval.ScenarioArm, outcome aoeval.ArmOutcome) (aoeval.JudgeVerdict, error) {
	var prompt strings.Builder
	prompt.WriteString("You are a strict cross-family judge. Grade the OUTPUT against the acceptance vectors.\n")
	prompt.WriteString("Goal: " + sc.Goal + "\n")
	prompt.WriteString("Expected outcome: " + sc.ExpectedOutcome + "\n")
	prompt.WriteString("Acceptance vectors:\n")
	for _, v := range sc.AcceptanceVectors {
		fmt.Fprintf(&prompt, "- %s (threshold %.2f)\n", v.Dimension, v.Threshold)
	}
	prompt.WriteString("\nOUTPUT:\n" + outcome.Output + "\n\n")
	prompt.WriteString(`Return ONLY strict JSON, no prose: {"vectors":[{"dimension":"...","pass":true,"score":0.0}],"aggregate_score":0.0}. aggregate_score is in [0,1].`)
	out, _, err := runCodexExec(ctx, prompt.String())
	if err != nil {
		return aoeval.JudgeVerdict{}, err
	}
	verdict, err := parseJudgeJSON(out)
	if err != nil {
		return aoeval.JudgeVerdict{}, fmt.Errorf("judge (%s) returned unparseable output: %w", arm, err)
	}
	return verdict, nil
}

// goldPointers shells the KF-A decision-point pull. Best-effort: a missing gold
// wiki yields no pointers (the control/treatment difference simply narrows).
func goldPointers(ctx context.Context, query string) string {
	if _, err := os.Stat(".ao/wiki"); err != nil {
		return ""
	}
	cmd := exec.CommandContext(ctx, "ao", "lookup", "--query", query, "--gold", "--pointers", "--limit", "3")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var codexTokensRe = regexp.MustCompile(`(?i)tokens used[^\d]*([\d,]+)`)

// runCodexExec runs a single non-interactive codex turn and returns stdout plus
// a best-effort token count (parsed from codex's "tokens used" line, else
// estimated from output length). Law 0: codex exec only; no Claude print-mode.
func runCodexExec(ctx context.Context, prompt string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "codex", "exec", "--sandbox", "read-only", "--skip-git-repo-check", prompt)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		return "", 0, fmt.Errorf("codex exec: %w", err)
	}
	return text, parseCodexTokens(text), nil
}

// parseJudgeJSON extracts the first JSON object from the judge's output (codex
// may wrap it in prose) and decodes it into a JudgeVerdict.
func parseJudgeJSON(text string) (aoeval.JudgeVerdict, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
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
	if m := codexTokensRe.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", "")); err == nil {
			return n
		}
	}
	// Fallback estimate: ~4 chars/token.
	return len(text) / 4
}
