// practices: [dora-metrics, lean-startup]
package main

import (
	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

var migrateToMD bool

func init() {
	migrateCmd := &cobra.Command{
		Use:     "migrate",
		Short:   "Migrate goals to latest format",
		Aliases: []string{"mg"},
		GroupID: "management",
		Long: `Migrate goals between formats.

Without flags, migrates GOALS.yaml from version 1 to version 2:
  - Sets version to 2
  - Adds mission field if missing
  - Sets goal type to "health" for goals without a type
  - Backs up original to GOALS.yaml.v1.bak

With --to-md, converts GOALS.yaml to GOALS.md (version 4):
  - Carries over mission and all gates
  - Groups goals by pillar to generate directives
  - Adds default north/anti stars
  - Preserves original YAML file

Examples:
  ao goals migrate                       # v1 YAML → v2 YAML
  ao goals migrate --to-md               # YAML → GOALS.md
  ao goals migrate --to-md --file g.yaml # Custom source file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return goals.RunMigrate(goals.MigrateOptions{
				ToMD:      migrateToMD,
				GoalsFile: resolveGoalsFile(),
			})
		},
	}
	migrateCmd.Flags().BoolVar(&migrateToMD, "to-md", false, "Convert GOALS.yaml to GOALS.md format")
	goalsCmd.AddCommand(migrateCmd)
}

// directivesFromPillars delegates to goals.DirectivesFromPillars (used by tests).
func directivesFromPillars(gs []goals.Goal) []goals.Directive {
	return goals.DirectivesFromPillars(gs)
}
