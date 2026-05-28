// practices: [property-based-testing, llm-eval-harness]
package main

import "github.com/spf13/cobra"

var scenarioCmd = &cobra.Command{
	Use:   "scenario",
	Short: "Manage holdout scenarios for behavioral validation",
	Long: `Create, list, and validate holdout scenarios stored in .agents/holdout/.

Scenarios are behavioral validation specs that implementing agents never see.
They are evaluated by council judges during validation (STEP 1.8) to assess
whether the implementation satisfies user intent.`,
}

func init() {
	rootCmd.AddCommand(scenarioCmd)
}
