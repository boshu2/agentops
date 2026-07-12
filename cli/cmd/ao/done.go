// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	doneapp "github.com/boshu2/agentops/cli/internal/done"
)

var (
	doneSHA           string
	doneReason        string
	doneForceNoVerdct bool
	doneJSON          bool
)

var doneCmd = &cobra.Command{
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
	RunE: runDone,
}

func init() {
	rootCmd.AddCommand(doneCmd)
	doneCmd.Flags().StringVar(&doneSHA, "sha", "", "Commit sha (or >=7-char prefix) the bead landed as (default: HEAD at cwd)")
	doneCmd.Flags().StringVarP(&doneReason, "reason", "r", "Done", "Close reason prose (the verdict stamp is appended)")
	doneCmd.Flags().BoolVar(&doneForceNoVerdct, "force-no-verdict", false, "Close without a verdict, stamping an explicit UNVERIFIED marker")
	doneCmd.Flags().BoolVar(&doneJSON, "json", false, "Emit machine-readable JSON (stdout-as-data)")
}

func runDone(command *cobra.Command, args []string) error {
	command.SilenceUsage = true
	ctx := command.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := newDoneService().Execute(ctx, doneapp.Request{
		BeadID: args[0], SHA: doneSHA, Reason: doneReason, ForceNoVerdict: doneForceNoVerdct,
	})
	if err != nil {
		return err
	}
	out := command.OutOrStdout()
	if doneJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	if output := strings.TrimSpace(result.TrackerOutput); output != "" {
		fmt.Fprintln(out, output)
	}
	fmt.Fprintf(out, "closed %s at %s %s\n", result.BeadID, result.CommitSHA[:doneapp.MinimumSHAPrefix], result.Stamp)
	if result.Disposition == doneapp.DispositionUnverified {
		fmt.Fprintf(out, "note: UNVERIFIED close — run 'ao verify %s' next time so done carries proof\n", result.BeadID)
	}
	return nil
}
