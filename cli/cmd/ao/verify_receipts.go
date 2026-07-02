// practices: [design-by-contract, code-complete]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// defaultReceiptsScript is the repo-relative membrane-receipts generator. It ALSO
// rides in the embedded pawl bundle (cli/embedded/pawl/scripts/, synced by
// `make sync-hooks`), so the stranger path renders a proof page with no checkout.
const defaultReceiptsScript = "scripts/gen-membrane-receipts.sh"

// verifyReceiptsCmd renders the membrane-receipts proof page for the CURRENT repo
// from its provenance ledger — the portable twin of the in-repo generator
// (age-rk3r.12). It REUSES the pawl embedded-bundle machinery (the aoBinaryInside
// trust split, extractPawlBundle, and the sanitized cold env), so any repo with
// verify verdicts in its ledger produces its own receipts zero-config and NEVER
// trusts a generator script from the repo under review.
var verifyReceiptsCmd = &cobra.Command{
	Use:   "receipts",
	Short: "Render this repo's membrane-receipts proof page from its provenance ledger",
	Long: `Render the membrane-receipts proof page for THIS git repository from its
provenance ledger (docs/provenance/ledger.jsonl). Every number on the page is
derived from the ledger — nothing is hand-written — and the generator REFUSES to
render (nonzero exit, nothing written) when 'ao provenance verify' reports a
broken or tampered hash chain. Outputs land in the repo:

  docs/evidence/membrane-receipts.md    (human page)
  docs/releases/membrane-receipts.json  (machine twin for claim citations)

Inside the AgentOps checkout it runs the live repo generator (dogfood). Anywhere
else it runs the EMBEDDED generator against YOUR repo with a sanitized
environment and the running binary pinned as ao (no ao-on-PATH needed), so any
repo produces its own receipts zero-config — and no script from the repo under
review is ever executed.

Exit code IS the outcome (propagated verbatim from the generator):
  0  receipts rendered
  1  REFUSED — the ledger chain failed to verify (nothing written; the break is named)
  2  script/environment error (no ledger, missing jq, bad layout)

Examples:
  ao verify receipts                     # render this repo's proof page`,
	Args: cobra.NoArgs,
	RunE: runVerifyReceipts,
}

func init() {
	verifyCmd.AddCommand(verifyReceiptsCmd)
}

// runVerifyReceipts renders the receipts page for the repo at cwd, mirroring the
// pawl review trust split: run the LIVE repo generator only when the running ao
// binary physically lives inside the resolved checkout (forge-proof dogfood);
// otherwise run the EMBEDDED generator against the user's own git repo. The
// generator's exit code propagates verbatim through *pawlReviewExitError (Execute
// maps it to os.Exit): 0 rendered · 1 REFUSED on a broken chain · 2 script error.
func runVerifyReceipts(cmd *cobra.Command, _ []string) error {
	// EDGE 1 — genuine in-AgentOps dogfood: run the LIVE repo generator so a script
	// edit is immediately exercised. The trust test is aoBinaryInside (forge-proof,
	// not marker files). nil env: the in-checkout script keeps its own ao resolution
	// (PATH → build-from-cli/) for this repo's own workflow.
	if repoRoot, err := resolveAgentsRepoRoot(); err == nil && aoBinaryInside(repoRoot) {
		script := filepath.Join(repoRoot, defaultReceiptsScript)
		if _, statErr := os.Stat(script); statErr != nil {
			return fmt.Errorf("membrane-receipts generator not found at %s: %w", script, statErr)
		}
		return runForwardedPawlScript(cmd, script, repoRoot, "", nil, nil)
	}
	// Stranger path: not a genuine AgentOps checkout (installed ao, or forged
	// markers). Run the EMBEDDED generator against the user's OWN git repo.
	return runVerifyReceiptsEmbedded(cmd)
}

// runVerifyReceiptsEmbedded materializes the embedded pawl bundle and runs the
// receipts generator against the user's git repo. It reuses pawlReviewColdEnv — the
// SAME sanitized stranger-path env as `ao pawl review`: PATH stripped of
// repo-controlled entries, BASH_ENV/ENV neutralized, and AO_BIN pinned to the
// trusted running binary. That pin means no ao-on-PATH is needed AND the generator's
// chain-verify never resolves or builds ao from the untrusted repo — its
// PAWL_UNTRUSTED_REPO=1 signal makes the build-from-cli/ fallback fail-closed.
func runVerifyReceiptsEmbedded(cmd *cobra.Command) error {
	startDir, err := resolveProjectDir()
	if err != nil {
		return err
	}
	userRoot, err := gitToplevel(startDir)
	if err != nil {
		return fmt.Errorf("ao verify receipts must run inside a git repository (it renders that repo's proof page): %w", err)
	}
	cacheDir, cleanup, err := extractPawlBundle()
	if err != nil {
		return fmt.Errorf("preparing embedded receipts generator: %w", err)
	}
	defer cleanup()
	script := filepath.Join(cacheDir, "scripts", filepath.Base(defaultReceiptsScript))
	if _, statErr := os.Stat(script); statErr != nil {
		return fmt.Errorf("embedded receipts generator missing from bundle at %s: %w", script, statErr)
	}
	return runForwardedPawlScript(cmd, script, userRoot, userRoot, nil, pawlReviewColdEnv(userRoot))
}
