// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// loopAppendCmd exposes productionLoopWriter (cycle 109) via the CLI.
// Companion to cycle 144's `ao loop history` (which exposes
// LoopReader). Closes the BC3 reader+writer pair on the CLI side.
//
// Built using the cycle-147 cli-wiring template.
var loopAppendCmd = &cobra.Command{
	Use:   "append --mode <m> --result <r> [flags]",
	Short: "Append a cycle entry via BC3 LoopWriterPort",
	Long: `Append a new entry to .agents/evolve/cycle-history.jsonl via the
typed BC3 LoopWriterPort (productionLoopWriter, cycle 109).

If --cycle is 0 or omitted, the writer auto-assigns max(existing)+1
(matches the port contract). --mode and --result are required; the
rest are optional.

Examples:
  ao loop append --mode evolve --result improved
  ao loop append --mode evolve --result unchanged --commit deadbeef
  ao loop append --cycle 200 --mode test --result improved --milestone "test entry"`,
	RunE: runLoopAppend,
}

type loopAppendOptions struct {
	cycle     int
	mode      string
	result    string
	commit    string
	milestone string
	writer    io.Writer
	appendFn  func(ctx context.Context, opts loopAppendOptions) (ports.CycleEntry, error)
}

func init() {
	loopAppendCmd.Flags().Int("cycle", 0, "cycle number (0 = auto-assign max+1)")
	loopAppendCmd.Flags().String("mode", "", "cycle mode (required)")
	loopAppendCmd.Flags().String("result", "", "cycle result: improved|harvested|unchanged|idle (required)")
	loopAppendCmd.Flags().String("commit", "", "git commit SHA (optional)")
	loopAppendCmd.Flags().String("milestone", "", "milestone note (optional)")
	_ = loopAppendCmd.MarkFlagRequired("mode")
	_ = loopAppendCmd.MarkFlagRequired("result")
	loopCmd.AddCommand(loopAppendCmd)
}

func runLoopAppend(cmd *cobra.Command, _ []string) error {
	cycle, _ := cmd.Flags().GetInt("cycle")
	mode, _ := cmd.Flags().GetString("mode")
	result, _ := cmd.Flags().GetString("result")
	commit, _ := cmd.Flags().GetString("commit")
	milestone, _ := cmd.Flags().GetString("milestone")
	return loopAppendRun(cmd.Context(), loopAppendOptions{
		cycle:     cycle,
		mode:      mode,
		result:    result,
		commit:    commit,
		milestone: milestone,
		writer:    cmd.OutOrStdout(),
	})
}

func loopAppendRun(ctx context.Context, opts loopAppendOptions) error {
	if opts.mode == "" {
		return errors.New("loop append: --mode required")
	}
	if opts.result == "" {
		return errors.New("loop append: --result required")
	}
	fn := opts.appendFn
	if fn == nil {
		fn = loopAppendViaPort
	}
	entry, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("loop append: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fmt.Fprintf(opts.writer, "appended cycle=%d mode=%q result=%q\n", entry.Number, entry.Mode, entry.Result)
	return nil
}

func loopAppendViaPort(ctx context.Context, opts loopAppendOptions) (ports.CycleEntry, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return ports.CycleEntry{}, err
	}
	historyPath := filepath.Join(cwd, ".agents", "evolve", "cycle-history.jsonl")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		return ports.CycleEntry{}, fmt.Errorf("mkdir: %w", err)
	}
	w := newProductionLoopWriter(historyPath)
	return w.Append(ctx, ports.CycleEntry{
		Number:    opts.cycle,
		Mode:      opts.mode,
		Result:    opts.result,
		Commit:    opts.commit,
		Milestone: opts.milestone,
	})
}
