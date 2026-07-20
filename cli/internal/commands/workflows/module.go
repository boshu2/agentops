// Package workflows owns Cobra presentation for the `ao workflows` command
// family. The module builds its command tree and delegates every direct
// filesystem/git effect (checkout resolution, target resolution, and the
// link/unlink sweeps) to internal/workflowsapp, so this package performs no
// direct effect — mirroring the skills / skillsapp split.
package workflows

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	"github.com/boshu2/agentops/cli/internal/workflowsapp"
)

// Module owns Cobra presentation for the workflows command family.
type Module struct {
	host clicontract.HostOptions

	// link / unlink
	linkInto   string
	linkJSON   bool
	unlinkInto string
	unlinkJSON bool
}

// NewModule constructs the workflows command module from its host seams.
func NewModule(host clicontract.HostOptions) *Module {
	return &Module{host: host}
}

// Command builds the `ao workflows` command tree.
func (m *Module) Command() *cobra.Command {
	root := &cobra.Command{
		Use:     "workflows",
		Short:   "Install the repo's Claude-harness workflow scripts into a project",
		GroupID: "knowledge",
		Long: `Tooling for the top-level workflows/ source-of-truth: the Claude-harness
workflow scripts (workflows/*.js). Workflows are a CLAUDE-ONLY runtime
adapter — the same doctrine as skills-codex/ being Codex-only — installed
into the project-local .claude/workflows/ directory where the Claude Code
harness resolves named workflows. There is no multi-runtime fan-out.

The repo bans tracked symlinks, so installation is this runtime link step,
not tracked links: run ` + "`ao workflows link`" + ` from inside the agentops checkout
to install into the current project.`,
	}
	root.AddCommand(m.linkCommand())
	root.AddCommand(m.unlinkCommand())
	return root
}

func (m *Module) linkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Symlink the checkout's workflows/*.js into a project's .claude/workflows/",
		Long: `Scan the agentops checkout's workflows/ directory and create a symlink in
the target project's .claude/workflows/ for every workflow script (*.js)
that has no entry yet. The SOURCE is the checkout the command runs from
(resolved by walking up from the current directory, fail-closed outside the
repo); the TARGET is the current working directory's git root joined with
.claude/workflows/ (created if absent), or the single dir named by --into.

Workflows are a Claude-only runtime adapter (skills-codex/ is the Codex
twin doctrine): exactly one destination, no multi-runtime fan-out.

Idempotent and non-destructive: a script already linked to this checkout is
left alone. A pre-existing REAL file, or a symlink pointing anywhere else,
is reported as a conflict and never replaced — resolving ownership of an
operator-owned path is operator judgment. Conflicts do not fail the exit;
inspect and resolve the named path explicitly.

  ao workflows link                       # link into <git-root>/.claude/workflows
  ao workflows link --dry-run             # preview without writing
  ao workflows link --dry-run --json      # machine-readable preview
  ao workflows link --into /path/to/dir   # link into ONE specific dir`,
		Args: cobra.NoArgs,
		RunE: m.runLink,
	}
	cmd.Flags().StringVar(&m.linkInto, "into", "", "Link into this single dir instead of <cwd-git-root>/.claude/workflows")
	cmd.Flags().BoolVar(&m.linkJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func (m *Module) runLink(cmd *cobra.Command, _ []string) error {
	srcDir, err := workflowsapp.ResolveRepoWorkflowsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	dest, err := workflowsapp.ResolveTargetDir(m.linkInto)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	res, err := workflowsapp.LinkWorkflows(srcDir, dest, m.host.DryRun())
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	if m.linkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	workflowsapp.RenderLinkResult(cmd.OutOrStdout(), res)
	return nil
}

func (m *Module) unlinkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink",
		Short: "Remove the workflow symlinks that `workflows link` minted",
		Long: `The clean uninstall inverse of ` + "`ao workflows link`" + `. Sweep the target
project's .claude/workflows/ (or the single dir named by --into) and remove
EXACTLY the symlinks that link minted — those whose target resolves into
THIS checkout's workflows/ tree. A foreign symlink pointing elsewhere and a
real file are both reported as foreign and never removed. A stale link to a
workflow since removed from the checkout is still cleaned up.

Must be run from inside the agentops checkout — it needs the checkout's
workflows/ path to know which links are its own.

  ao workflows unlink                     # remove owned links from <git-root>/.claude/workflows
  ao workflows unlink --dry-run           # preview without removing
  ao workflows unlink --into /path/to/dir # sweep ONE specific dir`,
		Args: cobra.NoArgs,
		RunE: m.runUnlink,
	}
	cmd.Flags().StringVar(&m.unlinkInto, "into", "", "Sweep this single dir instead of <cwd-git-root>/.claude/workflows")
	cmd.Flags().BoolVar(&m.unlinkJSON, "json", false, "Emit machine-readable JSON")
	return cmd
}

func (m *Module) runUnlink(cmd *cobra.Command, _ []string) error {
	srcDir, err := workflowsapp.ResolveRepoWorkflowsDir()
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	dest, err := workflowsapp.ResolveTargetDir(m.unlinkInto)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	res, err := workflowsapp.UnlinkWorkflows(srcDir, dest, m.host.DryRun())
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	if m.unlinkJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	workflowsapp.RenderUnlinkResult(cmd.OutOrStdout(), res)
	return nil
}

// Contract documents the workflows family's effect and profile shape for the
// family architecture gate, mirroring the skills module: the family does not
// attach a capabilities contract, so the composition does not attach this one
// either.
func (*Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.workflows",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectFilesystem,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}
