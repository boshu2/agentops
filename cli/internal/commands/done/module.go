// Package done owns Cobra presentation for the done command family.
package done

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
	doneapp "github.com/boshu2/agentops/cli/internal/done"
)

type UseCases interface {
	Execute(context.Context, doneapp.Request) (doneapp.Result, error)
}

type Module struct{ useCases UseCases }

func NewModule(useCases UseCases) Module { return Module{useCases: useCases} }

func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID: "ao.done",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileFlywheel |
			clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:    clicontract.ArgsPolicy{Name: "exact-1", Validate: cobra.ExactArgs(1)},
		Output:  clicontract.OutputNone,
		Effects: clicontract.EffectFilesystem | clicontract.EffectProcess | clicontract.EffectTracker | clicontract.EffectEnvironment,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

type options struct {
	sha, reason    string
	forceNoVerdict bool
	jsonOutput     bool
}

func (module Module) Command() *cobra.Command {
	options := options{reason: "Done"}
	command := &cobra.Command{
		Use:   "done <bead-id>",
		Short: "Close a bead with a verdict-referenced stamp — no verdict = not done",
		Long: `Close a bead through the membrane's bookkeeping half: the close reason is
stamped with the ledger proof that the landed commit was actually reviewed.

Resolution order for the target commit: --sha wins; otherwise the HEAD of the
git repository at the current working directory. The commit is looked up in
the committed provenance ledger (docs/provenance/ledger.jsonl):

  CONFIRMED verdict bound to the commit
      -> close via br with "[verdict:<sha7>:CONFIRMED]" appended to the reason.
  no verdict, and every changed file of the commit is under docs/provenance/
      -> the #trivial waiver class; close with "[verdict:<sha7>:waived-trivial]".
  no verdict otherwise
      -> REFUSE (non-zero exit) and name the command that produces one
         (ao verify / ao pawl review). --force-no-verdict is the escape hatch:
         it closes with an explicit, greppable "[verdict:<sha7>:UNVERIFIED]"
         stamp instead of blocking forever.

The close itself shells out to the br CLI on PATH with the environment as-is:
run 'ao done' where 'ao beads dir' resolves the live ledger (BEADS_DIR is
respected when already exported; in linked worktrees the resolution walks
git's common directory back to the canonical _beads — no path is hardcoded).

Posture: warn-first. Nothing intercepts a raw 'br close'; the lever is this
recommended close path plus the warn-only verdict-close-rate gate.

Examples:
  ao done ag-x31t.4                       # close against HEAD's verdict
  ao done ag-x31t.4 --sha 4f2a91c         # close against an explicit commit
  ao done ag-x31t.4 --force-no-verdict    # honest UNVERIFIED escape hatch`,
		Args: cobra.ExactArgs(1),
	}
	flags := command.Flags()
	flags.StringVar(&options.sha, "sha", "", "Commit sha (or >=7-char prefix) the bead landed as (default: HEAD at cwd)")
	flags.StringVarP(&options.reason, "reason", "r", "Done", "Close reason prose (the verdict stamp is appended)")
	flags.BoolVar(&options.forceNoVerdict, "force-no-verdict", false, "Close without a verdict, stamping an explicit UNVERIFIED marker")
	flags.BoolVar(&options.jsonOutput, "json", false, "Emit machine-readable JSON (stdout-as-data)")
	command.RunE = func(command *cobra.Command, args []string) error {
		command.SilenceUsage = true
		result, err := module.useCases.Execute(command.Context(), doneapp.Request{
			BeadID: args[0], SHA: options.sha, Reason: options.reason, ForceNoVerdict: options.forceNoVerdict,
		})
		if err != nil {
			return err
		}
		return render(command, result, options.jsonOutput)
	}
	return command
}

func render(command *cobra.Command, result doneapp.Result, jsonOutput bool) error {
	output := command.OutOrStdout()
	if jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	if trackerOutput := strings.TrimSpace(result.TrackerOutput); trackerOutput != "" {
		fmt.Fprintln(output, trackerOutput)
	}
	fmt.Fprintf(output, "closed %s at %s %s\n", result.BeadID, result.CommitSHA[:doneapp.MinimumSHAPrefix], result.Stamp)
	if result.Disposition == doneapp.DispositionUnverified {
		fmt.Fprintf(output, "note: UNVERIFIED close — run 'ao verify %s' next time so done carries proof\n", result.BeadID)
	}
	return nil
}
