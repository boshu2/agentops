// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// loopHypothesisCmd is the BC3 Loop hypothesis-ledger CLI group. It
// exposes productionHypothesisLedger (soc-y5vh.6) so /evolve's
// hypothesis tracking (convergence-mechanics.md Mechanism 3) flows
// through the typed HypothesisLedgerPort instead of direct
// .agents/evolve/hypotheses.jsonl shell reads. soc-y5vh.8.
//
// Sibling pattern: ao loop append/history (loop_append.go, loop.go).
var loopHypothesisCmd = &cobra.Command{
	Use:   "hypothesis",
	Short: "BC3 Loop hypothesis-ledger operations (list, append)",
	Long:  `Operations on the /evolve hypothesis ledger (.agents/evolve/hypotheses.jsonl) via the typed BC3 HypothesisLedgerPort.`,
}

var loopHypothesisListCmd = &cobra.Command{
	Use:   "list",
	Short: "List evolve hypotheses via the BC3 HypothesisLedgerPort",
	Long: `Read .agents/evolve/hypotheses.jsonl via the typed BC3
HypothesisLedgerPort. Emits one JSON HypothesisRecord per line in
append order — a typed replacement for inline jq over the raw ledger.`,
	RunE: runLoopHypothesisList,
}

var loopHypothesisAppendCmd = &cobra.Command{
	Use:   "append --id <id> --hypothesis <h> --measure <m> [flags]",
	Short: "Append a hypothesis record via the BC3 HypothesisLedgerPort",
	Long: `Append a falsifiable hypothesis to .agents/evolve/hypotheses.jsonl
via the typed BC3 HypothesisLedgerPort. --id is required and must be
unique; a patch names what landed, --check-at-cycle names the future
cycle that evaluates the measure.

Example:
  ao loop hypothesis append --id H210.1 --cycle-landed 210 --check-at-cycle 225 \
    --patch "Step 1.5 typed CI probe" --hypothesis "removes gh shell-outs" \
    --measure "grep -c gh in evolve hot path"`,
	RunE: runLoopHypothesisAppend,
}

type loopHypothesisListOptions struct {
	writer io.Writer
	listFn func(ctx context.Context, opts loopHypothesisListOptions) ([]ports.HypothesisRecord, error)
}

type loopHypothesisAppendOptions struct {
	id           string
	patch        string
	hypothesis   string
	measure      string
	verdict      string
	cycleLanded  int
	checkAtCycle int
	writer       io.Writer
	appendFn     func(ctx context.Context, opts loopHypothesisAppendOptions) (ports.HypothesisRecord, error)
}

func init() {
	loopHypothesisAppendCmd.Flags().String("id", "", "unique hypothesis ID, e.g. H210.1 (required)")
	loopHypothesisAppendCmd.Flags().String("patch", "", "one-line description of what landed")
	loopHypothesisAppendCmd.Flags().String("hypothesis", "", "expected effect of the patch")
	loopHypothesisAppendCmd.Flags().String("measure", "", "how the effect is verified")
	loopHypothesisAppendCmd.Flags().String("verdict", "PENDING", "verdict: PENDING|VERIFIED|FALSIFIED")
	loopHypothesisAppendCmd.Flags().Int("cycle-landed", 0, "cycle the patch landed")
	loopHypothesisAppendCmd.Flags().Int("check-at-cycle", 0, "future cycle that evaluates the measure")
	_ = loopHypothesisAppendCmd.MarkFlagRequired("id")

	loopHypothesisCmd.AddCommand(loopHypothesisListCmd)
	loopHypothesisCmd.AddCommand(loopHypothesisAppendCmd)
	loopCmd.AddCommand(loopHypothesisCmd)
}

// evolveHypothesesPath resolves the project-local hypothesis ledger path.
func evolveHypothesesPath() (string, error) {
	cwd, err := resolveProjectDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".agents", "evolve", "hypotheses.jsonl"), nil
}

// appendHypothesisAt appends one record to the ledger at path, creating
// the parent directory. Path-explicit so tests exercise the real
// production adapter against a temp ledger.
func appendHypothesisAt(ctx context.Context, path string, rec ports.HypothesisRecord) (ports.HypothesisRecord, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ports.HypothesisRecord{}, fmt.Errorf("mkdir: %w", err)
	}
	return newProductionHypothesisLedger(path).Append(ctx, rec)
}

// listHypothesesAt reads all records from the ledger at path.
func listHypothesesAt(ctx context.Context, path string) ([]ports.HypothesisRecord, error) {
	return newProductionHypothesisLedger(path).List(ctx)
}

func runLoopHypothesisList(cmd *cobra.Command, _ []string) error {
	return loopHypothesisListRun(cmd.Context(), loopHypothesisListOptions{writer: cmd.OutOrStdout()})
}

func loopHypothesisListRun(ctx context.Context, opts loopHypothesisListOptions) error {
	fn := opts.listFn
	if fn == nil {
		fn = loopHypothesisListViaPort
	}
	records, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("loop hypothesis list: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	enc := json.NewEncoder(opts.writer)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("loop hypothesis list encode: %w", err)
		}
	}
	return nil
}

func loopHypothesisListViaPort(ctx context.Context, _ loopHypothesisListOptions) ([]ports.HypothesisRecord, error) {
	path, err := evolveHypothesesPath()
	if err != nil {
		return nil, err
	}
	return listHypothesesAt(ctx, path)
}

func runLoopHypothesisAppend(cmd *cobra.Command, _ []string) error {
	id, _ := cmd.Flags().GetString("id")
	patch, _ := cmd.Flags().GetString("patch")
	hypothesis, _ := cmd.Flags().GetString("hypothesis")
	measure, _ := cmd.Flags().GetString("measure")
	verdict, _ := cmd.Flags().GetString("verdict")
	cycleLanded, _ := cmd.Flags().GetInt("cycle-landed")
	checkAtCycle, _ := cmd.Flags().GetInt("check-at-cycle")
	return loopHypothesisAppendRun(cmd.Context(), loopHypothesisAppendOptions{
		id:           id,
		patch:        patch,
		hypothesis:   hypothesis,
		measure:      measure,
		verdict:      verdict,
		cycleLanded:  cycleLanded,
		checkAtCycle: checkAtCycle,
		writer:       cmd.OutOrStdout(),
	})
}

func loopHypothesisAppendRun(ctx context.Context, opts loopHypothesisAppendOptions) error {
	if opts.id == "" {
		return errors.New("loop hypothesis append: --id required")
	}
	fn := opts.appendFn
	if fn == nil {
		fn = loopHypothesisAppendViaPort
	}
	rec, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("loop hypothesis append: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	fmt.Fprintf(opts.writer, "appended hypothesis id=%q verdict=%q check_at_cycle=%d\n",
		rec.ID, rec.Verdict, rec.CheckAtCycle)
	return nil
}

func loopHypothesisAppendViaPort(ctx context.Context, opts loopHypothesisAppendOptions) (ports.HypothesisRecord, error) {
	path, err := evolveHypothesesPath()
	if err != nil {
		return ports.HypothesisRecord{}, err
	}
	return appendHypothesisAt(ctx, path, ports.HypothesisRecord{
		ID:           opts.id,
		Patch:        opts.patch,
		Hypothesis:   opts.hypothesis,
		Measure:      opts.measure,
		Verdict:      ports.HypothesisVerdict(opts.verdict),
		CycleLanded:  opts.cycleLanded,
		CheckAtCycle: opts.checkAtCycle,
	})
}
