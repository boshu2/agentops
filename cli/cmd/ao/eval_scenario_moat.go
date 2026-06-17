// practices: [llm-eval-harness]
package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	aoeval "github.com/boshu2/agentops/cli/internal/eval"
)

var (
	evalScenarioMoatScorecards []string
	evalScenarioMoatOutput     string
)

var evalScenarioMoatCmd = &cobra.Command{
	Use:   "scenario-moat",
	Short: "Aggregate moat-eligible scenario A/B scorecards into a publication verdict",
	Long: `Render a moat positive/null/inconclusive verdict over one or more
ScenarioDeltaScorecard JSON artifacts from ao eval scenario-ab.

The claim surface fail-closes on any scorecard with moat_eligible=false — a
fact-recall/plumbing scorecard can pass its own gate but must NEVER be aggregated
into a moat verdict (age-6ys/age-sb0). See docs/evals/applied-ood-claim-rule.md.`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if len(evalScenarioMoatScorecards) == 0 {
			return fmt.Errorf("at least one --scorecard path is required")
		}
		cards := make([]aoeval.ScenarioDeltaScorecard, 0, len(evalScenarioMoatScorecards))
		for _, path := range evalScenarioMoatScorecards {
			card, err := aoeval.LoadScenarioDeltaScorecard(path)
			if err != nil {
				return err
			}
			cards = append(cards, card)
		}
		result, err := aoeval.AggregateMoatClaim(cards)
		if err != nil {
			var ineligible aoeval.ErrMoatIneligibleScorecard
			if errors.As(err, &ineligible) {
				fmt.Fprintf(cmd.ErrOrStderr(), "REJECTED: %s\n", err)
			}
			return err
		}
		if err := aoeval.WriteMoatClaimResult(evalScenarioMoatOutput, result); err != nil {
			return err
		}
		if GetOutput() == "json" {
			return writeEvalJSON(cmd, result)
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"scenario-moat: verdict=%s scenarios=%d mean_delta=%.4f\n  %s\n",
			result.Verdict, result.ScenarioCount, result.MeanAggregateDelta, result.Reason,
		)
		if evalScenarioMoatOutput != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Moat claim result: %s\n", evalScenarioMoatOutput)
		}
		if result.Verdict == aoeval.MoatVerdictInconclusive {
			return fmt.Errorf("moat claim inconclusive — cannot publish positive or honest null")
		}
		return nil
	},
}

func init() {
	evalScenarioMoatCmd.Flags().StringArrayVar(&evalScenarioMoatScorecards, "scorecard", nil, "Path to a ScenarioDeltaScorecard JSON (repeatable)")
	evalScenarioMoatCmd.Flags().StringVar(&evalScenarioMoatOutput, "output", "", "Write the MoatClaimResult JSON to this path")
	evalCmd.AddCommand(evalScenarioMoatCmd)
}
