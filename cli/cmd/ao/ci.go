// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// ciCmd is the parent for BC2 Validation CLI surfaces. Slice 2 of
// soc-y5vh.5 (cycle 145): exposes productionCIStatus (cycle 117) to
// scripts and operators. Companion to cycle 144's `ao loop`.
var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "BC2 CI status operations (latest run by SHA, recent runs)",
	Long:  `Operations on CI run history via the typed BC2 CIStatusPort. The 'latest' subcommand wraps 'gh run list --commit <sha>' through productionCIStatus; 'recent' wraps the unbound-by-sha variant.`,
}

var ciLatestCmd = &cobra.Command{
	Use:   "latest <sha>",
	Short: "Get the most recent CI run for a given SHA via BC2 CIStatusPort",
	Long: `Get the most recent CI run for a given commit SHA via the typed
BC2 CIStatusPort. Wraps 'gh run list --commit <sha> --json ...'
behind productionCIStatus.

Emits one JSON object (or empty for no run). Useful as a typed
replacement for inline gh shell-outs in /evolve Step 1.5 (healing-
first classifier) and similar consumers.

Examples:
  ao ci latest abc123def    # latest run for that SHA
  ao ci latest HEAD          # if your shell resolves HEAD first`,
	Args: cobra.ExactArgs(1),
	RunE: runCILatest,
}

var ciRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "List recent CI runs via BC2 CIStatusPort",
	Long: `List recent CI runs (any SHA) via productionCIStatus. Default
limit is 10; cap at 50 (matches the port contract's adapter cap).

Emits one JSON object per run.

Examples:
  ao ci recent            # last 10 runs
  ao ci recent --limit 5  # last 5
  ao ci recent --limit 0  # all available (capped at 50)`,
	RunE: runCIRecent,
}

type ciStatusOptions struct {
	sha    string
	limit  int
	writer io.Writer
	// statusFn lets tests substitute the port without calling gh
	statusFn func(ctx context.Context, opts ciStatusOptions) ([]ports.CIRun, error)
}

func init() {
	ciCmd.GroupID = "core"
	rootCmd.AddCommand(ciCmd)

	ciCmd.AddCommand(ciLatestCmd)

	ciRecentCmd.Flags().Int("limit", 10, "max runs to emit (0 = all up to port cap of 50)")
	ciCmd.AddCommand(ciRecentCmd)
}

func runCILatest(cmd *cobra.Command, args []string) error {
	return ciStatusRun(cmd.Context(), ciStatusOptions{
		sha:    args[0],
		writer: cmd.OutOrStdout(),
	})
}

func runCIRecent(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	return ciStatusRun(cmd.Context(), ciStatusOptions{
		limit:  limit,
		writer: cmd.OutOrStdout(),
	})
}

func ciStatusRun(ctx context.Context, opts ciStatusOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	statusFn := opts.statusFn
	if statusFn == nil {
		statusFn = ciStatusViaPort
	}
	runs, err := statusFn(ctx, opts)
	if err != nil {
		return fmt.Errorf("ci status: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	for _, r := range runs {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("ci status encode: %w", err)
		}
	}
	return nil
}

// ciStatusViaPort wires productionCIStatus (cycle 117) to gh. If
// opts.sha is non-empty, calls Latest; otherwise Recent.
func ciStatusViaPort(ctx context.Context, opts ciStatusOptions) ([]ports.CIRun, error) {
	c := newProductionCIStatus()
	if opts.sha != "" {
		run, err := c.Latest(ctx, opts.sha)
		if err != nil {
			return nil, err
		}
		if run.Sha == "" && run.Workflow == "" {
			return []ports.CIRun{}, nil
		}
		return []ports.CIRun{run}, nil
	}
	return c.Recent(ctx, opts.limit)
}
