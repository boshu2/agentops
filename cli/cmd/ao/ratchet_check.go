package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

func init() {
	checkSubCmd := &cobra.Command{
		Use:     "check <step>",
		Aliases: []string{"c"},
		GroupID: "inspection",
		Short:   "Check if step gate is met",
		Long: `Check if prerequisites are satisfied for a workflow step.

Returns exit code 0 if gate passes, 1 if not.

Steps: research, pre-mortem, plan, implement, crank, vibe, post-mortem
Aliases: premortem, postmortem, autopilot, validate, review

Examples:
  ao ratchet check research
  ao ratchet check plan
  ao ratchet check implement || echo "Run /plan first"`,
		Args: cobra.ExactArgs(1),
		RunE: runRatchetCheck,
	}
	ratchetCmd.AddCommand(checkSubCmd)
}

// runRatchetCheck validates a step gate.
func runRatchetCheck(cmd *cobra.Command, args []string) error {
	stepName := args[0]
	step := ratchet.ParseStep(stepName)
	if step == "" {
		return fmt.Errorf("unknown step: %s", stepName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	checker, err := ratchet.NewGateChecker(cwd)
	if err != nil {
		return fmt.Errorf("create gate checker: %w", err)
	}

	result, err := checker.Check(step)
	if err != nil {
		return fmt.Errorf("check gate: %w", err)
	}

	// Output result
	w := cmd.OutOrStdout()
	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	default:
		if result.Passed {
			fmt.Fprintf(w, "GATE PASSED: %s\n", result.Message)
			if result.Input != "" {
				fmt.Fprintf(w, "Input: %s (%s)\n", result.Input, result.Location)
			}
		} else {
			return fmt.Errorf("gate failed: %s", result.Message)
		}
	}

	return nil
}
