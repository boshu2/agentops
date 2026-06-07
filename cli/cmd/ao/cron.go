// practices: [dora-metrics, lean-startup]
package main

import (
	"github.com/spf13/cobra"
)

// cronCmd is the compatibility parent for cron-loop helpers that moved behind
// the MTO/factory boundary during lean-image distillation.
var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Compatibility shims for relocated cron-fire scheduling",
	Long: `Compatibility shims for cron-fire scheduling surfaces that now belong
behind the MTO/factory boundary. AO keeps command discovery and route notices;
fleet cadence and prompt re-arming are owned by MTO.`,
}

func init() {
	cronCmd.GroupID = "workflow"
	rootCmd.AddCommand(cronCmd)
}
