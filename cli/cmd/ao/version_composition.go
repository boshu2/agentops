// practices: [continuous-delivery, supply-chain-integrity]
package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	versioncommands "github.com/boshu2/agentops/cli/internal/commands/version"
)

func init() {
	rootCmd.AddCommand(newVersionCommand())
}

// newVersionCommand wires the version command module to its host seams: the
// build-time version string (set via ldflags on main.version) and the global
// -o/--output mode. Unlike the other W3 families, version carries a real
// CommandContract; this composition attaches it to the command tree so the
// capabilities surface is unchanged by the carve-out.
func newVersionCommand() *cobra.Command {
	module := versioncommands.NewModule(versioncommands.HostOptions{
		Version:    func() string { return version },
		OutputMode: GetOutput,
	})
	command := module.Command()
	if err := clicontract.Attach(command, module.Contract()); err != nil {
		panic(err)
	}
	return command
}
