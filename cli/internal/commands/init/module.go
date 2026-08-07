// Package init owns Cobra presentation for the `ao init` command. The module
// builds its command with a host-provided dry-run seam and delegates every
// filesystem effect to internal/initapp, so this package performs no direct
// effect.
package init

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/initapp"
)

// Module owns Cobra presentation for the init command.
type Module struct {
	host clicontract.HostOptions
}

// NewModule constructs the init command module from its host seams.
func NewModule(host clicontract.HostOptions) Module {
	return Module{host: host}
}

// Contract declares init's real behavior for the family architecture gate: it
// takes no positional args, emits text, creates local evidence directories
// (filesystem), and exits 0 on success or 1 on a working-directory or
// directory-creation failure. The init family attached no capabilities contract
// before the carve-out, so the composition does not attach this one either.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.init",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "none", Validate: cobra.NoArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao init` command. The RunE closure delegates entirely to
// internal/initapp so this module performs no direct filesystem effect.
func (m Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create local AgentOps evidence directories",
		Long: `Create local evidence and verdict directories, then add one commented,
marker-delimited block to this directory's .gitignore (creating the file if it
does not exist). The block ignores only machine-local scratch the loop writes:

  .agents/ao/index/       derived search index
  .agents/ao/sessions/    local session transcripts
  .agents/ao/provenance/  machine-local provenance ledger
  __pycache__/            Python bytecode caches

It deliberately does NOT ignore .agents/ao/intents/ or .agents/ao/verdicts/:
whether loop evidence belongs in version control is your repository's policy,
not this CLI's. Delete the block to track everything.

Running init again is safe — the block is appended once and never duplicated,
and an edited block is left alone. This command does not initialize Git,
install hooks, select work, or start a runtime.`,
		Args:    cobra.NoArgs,
		GroupID: "start",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return initapp.Run(initapp.RunOptions{
				DryRun: m.host.DryRun(),
				Stdout: cmd.OutOrStdout(),
			})
		},
	}
}
