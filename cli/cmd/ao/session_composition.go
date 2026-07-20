// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	sessioncommands "github.com/boshu2/agentops/cli/internal/commands/session"
)

func init() {
	rootCmd.AddCommand(newSessionCommand())
}

// newSessionCommand wires the session command module and attaches the optional
// `ao session handoff` writer, which is a separate command (defined in
// handoff.go) that shares this parent. The module owns the session parent plus
// its bootstrap and rehydrate subcommands and delegates all filesystem effects
// to internal/sessionapp. The session family attaches no CommandContract to the
// command tree, preserving its pre-migration capabilities surface.
func newSessionCommand() *cobra.Command {
	command := sessioncommands.NewModule(clicontract.HostOptions{OutputMode: GetOutput}).Command()
	command.AddCommand(handoffCmd)
	return command
}
