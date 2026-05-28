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

	loopVerifyCmd.Flags().Int("max-idle", 5, "max acceptable trailing idle/unchanged streak before flagging dormancy")
	loopCmd.AddCommand(loopVerifyCmd)
}

// loopVerifyCmd audits cycle-history.jsonl integrity using the typed
// BC3 LoopReaderPort. Cycle 163: first NEW consumer of the cycle-161
// CycleEntry widening (uses the StartedAt field added in soc-ckc4).
//
// Audits performed:
//   - monotonic Number ordering (no out-of-order cycles)
//   - no duplicate Number values (catches double-appends)
//   - non-empty StartedAt on every entry (the cycle-161 widening
//     surfaces operator hand-edits that forgot the timestamp)
//   - IdleStreak < threshold (sanity check on dormancy)
var loopVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Audit cycle-history.jsonl integrity via BC3 LoopReaderPort",
	Long: `Audit .agents/evolve/cycle-history.jsonl integrity via the typed
BC3 LoopReaderPort. Reports any of:
  - non-monotonic Number ordering
  - duplicate Number values
  - empty StartedAt fields (since cycle 161 they are required)
  - excessive trailing IdleStreak (>= --max-idle, default 5)

Exit code is 0 when all checks pass, 1 when any issue is found.
Useful as a pre-commit gate or CI assertion against a hand-edited ledger.`,
	RunE: runLoopVerify,
}

func runLoopVerify(cmd *cobra.Command, _ []string) error {
	maxIdle, _ := cmd.Flags().GetInt("max-idle")
	if maxIdle == 0 {
		maxIdle = 5
	}
	return loopVerifyRun(cmd.Context(), loopVerifyOptions{
		writer:  cmd.OutOrStdout(),
		maxIdle: maxIdle,
	})
}

type loopVerifyOptions struct {
	writer   io.Writer
	maxIdle  int
	verifyFn func(ctx context.Context, opts loopVerifyOptions) ([]string, error)
}

func loopVerifyRun(ctx context.Context, opts loopVerifyOptions) error {
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fn := opts.verifyFn
	if fn == nil {
		fn = loopVerifyViaPort
	}
	issues, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("loop verify: %w", err)
	}
	if len(issues) == 0 {
		fmt.Fprintln(opts.writer, "PASS — no integrity issues found")
		return nil
	}
	fmt.Fprintf(opts.writer, "FAIL — %d issue(s):\n", len(issues))
	for _, issue := range issues {
		fmt.Fprintf(opts.writer, "  - %s\n", issue)
	}
	return fmt.Errorf("cycle-history integrity check failed: %d issue(s)", len(issues))
}

// loopVerifyViaPort uses productionLoopReader (cycle 108) +
// CycleEntry widening (cycle 161) to audit the local ledger.
func loopVerifyViaPort(ctx context.Context, opts loopVerifyOptions) ([]string, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return nil, err
	}
	historyPath := filepath.Join(cwd, ".agents", "evolve", "cycle-history.jsonl")
	reader := newProductionLoopReader(historyPath)

	entries, err := reader.Range(ctx, 1, 1<<30)
	if err != nil {
		return nil, fmt.Errorf("range: %w", err)
	}
	idleStreak, err := reader.IdleStreak(ctx)
	if err != nil {
		return nil, fmt.Errorf("idle-streak: %w", err)
	}

	return checkLoopIntegrity(entries, idleStreak, opts.maxIdle), nil
}

// checkLoopIntegrity is the pure-Go audit logic, separated for
// testability. Returns a list of issue strings; empty means OK.
func checkLoopIntegrity(entries []ports.CycleEntry, idleStreak, maxIdle int) []string {
	issues := []string{}
	seen := make(map[int]bool, len(entries))
	prev := 0
	for _, e := range entries {
		if e.Number <= prev {
			issues = append(issues, fmt.Sprintf("non-monotonic: cycle %d follows cycle %d", e.Number, prev))
		}
		if seen[e.Number] {
			issues = append(issues, fmt.Sprintf("duplicate cycle number: %d", e.Number))
		}
		seen[e.Number] = true
		if e.StartedAt == "" {
			issues = append(issues, fmt.Sprintf("cycle %d missing StartedAt timestamp", e.Number))
		}
		prev = e.Number
	}
	if idleStreak > maxIdle {
		issues = append(issues, fmt.Sprintf("trailing IdleStreak=%d exceeds max-idle=%d", idleStreak, maxIdle))
	}
	return issues
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
