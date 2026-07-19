// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"github.com/spf13/cobra"

	skillscommands "github.com/boshu2/agentops/cli/internal/commands/skills"
)

func init() {
	rootCmd.AddCommand(newSkillsCommand())
}

// newSkillsCommand wires the skills command module to its host seams. The global
// --dry-run flag drives link/unlink; skills-root resolution and the link/unlink
// filesystem sweeps are host effects delegated to internal/skillsapp. Skills had
// no attached capabilities contract before the carve-out, so this composition
// does not attach the module's contract either — the capabilities surface is
// unchanged.
func newSkillsCommand() *cobra.Command {
	module := skillscommands.NewModule(skillscommands.HostOptions{
		DryRun: GetDryRun,
	})
	return module.Command()
}
