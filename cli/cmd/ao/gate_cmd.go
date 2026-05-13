// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// gateCmd is the parent for BC2 GateRunnerPort CLI surfaces. Built
// using the cycle-147 cli-wiring template.
//
// Note: `gate` already existed as a top-level command before this
// session (human review gates). The cobra surface accepts subcommand
// addition without conflict — `ao gate run <name>` is the new BC2
// surface; the prior `ao gate ...` subcommands stay unaffected.
var gateRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a check-*.sh gate via BC2 GateRunnerPort and emit verdict",
	Long: `Invoke a check-*.sh gate via the typed BC2 GateRunnerPort
(productionGateRunner, cycle 115) and emit a GateVerdict (JSON).

The <name> argument is the gate name without the 'check-' prefix
or '.sh' suffix — productionGateRunner resolves to
scripts/check-<name>.sh.

Useful as a typed alternative to 'bash scripts/check-<name>.sh; echo $?'
for scripts that want structured output (status, reason, log tail).

Examples:
  ao gate run compile-health           # runs scripts/check-compile-health.sh
  ao gate run three-gap-supergate      # runs the supergate
  ao gate run xxx-does-not-exist        # emits UNKNOWN status`,
	Args: cobra.ExactArgs(1),
	RunE: runGateRun,
}

type gateRunOptions struct {
	name   string
	writer io.Writer
	runFn  func(ctx context.Context, opts gateRunOptions) (ports.GateVerdict, error)
}

func init() {
	// 'gate' parent already exists in cli/cmd/ao/gate.go; just add
	// the 'run' subcommand. Use existing gateCmd from that file.
	gateCmd.AddCommand(gateRunCmd)
}

func runGateRun(cmd *cobra.Command, args []string) error {
	return gateRunRun(cmd.Context(), gateRunOptions{
		name:   args[0],
		writer: cmd.OutOrStdout(),
	})
}

func gateRunRun(ctx context.Context, opts gateRunOptions) error {
	if opts.name == "" {
		return errors.New("gate run: name required")
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.runFn
	if fn == nil {
		fn = gateRunViaPort
	}
	verdict, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("gate run: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	if err := enc.Encode(verdict); err != nil {
		return fmt.Errorf("gate run encode: %w", err)
	}
	return nil
}

// gateRunViaPort wires productionGateRunner (cycle 115) rooted at
// the project root.
func gateRunViaPort(ctx context.Context, opts gateRunOptions) (ports.GateVerdict, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return ports.GateVerdict{}, err
	}
	g := newProductionGateRunner(cwd)
	return g.Run(ctx, ports.GateRunRequest{Name: ports.GateName(opts.name)})
}
