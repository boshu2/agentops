// practices: [bdd, llm-eval-harness]
package main

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var (
	scenariosDirective   int
	scenariosDirectiveID string
	scenariosCreate      string
	scenariosThreshold   float64
	scenariosStatus      string
	scenariosSource      string
	scenariosLint        bool
	scenariosStrict      bool
)

var goalsScenariosCmd = &cobra.Command{
	Use:     "scenarios",
	Short:   "List or create the holdout scenarios linked to GOALS.md directives",
	GroupID: "analysis",
	Args:    cobra.NoArgs,
	Long: `List or create the executable-spec scenarios linked to GOALS.md directives.

Listing is read-only: directive membership comes from each directive's
"**Scenarios:**" attribute line; scenario content is resolved from
spec/scenarios/ then .agents/holdout/ (see docs/adr/ADR-0003).

  ao goals scenarios                       list every directive and its links
  ao goals scenarios --directive 2         filter to directive #2
  ao goals scenarios --directive-id d-foo  filter to a stable directive ID
  ao goals scenarios -o json               machine-readable directive→scenarios map

Creating scaffolds a promoted spec scenario and links it bidirectionally:

  ao goals scenarios --create "<goal>" --directive 2

The new scenario JSON carries the directive's stable directive_id, and the
directive's "**Scenarios:**" line gains the scenario ID via the non-lossy
patcher (no other byte of GOALS.md changes).`,
	RunE: runGoalsScenarios,
}

// runGoalsScenarios dispatches between link lint, --create, and the listing.
func runGoalsScenarios(cmd *cobra.Command, _ []string) error {
	if scenariosLint {
		return goals.RunLint(goals.LintOptions{
			GoalsFile: resolveGoalsFile(),
			Strict:    scenariosStrict,
			JSON:      goalsJSONOutput(),
			Stdout:    cmd.OutOrStdout(),
		})
	}
	if scenariosCreate != "" {
		if scenariosDirective == 0 {
			return fmt.Errorf("--create requires --directive <n> to name the target directive")
		}
		return goals.RunScenarioCreate(goals.ScenarioCreateOptions{
			GoalsFile:    resolveGoalsFile(),
			DirectiveNum: scenariosDirective,
			Goal:         scenariosCreate,
			Threshold:    scenariosThreshold,
			Status:       scenariosStatus,
			Source:       scenariosSource,
			JSON:         goalsJSONOutput(),
			Stdout:       cmd.OutOrStdout(),
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
	goalsScenariosCmd.Flags().IntVar(&scenariosDirective, "directive", 0, "Directive display number (filter when listing, target when creating)")
	goalsScenariosCmd.Flags().StringVar(&scenariosDirectiveID, "directive-id", "", "Filter listing to one directive by stable Directive ID")
	goalsScenariosCmd.Flags().StringVar(&scenariosCreate, "create", "", "Create a scenario from this goal description and link it to --directive")
	goalsScenariosCmd.Flags().Float64Var(&scenariosThreshold, "threshold", 0.8, "Satisfaction threshold for a created scenario")
	goalsScenariosCmd.Flags().StringVar(&scenariosStatus, "status", "draft", "Status for a created scenario (active, draft, retired)")
	goalsScenariosCmd.Flags().StringVar(&scenariosSource, "source", "human", "Source for a created scenario (human, agent, prod-telemetry)")
	goalsScenariosCmd.Flags().BoolVar(&scenariosLint, "lint", false, "Lint the directive↔scenario link graph instead of listing")
	goalsScenariosCmd.Flags().BoolVar(&scenariosStrict, "strict", false, "With --lint, exit non-zero on warnings as well as errors")
	_ = goalsScenariosCmd.RegisterFlagCompletionFunc("status", staticCompletionFunc("active", "draft", "retired"))
	_ = goalsScenariosCmd.RegisterFlagCompletionFunc("source", staticCompletionFunc("human", "agent", "prod-telemetry"))
	goalsCmd.AddCommand(goalsScenariosCmd)
}
