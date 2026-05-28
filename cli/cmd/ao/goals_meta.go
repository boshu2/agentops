// practices: [dora-metrics, lean-startup]
package main

import (
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var goalsMetaCmd = &cobra.Command{
	Use:     "meta",
	Short:   "Run and report meta-goals only",
	GroupID: "management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return goals.RunMeta(goals.MetaOptions{
			GoalsFile: resolveGoalsFile(),
			Timeout:   time.Duration(goalsTimeout) * time.Second,
			JSON:      goalsJSONOutput(),
		})
	},
}

func init() {
	goalsCmd.AddCommand(goalsMetaCmd)
}
