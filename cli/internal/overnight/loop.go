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

	priorIterations, degraded := loadRunLoopHistory(opts, degraded)
	result := newRunLoopResult(priorIterations, degraded, opts)
	if err := validateRunLoopOptions(opts); err != nil {
		return result, err
	}

	fmt.Fprintf(log, "overnight: RunLoop starting (budget=%s, max_iter=%d, epsilon=%g, K=%d, warn_only=%v)\n",
		opts.RunTimeout, opts.MaxIterations, opts.PlateauEpsilon, opts.PlateauWindowK, opts.WarnOnly)

	loopCtx, cancel := context.WithTimeout(ctx, opts.RunTimeout)
	defer cancel()

	startedAt := time.Now()
	state := newRunLoopState(opts, log, result, priorIterations)
	iterIndex := len(priorIterations) // Resume from N+1 (incremented at top of loop)

	for {
		if state.shouldStop(loopCtx, iterIndex) {
			return result, nil
		}

		iterIndex++
		iterStart := time.Now()
		iter := newLoopIteration(opts.RunID, iterIndex, iterStart)
		fmt.Fprintf(log, "overnight: iteration %d starting (id=%s)\n", iterIndex, iter.ID)

		ingest, err := state.runIngestStage(loopCtx, &iter, iterStart, iterIndex)
		if err != nil {
			return result, err
		}
		cp, err := state.newCheckpointStage(&iter, iterStart, iterIndex)
		if err != nil {
			return result, err
		}
		if err := state.runReduceStage(loopCtx, ingest, cp, &iter, iterStart, iterIndex); err != nil {
			return result, err
		}

		measure, measureErr := state.runMeasureStage(loopCtx, cp, iterIndex)
		if measureErr != nil {
			if state.handleMeasureError(cp, &iter, iterStart, iterIndex, measureErr) {
				return result, nil
			}
			continue
		}
		state.consecutiveMeasureFailures = 0
		if measure != nil {
			iter.Measure = measureSummary(measure)
			iter.Degraded = append(iter.Degraded, measure.Degraded...)
		}

		if state.applyFitnessHalt(cp, &iter, iterStart, iterIndex, measure) {
			return result, nil
		}
		if err := state.commitIteration(cp, &iter, iterStart, iterIndex); err != nil {
			return result, err
		}
		state.finishSuccessfulIteration(&iter, iterStart, startedAt, iterIndex, measure.FitnessSnapshot)

		if fi := getFaultInjectionAfterIter(); fi > 0 && iterIndex == fi {
			panic(fmt.Sprintf("overnight: test fault injection at iter %d", iterIndex))
		}
	}
}

type runLoopState struct {
	opts                       RunLoopOptions
	log                        io.Writer
	result                     *RunLoopResult
	plateau                    *PlateauState
	prevSnapshot               *FitnessSnapshot
	consecutiveMeasureFailures int
}

func loadRunLoopHistory(opts RunLoopOptions, degraded []string) ([]IterationSummary, []string) {
	if opts.OutputDir == "" || opts.RunID == "" {
		return nil, degraded
	}
	iterDir := filepath.Join(opts.OutputDir, opts.RunID, "iterations")
	prior, rejected, loadErr := LoadIterations(iterDir, opts.RunID)
	if loadErr != nil {
		degraded = append(degraded, fmt.Sprintf("rehydrate: %v", loadErr))
	}
	for _, rej := range rejected {
		degraded = append(degraded, fmt.Sprintf("rehydrate rejected: %s", rej))
	}
	return prior, degraded
}

func newRunLoopResult(priorIterations []IterationSummary, degraded []string, opts RunLoopOptions) *RunLoopResult {
	result := &RunLoopResult{
		Iterations: priorIterations,
		Degraded:   degraded,
	}
	if opts.WarnOnlyBudget != nil {
		result.WarnOnlyBudgetInitial = opts.WarnOnlyBudget.Initial
		result.WarnOnlyBudgetRemaining = opts.WarnOnlyBudget.Remaining
	}
	return result
}

func validateRunLoopOptions(opts RunLoopOptions) error {
	if opts.Cwd == "" {
		return fmt.Errorf("overnight: RunLoopOptions.Cwd is required")
	}
	if opts.OutputDir == "" {
		return fmt.Errorf("overnight: RunLoopOptions.OutputDir is required")
	}
	if opts.RunID == "" {
		return fmt.Errorf("overnight: RunLoopOptions.RunID is required")
	}
	return nil
}

