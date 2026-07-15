package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:   "quick-start",
	Short: "Show the single-pass AgentOps workflow",
	Long: `AgentOps is a small semantic evidence layer around agent work.

Run the RPI skill for one pass:
  Plan -> Implement -> fresh Validate -> durable verdict -> report and stop

The CLI does not claim work, retry, manage Git, or deliver changes. Use
ao gate check for deterministic repository checks and ao provenance for
generic evidence inspection.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop")
		fmt.Fprintln(cmd.OutOrStdout(), "Deterministic checks: ao gate check")
		fmt.Fprintln(cmd.OutOrStdout(), "Semantic judgment: invoke the Validate skill from a fresh context")
	},
}

func init() {
	quickstartCmd.GroupID = "start"
	rootCmd.AddCommand(quickstartCmd)
}
