// practices: [dora-metrics, lean-startup]
package main

import (
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var goalsDriftCmd = &cobra.Command{
	Use:     "drift",
	Aliases: []string{"d"},
	Short:   "Compare snapshots for regressions",
	GroupID: "analysis",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunDrift(goals.DriftOptions{
			GoalsFile: resolveGoalsFile(),
			Timeout:   time.Duration(goalsTimeout) * time.Second,
			JSON:      goalsJSON,
		})
	},
}

func init() {
	goalsCmd.AddCommand(goalsDriftCmd)
}
