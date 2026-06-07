// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	// Register the seed checks the refinery's full gate runs.
	_ "github.com/boshu2/agentops/cli/internal/gates/checks"
	"github.com/boshu2/agentops/cli/internal/refinery"
)

// `ao refinery` is the bushido continuous-validation backstop (ag-qidx P2): it
// watches main, runs the full gate on each new commit, and on a DETERMINISTIC
// blocking failure raises a poison beacon + files a fix-bead + alerts. It NEVER
// blind-reverts (the repo's flake rate would make auto-revert fight developers)
// and it is a backstop, not a gatekeeper — if it is down, merges still succeed.

var refineryInterval time.Duration

var refineryCmd = &cobra.Command{
	Use:   "refinery",
	Short: "Continuous main-validation backstop (run on bushido)",
	Long: `Watch main, run the full gate on each new commit, and on a
deterministic blocking failure raise a poison beacon + file a fix-bead. Never
reverts; resumes from .refinery-state across restarts.

  ao refinery once               # evaluate main HEAD once (one tick)
  ao refinery run --interval 5m  # loop until interrupted (systemd service)`,
}

var refineryOnceCmd = &cobra.Command{
	Use:   "once",
	Short: "Evaluate main HEAD once",
	Args:  cobra.NoArgs,
	RunE:  runRefineryOnce,
}

var refineryRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the refinery loop until interrupted",
	Args:  cobra.NoArgs,
	RunE:  runRefineryRun,
}

func init() {
	refineryRunCmd.Flags().DurationVar(&refineryInterval, "interval", 5*time.Minute, "poll interval")
	refineryCmd.AddCommand(refineryOnceCmd, refineryRunCmd)
	rootCmd.AddCommand(refineryCmd)
}

func runRefineryOnce(cmd *cobra.Command, _ []string) error {
	root, err := gateRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	res, err := refinery.NewProduction(root).RunOnce(cmd.Context())
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	switch {
	case res.Skipped:
		fmt.Fprintf(out, "refinery: %s unchanged — nothing to do\n", res.SHA)
	case res.Green:
		fmt.Fprintf(out, "refinery: %s GREEN\n", res.SHA)
	case len(res.Deterministic) > 0:
		fmt.Fprintf(out, "refinery: %s POISONED by %v — fix-bead %s filed (no revert)\n", res.SHA, res.Deterministic, res.FixBead)
	default:
		fmt.Fprintf(out, "refinery: %s had failures but none reproduced (flaky) — not escalated\n", res.SHA)
	}
	return nil
}

func runRefineryRun(cmd *cobra.Command, _ []string) error {
	root, err := gateRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "refinery: looping every %s (Ctrl-C to stop)\n", refineryInterval)
	return refinery.NewProduction(root).Loop(cmd.Context(), refineryInterval)
}
