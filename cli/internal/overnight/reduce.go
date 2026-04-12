package overnight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/harvest"
	"github.com/boshu2/agentops/cli/internal/lifecycle"
)

// reduceStageRecorder is an optional test hook called at the start of
// each REDUCE stage. Tests install it via TestRunReduce_FullStageOrder_Enforced
// to assert the full execution order. Production leaves it nil.
var reduceStageRecorder func(stageName string)

// ReduceResult is the output of a single REDUCE stage.
//
// REDUCE is the only Dream stage that mutates .agents/, so the result
// includes checkpoint-level integrity metadata alongside the per-stage
// counters. Callers use this struct to decide commit-or-rollback in the
// outer loop.
type ReduceResult struct {
	// HarvestPromoted is the count of artifacts promoted by harvest.
	HarvestPromoted int

	// DedupMerged is the count of near-duplicate learnings removed by
	// lifecycle.ExecuteDedup.
	DedupMerged int

	// MaturityTempered is the count of learnings whose maturity field
	// was tempered during REDUCE. Always zero in the first slice —
	// the in-process maturity-temper entry is a follow-up.
	MaturityTempered int

	// DefragPruned is the count of orphan learnings removed by
	// lifecycle.ExecutePrune.
	DefragPruned int

	// CloseLoopPromoted is the count of learnings promoted to artifacts
	// by lifecycle.ExecuteCloseLoop.
	CloseLoopPromoted int

	// FindingsRouted is the count of findings routed to next-work.jsonl
	// by RouteFindings.
	FindingsRouted int

	// InjectRefreshed indicates whether the inject-cache refresh stage
	// ran successfully. Flipped to true by Wave 4 Issue 16 when the
	// inject-refresh stage completes without error.
	InjectRefreshed bool

	// InjectRefreshResult is the structured outcome of the
	// inject-cache refresh stage. Nil when the stage never ran (for
	// example, when the caller overrode refreshInjectCacheFn in a way
	// that bypassed the stage). Populated in all other cases — the
	// stage is best-effort and captures degraded notes rather than
	// rolling back the iteration.
	InjectRefreshResult *InjectRefreshResult

	// MetadataIntegrity is the report from checkpoint.VerifyMetadataRoundTrip.
	MetadataIntegrity MetadataIntegrityReport

	// CheckpointPath is the absolute staging dir of the checkpoint that
	// REDUCE drove, for debugging and morning-report breadcrumbs.
	CheckpointPath string

	// RolledBack is true iff RunReduce invoked cp.Rollback() internally.
	RolledBack bool

	// RollbackReason is the human-readable explanation for the rollback,
	// empty when RolledBack is false.
	RollbackReason string

	// Degraded lists substage notes for soft-failed stages.
	Degraded []string

	// StageFailures maps substage name to error string for stages that
	// returned a hard error.
	StageFailures map[string]string

	// Duration is the wall-clock time RunReduce took end-to-end.
	Duration time.Duration
}

// reduceStage is a small struct used by RunReduce to label each ordered
// step in the stage order. It stays package-private because the stage
// order is a contract that lives in the RunReduce implementation, not
// in the public API.
type reduceStage struct {
	name string
	run  func() error
}

// RunReduce executes the serial REDUCE stage through the checkpoint overlay.
//
// Stage order (contract — see plan Implementation Section 1):
//
//  1. harvest.Promote(catalog, dest, dryRun=false)
//  2. lifecycle.ExecuteDedup(cwd, dryRun=false)
//  3. maturity temper (stub — deferred to follow-up slice)
//  4. lifecycle.ExecutePrune(cwd, dryRun=false, staleDays=30)
//  5. lifecycle.ExecuteCloseLoop(cwd, closeLoopCallbacks) — skipped when
//     the callback set is nil so tests can exercise rollback without
//     wiring the full cmd/ao helper graph.
//  6. RouteFindings(cwd) — findings → next-work router.
//  7. RefreshInjectCache(ctx, cwd) — best-effort inject-cache refresh
//     (Wave 4 Issue 16). Closes PRODUCT.md Gap #1's loop framing
//     ("harvest → forge → INJECT → report"). Failures here are
//     captured as degraded notes on the result and do NOT trigger a
//     rollback: a stale inject cache is less bad than discarding the
//     compounded corpus this iteration already landed.
//  8. VerifyMetadataRoundTrip(cp) — frontmatter strip guard (pm-005).
//
// If ANY stage (1-6) returns an error OR the integrity check in stage 8
// fails, RunReduce invokes cp.Rollback() and returns a non-nil error
// with a populated RollbackReason on the result. Partial counters are
// preserved so the morning report can show what landed before the
// rollback.
//
// RunReduce does NOT call cp.Commit() itself — that responsibility
// belongs to the outer loop (RunLoop, Wave 4). The caller decides to
// commit or rollback based on the subsequent MEASURE result.
//
// The closeLoopCallbacks parameter lets tests inject stubs and lets the
// command layer inject real cmd/ao helpers. When every required
// callback field is nil, stage 5 is skipped with a degraded note; this
// allows Wave 3 tests to exercise the rollback logic without needing
// the full cmd/ao wiring (that lands in Wave 4).
//
//nolint:gocyclo // RunReduce keeps the REDUCE stage table and rollback boundary together.
func RunReduce(
	ctx context.Context,
	opts RunLoopOptions,
	ingest *IngestResult,
	cp *Checkpoint,
	closeLoopCallbacks lifecycle.CloseLoopOpts,
	log io.Writer,
) (*ReduceResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = io.Discard
	}
	started := time.Now()
	result := newReduceResult(cp)
	if err := validateReduceInputs(opts, cp); err != nil {
		return result, err
	}

	runner := newReduceRunner(ctx, opts, ingest, cp, closeLoopCallbacks, log, result, started)
	if err := runner.runStages(); err != nil {
		return result, err
	}
	if err := runner.verifyMetadataIntegrity(); err != nil {
		return result, err
	}

	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/reduce: done in %s\n", result.Duration)
	return result, nil
}

