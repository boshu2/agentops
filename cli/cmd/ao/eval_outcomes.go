package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
	"github.com/spf13/cobra"
)

// compileOutcomesRubric projects a locked eval Task + its grading criteria into a
// holdout-safe Outcomes rubric, then re-scans the result (defense-in-depth guard
// layer 3) against the run's holdout answer values. It REFUSES to return a rubric
// that would carry any holdout value across the cloud boundary — Managed Agents
// are not ZDR, so a leak is permanent. ProjectRubric already strips ground truth
// by construction; this re-scan is the deny-by-default backstop.
func compileOutcomesRubric(task evalsubstrate.Task, criteria []evalsubstrate.Criterion, judgeContentHash string, holdoutValues []string) (evalsubstrate.Rubric, error) {
	r := evalsubstrate.ProjectRubric(task, criteria, judgeContentHash)
	if hit, found := r.ContainsAny(holdoutValues); found {
		return evalsubstrate.Rubric{}, fmt.Errorf("outcomes compile: holdout value %q would leak into the rubric payload; refusing (Managed Agents are not ZDR)", hit)
	}
	return r, nil
}

// outcomesCompileInput is the JSON shape accepted by `ao eval outcomes compile`.
// holdout_values feeds the re-scan guard and is NEVER copied into the output.
type outcomesCompileInput struct {
	Task             evalsubstrate.Task        `json:"task"`
	Criteria         []evalsubstrate.Criterion `json:"criteria"`
	JudgeContentHash string                    `json:"judge_content_hash"`
	HoldoutValues    []string                  `json:"holdout_values,omitempty"`
}

var evalOutcomesCmd = &cobra.Command{
	Use:   "outcomes",
	Short: "Project the locked eval substrate into Outcomes grading payloads (holdout-safe)",
	Long: "Outcomes is a derived projection of the locked eval substrate (SCHEMA.md), " +
		"never an alternate authority. Subcommands compile holdout-safe rubric payloads " +
		"and ingest returned scores into the one verdict format.",
}

var evalOutcomesCompileCmd = &cobra.Command{
	Use:   "compile <input.json>",
	Short: "Compile a holdout-safe Outcomes rubric payload from a locked Task + criteria",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvalOutcomesCompile,
}

func runEvalOutcomesCompile(cmd *cobra.Command, args []string) error {
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read %s: %w", args[0], err)
	}
	var in outcomesCompileInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("parse %s: %w", args[0], err)
	}
	rubric, err := compileOutcomesRubric(in.Task, in.Criteria, in.JudgeContentHash, in.HoldoutValues)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(rubric, "", "  ")
	if err != nil {
		return fmt.Errorf("encode rubric: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func init() {
	evalOutcomesCmd.AddCommand(evalOutcomesCompileCmd)
	evalCmd.AddCommand(evalOutcomesCmd)
}
