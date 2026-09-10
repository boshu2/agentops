// Package quickstart owns Cobra presentation for the `ao quick-start` command.
// The command prints a static native execution brief, so it performs no
// filesystem, process, or clock effect.
package quickstart

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// Module owns Cobra presentation for the quick-start command.
type Module struct{}

// NewModule constructs the quick-start command module.
func NewModule() Module {
	return Module{}
}

// Contract declares quick-start's real behavior: it takes no positional args,
// emits a static text summary to stdout, is a pure computation (no filesystem,
// process, or clock effect), and exits 0 on success or 1 on an output-write
// failure.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.quick-start",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "no-args", Validate: cobra.NoArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao quick-start` command.
func (Module) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "quick-start",
		Short: "Show the native AgentOps execution brief",
		Long: `Use your native coding agent and shell to complete the accepted work.
Read the repository's brief and follow its scope, checks and delivery policy.
No skill installation, workflow-skill loading, bootstrap or init is required.

Use ao gate check for deterministic repository checks. A fresh independent
context judges the exact final change against unchanged acceptance; the
Validate skill is an optional way to conduct that required judgment.
Specialists and machine-readable evidence persistence are optional.`,
		Args:    cobra.NoArgs,
		GroupID: "start",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), `NATIVE AGENTOPS

1. Read the repository brief; keep accepted behavior and write scope fixed.
2. Use your native agent and shell to implement, run checks and repair known failures.
3. Run required repository checks; ao gate check supplies deterministic facts.
4. Obtain fresh independent judgment of the exact final change against unchanged acceptance.
5. Report the result, checked and not_checked scope, and follow repository delivery policy.

No skills, bootstrap or ao init are required. Missing independent judgment or
unchecked acceptance means NOT_PROVEN; passing checks alone do not prove completion.
Specialists, including the Validate skill, are optional. From an AgentOps checkout:
  ao skills find "your task"             # discover an optional specialist
  ao skills link --skill security        # install only a selected specialist
  ao demo --rpi                          # show the optional full workflow

Artifact persistence: optional; ao provenance provides explicit evidence tools.
Git, work tracking and delivery remain with the caller and repository.`)
		},
	}
}