type reduceRunner struct {
	ctx                context.Context
	opts               RunLoopOptions
	ingest             *IngestResult
	cp                 *Checkpoint
	closeLoopCallbacks lifecycle.CloseLoopOpts
	closeLoopWired     bool
	log                io.Writer
	result             *ReduceResult
	started            time.Time
	stagingCwd         string
}

func newReduceResult(cp *Checkpoint) *ReduceResult {
	result := &ReduceResult{StageFailures: map[string]string{}}
	if cp != nil {
		result.CheckpointPath = cp.StagingDir
	}
	return result
}

func validateReduceInputs(opts RunLoopOptions, cp *Checkpoint) error {
	if opts.Cwd == "" {
		return fmt.Errorf("overnight: RunReduce requires RunLoopOptions.Cwd")
	}
	if cp == nil {
		return fmt.Errorf("overnight: RunReduce requires a non-nil Checkpoint")
	}
	return nil
}

func newReduceRunner(
	ctx context.Context,
	opts RunLoopOptions,
	ingest *IngestResult,
	cp *Checkpoint,
	closeLoopCallbacks lifecycle.CloseLoopOpts,
	log io.Writer,
	result *ReduceResult,
	started time.Time,
) *reduceRunner {
	return &reduceRunner{
		ctx:                ctx,
		opts:               opts,
		ingest:             ingest,
		cp:                 cp,
		closeLoopCallbacks: closeLoopCallbacks,
		closeLoopWired:     closeLoopCallbacksPresent(closeLoopCallbacks),
		log:                log,
		result:             result,
		started:            started,
		stagingCwd:         cp.StagingDir,
	}
}

func (r *reduceRunner) stages() []reduceStage {
	return []reduceStage{
		{name: "harvest-promote", run: r.runHarvestPromote},
		{name: "dedup", run: r.runDedup},
		{name: "maturity-temper", run: r.runMaturityTemper},
		{name: "defrag-prune", run: r.runDefragPrune},
		{name: "close-loop", run: r.runCloseLoop},
		{name: "findings-router", run: r.runFindingsRouter},
		{name: "inject-refresh", run: r.runInjectRefresh},
	}
}

func (r *reduceRunner) runStages() error {
	for _, stage := range r.stages() {
		if err := ctxCheck(r.ctx); err != nil {
			r.result.Duration = stageDurationSince(r.started)
			r.rollback(fmt.Sprintf("context cancelled at %s: %v", stage.name, err))
			return err
		}
		fmt.Fprintf(r.log, "overnight/reduce: %s start\n", stage.name)
		if err := stage.run(); err != nil {
			r.result.StageFailures[stage.name] = err.Error()
			r.result.Duration = stageDurationSince(r.started)
			r.rollback(fmt.Sprintf("stage %s failed: %v", stage.name, err))
			return fmt.Errorf("overnight/reduce: stage %s: %w", stage.name, err)
		}
		fmt.Fprintf(r.log, "overnight/reduce: %s done\n", stage.name)
	}
	return nil
}

func (r *reduceRunner) rollback(reason string) {
	r.result.RolledBack = true
	r.result.RollbackReason = reason
	if rbErr := r.cp.Rollback(); rbErr != nil {
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("rollback failed: %v", rbErr))
	}
	fmt.Fprintf(r.log, "overnight/reduce: rolled back (%s)\n", reason)
}

func (r *reduceRunner) recordStage(stageName string) {
	if reduceStageRecorder != nil {
		reduceStageRecorder(stageName)
	}
}

