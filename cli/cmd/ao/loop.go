// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// loopCmd is the parent for BC3 Loop CLI surfaces. Per soc-y5vh.5
// (cycle 143 prerequisite audit), this is the first cobra wiring
// that exposes a production adapter (productionLoopReader, cycle
// 108) to scripts and operators. Other Loop subcommands (`history`
// is first; future: `write`, `tail`) join this group.
var loopCmd = &cobra.Command{
	Use:   "loop",
	Short: "BC3 Loop operations (cycle history, etc.)",
	Long:  `Operations on the /evolve cycle history and related Loop bounded-context state. The 'history' subcommand reads .agents/evolve/cycle-history.jsonl via the typed BC3 LoopReaderPort.`,
}

// loopHistoryCmd reads cycles from .agents/evolve/cycle-history.jsonl
// via productionLoopReader (cycle 108 adapter). The first cobra wiring
// for soc-y5vh.5 — unblocks soc-y5vh.4 (script wrapper) and any
// future caller that wants typed access to the cycle ledger.
var loopHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Read /evolve cycle history via the BC3 LoopReaderPort",
	Long: `Read .agents/evolve/cycle-history.jsonl via the typed BC3 LoopReaderPort.
Emits one JSON object per cycle to stdout. Useful as a replacement for
inline awk/jq parsing in shell scripts.

Examples:
  ao loop history --limit 5             # last 5 cycles
  ao loop history --start 100 --end 110 # cycles 100-110
  ao loop history --latest              # only the most recent cycle`,
	RunE: runLoopHistory,
}

type loopHistoryOptions struct {
	limit     int
	start     int
	end       int
	latest    bool
	writer    io.Writer
	historyFn func(ctx context.Context, opts loopHistoryOptions) ([]ports.CycleEntry, error)
}

func init() {
	loopCmd.GroupID = "core"
	rootCmd.AddCommand(loopCmd)

	loopHistoryCmd.Flags().Int("limit", 0, "max entries to emit (0 = all)")
	loopHistoryCmd.Flags().Int("start", 0, "start cycle number (inclusive; 0 = unbounded)")
	loopHistoryCmd.Flags().Int("end", 0, "end cycle number (inclusive; 0 = unbounded)")
	loopHistoryCmd.Flags().Bool("latest", false, "emit only the latest entry")
	loopCmd.AddCommand(loopHistoryCmd)
}

func runLoopHistory(cmd *cobra.Command, _ []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	start, _ := cmd.Flags().GetInt("start")
	end, _ := cmd.Flags().GetInt("end")
	latest, _ := cmd.Flags().GetBool("latest")

	opts := loopHistoryOptions{
		limit:  limit,
		start:  start,
		end:    end,
		latest: latest,
		writer: cmd.OutOrStdout(),
	}
	return loopHistoryRun(cmd.Context(), opts)
}

func loopHistoryRun(ctx context.Context, opts loopHistoryOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	historyFn := opts.historyFn
	if historyFn == nil {
		historyFn = loadCycleHistoryViaPort
	}
	entries, err := historyFn(ctx, opts)
	if err != nil {
		return fmt.Errorf("loop history: %w", err)
	}
	if opts.limit > 0 && len(entries) > opts.limit {
		entries = entries[len(entries)-opts.limit:]
	}
	enc := json.NewEncoder(opts.writer)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("loop history encode: %w", err)
		}
	}
	return nil
}

// loadCycleHistoryViaPort wires productionLoopReader (cycle 108) to the
// cycle-history.jsonl path. opts.latest/start/end select the slice.
func loadCycleHistoryViaPort(ctx context.Context, opts loopHistoryOptions) ([]ports.CycleEntry, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return nil, err
	}
	historyPath := filepath.Join(cwd, ".agents", "evolve", "cycle-history.jsonl")
	reader := newProductionLoopReader(historyPath)

	if opts.latest {
		entry, err := reader.Latest(ctx)
		if err != nil {
			return nil, err
		}
		if entry.Number == 0 {
			return []ports.CycleEntry{}, nil
		}
		return []ports.CycleEntry{entry}, nil
	}
	if opts.start > 0 || opts.end > 0 {
		// Set unbounded ends to extreme values so Range returns the
		// open-side slice.
		s := opts.start
		if s == 0 {
			s = 1
		}
		e := opts.end
		if e == 0 {
			e = 1 << 30
		}
		return reader.Range(ctx, s, e)
	}
	// Default: full range. Use Range with sentinel bounds.
	return reader.Range(ctx, 1, 1<<30)
}
