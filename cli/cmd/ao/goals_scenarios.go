// practices: [bdd-gherkin, llm-eval-harness]
package main

import (
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var (
	scenariosDirective   int
	scenariosDirectiveID string
	scenariosLint        bool
	scenariosStrict      bool
)

var goalsScenariosCmd = &cobra.Command{
	Use:     "scenarios",
	Short:   "Inspect holdout scenarios linked to GOALS.md directives",
	GroupID: "analysis",
	Args:    cobra.NoArgs,
	Long: `Inspect the executable-spec scenarios linked to GOALS.md directives.

Directive membership comes from each directive's
"**Scenarios:**" attribute line; scenario content is resolved from
spec/scenarios/ then .agents/holdout/ (see docs/adr/ADR-0003).

  ao goals scenarios                       list every directive and its links
  ao goals scenarios --directive 2         filter to directive #2
  ao goals scenarios --directive-id d-foo  filter to a stable directive ID
  ao goals scenarios -o json               machine-readable directive→scenarios map
  ao goals scenarios --lint                report link-graph defects`,
	RunE: runGoalsScenarios,
}

// runGoalsScenarios dispatches between read-only link lint and listing.
func runGoalsScenarios(cmd *cobra.Command, _ []string) error {
	if scenariosLint {
		return goals.RunLint(goals.LintOptions{
			GoalsFile: resolveGoalsFile(),
			Strict:    scenariosStrict,
			JSON:      goalsJSONOutput(),
			Stdout:    cmd.OutOrStdout(),
		})
	}
	return goals.RunScenarios(goals.ScenariosOptions{
		GoalsFile:    resolveGoalsFile(),
		DirectiveNum: scenariosDirective,
		DirectiveID:  scenariosDirectiveID,
		JSON:         goalsJSONOutput(),
		Stdout:       cmd.OutOrStdout(),
		Stderr:       cmd.ErrOrStderr(),
	})
}

func init() {
	goalsScenariosCmd.Flags().IntVar(&scenariosDirective, "directive", 0, "Filter by directive display number")
	goalsScenariosCmd.Flags().StringVar(&scenariosDirectiveID, "directive-id", "", "Filter listing to one directive by stable Directive ID")
	goalsScenariosCmd.Flags().BoolVar(&scenariosLint, "lint", false, "Lint the directive↔scenario link graph instead of listing")
	goalsScenariosCmd.Flags().BoolVar(&scenariosStrict, "strict", false, "With --lint, exit non-zero on warnings as well as errors")
	goalsCmd.AddCommand(goalsScenariosCmd)
}
