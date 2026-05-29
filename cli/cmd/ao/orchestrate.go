// practices: [hexagonal-architecture, safe-degradation]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// orchestrateCmd is the parent for orchestration-backend tooling. It wires
// the library-only OrchestrationPort (internal/orchestration + the typed
// port in internal/ports) into the live `ao` command surface.
var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate",
	Short: "Resolve and inspect the orchestration backend ladder",
	Long: `Tooling for the orchestration safe-degradation ladder
(NTM -> Claude-native -> beads floor). Subcommands resolve which backend
a unit of work would run on, honoring an explicit pin, the
AGENTOPS_ORCHESTRATION env override, and an opt-out to the beads floor.`,
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
	orchestrateSelectCmd.Flags().BoolVar(&orchestrateSelectJSON, "json", false,
		"Emit the selection trace as JSON")
	orchestrateSelectCmd.Flags().StringVar(&orchestrateSelectPin, "pin", "",
		"Force a backend: ntm|claude|codex|beads (overrides --opt-out and availability)")
	orchestrateSelectCmd.Flags().BoolVar(&orchestrateSelectOptOut, "opt-out", false,
		"Bypass swarm engines and run on the beads floor")
	_ = orchestrateSelectCmd.RegisterFlagCompletionFunc("pin",
		staticCompletionFunc("ntm", "claude", "codex", "beads"))
}

// execCommandRunner is the production CommandRunner adapter: it shells out
// via os/exec so ProbeNTM actually invokes `ntm --robot-capabilities`. It
// is a thin consumer of the orchestration package and adds no behavior of
// its own.
type execCommandRunner struct{}

// Run executes name with args and returns the combined output. A non-zero
// exit (or a missing binary) surfaces as an error, which ProbeNTM reads as
// the canonical "tool absent or unusable" degradation signal.
func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// compile-time assertion that the adapter satisfies the probe's contract.
var _ orchestration.CommandRunner = execCommandRunner{}

// workSpecFromFlags maps the command's flag values onto a port WorkSpec.
// It is split out from the cobra plumbing so the flag->intent mapping can
// be unit-tested without constructing a command.
func workSpecFromFlags(pin string, optOut bool) ports.WorkSpec {
	return ports.WorkSpec{
		OptOut: optOut,
		Pin:    ports.Backend(strings.TrimSpace(pin)),
	}
}

// runOrchestrateSelect builds the production Selector over an exec-backed
// runner and resolves the backend for the flag-derived WorkSpec.
func runOrchestrateSelect(cmd *cobra.Command, _ []string) error {
	selector := orchestration.NewSelector(execCommandRunner{})
	work := workSpecFromFlags(orchestrateSelectPin, orchestrateSelectOptOut)

	trace, err := selector.Select(cmd.Context(), work)
	if err != nil {
		return fmt.Errorf("selecting orchestration backend: %w", err)
	}

	return emitSelectionTrace(cmd, trace, orchestrateSelectJSON)
}

// emitSelectionTrace renders a SelectionTrace as JSON (when jsonOut) or as
// a human-readable summary. Kept separate so both branches are testable
// against an injected writer.
func emitSelectionTrace(cmd *cobra.Command, trace ports.SelectionTrace, jsonOut bool) error {
	out := cmd.OutOrStdout()
	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(trace)
	}

	fmt.Fprintf(out, "Backend: %s\n", trace.Chosen)
	fmt.Fprintf(out, "Reason:  %s\n", trace.Reason)
	considered := make([]string, 0, len(trace.Considered))
	for _, b := range trace.Considered {
		considered = append(considered, string(b))
	}
	fmt.Fprintf(out, "Ladder:  %s\n", strings.Join(considered, " -> "))
	return nil
}
