// Package demo owns Cobra presentation for the `ao demo` command. The module
// builds its command with constructor-scoped flag state and renders static
// explanatory text, performing no filesystem, process, or clock effect.
package demo

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/clicontract"
)

// Module owns Cobra presentation for the demo command.
type Module struct{}

// NewModule constructs the demo command module. Demo reads no ambient CLI
// seams and performs no effect.
func NewModule() Module {
	return Module{}
}

// Contract declares demo's real behavior for the family architecture gate: it
// accepts (and ignores) arbitrary positional args exactly as Cobra does today,
// emits static text, is a pure render with no effect, and exits 0 on success or
// 1 on an output-write failure. The demo family attached no capabilities
// contract before the carve-out, so the composition does not attach this one
// either.
func (Module) Contract() clicontract.CommandContract {
	return clicontract.CommandContract{
		ID:       "ao.demo",
		Profiles: clicontract.ProfileDefault | clicontract.ProfileLegacy | clicontract.ProfileCombined,
		Args:     clicontract.ArgsPolicy{Name: "arbitrary", Validate: cobra.ArbitraryArgs},
		Output:   clicontract.OutputText,
		Effects:  clicontract.EffectPure,
		ExitClasses: map[int]clicontract.ExitClass{
			0: clicontract.ExitSuccess,
			1: clicontract.ExitFailure,
		},
	}
}

// Command builds the `ao demo` command with constructor-scoped --quick and
// --concepts flags.
func (Module) Command() *cobra.Command {
	var (
		quick    bool
		concepts bool
	)
	command := &cobra.Command{
		Use:   "demo",
		Short: "Show one RPI traversal",
		Long: `Show the AgentOps product boundary:

  RPI -> Plan -> Implement -> fresh Validate -> report and stop

The repository keeps its own Git, CI, tracker, release, and delivery policy.`,
		GroupID: "start",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if concepts {
				return showConcepts(cmd.OutOrStdout())
			}
			return quickDemo(cmd.OutOrStdout())
		},
	}
	command.Flags().BoolVar(&quick, "quick", false, "show the compact one-pass example")
	command.Flags().BoolVar(&concepts, "concepts", false, "explain the product boundary")
	return command
}

func showConcepts(w io.Writer) error {
	fmt.Fprintln(w, `AGENTOPS PRODUCT BOUNDARY

AgentOps shapes one behavior, runs one bounded implementation experiment,
obtains one fresh independent judgment over exact content, reports it, and
stops. Machine-readable verdict persistence is optional and consumer-driven.

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
5. Validate returns one fresh validation result; verdict.v2 persistence is optional.
6. RPI reports PASS, FAIL, NOT_PROVEN, NOT_PLANNED, or NOT_BUILT and stops.

No Git repository or ao binary is required for this semantic traversal.`)
	return nil
}
