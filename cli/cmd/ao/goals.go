// practices: [dora-metrics, lean-startup]
package main

import (
	"os"

	"github.com/spf13/cobra"
)

var goalsCmd = &cobra.Command{
	Use:   "goals",
	Short: "Fitness goal measurement and validation",
	Args:  cobra.NoArgs,
	Long: `Track, measure, and validate project fitness goals.

Supports both GOALS.yaml (versions 1-3) and GOALS.md (version 4) formats.
When both exist, GOALS.md takes precedence.

Measurement:
  measure (m)   Run goal checks and produce a snapshot
  validate (v)  Validate goals structure and wiring

Analysis:
  drift (d)     Compare snapshots for regressions
  history (h)   Show goal measurement history
  export (e)    Export latest snapshot as JSON

Management:
  init          Bootstrap a new GOALS.md interactively
  add (a)       Add a new goal
  steer         Manage directives (add/remove/prioritize)
  prune (p)     Remove stale gates
  migrate (mg)  Migrate between formats
  meta          Run and report meta-goals only`,
}

const defaultGoalsTimeoutSeconds = 240

// Shared flags
var (
	goalsFile    string // --file, auto-detects GOALS.md then GOALS.yaml
	goalsTimeout int    // --timeout in seconds, default defaultGoalsTimeoutSeconds
)

// goalsJSONOutput reports whether the goals family should emit JSON. It reads
// the global -o/--output flag (set to "json" by either -o json or --json) so
// the goals subcommands honor the same output flag as the rest of the CLI
// instead of a disconnected local --json bool.
func goalsJSONOutput() bool {
	return GetOutput() == "json"
}

func init() {
	goalsCmd.AddGroup(
		&cobra.Group{ID: "measurement", Title: "Measurement:"},
		&cobra.Group{ID: "analysis", Title: "Analysis:"},
		&cobra.Group{ID: "management", Title: "Management:"},
	)
	goalsCmd.PersistentFlags().StringVar(&goalsFile, "file", "", "Path to goals file (auto-detects GOALS.md then GOALS.yaml)")
	goalsCmd.PersistentFlags().IntVar(&goalsTimeout, "timeout", defaultGoalsTimeoutSeconds, "Check timeout in seconds")
	goalsCmd.GroupID = "workflow"
	rootCmd.AddCommand(goalsCmd)
}

// resolveGoalsFile returns the goals file path, auto-detecting if not explicitly set.
func resolveGoalsFile() string {
	if goalsFile != "" {
		return goalsFile
	}
	// Prefer GOALS.md (v4), fall back to GOALS.yaml
	if info, err := os.Stat("GOALS.md"); err == nil && !info.IsDir() {
		return "GOALS.md"
	}
	if info, err := os.Stat("GOALS.yaml"); err == nil && !info.IsDir() {
		return "GOALS.yaml"
	}
	return "GOALS.md" // Default for new projects
}