func newRunLoopState(
	opts RunLoopOptions,
	log io.Writer,
	result *RunLoopResult,
	priorIterations []IterationSummary,
) *runLoopState {
	state := &runLoopState{
		opts:         opts,
		log:          log,
		result:       result,
		plateau:      NewPlateauState(opts.PlateauWindowK, opts.PlateauEpsilon),
		prevSnapshot: lastCompoundedSnapshot(priorIterations),
	}
	replayPriorDeltas(state.plateau, priorIterations, opts.PlateauWindowK)
	return state
}

func lastCompoundedSnapshot(priorIterations []IterationSummary) *FitnessSnapshot {
	for i := len(priorIterations) - 1; i >= 0; i-- {
		if !priorIterations[i].Status.IsCorpusCompounded() {
			continue
		}
		if snap := mapToSnapshot(priorIterations[i].FitnessAfter); snap != nil {
			return snap
		}
		break
	}
	return nil
}

func replayPriorDeltas(plateau *PlateauState, priorIterations []IterationSummary, windowK int) {
	if len(priorIterations) == 0 || windowK <= 0 {
		return
	}
	var successfulTail []IterationSummary
	for i := len(priorIterations) - 1; i >= 0 && len(successfulTail) < windowK; i-- {
		if priorIterations[i].Status.IsCorpusCompounded() {
			successfulTail = append([]IterationSummary{priorIterations[i]}, successfulTail...)
		}
	}
	for _, it := range successfulTail {
		_ = plateau.Observe(it.FitnessDelta)
	}
}

func (s *runLoopState) shouldStop(loopCtx context.Context, iterIndex int) bool {
	select {
	case <-loopCtx.Done():
		if errors.Is(loopCtx.Err(), context.DeadlineExceeded) {
			s.result.BudgetExhausted = true
			fmt.Fprintln(s.log, "overnight: RunLoop halted — wall-clock budget exhausted")
		} else {
			fmt.Fprintf(s.log, "overnight: RunLoop halted — context cancelled: %v\n", loopCtx.Err())
		}
		return true
	default:
	}

	if s.opts.MaxIterations > 0 && iterIndex >= s.opts.MaxIterations {
		fmt.Fprintf(s.log, "overnight: RunLoop halted — max iterations (%d) reached\n", s.opts.MaxIterations)
		return true
	}
	return false
}

func newLoopIteration(runID string, iterIndex int, iterStart time.Time) IterationSummary {
	return IterationSummary{
		ID:        IterationID(fmt.Sprintf("%s-iter-%d", runID, iterIndex)),
		Index:     iterIndex,
		StartedAt: iterStart,
		Status:    StatusDone,
	}
}

func finishIteration(iter *IterationSummary, iterStart time.Time) {
	iter.FinishedAt = time.Now()
	iter.Duration = iter.FinishedAt.Sub(iterStart).String()
}

func (s *runLoopState) iterationDir() string {
	return filepath.Join(s.opts.OutputDir, s.opts.RunID, "iterations")
}

func (s *runLoopState) persistIterationDuring(iter IterationSummary, context string) {
	if writeErr := writeIterationAtomic(s.iterationDir(), iter); writeErr != nil {
		s.result.Degraded = append(s.result.Degraded,
			fmt.Sprintf("persist iter-%d (%s): %v", iter.Index, context, writeErr))
	}
}

func (s *runLoopState) finishPersistAndAppend(iter *IterationSummary, iterStart time.Time, context string) {
	finishIteration(iter, iterStart)
	s.persistIterationDuring(*iter, context)
	s.result.Iterations = append(s.result.Iterations, *iter)
}

