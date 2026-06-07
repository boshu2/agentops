// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"github.com/boshu2/agentops/cli/internal/adapters/sessionspawn"
	"github.com/spf13/cobra"
)

var (
	spawnDryRun bool
	spawnNoTmux bool
	spawnDate   string
)

var sessionSpawnCmd = &cobra.Command{
	Use:   "spawn <template-path>",
	Short: "Cold-start a session from a TOML template",
	Long: `Read a session template, expand variables, run init steps, and create
a tmux session with the configured panes.

The template defines the session role, init steps (context loading, handoff
replay, bead ownership scan), tmux layout, heartbeat cadence, and exit hooks.

Examples:
  ao session spawn ~/.agentops/sessions/claude-validator.toml
  ao session spawn ~/.agentops/sessions/claude-validator.toml --dry-run
  ao session spawn ~/.agentops/sessions/claude-validator.toml --no-tmux`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionSpawn,
}

func init() {
	sessionsCmd.AddCommand(sessionSpawnCmd)
	sessionSpawnCmd.Flags().BoolVar(&spawnDryRun, "dry-run", false, "Print expanded template and init steps without executing")
	sessionSpawnCmd.Flags().BoolVar(&spawnNoTmux, "no-tmux", false, "Run init steps but skip tmux session creation")
	sessionSpawnCmd.Flags().StringVar(&spawnDate, "date", "", "Override date for template expansion (default: today, YYYY-MM-DD)")
}

func runSessionSpawn(cmd *cobra.Command, args []string) error {
	return sessionspawn.Run(sessionspawn.Options{
		TemplatePath: args[0],
		DryRun:       spawnDryRun,
		NoTmux:       spawnNoTmux,
		Date:         spawnDate,
		Out:          cmd.OutOrStdout(),
	})
}
