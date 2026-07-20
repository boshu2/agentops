// practices: [dora-metrics, lean-startup]
package main

import (
	"os"

	"github.com/spf13/cobra"

	goalscommands "github.com/boshu2/agentops/cli/internal/commands/goals"
)

func init() {
	rootCmd.AddCommand(newGoalsCommand())
}

// newGoalsCommand wires the goals command module to its host seams. Goals reads
// the global -o/--output flag and -v/--verbose flag, resolves scenario evidence
// against the working directory, and delegates all filesystem, process, and
// clock effects to internal/goals. The goals family attaches no CommandContract
// to the command tree, preserving its pre-migration capabilities surface.
func newGoalsCommand() *cobra.Command {
	module := goalscommands.NewModule(goalscommands.HostOptions{
		OutputMode:  GetOutput,
		Verbose:     GetVerbose,
		ProjectRoot: goalsProjectRoot,
	})
	return module.Command()
}

// goalsProjectRoot resolves the project root scenario evidence is read against.
// CLI invocations run from the project root.
func goalsProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
