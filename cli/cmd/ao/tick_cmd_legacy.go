//go:build legacy

// practices: [design-by-contract, hexagonal-architecture]
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// The `ao tick` RPI/factory command surface (ADR-0012, age-h4y3). Archived behind
// //go:build legacy: the default `ao` binary omits the `tick` command and its
// subcommands. The tick ENGINE (tickRuntime, newTickRuntime, tickPassthrough,
// tickSmoke, tickVerdictIdentity, the verdict/council gates, tickExitCouncil …)
// stays UNTAGGED in tick.go because the spine consumes it (ao claim, ao converge,
// ao eval chaos, converge_canary). The sibling top-level commands defined in
// tick.go (ready, close, verdict-gate, council-gate, install-guards,
// guard-status, chaos-test) are spine commands and also stay. Only the operator
// `tick` navigation surface is archived here.

var tickCmd = &cobra.Command{
	Use:   "tick",
	Short: "Typed port of the assured loop tick oracle",
	Long: `Run the typed AgentOps port of the control-plane tick helper.

The shell oracle remains the regression source until conformance proves the Go
surface. This command preserves the same state-store boundary: br is the work
bus, git is the durable ledger, and only explicit scoped paths are staged.`,
}

var tickNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Print the next ready bead id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rt := newTickRuntime(cmd)
		id := tickNextReady(rt)
		if id != "" {
			fmt.Fprintln(rt.stdout, id)
		}
		return nil
	},
}

var tickStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print ready-work status or convergence state",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickStatus(newTickRuntime(cmd))
	},
}

var tickClaimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a bead through br",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickTrackerPassthrough(newTickRuntime(cmd), "update", args[0], "--claim")
	},
}

var tickReopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a bead after failed validation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickTrackerPassthrough(newTickRuntime(cmd), "update", args[0], "--status", "open")
	},
}

var tickCloseCmd = &cobra.Command{
	Use:   "close <id> <commit-message> <evidence-ref> [paths...]",
	Short: "Close a bead and persist the explicit ledger/evidence paths",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickClose(newTickRuntime(cmd), args[0], args[1], args[2], args[3:])
	},
}

var tickVerdictGateCmd = &cobra.Command{
	Use:   "verdict-gate <file|->",
	Short: "Reject verdicts without commands and independent judge identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickRunVerdictGate(newTickRuntime(cmd), args[0])
	},
}

var tickCouncilGateCmd = &cobra.Command{
	Use:   "council-gate <verdict1> <verdict2> [...]",
	Short: "Fail-closed two-plus judge verdict aggregation",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tickCouncilGate(newTickRuntime(cmd), args)
	},
}

var tickInstallGuardsCmd = &cobra.Command{
	Use:   "install-guards",
	Short: "Install repo-local git guard hooks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickInstallGuards(newTickRuntime(cmd))
	},
}

var tickGuardStatusCmd = &cobra.Command{
	Use:   "guard-status",
	Short: "Verify guard hook and validator launcher installation",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickGuardStatus(newTickRuntime(cmd))
	},
}

var tickSmokeCmd = &cobra.Command{
	Use:   "smoke",
	Short: "Run a read-only smoke test of the tick membrane",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return tickSmoke(newTickRuntime(cmd))
	},
}

func init() {
	tickCmd.GroupID = "workflow"
	rootCmd.AddCommand(tickCmd)
	tickCmd.AddCommand(
		tickNextCmd,
		tickStatusCmd,
		tickClaimCmd,
		tickReopenCmd,
		tickCloseCmd,
		tickVerdictGateCmd,
		tickCouncilGateCmd,
		tickInstallGuardsCmd,
		tickGuardStatusCmd,
		tickSmokeCmd,
	)
}