func (r *reduceRunner) runHarvestPromote() error {
	r.recordStage("harvest-promote")
	if r.ingest == nil || r.ingest.HarvestCatalog == nil {
		r.result.Degraded = append(r.result.Degraded, "harvest-promote: no catalog from INGEST, skipped")
		return nil
	}
	dest := filepath.Join(r.stagingCwd, ".agents", "learnings")
	count, err := harvest.Promote(r.ingest.HarvestCatalog, dest, false)
	r.result.HarvestPromoted = count
	return err
}

func (r *reduceRunner) runDedup() error {
	r.recordStage("dedup")
	dr, err := lifecycle.ExecuteDedup(r.stagingCwd, false)
	if err != nil {
		return err
	}
	if dr != nil {
		r.result.DedupMerged = len(dr.Deleted)
	}
	return nil
}

func (r *reduceRunner) runMaturityTemper() error {
	r.recordStage("maturity-temper")
	r.result.Degraded = append(r.result.Degraded, "maturity-temper: in-process entry deferred to follow-up")
	return nil
}

func (r *reduceRunner) runDefragPrune() error {
	r.recordStage("defrag-prune")
	pr, err := lifecycle.ExecutePrune(r.stagingCwd, false, 30)
	if err != nil {
		return err
	}
	if pr != nil {
		r.result.DefragPruned = len(pr.Deleted)
	}
	return nil
}

func (r *reduceRunner) runCloseLoop() error {
	r.recordStage("close-loop")
	if !r.closeLoopWired {
		r.result.Degraded = append(r.result.Degraded, "close-loop: callbacks not wired")
		return nil
	}
	clr, err := lifecycle.ExecuteCloseLoop(r.stagingCwd, r.closeLoopCallbacks)
	if err != nil {
		return err
	}
	if clr != nil {
		r.result.CloseLoopPromoted = clr.AutoPromote.Promoted
	}
	return nil
}

func (r *reduceRunner) runFindingsRouter() error {
	r.recordStage("findings-router")
	routed, degraded, err := RouteFindings(r.stagingCwd)
	if err != nil {
		return err
	}
	r.result.FindingsRouted = routed
	for _, d := range degraded {
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("findings-router: %s", d))
	}
	return nil
}

func (r *reduceRunner) runInjectRefresh() error {
	r.recordStage("inject-refresh")
	ir, err := refreshInjectCacheFn(r.ctx, r.stagingCwd, r.log)
	if ir != nil {
		r.result.InjectRefreshResult = ir
		r.result.InjectRefreshed = ir.Succeeded
		for _, d := range ir.Degraded {
			r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("inject-refresh: %s", d))
		}
	}
	if err != nil {
		r.result.Degraded = append(r.result.Degraded, fmt.Sprintf("inject-refresh: soft-failed: %v", err))
	}
	return nil
}

func (r *reduceRunner) verifyMetadataIntegrity() error {
	if err := ctxCheck(r.ctx); err != nil {
		r.result.Duration = stageDurationSince(r.started)
		r.rollback(fmt.Sprintf("context cancelled before integrity check: %v", err))
		return err
	}
	r.result.MetadataIntegrity = VerifyMetadataRoundTrip(r.cp)
	if r.result.MetadataIntegrity.Pass {
		return nil
	}
	stripped := len(r.result.MetadataIntegrity.StrippedFields)
	reason := fmt.Sprintf("metadata integrity failed: %d stripped field(s)", stripped)
	r.result.Duration = stageDurationSince(r.started)
	r.rollback(reason)
	return fmt.Errorf("overnight/reduce: %s", reason)
}

// closeLoopCallbacksPresent reports whether the caller has wired enough
// of the close-loop callback surface for ExecuteCloseLoop to run. The
// lifecycle package enforces its own required-field checks, but
// RunReduce looks at the same required set first so a fully-zero opts
// value is treated as "skip this stage" instead of a hard error.
//
// The required set mirrors the checks in lifecycle.ExecuteCloseLoop: if
// any of the core callbacks is nil, we skip; otherwise we let
// ExecuteCloseLoop run and enforce its own invariants.
func closeLoopCallbacksPresent(opts lifecycle.CloseLoopOpts) bool {
	if opts.ResolveIngestFiles == nil {
		return false
	}
	if opts.IngestFilesToPool == nil {
		return false
	}
	if opts.AutoPromoteFn == nil {
		return false
	}
	if opts.ProcessCitationFeedback == nil {
		return false
	}
	if opts.PromoteCitedLearnings == nil {
		return false
	}
	if opts.PromoteToMemory == nil {
		return false
	}
	// Either ApplyMaturityFn or FindLearningFile must be present.
	if opts.ApplyMaturityFn == nil && opts.FindLearningFile == nil {
		return false
	}
	return true
}

// ErrReduceRollback is a sentinel used in tests to assert that a
// failing reduce stage drove a rollback and not just a soft-fail.
var ErrReduceRollback = errors.New("overnight: reduce rolled back")
