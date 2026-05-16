// practices: [dora-metrics, lean-startup]
package main

import (
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var goalsHistoryGoalID string
var goalsHistorySince string

var goalsHistoryCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"h"},
	Short:   "Show goal measurement history",
	GroupID: "analysis",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunHistory(goals.HistoryOptions{
			GoalID: goalsHistoryGoalID,
			Since:  goalsHistorySince,
			JSON:   goalsJSONOutput(),
		})
	},
}

func init() {
	goalsHistoryCmd.Flags().StringVar(&goalsHistoryGoalID, "goal", "", "Filter history to a specific goal")
	goalsHistoryCmd.Flags().StringVar(&goalsHistorySince, "since", "", "Show entries since date (YYYY-MM-DD)")
	goalsCmd.AddCommand(goalsHistoryCmd)
}
