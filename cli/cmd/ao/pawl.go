// practices: [design-by-contract, code-complete]
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var pawlCmd = &cobra.Command{
	Use:   "pawl",
	Short: "Cross-family membrane review and verdict tooling (the in-repo acceptance pawl)",
	Long: `The pawl is AgentOps's acceptance gate: a change reaches "done" only with an
INDEPENDENT cross-family verdict (never the author, never the same model). 'ao pawl
review' runs that review and, on CONFIRMED, writes the commit-bound verdict the
push-to-main gate enforces.`,
	Args: cobra.NoArgs,
}

// pawlReviewExitError carries scripts/pawl-review.sh's exit code so it propagates
// VERBATIM through ao (the exit code IS the verdict, like ao plan-pawl / ao validate):
// 0 CONFIRMED+written · 3 REFUTED · 4 --converge advisory-only (no lineage) · 2 usage · 1 hard error.
type pawlReviewExitError struct{ code int }

func (e *pawlReviewExitError) Error() string { return "" }

// ExitCode returns the process exit code this verdict maps to.
func (e *pawlReviewExitError) ExitCode() int { return e.code }

const defaultPawlReviewScript = "scripts/pawl-review.sh"

var pawlReviewCmd = &cobra.Command{
	Use:   "review <bead-id> [--scope head|staged] [--converge] [--author-family <fam>] [--context <s>]",
	Short: "Run the cross-family (codex) membrane review; on CONFIRMED write the commit-bound verdict",
	Long: `Wrap scripts/pawl-review.sh and surface it on the ao CLI. Dispatches the codex
refuter against the commit and, on CONFIRMED, writes + verifies the commit-bound pawl
verdict the pre-push gate requires (REFUTED prints the defects + exits 3; --converge is
advisory-only without adversarial lineage and exits 4). LAW 0: the refuter is codex (a
cross-family reviewer), never a same-model self-review. All arguments after 'review' are
forwarded verbatim to the script.`,
	// Forward all flags verbatim to the script (it owns the flag contract).
	DisableFlagParsing: true,
	RunE:               runPawlReview,
}

func init() {
	rootCmd.AddCommand(pawlCmd)
	pawlCmd.AddCommand(pawlReviewCmd)
}

func runPawlReview(cmd *cobra.Command, args []string) error {
	// -h/--help is for THIS command, not the script.
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return cmd.Help()
		}
	}
	repoRoot, err := resolveAgentsRepoRoot()
	if err != nil {
		return err
	}
	script := filepath.Join(repoRoot, defaultPawlReviewScript)
	if _, statErr := os.Stat(script); statErr != nil {
		return fmt.Errorf("pawl-review script not found at %s: %w", script, statErr)
	}

	c := exec.Command("bash", append([]string{script}, args...)...) // #nosec G204 -- args are operator-supplied pawl flags forwarded to a fixed in-repo script.
	c.Dir = repoRoot
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	runErr := c.Run()
	if runErr == nil {
		return nil
	}
	// The script's exit code is the verdict — propagate it verbatim, with no extra
	// cobra usage/error noise (the script already printed the verdict + defects).
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return &pawlReviewExitError{code: exitErr.ExitCode()}
	}
	return runErr
}
