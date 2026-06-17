// Package main — `ao next-work materialize` subcommand (ag-9jle.3).
//
// Closes the missing half of the BDD-Gherkin-wave loop: lessons -> beads.
// Harvested follow-ups land in .agents/rpi/next-work.jsonl as a queue but never
// become durable beads, so the flywheel executes without compounding into the
// tracker. This command reads unmaterialized items from the queue and creates
// one durable bead per item via `br create`, carrying provenance
// (source_epic + proof_ref) in the bead description.
//
// Design (locked, ag-9jle.3 + handoff 2026-05-30):
//   - CLI owns the deterministic core; skills (post-mortem harvest, crank
//     wave-close) just CALL `ao next-work materialize`.
//   - Idempotent: an item is materialized once. The per-item bead_id field
//     (rpi.NextWorkItem.BeadID) is the back-reference; a set bead_id means the
//     item already has a durable bead and is skipped on re-run.
//   - Provenance rides the br description footer. The actual graph edge is
//     deferred to ag-x31t.4's future `ao provenance add`; this command only
//     records the anchor fields.
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/adapters/mto/nextworkmaterialize"
	"github.com/spf13/cobra"
)

const defaultMaterializedBy = nextworkmaterialize.DefaultMaterializedBy

var (
	nextWorkMaterializeFile       string
	nextWorkMaterializeDryRun     bool
	nextWorkMaterializeJSON       bool
	nextWorkMaterializeSourceEpic string
	nextWorkMaterializeMaterialBy string
)

var nextWorkCmd = &cobra.Command{
	Use:   "next-work",
	Short: "Operate on the harvested next-work queue (.agents/rpi/next-work.jsonl)",
	Long: `Commands that act on the carry-forward next-work queue that /post-mortem
harvests into and /evolve and /rpi loop consume from.`,
}

var nextWorkMaterializeCmd = &cobra.Command{
	Use:   "materialize",
	Short: "Create durable beads from unmaterialized harvested follow-ups",
	Long: `Read .agents/rpi/next-work.jsonl and create one durable bead per
unmaterialized item via 'br create', closing the lessons -> beads half of the
loop so harvested work compounds into the tracker instead of living only in a
queue.

Each created bead carries provenance (source_epic + proof_ref) in its description
footer, plus the labels 'next-work,materialized'. The item's bead_id field is
set as a back-reference only after 'br show <id>' verifies the created bead, so
re-running is idempotent: already-materialized, already-consumed, and
held-for-review items are skipped.

When br is not on PATH the command degrades gracefully (warns, exits 0) unless
--dry-run is set.

Examples:
  ao next-work materialize
  ao next-work materialize --dry-run
  ao next-work materialize --json
  ao next-work materialize --source-epic ag-9jle`,
	Args: cobra.NoArgs,
	RunE: runNextWorkMaterialize,
}

func init() {
	f := nextWorkMaterializeCmd.Flags()
	f.StringVar(&nextWorkMaterializeFile, "file", "", "Path to next-work.jsonl (default: <cwd>/.agents/rpi/next-work.jsonl)")
	f.BoolVar(&nextWorkMaterializeDryRun, "dry-run", false, "Show what would be created without creating beads or mutating the queue")
	f.BoolVar(&nextWorkMaterializeJSON, "json", false, "Emit a machine-readable JSON summary")
	f.StringVar(&nextWorkMaterializeSourceEpic, "source-epic", "", "Only materialize items whose batch source_epic equals this value")
	f.StringVar(&nextWorkMaterializeMaterialBy, "materialized-by", defaultMaterializedBy, "Actor recorded in provenance metadata")
	nextWorkCmd.AddCommand(nextWorkMaterializeCmd)
	rootCmd.AddCommand(nextWorkCmd)
}

func runNextWorkMaterialize(cmd *cobra.Command, _ []string) error {
	return nextworkmaterialize.Run(nextworkmaterialize.Options{
		File:             nextWorkMaterializeFile,
		DryRun:           nextWorkMaterializeDryRun,
		JSON:             nextWorkMaterializeJSON,
		SourceEpic:       nextWorkMaterializeSourceEpic,
		MaterializedBy:   nextWorkMaterializeMaterialBy,
		Out:              cmd.OutOrStdout(),
		ErrOut:           cmd.ErrOrStderr(),
		TrackerAvailable: nextWorkTrackerAvailable,
		ExecTracker:      execNextWorkTracker,
	})
}

var nextWorkTrackerAvailable = func() bool {
	_, err := exec.LookPath("br")
	return err == nil
}

var execNextWorkTracker = func(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c := beadsTrackerCommandContext(ctx, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return out, fmt.Errorf("br %s: %w: %s", strings.Join(args, " "), err, detail)
		}
		return out, fmt.Errorf("br %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
