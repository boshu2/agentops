// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"time"

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
// its bootstrap, rehydrate, prune-agents and read-source subcommands. The
// module attaches its family and read-source contracts and delegates effects
// to the session and source-reading application packages.
func newSessionCommand() *cobra.Command {
	command := sessioncommands.NewModule(clicontract.HostOptions{
		OutputMode: GetOutput,
		DryRun:     GetDryRun,
		ProjectRoot: func() string {
			root, err := repoRootOrCwd()
			if err != nil {
				return ""
			}
			return root
		},
		Now: time.Now,
	}).Command()
	command.AddCommand(handoffCmd)
	return command
}
