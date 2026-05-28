// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var scenarioInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .agents/holdout/ directory for scenario storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		holdoutDir := filepath.Join(".agents", "holdout")
		if err := os.MkdirAll(holdoutDir, 0755); err != nil {
			return fmt.Errorf("creating holdout directory: %w", err)
		}

		readmePath := filepath.Join(holdoutDir, "README.md")
		if _, err := os.Stat(readmePath); os.IsNotExist(err) {
			readme := `# Holdout Scenarios

This directory contains behavioral validation scenarios that implementing
agents cannot see. Access is blocked by the holdout-isolation-gate hook.

Evaluator agents (with AGENTOPS_HOLDOUT_EVALUATOR=1) can read these files
during validation STEP 1.8.

## Schema

Scenarios follow schemas/scenario.v1.schema.json.

## Usage

` + "```" + `bash
ao scenario init      # Create this directory
ao scenario list      # List active scenarios
ao scenario validate  # Validate schema compliance
` + "```" + `
`
			if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
				return fmt.Errorf("writing README: %w", err)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Initialized holdout directory at %s\n", holdoutDir)
		return nil
	},
}

func init() {
	scenarioCmd.AddCommand(scenarioInitCmd)
}
