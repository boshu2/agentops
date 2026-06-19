package main

import (
	"fmt"
	"os"
	"time"

	"github.com/boshu2/agentops/cli/internal/worktree"
	"github.com/spf13/cobra"
)

var worktreeCreateJSON bool

func init() {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an isolated per-session worktree for concurrency-safe work",
		Long: `Create a detached git worktree off the current commit so concurrent
sessions never share one mutable working tree — the failure mode where another
session's reset/checkout silently wipes your uncommitted work. Prints the new
worktree path; cd into it to work, then push from there.

Pairs with 'ao orchestrate preflight' (the worktree_isolation admission check)
and the NTM/swarm substrate: each fanned-out agent gets its own worktree, and
they coordinate only at the durable boundary (origin/main, the bead ledger,
agent-mail) — never through a shared working tree.`,
		Args: cobra.NoArgs,
		RunE: runWorktreeCreate,
	}
	cmd.Flags().BoolVar(&worktreeCreateJSON, "json", false, "Emit {path, run_id} as JSON")
	worktreeCmd.AddCommand(cmd)
}

func runWorktreeCreate(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, runID, err := worktree.CreateWorktree(cwd, 60*time.Second, nil)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	if worktreeCreateJSON {
		fmt.Fprintf(cmd.OutOrStdout(), "{\"path\":%q,\"run_id\":%q}\n", path, runID)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	return nil
}
