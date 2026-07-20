package main

import (
	quickstartcommands "github.com/boshu2/agentops/cli/internal/commands/quickstart"
)

func init() {
	rootCmd.AddCommand(quickstartcommands.NewModule().Command())
}
