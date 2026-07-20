// practices: [twelve-factor-app, ai-assisted-dev, pragmatic-programmer]
package main

import (
	robotdocscommands "github.com/boshu2/agentops/cli/internal/commands/robotdocs"
)

func init() {
	rootCmd.AddCommand(robotdocscommands.NewModule().Command())
}
