// practices: [dora-metrics, lean-startup]
package main

import (
	"github.com/spf13/cobra"
)

// cronCmd is the parent command for cron-loop helpers used by the /evolve
// loop's cron-fire continuity primitive. Today it carries `self-adjust`
// (soc-un0m); future subcommands belong on this same parent so the operator
// surface stays consistent ("manage the cron contract").
var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Cron-fire loop helpers (used by /evolve --mode=loop)",
	Long: `Helpers for the /evolve --mode=loop cron-fire continuity primitive.

The /evolve loop runs as a recurring cron-fire that the agent re-arms each
cycle. These subcommands are the mechanical surfaces the agent calls to
participate in that contract:

  ao cron self-adjust ...   Render the next cycle's cron prompt from the
                            versioned template + last-cycle context, and emit
                            a JSON spec the harness uses to re-arm the cron.

See docs/plans/2026-05-21-evolve-loop-epic-design.md §A4 for the full design.`,
}

func init() {
	cronCmd.GroupID = "workflow"
	rootCmd.AddCommand(cronCmd)
}
