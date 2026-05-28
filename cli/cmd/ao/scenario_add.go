// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/scenario"
	"github.com/spf13/cobra"
)

// scenarioFile aliases the scenario package's Scenario type so other files in
// this package (and tests) keep a stable local name.
type scenarioFile = scenario.Scenario

// scenarioAcceptanceVector aliases the scenario package's AcceptanceVector.
type scenarioAcceptanceVector = scenario.AcceptanceVector

// scenarioIDPattern aliases the scenario package's ID pattern (used by
// scenario validate).
var scenarioIDPattern = scenario.IDPattern

// scenarioAddNow is the injectable clock for deterministic IDs in tests.
var scenarioAddNow = time.Now

var scenarioAddFlags = struct {
	Narrative       string
	ExpectedOutcome string
	Threshold       float64
	Status          string
	Source          string
}{
	Threshold: 0.8,
	Status:    "draft",
	Source:    "human",
}

var scenarioAddCmd = &cobra.Command{
	Use:   "add <goal>",
	Short: "Author a holdout scenario from a goal description",
	Long: `Author a schema-compliant holdout scenario in .agents/holdout/.

The command infers narrative and expected-outcome text from the provided goal
unless explicit values are supplied. New scenarios default to draft so a human
or evaluator can review them before activation.`,
	Args: cobra.ExactArgs(1),
	RunE: runScenarioAdd,
}

// runScenarioAdd authors an ad hoc holdout scenario by delegating to the shared
// scenario.Create path — the same path `ao goals scenarios --create` uses.
func runScenarioAdd(cmd *cobra.Command, args []string) error {
	res, err := scenario.Create(scenario.CreateOptions{
		Goal:            args[0],
		Narrative:       scenarioAddFlags.Narrative,
		ExpectedOutcome: scenarioAddFlags.ExpectedOutcome,
		Threshold:       scenarioAddFlags.Threshold,
		Status:          scenarioAddFlags.Status,
		Source:          scenarioAddFlags.Source,
		Dir:             filepath.Join(".agents", "holdout"),
		Now:             scenarioAddNow,
	})
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res.Scenario)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created scenario %s at %s\n", res.Scenario.ID, res.Path)
	return nil
}

// validScenarioStatus reports whether status is an allowed scenario status.
func validScenarioStatus(status string) bool { return scenario.ValidStatus(status) }

// validScenarioSource reports whether source is an allowed scenario source.
func validScenarioSource(source string) bool { return scenario.ValidSource(source) }

func init() {
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.Narrative, "narrative", "", "Narrative description (default: inferred from goal)")
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.ExpectedOutcome, "expected-outcome", "", "Expected observable outcome (default: inferred from goal)")
	scenarioAddCmd.Flags().Float64Var(&scenarioAddFlags.Threshold, "threshold", 0.8, "Satisfaction threshold in [0,1]")
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.Status, "status", "draft", "Scenario status (active, draft, retired)")
	scenarioAddCmd.Flags().StringVar(&scenarioAddFlags.Source, "source", "human", "Scenario source (human, agent, prod-telemetry)")
	_ = scenarioAddCmd.RegisterFlagCompletionFunc("status", staticCompletionFunc("active", "draft", "retired"))
	_ = scenarioAddCmd.RegisterFlagCompletionFunc("source", staticCompletionFunc("human", "agent", "prod-telemetry"))
	scenarioCmd.AddCommand(scenarioAddCmd)
}
