// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// harnessCmd is the parent for BC5 HarnessPort CLI surfaces. Built
// using the cycle-147 cli-wiring template.
var harnessCmd = &cobra.Command{
	Use:   "harness",
	Short: "BC5 HarnessPort operations (sync state across skill harnesses)",
	Long:  `Inspect the sync state between the canonical skills/ tree and the skills-codex/ mirror via the typed BC5 HarnessPort. Useful as a typed alternative to scripts/audit-codex-parity.sh for drift detection.`,
}

var harnessStatusCmd = &cobra.Command{
	Use:   "status [--skill <name>] [--out-of-sync-only]",
	Short: "Emit (skill, harness) sync state via BC5 HarnessPort",
	Long: `Emit HarnessSkillSync entries via the typed BC5 HarnessPort
(productionHarness, cycle 111). Each entry names one (skill, harness)
pair with its SHA-256 content hash and OutOfSync flag.

Useful as a drift-detection surface that future audit gates can
consume programmatically instead of parsing audit-codex-parity.sh
output.

Examples:
  ao harness status                       # all (skill, harness) pairs
  ao harness status --skill evolve        # only the 'evolve' skill
  ao harness status --out-of-sync-only    # only entries with OutOfSync=true`,
	RunE: runHarnessStatus,
}

type harnessStatusOptions struct {
	skill         string
	outOfSyncOnly bool
	writer        io.Writer
	statusFn      func(ctx context.Context, opts harnessStatusOptions) ([]ports.HarnessSkillSync, error)
}

func init() {
	harnessCmd.GroupID = "core"
	rootCmd.AddCommand(harnessCmd)

	harnessStatusCmd.Flags().String("skill", "", "filter to one skill name (empty = all)")
	harnessStatusCmd.Flags().Bool("out-of-sync-only", false, "emit only entries with OutOfSync=true")
	harnessCmd.AddCommand(harnessStatusCmd)
}

func runHarnessStatus(cmd *cobra.Command, _ []string) error {
	skill, _ := cmd.Flags().GetString("skill")
	oos, _ := cmd.Flags().GetBool("out-of-sync-only")
	return harnessStatusRun(cmd.Context(), harnessStatusOptions{
		skill:         skill,
		outOfSyncOnly: oos,
		writer:        cmd.OutOrStdout(),
	})
}

func harnessStatusRun(ctx context.Context, opts harnessStatusOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.statusFn
	if fn == nil {
		fn = harnessStatusViaPort
	}
	entries, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("harness status: %w", err)
	}
	enc := json.NewEncoder(opts.writer)
	for _, e := range entries {
		if opts.outOfSyncOnly && !e.OutOfSync {
			continue
		}
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("harness status encode: %w", err)
		}
	}
	return nil
}

// harnessStatusViaPort wires productionHarness (cycle 111) rooted at
// the project root. The adapter walks skills/ and skills-codex/
// relative to that root.
func harnessStatusViaPort(ctx context.Context, opts harnessStatusOptions) ([]ports.HarnessSkillSync, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return nil, err
	}
	h := newProductionHarness(cwd)
	if opts.skill != "" {
		return h.StatusForSkill(ctx, opts.skill)
	}
	return h.Status(ctx)
}
