// `ao beads exec [args...]` is the one tracker-agnostic CRUD passthrough.
// Most users track with bd; this repository tracks with br. The adapter keeps
// their intentional ledger, child-enumeration, and JSON-shape differences.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

var beadsExecCmd = &cobra.Command{
	Use:   "exec [args...]",
	Short: "Run a bead CRUD command against whichever tracker (bd or br) this environment uses",
	Long: `Forward a bead command verbatim to the resolved beads tracker (bd or br),
setting that tracker's ledger correctly and propagating its exit code unchanged.

This is the ONE tracker-agnostic entry point: skills and callers use
'ao beads exec <verb> ...' instead of hardcoding 'br' or 'bd'. The tracker is
resolved exactly as 'ao beads tracker' reports it (AGENTOPS_TRACKER > config >
ledger > binary). Examples (all work against either tracker):

  ao beads exec ready
  ao beads exec close <id> -r "Done"
  ao beads exec update <id> --status in_progress
  ao beads exec create "title" --type task
  ao beads exec list --json

Ledger wiring:
  br — BEADS_DIR is set to the resolved ledger dir (worktree-aware).
  bd — the child runs from the repo root so .beads/ auto-discovery resolves;
       no BEADS_DIR is set.

Children (hard divergence — bd has 'children', br does not):
  ao beads exec children <epic>
       bd: forwarded to 'bd children <epic>' (bd's native list output).
       br: synthesized from 'br show <epic> --json' — the id of every dependent
           with a parent-child edge, one per line.

All flags are forwarded to the tracker verbatim (flag parsing is disabled), so
tracker flags never collide with ao's. Only -h/--help is intercepted here.`,
	DisableFlagParsing: true,
	RunE:               runBeadsExec,
}

func init() {
	beadsCmd.AddCommand(beadsExecCmd)
}

func runBeadsExec(cmd *cobra.Command, args []string) error {
	for _, argument := range args {
		if argument == "--help" || argument == "-h" {
			return cmd.Help()
		}
	}
	err := currentBeadsExecutor().Execute(context.Background(), args, beadsapp.ExecStreams{
		Stdin:  cmd.InOrStdin(),
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
	})
	var adapterExit *beadsapp.ExitError
	if errors.As(err, &adapterExit) {
		err = &beadsExitError{code: adapterExit.ExitCode()}
	}
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	}
	return err
}

func currentBeadsExecutor() *beadsadapter.Executor {
	return beadsadapter.NewExecutor(currentBeadsTracker())
}

// These pure compatibility delegates remain until the yield family moves its
// tracker-child formatting onto the shared application policy.
func beadsExecChildEnv(res trackerResolution, _ string) []string {
	return beadsapp.ChildEnvironment(os.Environ(), beadsapp.TrackerResolution{Tracker: res.Tracker, LedgerDir: res.LedgerDir})
}

func beadsExecChildDir(res trackerResolution, cwd string) string {
	return beadsapp.ChildDirectory(beadsapp.TrackerResolution{Tracker: res.Tracker, LedgerDir: res.LedgerDir}, cwd)
}

// Kept as a package-main test seam until the legacy white-box tests move with
// their final owner.
func canonicalizeBDReadJSON(verb string, raw []byte) ([]byte, error) {
	return beadsapp.CanonicalizeBDReadJSON(verb, raw)
}
