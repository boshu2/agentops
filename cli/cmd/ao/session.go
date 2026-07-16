package main

import "github.com/spf13/cobra"

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Inspect or export session evidence",
	Args:  cobra.NoArgs,
}

func init() {
	sessionCmd.GroupID = "workflow"
	rootCmd.AddCommand(sessionCmd)
}