func (s *runLoopState) runIngestStage(
	loopCtx context.Context,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
) (*IngestResult, error) {
	var ingest *IngestResult
	var ingestErr error
	if injector := getTestIngestFaultInjector(); injector != nil {
		ingestErr = injector(iterIndex)
	} else {
		ingest, ingestErr = RunIngest(loopCtx, s.opts, s.log)
	}
	if ingestErr != nil {
		iter.Status = StatusFailed
		iter.Error = fmt.Sprintf("ingest: %v", ingestErr)
		s.finishPersistAndAppend(iter, iterStart, "during ingest failure")
		fmt.Fprintf(s.log, "overnight: iteration %d INGEST failed: %v\n", iterIndex, ingestErr)
		return nil, fmt.Errorf("overnight: iteration %d ingest: %w", iterIndex, ingestErr)
	}
	if ingest != nil {
		iter.Ingest = ingestSummary(ingest)
		iter.Degraded = append(iter.Degraded, ingest.Degraded...)
	}
	return ingest, nil
}

func (s *runLoopState) newCheckpointStage(
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
) (*Checkpoint, error) {
	cp, cpErr := NewCheckpoint(s.opts.Cwd, string(iter.ID), s.opts.CheckpointMaxBytes)
	if cpErr != nil {
		iter.Status = StatusFailed
		iter.Error = fmt.Sprintf("checkpoint: %v", cpErr)
		s.finishPersistAndAppend(iter, iterStart, "during checkpoint failure")
		fmt.Fprintf(s.log, "overnight: iteration %d checkpoint failed: %v\n", iterIndex, cpErr)
		return nil, fmt.Errorf("overnight: iteration %d checkpoint: %w", iterIndex, cpErr)
	}
	return cp, nil
}

func (s *runLoopState) runReduceStage(
	loopCtx context.Context,
	ingest *IngestResult,
	cp *Checkpoint,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
) error {
	var emptyCloseLoop lifecycle.CloseLoopOpts
	reduce, reduceErr := RunReduce(loopCtx, s.opts, ingest, cp, emptyCloseLoop, s.log)
	if reduceErr != nil {
		iter.Status = StatusRolledBackPreCommit
		iter.Error = fmt.Sprintf("reduce: %v", reduceErr)
		if reduce != nil {
			iter.Reduce = reduceSummary(reduce)
		}
		s.finishPersistAndAppend(iter, iterStart, "during reduce failure")
		fmt.Fprintf(s.log, "overnight: iteration %d REDUCE failed (rolled back): %v\n", iterIndex, reduceErr)
		return fmt.Errorf("overnight: iteration %d reduce: %w", iterIndex, reduceErr)
	}
	if reduce != nil {
		iter.Reduce = reduceSummary(reduce)
		iter.Degraded = append(iter.Degraded, reduce.Degraded...)
	}
	return nil
}

func (s *runLoopState) runMeasureStage(loopCtx context.Context, cp *Checkpoint, iterIndex int) (*MeasureResult, error) {
	if injector := getTestFitnessInjector(); injector != nil {
		snap, injectErr := injector(iterIndex)
		if injectErr != nil {
			return nil, injectErr
		}
		return &MeasureResult{FitnessSnapshot: snap}, nil
	}
	measureOpts := s.opts
	measureOpts.Cwd = cp.StagingDir
	return RunMeasure(loopCtx, measureOpts, s.log)
}

func (s *runLoopState) handleMeasureError(
	cp *Checkpoint,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
	measureErr error,
) bool {
	s.consecutiveMeasureFailures++
	iter.Status = StatusDegraded
	iter.Error = fmt.Sprintf("measure: %v", measureErr)
	finishIteration(iter, iterStart)
	if rbErr := cp.Rollback(); rbErr != nil {
		s.result.Degraded = append(s.result.Degraded,
			fmt.Sprintf("iter-%d rollback after measure failure: %v", iterIndex, rbErr))
	}
	s.persistIterationDuring(*iter, "during measure failure")
	s.result.Iterations = append(s.result.Iterations, *iter)
	if s.opts.MaxConsecutiveMeasureFailures != -1 &&
		s.consecutiveMeasureFailures >= s.opts.MaxConsecutiveMeasureFailures {
		s.result.MeasureFailureHalt = true
		s.result.FailureReason = fmt.Sprintf(
			"iteration %d: %d consecutive MEASURE failures reached cap %d",
			iterIndex, s.consecutiveMeasureFailures, s.opts.MaxConsecutiveMeasureFailures)
		fmt.Fprintf(s.log, "overnight: RunLoop halted — %s\n", s.result.FailureReason)
		return true
	}
	fmt.Fprintf(s.log, "overnight: iteration %d MEASURE failed (%d/%d, continuing): %v\n",
		iterIndex, s.consecutiveMeasureFailures, s.opts.MaxConsecutiveMeasureFailures, measureErr)
	return false
}

