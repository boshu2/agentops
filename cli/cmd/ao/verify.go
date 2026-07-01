// practices: [design-by-contract, code-complete]
package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// verifyCmd is the canonical front-door verb onto the pawl review engine
// (age-wedge-all-in-dyr0.1). It is a THIN alias: every argument is forwarded
// verbatim into runPawlReview — the exact RunE path `ao pawl review` uses — so
// the review engine, the embedded-bundle stranger path, the aoBinaryInside
// trust split, and the sanitized cold env are REUSED, never re-derived
// (a parallel implementation is a reject condition on this surface).
var verifyCmd = &cobra.Command{
	Use:   "verify <change-id> [--scope head|staged] [--converge] [--author-family <fam>] [--context <s>]",
	Short: "Independent cross-family verdict on your change — no verdict = not done",
	Long: `Run an independent cross-family review of your change and, on CONFIRMED, write
the commit-bound verdict to the provenance ledger. No verdict = not done: the
reviewer is a fresh-context model from a DIFFERENT family than the author
(never a same-model self-review), so "looks good to me" from the agent that
wrote the code never counts as done.

'ao verify' is the front door to the same engine as 'ao pawl review' (the
advanced surface — service panes, converge lineage, gate wiring); every
argument is forwarded verbatim, and the exit code IS the verdict:

  0  CONFIRMED — verdict written and bound to the commit
  3  REFUTED — defects printed; fix and re-run
  4  advisory-only (--converge without adversarial lineage)
  2  usage error
  1  hard error — always fail-closed, never a silent pass

Inside the AgentOps checkout it runs the live repo scripts (dogfood); anywhere
else it runs the embedded review bundle against YOUR git repository with a
sanitized environment, so any repo works zero-config.

THREAT MODEL: single-operator, own-repo. The sanitized stranger path stops a
repo under review from hijacking the reviewer (planted binaries, BASH_ENV,
external diff), but it is NOT a defense for adversarial multi-tenant hosting —
you verify code you chose to check out.

If the run fails before a verdict (no codex/agy reviewer installed, not a git
repository, bash missing), that is an environment problem, not a verdict —
run 'ao doctor' to diagnose.

Examples:
  ao verify my-change-123                # review + certify HEAD
  ao verify my-change --scope staged     # review staged work (advisory — no commit to bind)`,
	// The pawl review surface owns the flag contract; forward everything verbatim.
	DisableFlagParsing: true,
	RunE:               runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

// runVerify delegates to runPawlReview — the SAME code path `ao pawl review`
// executes — and only decorates non-verdict failures with the `ao doctor`
// pointer. Verdict exit codes (*pawlReviewExitError) propagate verbatim:
// the exit code IS the verdict.
func runVerify(cmd *cobra.Command, args []string) error {
	err := runPawlReview(cmd, args)
	var exitErr *pawlReviewExitError
	if err == nil || errors.As(err, &exitErr) {
		return err
	}
	return fmt.Errorf("ao verify: %w\nthis is an environment failure, not a verdict (fail-closed — never a silent pass); run 'ao doctor' to diagnose reviewer/tooling setup", err)
}
