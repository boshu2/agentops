// practices: [hexagonal-architecture, safe-degradation]
package main

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/adapters/mto/orchestrationselect"
	"github.com/spf13/cobra"
)

// orchestrateCmd is the parent for orchestration-backend tooling. It wires
// the library-only OrchestrationPort (internal/orchestration + the typed
// port in internal/ports) into the live `ao` command surface.
var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Out-of-session orchestration instruments (windshield)",
	Long: `Instrument lane for out-of-session multi-model orchestration: tools,
preflight, verify, route, status, shape, and backend select. Human atm/am
procedure stays in skills; profiles contract is SOT in docs/contracts/.`,
}

var (
	orchestrateSelectJSON   bool
	orchestrateSelectPin    string
	orchestrateSelectOptOut bool
)

var orchestrateSelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Select the orchestration backend for a unit of work",
	Long: `Resolve the orchestration backend via the safe-degradation ladder
NTM -> Claude-native -> beads floor.

NTM availability is detected by capability — this shells out to
` + "`ntm --robot-capabilities`" + ` and degrades gracefully when ntm is
absent. Resolution order (first match wins):

  1. --pin <ntm|claude|codex|beads>  forces that backend.
  2. AGENTOPS_ORCHESTRATION env       acts as an explicit pin / opt-out.
  3. --opt-out                        routes to the beads floor.
  4. NTM probe reports available      -> ntm.
  5. otherwise                        -> claude (beads floor remains).`,
	RunE: runOrchestrateSelect,
}

func init() {
	orchestrateCmd.GroupID = "workflow"
	rootCmd.AddCommand(orchestrateCmd)
	orchestrateCmd.AddCommand(orchestrateSelectCmd)
	initOrchestrateInstrumentLane()
	orchestrateSelectCmd.Flags().BoolVar(&orchestrateSelectJSON, "json", false,
		"Emit the selection trace as JSON")
	orchestrateSelectCmd.Flags().StringVar(&orchestrateSelectPin, "pin", "",
		"Force a backend: ntm|claude|codex|beads (overrides --opt-out and availability)")
	orchestrateSelectCmd.Flags().BoolVar(&orchestrateSelectOptOut, "opt-out", false,
		"Bypass swarm engines and run on the beads floor")
	_ = orchestrateSelectCmd.RegisterFlagCompletionFunc("pin",
		staticCompletionFunc("ntm", "claude", "codex", "beads"))
}

// runOrchestrateSelect builds the production Selector over an exec-backed
// runner and resolves the backend for the flag-derived WorkSpec.
func runOrchestrateSelect(cmd *cobra.Command, _ []string) error {
	trace, err := orchestrationselect.Select(cmd.Context(), orchestrateSelectPin, orchestrateSelectOptOut)
	if err != nil {
		return fmt.Errorf("selecting orchestration backend: %w", err)
	}

	return orchestrationselect.RenderSelectionTrace(cmd.OutOrStdout(), trace, orchestrateSelectJSON)
}
