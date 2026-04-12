package overnight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
)

// IterationSummary is one pass of the INGEST → REDUCE → MEASURE wave.
//
// Persisted into overnightSummary.Iterations (dream-report schema v2) for
// downstream inspection by the morning report renderer and council judges.
type IterationSummary struct {
	ID         IterationID     `json:"id" yaml:"id"`
	Index      int             `json:"index" yaml:"index"` // 1-based
	StartedAt  time.Time       `json:"started_at" yaml:"started_at"`
	FinishedAt time.Time       `json:"finished_at" yaml:"finished_at"`
	Duration   string          `json:"duration" yaml:"duration"`
	Status     IterationStatus `json:"status" yaml:"status"` // See IterationStatus in types.go — exhaustive enum

	// Stage sub-summaries. Each is an opaque map so v1 readers can ignore
	// them without schema awareness.
	Ingest  map[string]any `json:"ingest,omitempty" yaml:"ingest,omitempty"`
	Reduce  map[string]any `json:"reduce,omitempty" yaml:"reduce,omitempty"`
	Measure map[string]any `json:"measure,omitempty" yaml:"measure,omitempty"`

	// FitnessBefore and FitnessAfter are opaque maps for v1 compatibility.
	// The real FitnessVector lives in cli/internal/corpus/fitness.go
	// (introduced by Issue 15) and is marshaled into these maps by MEASURE.
	FitnessBefore map[string]any `json:"fitness_before,omitempty" yaml:"fitness_before,omitempty"`
	FitnessAfter  map[string]any `json:"fitness_after,omitempty" yaml:"fitness_after,omitempty"`
	FitnessDelta  float64        `json:"fitness_delta" yaml:"fitness_delta"`

	Degraded []string `json:"degraded,omitempty" yaml:"degraded,omitempty"`
	Error    string   `json:"error,omitempty" yaml:"error,omitempty"`
}

// RunLoopResult is the aggregate output of a single Dream run.
//
// The command-layer caller copies these fields into overnightSummary's new
// v2 fields (Iterations, FitnessDelta, PlateauReason, RegressionReason)
// before finalizing the morning report.
type RunLoopResult struct {
	Iterations       []IterationSummary `json:"iterations" yaml:"iterations"`
	FitnessDelta     map[string]any     `json:"fitness_delta,omitempty" yaml:"fitness_delta,omitempty"`
	PlateauReason    string             `json:"plateau_reason,omitempty" yaml:"plateau_reason,omitempty"`
	RegressionReason string             `json:"regression_reason,omitempty" yaml:"regression_reason,omitempty"`
	BudgetExhausted  bool               `json:"budget_exhausted" yaml:"budget_exhausted"`

	// WarnOnlyBudgetInitial is the rescue ceiling observed at loop start
	// when a WarnOnlyRatchet was supplied. Zero means the ratchet was
	// disabled (legacy infinite warn-only path) — the morning report
	// renderer uses zero-guard to suppress the counter line.
	WarnOnlyBudgetInitial int `json:"warn_only_budget_initial" yaml:"warn_only_budget_initial"`

	// WarnOnlyBudgetRemaining is the live rescue counter at loop exit.
	// Callers copy this into overnightSummary.WarnOnlyRemaining for the
	// morning report.
	WarnOnlyBudgetRemaining int `json:"warn_only_budget_remaining" yaml:"warn_only_budget_remaining"`

	// MeasureFailureHalt is true when the loop halted because
	// MaxConsecutiveMeasureFailures was reached (C4). Distinct from
	// plateau/regression halts so the morning report can surface a
	// "systemic MEASURE breakage" diagnosis rather than claiming the
	// corpus plateaued.
	MeasureFailureHalt bool `json:"measure_failure_halt" yaml:"measure_failure_halt"`

	// FailureReason is the human-readable explanation for a
	// MeasureFailureHalt, including the iteration index, consecutive
	// count, and configured cap. Empty when MeasureFailureHalt is false.
	FailureReason string `json:"failure_reason,omitempty" yaml:"failure_reason,omitempty"`

	Degraded []string `json:"degraded,omitempty" yaml:"degraded,omitempty"`
}

