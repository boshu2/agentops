// practices: [ai-assisted-dev, pragmatic-programmer]
package main

import "github.com/spf13/cobra"

// evalSessionOutcomeCmd is the canonical `ao eval session-outcome` alias for the
// transcript reward-signal analyzer, folded under `ao eval`
// (age-focus-membrane-bookkeeper-m1wg.16). The hidden top-level
// `ao session-outcome` is preserved for back-compat (see session_outcome.go).
// Use "session-outcome" (not "outcomes") to avoid colliding with the existing
// `ao eval outcomes` rubric command.
var evalSessionOutcomeCmd = &cobra.Command{
	Use:   "session-outcome [transcript-path]",
	Short: "Analyze session transcript to derive reward signal",
	Long:  sessionOutcomeCmd.Long,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionOutcome,
}

func init() {
	bindSessionOutcomeFlags(evalSessionOutcomeCmd)
	evalCmd.AddCommand(evalSessionOutcomeCmd)
}
