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
	// Folded under `ao eval` (age-focus-membrane-bookkeeper-m1wg.16). The
	// add/init/list/validate children are attached to scenarioCmd elsewhere and
	// auto-follow this reparent. `ao eval scenario-ab` (ADR-0004 revival path) is
	// a distinct sibling and is unaffected.
	evalCmd.AddCommand(scenarioCmd)
}
