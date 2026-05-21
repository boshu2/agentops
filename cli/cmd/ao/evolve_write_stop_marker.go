// practices: [dora-metrics, lean-startup]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// evolveWriteStopMarker subcommand implements the mechanical no-self-stop
// contract for `ao evolve --mode=loop`. Under loop mode the agent MUST NOT
// write its own DORMANT/STOP/KILL markers; this command refuses with exit 1
// so a self-halting prompt cannot bypass operator-driven control. Burst mode
// preserves the legacy self-regulated behavior and writes the marker file
// under `.agents/evolve/`.
//
// See docs/plans/2026-05-21-evolve-loop-epic-design.md §A1 and
// skills/evolve/references/loop-mode.md for the broader contract.
//
// TODO: wire cli/internal/evolve.preferences.Load() after soc-6svt lands so
// `--mode` can fall back to preferences.yaml `mode_default` instead of the
// burst hard-coded default.
var (
	evolveWriteStopMarkerName   string
	evolveWriteStopMarkerReason string
	evolveWriteStopMarkerMode   string
)

var evolveWriteStopMarkerCmd = &cobra.Command{
	Use:   "write-stop-marker",
	Short: "Write an evolve stop marker (refused under --mode=loop)",
	Long: `Write a DORMANT, STOP, or KILL marker under .agents/evolve/.

Behavior depends on --mode:
  burst (default): writes .agents/evolve/<marker> with --reason content and exits 0.
  loop:            mechanically refuses with exit 1 and a stderr message pointing
                   operators at 'ao evolve operator-stop' for explicit intent.

This is the soc-hwax mechanical no-self-stop contract: under --mode=loop the
agent cannot self-halt, even if its prompt asks it to. Only the operator (or
a future 'ao evolve operator-stop' subcommand) may write a stop marker.`,
	Args: cobra.NoArgs,
	RunE: runEvolveWriteStopMarker,
}

func init() {
	evolveWriteStopMarkerCmd.Flags().StringVar(&evolveWriteStopMarkerName, "marker", "", "Marker name: dormant, stop, or kill")
	evolveWriteStopMarkerCmd.Flags().StringVar(&evolveWriteStopMarkerReason, "reason", "", "Reason text written to the marker file")
	evolveWriteStopMarkerCmd.Flags().StringVar(&evolveWriteStopMarkerMode, "mode", evolveModeBurst, "Execution contract: 'burst' or 'loop' (loop refuses unconditionally)")
	_ = evolveWriteStopMarkerCmd.MarkFlagRequired("marker")
	evolveCmd.AddCommand(evolveWriteStopMarkerCmd)
}

// runEvolveWriteStopMarker is the RunE for `ao evolve write-stop-marker`. It
// enforces the loop-mode refusal before touching the filesystem and wraps any
// IO error with context per repo conventions.
func runEvolveWriteStopMarker(cmd *cobra.Command, _ []string) error {
	mode := detectEvolveWriteStopMarkerMode(cmd)
	if err := validateEvolveMode(mode); err != nil {
		return err
	}
	if mode == evolveModeLoop {
		// Stderr surface is load-bearing: tests and operators key off this
		// string to confirm the loop contract is in force.
		return fmt.Errorf("STOP markers refused under --mode=loop. Use 'ao evolve operator-stop' for explicit operator intent.")
	}

	marker, err := normalizeStopMarkerName(evolveWriteStopMarkerName)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	dir := filepath.Join(cwd, ".agents", "evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create evolve marker dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, marker)
	if err := os.WriteFile(path, []byte(evolveWriteStopMarkerReason), 0o644); err != nil {
		return fmt.Errorf("write marker %s: %w", path, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Wrote evolve marker %s\n", path)
	return nil
}

// detectEvolveWriteStopMarkerMode resolves the effective mode for this
// invocation. It reads the local --mode flag (with burst as the documented
// default). When the soc-6svt preferences loader lands, this is the wiring
// point for the defaults → preferences → CLI resolution order described in
// the epic design memo.
func detectEvolveWriteStopMarkerMode(cmd *cobra.Command) string {
	if cmd == nil {
		return evolveWriteStopMarkerMode
	}
	if flag := cmd.Flag("mode"); flag != nil {
		if v := flag.Value.String(); v != "" {
			return v
		}
	}
	return evolveWriteStopMarkerMode
}

// normalizeStopMarkerName canonicalizes the --marker value to the on-disk
// file name. The contract accepts dormant|stop|kill and emits matching
// uppercase file names to align with the existing Step 1 kill-switch reader.
func normalizeStopMarkerName(name string) (string, error) {
	switch name {
	case "dormant":
		return "DORMANT", nil
	case "stop":
		return "STOP", nil
	case "kill":
		return "KILL", nil
	default:
		return "", fmt.Errorf("--marker must be one of: dormant, stop, kill (got %q)", name)
	}
}
