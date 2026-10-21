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

// Command builds the native demo and its explicitly selected RPI alternative.
func (Module) Command() *cobra.Command {
	var (
		quick    bool
		concepts bool
		rpi      bool
	)
	command := &cobra.Command{
		Use:   "demo",
		Short: "Show native execution with optional workflow examples",
		Long: `Show a native coding-agent change from accepted behavior through checks,
CI and delivery under repository policy, with one fresh read only where a
mistake is costly.

No skills or bootstrap are required. Specialists and evidence persistence are
optional. Use --rpi to show the full workflow-skill alternative.
The repository keeps its own Git, CI, tracker, release, and delivery policy.`,
		GroupID: "start",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if concepts {
				return showConcepts(cmd.OutOrStdout())
			}
			if rpi {
				return rpiDemo(cmd.OutOrStdout())
			}
			return quickDemo(cmd.OutOrStdout())
		},
	}
	command.Flags().BoolVar(&quick, "quick", false, "show the compact native example (the default)")
	command.Flags().BoolVar(&concepts, "concepts", false, "explain the product boundary")
	command.Flags().BoolVar(&rpi, "rpi", false, "show the optional full RPI workflow")
	command.MarkFlagsMutuallyExclusive("concepts", "rpi")
	return command
}

func showConcepts(w io.Writer) error {
	fmt.Fprintln(w, `AGENTOPS PRODUCT BOUNDARY

The native coding agent owns the approach and completes accepted work using
repository instructions, ordinary tools and deterministic checks. Checks and CI
gate an ordinary change; obtain one fresh author-distinct read only when the
caller asks, a mistake cannot be cheaply undone, or no deterministic check
covers the change. Using the Validate skill for that read is optional. No
workflow skills or bootstrap are required. Machine-readable verdict persistence
is optional.

AgentOps does not own retries, budgets, queues, work ownership, Git, closure, release,
or delivery. Specialists and multi-agent strategies are optional callers.`)
	return nil
}

func quickDemo(w io.Writer) error {
	fmt.Fprintln(w, `AGENTOPS NATIVE DEMO

Request: a parser must reject an empty value while preserving valid inputs.
1. Read repository instructions and the parser's accepted behavior.
2. Add a discriminating regression, reproduce the failure, and repair the parser
   with native coding-agent tools. Run the required package and repository checks.
3. Use ao gate check for deterministic facts; repair understood failures directly.
4. Checks and CI gate an ordinary change; obtain one fresh author-distinct read
   only when the caller asks, a mistake cannot be cheaply undone, or no
   deterministic check covers the change. That read checks the exact final change
   against unchanged acceptance; the author cannot issue a binding PASS.
5. Report the outcome and checked/not_checked scope, then follow repository delivery
   policy. A requested binding verdict reports missing proof or unchecked
   acceptance as NOT_PROVEN.

No skill installation, workflow-skill loading, bootstrap or ao init is required.
Specialists and evidence persistence are optional. The Validate skill can help
conduct a fresh read; ao provenance can record explicitly requested proof.
Use ao demo --rpi for the optional full workflow example.`)
	return nil
}

func rpiDemo(w io.Writer) error {
	fmt.Fprintln(w, `AGENTOPS RPI DEMO

1. Plan refines one active behavior and write scope in the existing intent source.
2. Implement runs one bounded RED -> GREEN -> refactor experiment.
3. The runtime derives changed paths, check receipts, and subject-manifest.v1.
4. Checks and CI gate an ordinary change; obtain one fresh author-distinct read
   only when the caller asks, a mistake cannot be cheaply undone, or no
   deterministic check covers the change.
5. That read is one round: Validate returns one fresh validation result over the
   exact intent and subject; verdict.v2 persistence is optional.
6. Repair findings, confirm each with a check; a repair does not start another review.
   RPI reports done, stopped, NOT_PLANNED or NOT_BUILT, with judgment none, PASS,
   FAIL or NOT_PROVEN.

No Git repository or ao binary is required for this semantic traversal.`)
	return nil
}
