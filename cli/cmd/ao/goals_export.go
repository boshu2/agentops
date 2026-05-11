// practices: [dora-metrics, lean-startup]
package main

import (
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var goalsExportCmd = &cobra.Command{
	Use:     "export",
	Aliases: []string{"e"},
	Short:   "Export latest snapshot as JSON (for CI)",
	GroupID: "analysis",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunExport(goals.ExportOptions{
			GoalsFile: resolveGoalsFile(),
			Timeout:   time.Duration(goalsTimeout) * time.Second,
		})
	},
}

func init() {
	goalsCmd.AddCommand(goalsExportCmd)
}