// ErrNotImplemented is returned by stage stubs that will be filled in by
// later waves (Wave 3 for stage drivers, Wave 4 for integration). Callers
// in tests distinguish this from a real error so skeleton compiles green
// without faking behavior.
var ErrNotImplemented = errors.New("overnight: stage not implemented yet (skeleton wave)")

// RunLoop executes the bounded Dream compounding loop.
//
// Each iteration: INGEST → NewCheckpoint → REDUCE → MEASURE → delta.
// Halt conditions (first to fire wins):
//
//   - Budget exhausted: ctx deadline expired or cumulative elapsed
//     exceeds opts.RunTimeout.
//   - Max iterations: opts.MaxIterations > 0 and reached.
//   - Plateau: opts.PlateauWindowK consecutive |delta| < opts.PlateauEpsilon.
//   - Regression: any metric dropped by more than opts.RegressionFloor.
//
// WarnOnly mode (the default for the first 2-3 production runs) turns
// plateau and regression halts into degraded notes instead of stopping,
// so operators can calibrate thresholds empirically before ratcheting
// to strict mode.
//
// REDUCE runs with nil close-loop callbacks at this layer — Dream's
// command-layer integration (cli/cmd/ao/overnight.go) is responsible
// for wiring cmd-package helpers into the loop if/when it wants
// in-process close-loop promotion. Until then, close-loop is a
// degraded-note stage inside REDUCE.
//
// Returning (non-nil, non-nil) is explicitly allowed: a partial result
// with a non-nil error describes "we ran N iterations, then hit error E."
// The caller should persist the partial result AND surface the error.
//
//nolint:gocyclo // RunLoop owns the durable overnight state machine and checkpoint boundary.
func RunLoop(ctx context.Context, opts RunLoopOptions) (*RunLoopResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts, degraded := opts.normalize()
	log := opts.LogWriter
	if log == nil {
		log = io.Discard
	}

	// Rehydrate from prior persisted iterations so a resumed run sees the
	// full history and picks up prevSnapshot correctly. This lives here
	// (NOT in recovery.go) because:
	//   1. recovery.go's RecoverFromCrash returns ([]string, error) and
	//      runs BEFORE RunLoop at cli/cmd/ao/overnight.go:261 — it cannot
	//      hand prevSnapshot across the call boundary.
	//   2. prevSnapshot is local to RunLoop and must be set before the
	//      first loop iteration runs.
	//   3. Rehydration is RunLoop's concern, not crash recovery's.
	//
	// Micro-epic 3 (C2 status lie fix, 2026-04-10): the rehydration
	// predicate is now IsCorpusCompounded(), not Status == "done". This
	// correctly includes post-commit regression halts (whose corpus DID
	// compound on disk) as valid rehydration baselines, while still
	// skipping pre-commit rollbacks. See types.go for the enum and
	// docs/contracts/dream-report.md's Status Precedence Truth Table.
	var priorIterations []IterationSummary
	if opts.OutputDir != "" && opts.RunID != "" {
		iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
		prior, rejected, loadErr := LoadIterations(iterDir, opts.RunID)
		if loadErr != nil {
			// Structural error (e.g., readdir failure with permission denied).
			// Surface via degraded and proceed with empty history — never halt.
			degraded = append(degraded, fmt.Sprintf("rehydrate: %v", loadErr))
		}
		for _, rej := range rejected {
			degraded = append(degraded, fmt.Sprintf("rehydrate rejected: %s", rej))
		}
		priorIterations = prior
	}

	result := &RunLoopResult{
		Iterations: priorIterations, // Resume with history intact
		Degraded:   degraded,
	}
	if opts.WarnOnlyBudget != nil {
		result.WarnOnlyBudgetInitial = opts.WarnOnlyBudget.Initial
		result.WarnOnlyBudgetRemaining = opts.WarnOnlyBudget.Remaining
	}

	if opts.Cwd == "" {
		return result, fmt.Errorf("overnight: RunLoopOptions.Cwd is required")
	}
	if opts.OutputDir == "" {
		return result, fmt.Errorf("overnight: RunLoopOptions.OutputDir is required")
	}
	if opts.RunID == "" {
		return result, fmt.Errorf("overnight: RunLoopOptions.RunID is required")
	}

	fmt.Fprintf(log, "overnight: RunLoop starting (budget=%s, max_iter=%d, epsilon=%g, K=%d, warn_only=%v)\n",
		opts.RunTimeout, opts.MaxIterations, opts.PlateauEpsilon, opts.PlateauWindowK, opts.WarnOnly)

	// Derive a ctx that also honors the run-timeout wall-clock budget.
	loopCtx, cancel := context.WithTimeout(ctx, opts.RunTimeout)
	defer cancel()

	startedAt := time.Now()
	plateau := NewPlateauState(opts.PlateauWindowK, opts.PlateauEpsilon)

	// Initialize prevSnapshot from the last iteration whose corpus is on
	// disk (IsCorpusCompounded). Pre-commit rollbacks and failed
	// iterations are skipped; post-commit regression halts are NOT
	// skipped because their corpus compounded before the halt fired.
	var prevSnapshot *FitnessSnapshot
	if len(priorIterations) > 0 {
		for i := len(priorIterations) - 1; i >= 0; i-- {
			if priorIterations[i].Status.IsCorpusCompounded() {
				if snap := mapToSnapshot(priorIterations[i].FitnessAfter); snap != nil {
					prevSnapshot = snap
				}
				break
			}
		}
	}

	// Replay prior deltas through the plateau window so a resumed run
	// preserves the prior streak. Only compounded iterations contribute
	// (IsCorpusCompounded) — an iteration that never committed has no
	// valid delta to replay. Walk the LAST K compounded iterations in
	// chronological order so the window reflects the most recent state.
	//
	// If the PRIOR run had already plateaued, the operator would have
	// halted and not resumed. Resuming implies plateau did NOT fire in
	// the prior run, so the boolean return from Observe is ignored.
	if len(priorIterations) > 0 && opts.PlateauWindowK > 0 {
		var successfulTail []IterationSummary
		for i := len(priorIterations) - 1; i >= 0 && len(successfulTail) < opts.PlateauWindowK; i-- {
			if priorIterations[i].Status.IsCorpusCompounded() {
				successfulTail = append([]IterationSummary{priorIterations[i]}, successfulTail...)
			}
		}
		for _, it := range successfulTail {
			_ = plateau.Observe(it.FitnessDelta)
		}
	}

	iterIndex := len(priorIterations) // Resume from N+1 (incremented at top of loop)
	// Micro-epic 5 (C4): consecutive MEASURE failure counter. Reset to
	// zero whenever an iteration completes the DELTA+HALT block (i.e.
	// MEASURE succeeded and we have a fitness snapshot). Incremented at
	// the MEASURE failure site below; checked against
	// opts.MaxConsecutiveMeasureFailures before the `continue` that
	// skips to the next iteration.
	consecutiveMeasureFailures := 0

	for {
		// Budget / cancellation check BEFORE starting a new iteration.
		select {
		case <-loopCtx.Done():
			if errors.Is(loopCtx.Err(), context.DeadlineExceeded) {
				result.BudgetExhausted = true
				fmt.Fprintln(log, "overnight: RunLoop halted — wall-clock budget exhausted")
			} else {
				fmt.Fprintf(log, "overnight: RunLoop halted — context cancelled: %v\n", loopCtx.Err())
			}
			return result, nil
		default:
		}

		if opts.MaxIterations > 0 && iterIndex >= opts.MaxIterations {
			fmt.Fprintf(log, "overnight: RunLoop halted — max iterations (%d) reached\n", opts.MaxIterations)
			return result, nil
		}

		iterIndex++
		iterID := IterationID(fmt.Sprintf("%s-iter-%d", opts.RunID, iterIndex))
		iterStart := time.Now()
		fmt.Fprintf(log, "overnight: iteration %d starting (id=%s)\n", iterIndex, iterID)

		iter := IterationSummary{
			ID:        iterID,
			Index:     iterIndex,
			StartedAt: iterStart,
			Status:    StatusDone,
		}

		// --- INGEST ---
		var ingest *IngestResult
		var ingestErr error
		if injector := getTestIngestFaultInjector(); injector != nil {
			ingestErr = injector(iterIndex)
		} else {
			ingest, ingestErr = RunIngest(loopCtx, opts, log)
		}
		if ingestErr != nil {
			iter.Status = StatusFailed
			iter.Error = fmt.Sprintf("ingest: %v", ingestErr)
			iter.FinishedAt = time.Now()
			iter.Duration = iter.FinishedAt.Sub(iterStart).String()
			// Best-effort persist of the failed iteration summary. Do NOT
			// override the original error; append a Degraded note so the
			// morning report shows both failures.
			iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
			if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("persist iter-%d (during ingest failure): %v", iterIndex, writeErr))
			}
			result.Iterations = append(result.Iterations, iter)
			fmt.Fprintf(log, "overnight: iteration %d INGEST failed: %v\n", iterIndex, ingestErr)
			return result, fmt.Errorf("overnight: iteration %d ingest: %w", iterIndex, ingestErr)
		}
		if ingest != nil {
			iter.Ingest = ingestSummary(ingest)
			iter.Degraded = append(iter.Degraded, ingest.Degraded...)
		}

		// --- CHECKPOINT ---
		cp, cpErr := NewCheckpoint(opts.Cwd, string(iterID), opts.CheckpointMaxBytes)
		if cpErr != nil {
			iter.Status = StatusFailed
			iter.Error = fmt.Sprintf("checkpoint: %v", cpErr)
			iter.FinishedAt = time.Now()
			iter.Duration = iter.FinishedAt.Sub(iterStart).String()
			iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
			if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("persist iter-%d (during checkpoint failure): %v", iterIndex, writeErr))
			}
			result.Iterations = append(result.Iterations, iter)
			fmt.Fprintf(log, "overnight: iteration %d checkpoint failed: %v\n", iterIndex, cpErr)
			return result, fmt.Errorf("overnight: iteration %d checkpoint: %w", iterIndex, cpErr)
		}

		// --- REDUCE ---
		// Close-loop callbacks are intentionally nil at this layer.
		// Downstream wiring (cli/cmd/ao/overnight.go) can provide its
		// own RunLoop wrapper if it wants in-process close-loop.
		var emptyCloseLoop lifecycle.CloseLoopOpts
		reduce, reduceErr := RunReduce(loopCtx, opts, ingest, cp, emptyCloseLoop, log)
		if reduceErr != nil {
			iter.Status = StatusRolledBackPreCommit
			iter.Error = fmt.Sprintf("reduce: %v", reduceErr)
			if reduce != nil {
				iter.Reduce = reduceSummary(reduce)
			}
			iter.FinishedAt = time.Now()
			iter.Duration = iter.FinishedAt.Sub(iterStart).String()
			iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
			if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("persist iter-%d (during reduce failure): %v", iterIndex, writeErr))
			}
			result.Iterations = append(result.Iterations, iter)
			// Checkpoint is already rolled back by RunReduce itself.
			fmt.Fprintf(log, "overnight: iteration %d REDUCE failed (rolled back): %v\n", iterIndex, reduceErr)
			return result, fmt.Errorf("overnight: iteration %d reduce: %w", iterIndex, reduceErr)
		}
		if reduce != nil {
			iter.Reduce = reduceSummary(reduce)
			iter.Degraded = append(iter.Degraded, reduce.Degraded...)
		}

		// --- MEASURE (Micro-epic 8 C1 Option A — moved pre-commit) ---
		// Fitness is now computed against the STAGING tree so a regression
		// halt can unwind the checkpoint BEFORE the live ~/.agents/ tree
		// is ever mutated. Under the legacy Option B shape this block lived
		// after cp.Commit() — see
		// .agents/council/2026-04-11-m8-assumption-validation-consolidated.md
		// for the rationale.
		//
		// Micro-epic 6 (C5) test injector path is unchanged: when a test
		// sets SetTestFitnessInjector it bypasses the real corpus.Compute
		// call entirely, so the staging redirect is a no-op for injected
		// fitness. Production runs honour the staging redirect by aliasing
		// opts.Cwd → cp.StagingDir for the measure call only (measure
		// never mutates, so the alias is safe).
		var measure *MeasureResult
		var measureErr error
		if injector := getTestFitnessInjector(); injector != nil {
			snap, injectErr := injector(iterIndex)
			if injectErr != nil {
				measureErr = injectErr
			} else {
				measure = &MeasureResult{FitnessSnapshot: snap}
			}
		} else {
			// Point the measure at the staging tree. RunLoopOptions is
			// passed by value so this local override does not leak.
			measureOpts := opts
			measureOpts.Cwd = cp.StagingDir
			measure, measureErr = RunMeasure(loopCtx, measureOpts, log)
		}
		if measureErr != nil {
			consecutiveMeasureFailures++
			iter.Status = StatusDegraded
			iter.Error = fmt.Sprintf("measure: %v", measureErr)
			iter.FinishedAt = time.Now()
			iter.Duration = iter.FinishedAt.Sub(iterStart).String()
			// Micro-epic 8: measure failed before commit — staging is
			// still live and must be cleaned up. Under Option B this block
			// was post-commit so rollback was a no-op on the live tree;
			// under Option A the live tree is still pristine and we must
			// drop staging so the next iter gets a clean slate.
			if rbErr := cp.Rollback(); rbErr != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("iter-%d rollback after measure failure: %v", iterIndex, rbErr))
			}
			iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
			if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("persist iter-%d (during measure failure): %v", iterIndex, writeErr))
			}
			result.Iterations = append(result.Iterations, iter)
			// Micro-epic 5 (C4): consecutive MEASURE failure cap. -1 is
			// unbounded (legacy behaviour); any non-negative value is a
			// hard halt after that many back-to-back failures. Under
			// Option A a MEASURE failure is pre-commit so the corpus is
			// NOT compounded — halting here leaves the live tree in the
			// last known-good state.
			if opts.MaxConsecutiveMeasureFailures != -1 &&
				consecutiveMeasureFailures >= opts.MaxConsecutiveMeasureFailures {
				result.MeasureFailureHalt = true
				result.FailureReason = fmt.Sprintf(
					"iteration %d: %d consecutive MEASURE failures reached cap %d",
					iterIndex, consecutiveMeasureFailures, opts.MaxConsecutiveMeasureFailures)
				fmt.Fprintf(log, "overnight: RunLoop halted — %s\n", result.FailureReason)
				return result, nil
			}
			fmt.Fprintf(log, "overnight: iteration %d MEASURE failed (%d/%d, continuing): %v\n",
				iterIndex, consecutiveMeasureFailures, opts.MaxConsecutiveMeasureFailures, measureErr)
			continue
		}
		// MEASURE succeeded — reset the consecutive-failure counter so a
		// transient flake earlier in the run does not poison a later
		// stretch of good iterations.
		consecutiveMeasureFailures = 0
		if measure != nil {
			iter.Measure = measureSummary(measure)
			iter.Degraded = append(iter.Degraded, measure.Degraded...)
		}

		// --- DELTA + HALT CHECK ---
		currSnapshot := measure.FitnessSnapshot
		iter.FitnessAfter = snapshotToMap(currSnapshot)
		if prevSnapshot != nil {
			iter.FitnessBefore = snapshotToMap(*prevSnapshot)
			composite, regressions, regressed := currSnapshot.Delta(prevSnapshot, nil, opts.RegressionFloor)
			iter.FitnessDelta = composite

			// effectiveWarnOnly: warn-only protection is active only while
			// the rescue budget has remaining capacity. When WarnOnlyBudget
			// is nil (tests, legacy callers) this is a plain pass-through
			// of opts.WarnOnly. When WarnOnlyBudget is supplied (cmd layer
			// in production), we honour Remaining==0 as "fall back to
			// strict halt" so the ratchet actually ratchets.
			effectiveWarnOnly := opts.WarnOnly
			if opts.WarnOnlyBudget != nil && opts.WarnOnlyBudget.Remaining <= 0 {
				effectiveWarnOnly = false
			}

			if regressed && !effectiveWarnOnly {
				// Micro-epic 8 Option A: strict-mode regression halts
				// BEFORE cp.Commit() — rollback discards staging and the
				// live tree is never mutated. The iter carries the new
				// StatusHaltedOnRegressionPreCommit, which
				// IsCorpusCompounded() correctly reports as false.
				iter.Status = StatusHaltedOnRegressionPreCommit
				iter.FinishedAt = time.Now()
				iter.Duration = iter.FinishedAt.Sub(iterStart).String()
				if rbErr := cp.Rollback(); rbErr != nil {
					result.Degraded = append(result.Degraded,
						fmt.Sprintf("iter-%d rollback after pre-commit regression halt: %v", iterIndex, rbErr))
				}
				iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
				if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
					result.Degraded = append(result.Degraded,
						fmt.Sprintf("persist iter-%d (during pre-commit regression halt): %v", iterIndex, writeErr))
				}
				// NOTE (M8): writeCommittedButFlaggedMarker is NOT called
				// on the pre-commit halt path — the iter never committed
				// so there is nothing "committed but flagged" to mark.
				// The legacy Option B path wrote that marker; Option A
				// removes the concept on the regression-halt branch.
				result.Iterations = append(result.Iterations, iter)
				result.RegressionReason = fmt.Sprintf("iteration %d: %d metric(s) breached regression floor %g: %v",
					iterIndex, len(regressions), opts.RegressionFloor, regressionNames(regressions))
				// Annotate the halt reason when the ratchet was the reason
				// warn-only protection lapsed, so the morning report can
				// distinguish "strict mode from flag" from "budget
				// exhausted mid-run".
				if opts.WarnOnly && opts.WarnOnlyBudget != nil && opts.WarnOnlyBudget.Remaining <= 0 {
					result.RegressionReason += " (warn-only budget exhausted)"
				}
				fmt.Fprintf(log, "overnight: RunLoop halted — %s\n", result.RegressionReason)
				return result, nil
			}
			plateauReached := plateau.Observe(composite)
			if plateauReached && !effectiveWarnOnly {
				// Micro-epic 8: plateau halts also roll back pre-commit.
				// A plateau is a strict-mode "no compounding" signal; no
				// point committing a mutation that doesn't move fitness.
				iter.Status = StatusHaltedOnRegressionPreCommit
				iter.FinishedAt = time.Now()
				iter.Duration = iter.FinishedAt.Sub(iterStart).String()
				if rbErr := cp.Rollback(); rbErr != nil {
					result.Degraded = append(result.Degraded,
						fmt.Sprintf("iter-%d rollback after pre-commit plateau halt: %v", iterIndex, rbErr))
				}
				iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
				if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
					result.Degraded = append(result.Degraded,
						fmt.Sprintf("persist iter-%d (during plateau halt): %v", iterIndex, writeErr))
				}
				result.Iterations = append(result.Iterations, iter)
				result.PlateauReason = plateau.Reason()
				if opts.WarnOnly && opts.WarnOnlyBudget != nil && opts.WarnOnlyBudget.Remaining <= 0 {
					result.PlateauReason += " (warn-only budget exhausted)"
				}
				fmt.Fprintf(log, "overnight: RunLoop halted — %s\n", result.PlateauReason)
				return result, nil
			}

			// Warn-only rescue path. At most ONE rescue is consumed per
			// iteration regardless of whether only regression, only
			// plateau, or both fired — otherwise a single pathological
			// iteration would drain the budget twice as fast as a series
			// of single-event iterations, which is surprising to operators.
			consumedRescue := false
			if regressed {
				iter.Degraded = append(iter.Degraded,
					fmt.Sprintf("regression beyond floor (warn-only): %d metric(s)", len(regressions)))
				consumedRescue = true
			}
			if plateauReached {
				iter.Degraded = append(iter.Degraded,
					fmt.Sprintf("plateau reached (warn-only): %s", plateau.Reason()))
				consumedRescue = true
			}
			if consumedRescue && opts.WarnOnlyBudget != nil {
				opts.WarnOnlyBudget.Remaining--
				result.WarnOnlyBudgetRemaining = opts.WarnOnlyBudget.Remaining
				if opts.WarnOnlyBudget.OnConsume != nil {
					if err := opts.WarnOnlyBudget.OnConsume(opts.WarnOnlyBudget.Remaining); err != nil {
						result.Degraded = append(result.Degraded,
							fmt.Sprintf("warn-only budget persist: %v", err))
					}
				}
				iter.Degraded = append(iter.Degraded,
					fmt.Sprintf("warn-only budget remaining: %d", opts.WarnOnlyBudget.Remaining))
			}
		}

		// --- COMMIT (Micro-epic 8 — moved post-MEASURE/DELTA) ---
		// Fitness passed (or was tolerated under warn-only). Promote the
		// staging tree into live so the next iteration sees the compounded
		// corpus. Under Option A this is the first point at which the live
		// ~/.agents/ tree is mutated — strict-mode regressions never reach
		// this line because they rolled back and returned above.
		if commitErr := cp.Commit(); commitErr != nil {
			iter.Status = StatusFailed
			iter.Error = fmt.Sprintf("commit: %v", commitErr)
			iter.FinishedAt = time.Now()
			iter.Duration = iter.FinishedAt.Sub(iterStart).String()
			iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
			if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("persist iter-%d (during commit failure): %v", iterIndex, writeErr))
			}
			result.Iterations = append(result.Iterations, iter)
			fmt.Fprintf(log, "overnight: iteration %d commit failed: %v\n", iterIndex, commitErr)
			return result, fmt.Errorf("overnight: iteration %d commit: %w", iterIndex, commitErr)
		}
		if injector := getTestPostCommitFaultInjector(); injector != nil {
			if err := injector(iterIndex, opts.Cwd); err != nil {
				result.Degraded = append(result.Degraded,
					fmt.Sprintf("iter-%d post-commit fault injection: %v", iterIndex, err))
			}
		}

		// Post-commit metadata integrity check (ratchet-forward per pm-V7).
		// Cannot unwind a successful commit; record a findings entry and
		// surface via degraded so the morning report shows the strip.
		// Under Option A this runs AFTER the pre-commit fitness gate, so a
		// metadata strip on a fitness-passing iter is a distinct second-
		// stage defect (not a fitness regression).
		if postReport := VerifyMetadataRoundTripPostCommit(cp); !postReport.Pass {
			msg := fmt.Sprintf("post-commit metadata integrity: %d stripped field(s)", len(postReport.StrippedFields))
			iter.Status = StatusHaltedOnRegressionPostCommit
			iter.Degraded = append(iter.Degraded, msg)
			fmt.Fprintf(log, "overnight: iteration %d %s\n", iterIndex, msg)
			// Log a structured finding the router will intentionally skip
			// (filename bypasses findingFilenameRe); surfaces in morning report.
			_ = logPostCommitFinding(opts.Cwd, iterIndex, postReport)
		}

		iter.FinishedAt = time.Now()
		iter.Duration = iter.FinishedAt.Sub(iterStart).String()

		// Persist BEFORE appending to in-memory state. Order matters: if
		// we append first and then persist, a write failure would leave
		// result.Iterations holding N entries while disk holds N-1, making
		// the morning report lie about what happened.
		//
		// A persist failure in the HAPPY path is NOT a hard halt — the
		// corpus on disk is correct (commit already succeeded), we just
		// lost the ability to record this iteration's summary for the
		// morning report or resume. Degrade + continue. The operator sees
		// the strip via result.Degraded.
		iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
		if writeErr := writeIterationAtomic(iterDir, iter); writeErr != nil {
			msg := fmt.Sprintf("persist iter-%d: %v", iterIndex, writeErr)
			iter.Degraded = append(iter.Degraded, msg)
			result.Degraded = append(result.Degraded, msg)
			fmt.Fprintf(log, "overnight: iteration %d persist failed (degraded, continuing): %v\n",
				iterIndex, writeErr)
		}

		result.Iterations = append(result.Iterations, iter)
		snap := currSnapshot
		prevSnapshot = &snap

		fmt.Fprintf(log, "overnight: iteration %d done (elapsed=%s, total=%s)\n",
			iterIndex, iter.Duration, time.Since(startedAt))

		// Test-only fault injection: panic after persist, before the next
		// iter starts. Used by TestRunLoop_CrashAtIter2_ResumeRehydrates.
		if fi := getFaultInjectionAfterIter(); fi > 0 && iterIndex == fi {
			panic(fmt.Sprintf("overnight: test fault injection at iter %d", iterIndex))
		}
	}
}

