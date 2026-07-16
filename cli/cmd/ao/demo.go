// practices: [pragmatic-programmer, agile-manifesto]
package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Show the one-pass AgentOps evidence loop",
	Long: `Show the AgentOps product boundary:

  RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report

The repository keeps its own Git, CI, tracker, release, and delivery policy.`,
	RunE: runDemo,
}

var (
	demoQuick    bool
	demoConcepts bool
)

func init() {
	demoCmd.GroupID = "start"
	rootCmd.AddCommand(demoCmd)
	demoCmd.Flags().BoolVar(&demoQuick, "quick", false, "show the compact one-pass example")
	demoCmd.Flags().BoolVar(&demoConcepts, "concepts", false, "explain the product boundary")
}

func runDemo(cmd *cobra.Command, _ []string) error {
	if demoConcepts {
		return showConcepts(cmd.OutOrStdout())
	}
	return quickDemo(cmd.OutOrStdout())
}

func showConcepts(w io.Writer) error {
	fmt.Fprintln(w, `AGENTOPS PRODUCT BOUNDARY

AgentOps shapes one behavior, runs one bounded implementation experiment,
obtains one fresh independent judgment over exact content, persists the verdict,
reports it, and stops.

It does not own retries, budgets, queues, work ownership, Git, closure, release,
or delivery. Learn and multi-agent strategies are optional callers.`)
	return nil
}

func quickDemo(w io.Writer) error {
	fmt.Fprintln(w, `AGENTOPS ONE-PASS DEMO

1. Plan refines one active behavior and write scope in the existing intent source.
2. Implement runs one bounded RED -> GREEN -> refactor experiment.
3. The runtime derives changed paths, check receipts, and subject-manifest.v1.
4. A distinct fresh context validates the exact intent and subject once.
5. Validate atomically stores verdict.v2 under .agentops/verdicts/sha256/.
6. RPI reports PASS, FAIL, NOT_PROVEN, NOT_PLANNED, or NOT_BUILT and stops.

No Git repository or ao binary is required for this semantic loop.`)
	return nil
}
