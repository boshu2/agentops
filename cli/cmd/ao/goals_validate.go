// practices: [dora-metrics, lean-startup]
package main

import (
	"os"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

// validateResult is a type alias for goals.ValidateResult (used by tests).
type validateResult = goals.ValidateResult

var goalsValidateCmd = &cobra.Command{
	Use:     "validate",
	Aliases: []string{"v"},
	Short:   "Validate GOALS.yaml structure and wiring",
	GroupID: "measurement",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunValidate(goals.ValidateOptions{
			GoalsFile: resolveGoalsFile(),
			JSON:      goalsJSONOutput(),
		})
	},
}

// outputValidateResult delegates to goals.OutputValidateResult (used by tests).
func outputValidateResult(result validateResult) error {
	return goals.OutputValidateResult(os.Stdout, goalsJSONOutput(), result)
}

func init() {
	goalsCmd.AddCommand(goalsValidateCmd)
}