// ingestSummary marshals an IngestResult into the opaque map the
// IterationSummary uses so downstream JSON consumers see it without
// pulling in Go types.
func ingestSummary(r *IngestResult) map[string]any {
	return map[string]any{
		"harvest_preview_count": r.HarvestPreviewCount,
		"forge_artifacts_mined": r.ForgeArtifactsMined,
		"provenance_audited":    r.ProvenanceAudited,
		"mine_findings_new":     r.MineFindingsNew,
	}
}

// reduceSummary marshals a ReduceResult into the opaque map the
// IterationSummary uses.
func reduceSummary(r *ReduceResult) map[string]any {
	return map[string]any{
		"harvest_promoted":    r.HarvestPromoted,
		"dedup_merged":        r.DedupMerged,
		"maturity_tempered":   r.MaturityTempered,
		"defrag_pruned":       r.DefragPruned,
		"close_loop_promoted": r.CloseLoopPromoted,
		"findings_routed":     r.FindingsRouted,
		"inject_refreshed":    r.InjectRefreshed,
		"rolled_back":         r.RolledBack,
	}
}

// measureSummary marshals a MeasureResult into the opaque map the
// IterationSummary uses.
func measureSummary(r *MeasureResult) map[string]any {
	out := map[string]any{
		"findings_resolved": r.FindingsResolved,
		"inject_visibility": r.InjectVisibility,
	}
	if r.Fitness != nil {
		out["fitness"] = r.Fitness
	}
	return out
}

