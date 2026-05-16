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

// loopConvergedCmd exposes the pure BC3 ConvergenceCheckPort
// (productionConvergenceCheck, soc-y5vh.7) via the CLI. It is the
// typed replacement for the hand-rolled bash predicate that reads
// .agents/evolve/session-convergence.json directly. soc-y5vh.8.
//
// ConvergenceCheckPort.Check is deliberately pure — it does not fetch
// CI, scan findings, or read fitness files. This command therefore
// takes caller-supplied evidence (the /evolve loop already has
// `ao ci recent` and the findings count) and runs the predicate.
var loopConvergedCmd = &cobra.Command{
	Use:   "converged",
	Short: "Evaluate the evolve convergence STOP predicate via the BC3 ConvergenceCheckPort",
	Long: `Evaluate the evolve loop's convergence STOP predicate via the typed
BC3 ConvergenceCheckPort. Emits a JSON object: converged, ci_green_streak,
unconsumed_high_medium, fitness_baseline_captured, reasons.

The default criteria are green CI streak >= 3, unconsumed HIGH+MEDIUM
findings <= 1, and a captured fitness baseline. The predicate is pure —
supply the evidence as flags; this command does not fetch CI itself.

Exit status is always 0 (this is a query). Callers branch on the
"converged" field, e.g. ao loop converged ... | jq -e .converged.

Examples:
  ao loop converged --green-streak 3 --unconsumed-high-medium 0 --fitness-baseline
  ao loop converged --green-streak 2 --unconsumed-high-medium 4`,
	RunE: runLoopConverged,
}

type loopConvergedOptions struct {
	greenStreak          int
	unconsumedHighMedium int
	fitnessBaseline      bool
	writer               io.Writer
	checkFn              func(ctx context.Context, opts loopConvergedOptions) (ports.ConvergenceResult, error)
}

// convergedReport is the snake_case JSON shape emitted to stdout — a
// stable, script-friendly projection of ports.ConvergenceResult.
type convergedReport struct {
	Converged               bool     `json:"converged"`
	CIGreenStreak           int      `json:"ci_green_streak"`
	UnconsumedHighMedium    int      `json:"unconsumed_high_medium"`
	FitnessBaselineCaptured bool     `json:"fitness_baseline_captured"`
	Reasons                 []string `json:"reasons"`
}

func init() {
	loopConvergedCmd.Flags().Int("green-streak", 0, "current leading green CI streak (caller-supplied evidence)")
	loopConvergedCmd.Flags().Int("unconsumed-high-medium", 0, "current unconsumed HIGH+MEDIUM finding count")
	loopConvergedCmd.Flags().Bool("fitness-baseline", false, "a fitness baseline artifact has been captured")
	loopCmd.AddCommand(loopConvergedCmd)
}

func runLoopConverged(cmd *cobra.Command, _ []string) error {
	greenStreak, _ := cmd.Flags().GetInt("green-streak")
	unconsumed, _ := cmd.Flags().GetInt("unconsumed-high-medium")
	fitnessBaseline, _ := cmd.Flags().GetBool("fitness-baseline")
	return loopConvergedRun(cmd.Context(), loopConvergedOptions{
		greenStreak:          greenStreak,
		unconsumedHighMedium: unconsumed,
		fitnessBaseline:      fitnessBaseline,
		writer:               cmd.OutOrStdout(),
	})
}

func loopConvergedRun(ctx context.Context, opts loopConvergedOptions) error {
	fn := opts.checkFn
	if fn == nil {
		fn = loopConvergedViaPort
	}
	result, err := fn(ctx, opts)
	if err != nil {
		return fmt.Errorf("loop converged: %w", err)
	}
	if opts.writer == nil {
		opts.writer = os.Stdout
	}
	report := convergedReport{
		Converged:               result.Converged,
		CIGreenStreak:           result.CIGreenStreak,
		UnconsumedHighMedium:    result.UnconsumedHighMedium,
		FitnessBaselineCaptured: result.FitnessBaselineCaptured,
		Reasons:                 result.Reasons,
	}
	if report.Reasons == nil {
		report.Reasons = []string{}
	}
	if err := json.NewEncoder(opts.writer).Encode(report); err != nil {
		return fmt.Errorf("loop converged encode: %w", err)
	}
	return nil
}

// loopConvergedViaPort runs productionConvergenceCheck against
// caller-supplied evidence. ConvergenceCheckPort.Check counts the
// leading green streak from RecentCIRuns, so the command synthesizes
// opts.greenStreak completed/success runs to express the streak.
func loopConvergedViaPort(ctx context.Context, opts loopConvergedOptions) (ports.ConvergenceResult, error) {
	n := opts.greenStreak
	if n < 0 {
		n = 0
	}
	runs := make([]ports.CIRun, 0, n)
	for i := 0; i < n; i++ {
		runs = append(runs, ports.CIRun{
			Status:     ports.CIRunStatusCompleted,
			Conclusion: ports.CIRunConclusionSuccess,
		})
	}
	return newProductionConvergenceCheck().Check(ctx, ports.ConvergenceInput{
		RecentCIRuns:            runs,
		UnconsumedHighMedium:    opts.unconsumedHighMedium,
		FitnessBaselineCaptured: opts.fitnessBaseline,
	})
}