type fitnessHaltEvaluation struct {
	regressions       []MetricRegression
	regressed         bool
	plateauReached    bool
	effectiveWarnOnly bool
}

func (s *runLoopState) applyFitnessHalt(
	cp *Checkpoint,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
	measure *MeasureResult,
) bool {
	eval, ok := s.evaluateFitness(iter, measure.FitnessSnapshot)
	if !ok {
		return false
	}
	if eval.regressed && !eval.effectiveWarnOnly {
		s.haltForRegression(cp, iter, iterStart, iterIndex, eval)
		return true
	}
	if eval.plateauReached && !eval.effectiveWarnOnly {
		s.haltForPlateau(cp, iter, iterStart, iterIndex)
		return true
	}
	s.consumeWarnOnlyRescue(iter, eval)
	return false
}

func (s *runLoopState) evaluateFitness(
	iter *IterationSummary,
	currSnapshot FitnessSnapshot,
) (fitnessHaltEvaluation, bool) {
	iter.FitnessAfter = snapshotToMap(currSnapshot)
	if s.prevSnapshot == nil {
		return fitnessHaltEvaluation{}, false
	}
	iter.FitnessBefore = snapshotToMap(*s.prevSnapshot)
	composite, regressions, regressed := currSnapshot.Delta(s.prevSnapshot, nil, s.opts.RegressionFloor)
	iter.FitnessDelta = composite
	eval := fitnessHaltEvaluation{
		regressions:       regressions,
		regressed:         regressed,
		effectiveWarnOnly: s.effectiveWarnOnly(),
	}
	if !regressed || eval.effectiveWarnOnly {
		eval.plateauReached = s.plateau.Observe(composite)
	}
	return eval, true
}

func (s *runLoopState) effectiveWarnOnly() bool {
	if s.opts.WarnOnlyBudget != nil && s.opts.WarnOnlyBudget.Remaining <= 0 {
		return false
	}
	return s.opts.WarnOnly
}

func (s *runLoopState) haltForRegression(
	cp *Checkpoint,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
	eval fitnessHaltEvaluation,
) {
	iter.Status = StatusHaltedOnRegressionPreCommit
	finishIteration(iter, iterStart)
	if rbErr := cp.Rollback(); rbErr != nil {
		s.result.Degraded = append(s.result.Degraded,
			fmt.Sprintf("iter-%d rollback after pre-commit regression halt: %v", iterIndex, rbErr))
	}
	s.persistIterationDuring(*iter, "during pre-commit regression halt")
	s.result.Iterations = append(s.result.Iterations, *iter)
	s.result.RegressionReason = fmt.Sprintf("iteration %d: %d metric(s) breached regression floor %g: %v",
		iterIndex, len(eval.regressions), s.opts.RegressionFloor, regressionNames(eval.regressions))
	if s.warnOnlyBudgetExhausted() {
		s.result.RegressionReason += " (warn-only budget exhausted)"
	}
	fmt.Fprintf(s.log, "overnight: RunLoop halted — %s\n", s.result.RegressionReason)
}

func (s *runLoopState) haltForPlateau(
	cp *Checkpoint,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
) {
	iter.Status = StatusHaltedOnRegressionPreCommit
	finishIteration(iter, iterStart)
	if rbErr := cp.Rollback(); rbErr != nil {
		s.result.Degraded = append(s.result.Degraded,
			fmt.Sprintf("iter-%d rollback after pre-commit plateau halt: %v", iterIndex, rbErr))
	}
	s.persistIterationDuring(*iter, "during plateau halt")
	s.result.Iterations = append(s.result.Iterations, *iter)
	s.result.PlateauReason = s.plateau.Reason()
	if s.warnOnlyBudgetExhausted() {
		s.result.PlateauReason += " (warn-only budget exhausted)"
	}
	fmt.Fprintf(s.log, "overnight: RunLoop halted — %s\n", s.result.PlateauReason)
}

func (s *runLoopState) warnOnlyBudgetExhausted() bool {
	return s.opts.WarnOnly && s.opts.WarnOnlyBudget != nil && s.opts.WarnOnlyBudget.Remaining <= 0
}

