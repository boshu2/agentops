// practices: [dora-metrics, lean-startup]
package main

import (
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var (
	goalsMeasureGoalID       string
	goalsMeasureDirectives   bool
	goalsMeasureExcludeTag   string
	goalsMeasureTotalTimeout int
)

var goalsMeasureCmd = &cobra.Command{
	Use:     "measure",
	Aliases: []string{"m"},
	Short:   "Run goal checks and produce a snapshot",
	GroupID: "measurement",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunMeasure(goals.MeasureOptions{
			GoalID:       goalsMeasureGoalID,
			ExcludeTag:   goalsMeasureExcludeTag,
			Directives:   goalsMeasureDirectives,
			GoalsFile:    resolveGoalsFile(),
			Timeout:      time.Duration(goalsTimeout) * time.Second,
			TotalTimeout: time.Duration(goalsMeasureTotalTimeout) * time.Second,
			JSON:         goalsJSONOutput(),
			Verbose:      verbose,
		})
	},
}

func init() {
	goalsMeasureCmd.Flags().StringVar(&goalsMeasureGoalID, "goal", "", "Measure a single goal by ID")
	goalsMeasureCmd.Flags().BoolVar(&goalsMeasureDirectives, "directives", false, "Output directives as JSON (skip gate checks)")
	goalsMeasureCmd.Flags().StringVar(&goalsMeasureExcludeTag, "exclude-tag", "", "Skip goals whose Tags include this value (e.g. long-cycle)")
	goalsMeasureCmd.Flags().IntVar(&goalsMeasureTotalTimeout, "total-timeout", 0, "Overall measurement timeout in seconds (0 disables)")
	goalsCmd.AddCommand(goalsMeasureCmd)
}