// snapshotToMap serializes a FitnessSnapshot's metric map for the
// IterationSummary's fitness_before / fitness_after fields.
func snapshotToMap(s FitnessSnapshot) map[string]any {
	out := make(map[string]any, len(s.Metrics))
	for k, v := range s.Metrics {
		out[k] = v
	}
	return out
}

// mapToSnapshot is the inverse of snapshotToMap. Returns nil if the input
// is nil or does not contain a valid metric map. Used during resume to
// rehydrate prevSnapshot from the last persisted iteration.
//
// json.Unmarshal of a numeric metric through a map[string]any yields
// float64, so the common case is handled by the float64 branch. int and
// json.Number are accepted for forward-compatibility with non-default
// decoders. Non-numeric values are silently dropped — Dream does not
// write non-numeric metrics today; if that changes, this helper should
// be updated in lock-step.
func mapToSnapshot(m map[string]any) *FitnessSnapshot {
	if m == nil {
		return nil
	}
	snap := FitnessSnapshot{Metrics: make(map[string]float64, len(m))}
	for k, v := range m {
		switch n := v.(type) {
		case float64:
			snap.Metrics[k] = n
		case int:
			snap.Metrics[k] = float64(n)
		case int64:
			snap.Metrics[k] = float64(n)
		case json.Number:
			if f, err := n.Float64(); err == nil {
				snap.Metrics[k] = f
			}
		}
	}
	if len(snap.Metrics) == 0 {
		return nil
	}
	return &snap
}

