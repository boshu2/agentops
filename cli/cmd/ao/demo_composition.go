// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"github.com/spf13/cobra"

	democommands "github.com/boshu2/agentops/cli/internal/commands/demo"
)

func init() {
	rootCmd.AddCommand(newDemoCommand())
}

// newDemoCommand wires the demo command module. Demo renders static explanatory
// text and performs no effect, so the module needs no host seams. The demo
// family attaches no CommandContract to the command tree, preserving its
// pre-migration capabilities surface.
func newDemoCommand() *cobra.Command {
	return democommands.NewModule().Command()
}
