// practices: [supply-chain-integrity]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	redactcommands "github.com/boshu2/agentops/cli/internal/commands/redact"
)

func init() {
	rootCmd.AddCommand(newRedactCommand())
}

// newRedactCommand wires the redact command module and attaches its real
// CommandContract to the command tree. Unlike most carved families, redact
// carries a contract; this composition attaches it so the capabilities surface
// is unchanged by the carve-out.
func newRedactCommand() *cobra.Command {
	module := redactcommands.NewModule()
	command := module.Command()
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	return command
}
