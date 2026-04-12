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

	result := &ReduceResult{
		StageFailures: map[string]string{},
	}
	if cp != nil {
		result.CheckpointPath = cp.StagingDir
	}

	if opts.Cwd == "" {
		return result, fmt.Errorf("overnight: RunReduce requires RunLoopOptions.Cwd")
	}
	if cp == nil {
		return result, fmt.Errorf("overnight: RunReduce requires a non-nil Checkpoint")
	}

	// rollback is a helper closure that drives Rollback and records the
	// reason onto result. It never shadows the primary error; the caller
	// still gets the original failure back.
	rollback := func(reason string) {
		result.RolledBack = true
		result.RollbackReason = reason
		if rbErr := cp.Rollback(); rbErr != nil {
			result.Degraded = append(result.Degraded,
				fmt.Sprintf("rollback failed: %v", rbErr))
		}
		fmt.Fprintf(log, "overnight/reduce: rolled back (%s)\n", reason)
	}

	closeLoopWired := closeLoopCallbacksPresent(closeLoopCallbacks)

	// stagingCwd is the pseudo-cwd every mutative stage targets. The
	// checkpoint lays out its staging tree so that StagingDir/.agents/<sub>
	// mirrors opts.Cwd/.agents/<sub>; passing StagingDir as the cwd to
	// lifecycle.* and RouteFindings routes every mutation into the
	// checkpoint's staging copy, where it can be rolled back or committed
	// atomically. Mutating opts.Cwd directly (the pre-Wave-4 bug caught in
	// vibe finding V1) would invert the Commit semantics and destroy the
	// corpus on first successful commit.
	stagingCwd := cp.StagingDir

	stages := []reduceStage{
		{
			name: "harvest-promote",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("harvest-promote")
				}
				if ingest == nil || ingest.HarvestCatalog == nil {
					result.Degraded = append(result.Degraded,
						"harvest-promote: no catalog from INGEST, skipped")
					return nil
				}
				// Promote INTO the staging copy of .agents/learnings so
				// the cross-rig consolidated artifacts land inside the
				// checkpoint boundary and are subject to Commit/Rollback.
				// Writing to ~/.agents/learnings from inside REDUCE would
				// leak outside the checkpoint (vibe finding V2).
				dest := filepath.Join(stagingCwd, ".agents", "learnings")
				count, err := harvest.Promote(ingest.HarvestCatalog, dest, false)
				result.HarvestPromoted = count
				return err
			},
		},
		{
			name: "dedup",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("dedup")
				}
				dr, err := lifecycle.ExecuteDedup(stagingCwd, false)
				if err != nil {
					return err
				}
				if dr != nil {
					result.DedupMerged = len(dr.Deleted)
				}
				return nil
			},
		},
		{
			name: "maturity-temper",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("maturity-temper")
				}
				result.Degraded = append(result.Degraded,
					"maturity-temper: in-process entry deferred to follow-up")
				return nil
			},
		},
		{
			name: "defrag-prune",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("defrag-prune")
				}
				pr, err := lifecycle.ExecutePrune(stagingCwd, false, 30)
				if err != nil {
					return err
				}
				if pr != nil {
					result.DefragPruned = len(pr.Deleted)
				}
				return nil
			},
		},
		{
			name: "close-loop",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("close-loop")
				}
				if !closeLoopWired {
					result.Degraded = append(result.Degraded,
						"close-loop: callbacks not wired")
					return nil
				}
				clr, err := lifecycle.ExecuteCloseLoop(stagingCwd, closeLoopCallbacks)
				if err != nil {
					return err
				}
				if clr != nil {
					result.CloseLoopPromoted = clr.AutoPromote.Promoted
				}
				return nil
			},
		},
		{
			name: "findings-router",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("findings-router")
				}
				routed, degraded, err := RouteFindings(stagingCwd)
				if err != nil {
					return err
				}
				result.FindingsRouted = routed
				for _, d := range degraded {
					result.Degraded = append(result.Degraded,
						fmt.Sprintf("findings-router: %s", d))
				}
				return nil
			},
		},
		{
			// inject-refresh is best-effort: an error here is captured
			// as a degraded note and does NOT trigger a rollback. A
			// stale inject cache is strictly less bad than discarding
			// the compounded corpus landed in stages 1-6. See pm-006
			// in the Wave 4 pre-mortem and PRODUCT.md Gap #1.
			//
			// Targets stagingCwd so the cache rebuilt pre-commit reflects
			// the staged corpus; Commit promotes the rebuilt cache along
			// with the compounded corpus in one atomic swap.
			name: "inject-refresh",
			run: func() error {
				if reduceStageRecorder != nil {
					reduceStageRecorder("inject-refresh")
				}
				ir, err := refreshInjectCacheFn(ctx, stagingCwd, log)
				if ir != nil {
					result.InjectRefreshResult = ir
					result.InjectRefreshed = ir.Succeeded
					for _, d := range ir.Degraded {
						result.Degraded = append(result.Degraded,
							fmt.Sprintf("inject-refresh: %s", d))
					}
				}
				if err != nil {
					// Capture as a soft failure: record the error
					// string on Degraded so the morning report can
					// surface it, but return nil so the stage loop
					// does not roll back the iteration.
					result.Degraded = append(result.Degraded,
						fmt.Sprintf("inject-refresh: soft-failed: %v", err))
				}
				return nil
			},
		},
	}

	for _, stage := range stages {
		if err := ctxCheck(ctx); err != nil {
			result.Duration = stageDurationSince(started)
			rollback(fmt.Sprintf("context cancelled at %s: %v", stage.name, err))
			return result, err
		}
		fmt.Fprintf(log, "overnight/reduce: %s start\n", stage.name)
		if err := stage.run(); err != nil {
			result.StageFailures[stage.name] = err.Error()
			result.Duration = stageDurationSince(started)
			rollback(fmt.Sprintf("stage %s failed: %v", stage.name, err))
			return result, fmt.Errorf("overnight/reduce: stage %s: %w", stage.name, err)
		}
		fmt.Fprintf(log, "overnight/reduce: %s done\n", stage.name)
	}

	// Stage 8: metadata integrity check. The round-trip guard compares
	// frontmatter keys in the staging snapshot against the live tree —
	// but REDUCE hasn't run cp.Commit() yet, so the live tree still
	// holds the pre-REDUCE state. VerifyMetadataRoundTrip is still
	// meaningful here: we confirm that any learning that existed at
	// staging time has not been stripped from the live tree by an
	// out-of-band mutation. Wave 4 will add a post-commit verification
	// pass once RunLoop is driving Commit.
	if err := ctxCheck(ctx); err != nil {
		result.Duration = stageDurationSince(started)
		rollback(fmt.Sprintf("context cancelled before integrity check: %v", err))
		return result, err
	}
	result.MetadataIntegrity = VerifyMetadataRoundTrip(cp)
	if !result.MetadataIntegrity.Pass {
		stripped := len(result.MetadataIntegrity.StrippedFields)
		reason := fmt.Sprintf("metadata integrity failed: %d stripped field(s)", stripped)
		result.Duration = stageDurationSince(started)
		rollback(reason)
		return result, fmt.Errorf("overnight/reduce: %s", reason)
	}

	result.Duration = stageDurationSince(started)
	fmt.Fprintf(log, "overnight/reduce: done in %s\n", result.Duration)
	return result, nil
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