// regressionNames extracts the metric names from a MetricRegression slice
// for inclusion in the RegressionReason string.
func regressionNames(rs []MetricRegression) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

// logPostCommitFinding writes a structured finding to .agents/findings/
// describing a post-commit metadata integrity failure. The filename
// intentionally does NOT match findingFilenameRe, so the findings router
// will skip it — post-commit strips belong in the morning report as
// warnings, not as auto-routed work.
func logPostCommitFinding(cwd string, iterIndex int, report MetadataIntegrityReport) error {
	findingsDir := filepath.Join(cwd, ".agents", "findings")
	if err := os.MkdirAll(findingsDir, 0o755); err != nil {
		return err
	}
	date := time.Now().Format("2006-01-02")
	name := fmt.Sprintf("f-%s-postcommit-iter-%d.md", date, iterIndex)
	path := filepath.Join(findingsDir, name)

	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: Post-commit metadata integrity drift (iter %d)\n", iterIndex)
	fmt.Fprintf(&b, "type: finding\nseverity: high\ndate: %s\n---\n\n", date)
	fmt.Fprintf(&b, "The post-commit VerifyMetadataRoundTripPostCommit check detected %d stripped field(s) "+
		"after Dream iteration %d landed. These fields were present in the pre-REDUCE baseline but are "+
		"missing from the post-commit live tree. This indicates late-stage corruption in the commit swap "+
		"itself, since the pre-commit check would have caught a reducer-layer strip.\n\n",
		len(report.StrippedFields), iterIndex)
	fmt.Fprintln(&b, "## Stripped fields")
	fmt.Fprintln(&b, "")
	for _, sf := range report.StrippedFields {
		fmt.Fprintf(&b, "- `%s`: key `%s`\n", sf.File, sf.Key)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