func (s *runLoopState) consumeWarnOnlyRescue(iter *IterationSummary, eval fitnessHaltEvaluation) {
	consumedRescue := false
	if eval.regressed {
		iter.Degraded = append(iter.Degraded,
			fmt.Sprintf("regression beyond floor (warn-only): %d metric(s)", len(eval.regressions)))
		consumedRescue = true
	}
	if eval.plateauReached {
		iter.Degraded = append(iter.Degraded,
			fmt.Sprintf("plateau reached (warn-only): %s", s.plateau.Reason()))
		consumedRescue = true
	}
	if !consumedRescue || s.opts.WarnOnlyBudget == nil {
		return
	}
	s.opts.WarnOnlyBudget.Remaining--
	s.result.WarnOnlyBudgetRemaining = s.opts.WarnOnlyBudget.Remaining
	if s.opts.WarnOnlyBudget.OnConsume != nil {
		if err := s.opts.WarnOnlyBudget.OnConsume(s.opts.WarnOnlyBudget.Remaining); err != nil {
			s.result.Degraded = append(s.result.Degraded, fmt.Sprintf("warn-only budget persist: %v", err))
		}
	}
	iter.Degraded = append(iter.Degraded,
		fmt.Sprintf("warn-only budget remaining: %d", s.opts.WarnOnlyBudget.Remaining))
}

func (s *runLoopState) commitIteration(
	cp *Checkpoint,
	iter *IterationSummary,
	iterStart time.Time,
	iterIndex int,
) error {
	if commitErr := cp.Commit(); commitErr != nil {
		iter.Status = StatusFailed
		iter.Error = fmt.Sprintf("commit: %v", commitErr)
		s.finishPersistAndAppend(iter, iterStart, "during commit failure")
		fmt.Fprintf(s.log, "overnight: iteration %d commit failed: %v\n", iterIndex, commitErr)
		return fmt.Errorf("overnight: iteration %d commit: %w", iterIndex, commitErr)
	}
	if msg := runTestPostCommitFaultInjector(iterIndex, s.opts.Cwd); msg != "" {
		s.result.Degraded = append(s.result.Degraded, msg)
	}
	s.recordPostCommitMetadataCheck(cp, iter, iterIndex)
	return nil
}

func (s *runLoopState) recordPostCommitMetadataCheck(cp *Checkpoint, iter *IterationSummary, iterIndex int) {
	postReport := VerifyMetadataRoundTripPostCommit(cp)
	if postReport.Pass {
		return
	}
	msg := fmt.Sprintf("post-commit metadata integrity: %d stripped field(s)", len(postReport.StrippedFields))
	iter.Status = StatusHaltedOnRegressionPostCommit
	iter.Degraded = append(iter.Degraded, msg)
	fmt.Fprintf(s.log, "overnight: iteration %d %s\n", iterIndex, msg)
	_ = logPostCommitFinding(s.opts.Cwd, iterIndex, postReport)
}

func (s *runLoopState) finishSuccessfulIteration(
	iter *IterationSummary,
	iterStart time.Time,
	startedAt time.Time,
	iterIndex int,
	currSnapshot FitnessSnapshot,
) {
	finishIteration(iter, iterStart)
	if writeErr := writeIterationAtomic(s.iterationDir(), *iter); writeErr != nil {
		msg := fmt.Sprintf("persist iter-%d: %v", iterIndex, writeErr)
		iter.Degraded = append(iter.Degraded, msg)
		s.result.Degraded = append(s.result.Degraded, msg)
		fmt.Fprintf(s.log, "overnight: iteration %d persist failed (degraded, continuing): %v\n",
			iterIndex, writeErr)
	}
	s.result.Iterations = append(s.result.Iterations, *iter)
	snap := currSnapshot
	s.prevSnapshot = &snap
	fmt.Fprintf(s.log, "overnight: iteration %d done (elapsed=%s, total=%s)\n",
		iterIndex, iter.Duration, time.Since(startedAt))
}

func runTestPostCommitFaultInjector(iterIndex int, cwd string) string {
	if injector := getTestPostCommitFaultInjector(); injector != nil {
		if err := injector(iterIndex, cwd); err != nil {
			return fmt.Sprintf("iter-%d post-commit fault injection: %v", iterIndex, err)
		}
	}
	return ""
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
