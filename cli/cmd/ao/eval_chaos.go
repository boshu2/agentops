// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import "github.com/spf13/cobra"

// evalChaosCmd is the canonical `ao eval chaos` alias for the read-only tick
// membrane smoke test, folded under `ao eval`
// (age-focus-membrane-bookkeeper-m1wg.16). The hidden top-level `ao chaos-test`
// is preserved for back-compat (see tick.go).
var evalChaosCmd = &cobra.Command{
	Use:   "chaos",
	Short: "Run a read-only smoke test of the tick membrane",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickSmoke(newTickRuntime(cmd))
	},
}

func init() {
	evalCmd.AddCommand(evalChaosCmd)
}
