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

func executeBeadsExec(cmd *cobra.Command, args []string) error {
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
		err = &beadsVerdictError{code: adapterExit.ExitCode()}
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
