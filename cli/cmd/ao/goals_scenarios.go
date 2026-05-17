// practices: [bdd, llm-eval-harness]
package main

import (
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var (
	scenariosDirective   int
	scenariosDirectiveID string
)

var goalsScenariosCmd = &cobra.Command{
	Use:     "scenarios",
	Short:   "List the holdout scenarios linked to each GOALS.md directive",
	GroupID: "analysis",
	Args:    cobra.NoArgs,
	Long: `List the executable-spec scenarios linked to each GOALS.md directive.

Read-only: never mutates GOALS.md or any scenario file. Directive membership
comes from each directive's "**Scenarios:**" attribute line; scenario content
is resolved from spec/scenarios/ then .agents/holdout/ (see docs/adr/ADR-0003).

  ao goals scenarios                       list every directive and its links
  ao goals scenarios --directive 2         filter to directive #2
  ao goals scenarios --directive-id d-foo  filter to a stable directive ID
  ao goals scenarios -o json               machine-readable directive→scenarios map`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunScenarios(goals.ScenariosOptions{
			GoalsFile:    resolveGoalsFile(),
			DirectiveNum: scenariosDirective,
			DirectiveID:  scenariosDirectiveID,
			JSON:         goalsJSONOutput(),
			Stdout:       cmd.OutOrStdout(),
			Stderr:       cmd.ErrOrStderr(),
		})
	},
}

func init() {
	goalsScenariosCmd.Flags().IntVar(&scenariosDirective, "directive", 0, "Filter to one directive by display number")
	goalsScenariosCmd.Flags().StringVar(&scenariosDirectiveID, "directive-id", "", "Filter to one directive by stable Directive ID")
	goalsCmd.AddCommand(goalsScenariosCmd)
}
