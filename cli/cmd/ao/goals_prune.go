// practices: [dora-metrics, lean-startup]
package main

import (
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

// Type aliases for test compatibility.
type pruneResult = goals.PruneResult
type staleGoal = goals.StaleGoal

var goalsPruneCmd = &cobra.Command{
	Use:     "prune",
	Aliases: []string{"p"},
	Short:   "Remove goals referencing nonexistent files",
	GroupID: "management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunPrune(goals.PruneOptions{
			GoalsFile: resolveGoalsFile(),
			DryRun:    dryRun,
			JSON:      goalsJSONOutput(),
		})
	},
}

// findMissingPath delegates to goals.FindMissingPath (used by tests).
func findMissingPath(check string) string {
	return goals.FindMissingPath(check)
}

func init() {
	goalsCmd.AddCommand(goalsPruneCmd)
}
