// practices: [agile-manifesto, dora-metrics]
package main

import (
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

// ComplexityLevel classifies the ceremony complexity of an RPI goal.
type ComplexityLevel = cliRPI.ComplexityLevel

const (
	ComplexityFast     = cliRPI.ComplexityFast
	ComplexityStandard = cliRPI.ComplexityStandard
	ComplexityFull     = cliRPI.ComplexityFull
)

var rpiCmd = &cobra.Command{
	Use:   "rpi",
	Short: "RPI lifecycle automation",
	Args:  cobra.NoArgs,
	Long: `Commands for automating the RPI lifecycle.

Commands:
  loop       Run continuous RPI cycles from next-work queue

The RPI loop reads .agents/rpi/next-work.jsonl for harvested work items
and spawns fresh Claude sessions for each cycle using the phased
Discovery -> Implementation -> Validation workflow (Ralph Wiggum pattern).`,
}

func init() {
	rpiCmd.GroupID = "workflow"
	rootCmd.AddCommand(rpiCmd)
}

func addRPISubcommand(cmd *cobra.Command) {
	rpiCmd.AddCommand(cmd)
}

func classifyComplexity(goal string) ComplexityLevel {
	return cliRPI.ClassifyComplexity(goal)
}
