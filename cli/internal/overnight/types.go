package overnight

import (
	"fmt"
	"io"
	"time"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
)

// IterationID is the stable identifier for a single iteration inside a
// Dream run. Format: "<run-id>-iter-<N>" where N is 1-based.
type IterationID string

// IterationStatus is the mechanically-verifiable status of a single Dream
// iteration. Values are exhaustive; each has distinct semantics that
// downstream consumers (morning report renderer, rehydration logic,
// invariant tests) depend on.
type IterationStatus string

const (
	// StatusDone: happy path. INGEST + REDUCE + COMMIT + MEASURE all
	// succeeded; the compounded corpus is permanently on disk; fitness
	// delta was non-regressing (or regression was tolerated in WarnOnly
	// mode). The iteration contributed forward progress.
	StatusDone IterationStatus = "done"

	// StatusDegraded: MEASURE failed before commit. The checkpoint is
	// rolled back and the live tree is unchanged. The loop continues
	// with a stale prevSnapshot from the last fully-done iteration.
	StatusDegraded IterationStatus = "degraded"

	// StatusRolledBackPreCommit: REDUCE failed BEFORE commit. The
	// checkpoint was rolled back by RunReduce itself; the live tree is
	// unchanged. No corpus mutation happened. This is a clean rollback
	// and the iteration should be skipped during rehydration.
	StatusRolledBackPreCommit IterationStatus = "rolled-back-pre-commit"

	// StatusHaltedOnRegressionPostCommit: the commit succeeded AND the
	// corpus was compounded, but a post-commit regression check fired.
	// The corpus is IN the live tree (checkpoint.Rollback post-commit
	// does not touch the live tree). The loop halted here in strict
	// mode OR continued with a degraded note in WarnOnly mode.
	// Rehydration MUST include this iteration when computing
	// prevSnapshot because the corpus state is the post-compound state.
	//
	// This replaces the legacy rolled-back string at the post-commit
	// regression halt site. The old string was a LIE: it claimed
	// rollback while the corpus stayed committed. Micro-epic 3 is the
	// fix.
	//
	// NOTE (Micro-epic 8 / na-h61): With Option A semantics, strict-mode
	// regression halts now fire BEFORE commit and use
	// StatusHaltedOnRegressionPreCommit. This status is retained for:
	//   (a) warn-only rescue paths that consume a rescue and commit anyway
	//       but then regress on a SECOND late-stage check (pm-V7 metadata
	//       integrity), and
	//   (b) backward compatibility with persisted iterations from pre-M8
	//       runs.
	StatusHaltedOnRegressionPostCommit IterationStatus = "halted-on-regression-post-commit"

	// StatusHaltedOnRegressionPreCommit (Micro-epic 8 / na-h61, C1 Option A):
	// fitness regression was detected BEFORE cp.Commit() and the checkpoint
	// was rolled back. The live tree is UNCHANGED — external observers of
	// ~/.agents/ never saw the partial/regressed state. No corpus mutation
	// happened; rehydration MUST skip this iteration exactly like
	// StatusRolledBackPreCommit.
	//
	// This is distinct from StatusRolledBackPreCommit because the rollback
	// reason is different (fitness regression, not REDUCE-stage failure),
	// and downstream reporters (morning report, next-work classifier) need
	// to distinguish "REDUCE blew up" from "fitness gate held the line".
	StatusHaltedOnRegressionPreCommit IterationStatus = "halted-on-regression-pre-commit"

	// StatusFailed: any unrecoverable error in INGEST, CHECKPOINT, or
	// COMMIT itself. Distinct from StatusRolledBackPreCommit because
	// the failure was NOT a clean rollback — it may have left partial
	// state that RecoverFromCrash handles on the next startup.
	StatusFailed IterationStatus = "failed"
)

// Validate returns an error if s is not one of the defined status
// constants. Useful as an invariant check in tests and for callers that
// want to reject unknown/legacy status strings at boundary points.
//
// Note: LoadIterations does NOT call Validate — legacy persisted files
// with the old rolled-back string are silently tolerated as a
// conservative fallback (they fail IsCorpusCompounded and are skipped
// during rehydration, matching pre-Micro-epic-3 behavior).
func (s IterationStatus) Validate() error {
	switch s {
	case StatusDone, StatusDegraded, StatusRolledBackPreCommit,
		StatusHaltedOnRegressionPostCommit, StatusHaltedOnRegressionPreCommit,
		StatusFailed:
		return nil
	case "":
		return fmt.Errorf("overnight: IterationStatus is empty")
	default:
		return fmt.Errorf("overnight: unknown IterationStatus %q", string(s))
	}
}

// IsCorpusCompounded reports whether the iteration's corpus mutation
// landed on disk. True for StatusDone and
// StatusHaltedOnRegressionPostCommit — both represent states where
// cp.Commit() succeeded before the iteration terminated. False for
// StatusDegraded and StatusRolledBackPreCommit (no mutation happened),
// and StatusFailed (may have partial state; RecoverFromCrash handles).
//
// This is the single source of truth for rehydration logic: an iteration
// with IsCorpusCompounded() == true is a valid prevSnapshot baseline
// regardless of whether the loop then halted.
func (s IterationStatus) IsCorpusCompounded() bool {
	switch s {
	case StatusDone, StatusHaltedOnRegressionPostCommit:
		return true
	}
	return false
}

// RunLoopOptions bundles every knob RunLoop consumes from the caller.
//
// All callers flow through the overnight command (cli/cmd/ao/overnight.go);
// the options struct is intentionally flat so flag-to-field mapping stays
// easy to audit. Defaults are documented inline; zero values are never
// interpreted as "do something special" — validators in RunLoop substitute
// the documented default instead.
type RunLoopOptions struct {
	// Cwd is the repository root. Dream's .agents/ lives under this path.
	Cwd string

	// OutputDir is where per-run artifacts are written. Typically
	// ".agents/overnight/<run-id>/".
	OutputDir string

	// RunID is the caller-assigned identifier for this Dream run. Used to
	// namespace per-iteration persistence under <OutputDir>/<RunID>/iterations/
	// so two runs sharing an OutputDir (the default `.agents/overnight/latest`
	// is shared by every run) cannot cross-contaminate each other's history.
	// Also used as the "<run-id>" prefix in IterationID values per the contract
	// at line 10. Required; RunLoop returns an error if empty.
	RunID string

	// RunTimeout caps the outer loop's wall-clock. Default: 2h.
	// Capped at 6h regardless of flag value.
	RunTimeout time.Duration

	// MaxIterations caps the outer loop count. 0 = budget-bounded only.
	MaxIterations int

	// PlateauEpsilon is the |delta| threshold that counts as "no progress"
	// for a single iteration. Default: 0.01.
	PlateauEpsilon float64

	// PlateauWindowK is the number of consecutive sub-epsilon deltas
	// required to declare a plateau. Default: 2. Never less than 2 — a
	// single noisy sample is absorbed by design.
	PlateauWindowK int

	// RegressionFloor is the maximum allowed drop for any single metric in
	// the fitness vector between iterations. Default: 0.05.
	RegressionFloor float64

	// WarnOnly, when true, turns plateau and regression detection into
	// warnings instead of halts. Used during the first 2-3 production
	// Dream runs to calibrate thresholds empirically. Default: true (opt
	// in to strict mode once thresholds are calibrated).
	WarnOnly bool

	// MaxConsecutiveMeasureFailures caps how many consecutive MEASURE
	// failures the loop tolerates before halting with
	// MeasureFailureHalt=true. Sentinel disambiguation:
	//   0  = normalize() substitutes the documented default (3)
	//   -1 = unbounded (legacy "continue forever" behaviour; degraded
	//        iterations accumulate without limit)
	//   >0 = halt after exactly that many consecutive failures
	//
	// Because 0 is the zero value for an unset field, callers use
	// WithMeasureFailureCap(n) to set both this field AND
	// explicitMeasureFailureCap atomically. This lets normalize()
	// distinguish "caller explicitly set 0 (halt on first failure)"
	// from "caller did not touch the field" — same sentinel pattern
	// used throughout the Go ecosystem for boolean-adjacent ints.
	MaxConsecutiveMeasureFailures int

	// explicitMeasureFailureCap is the companion sentinel for
	// MaxConsecutiveMeasureFailures. It is unexported so callers cannot
	// set it directly — they must go through WithMeasureFailureCap to
	// keep the two fields in sync. normalize() treats
	// explicitMeasureFailureCap=false as "apply default", regardless of
	// the MaxConsecutiveMeasureFailures value.
	explicitMeasureFailureCap bool

	// WarnOnlyBudget, when non-nil, enables the C3 warn-only ratchet:
	// warn-only rescues are counted down and once exhausted the loop
	// reverts to strict halting behaviour for the rest of the run. When
	// nil (the default), warn-only is unbounded — the legacy behaviour
	// preserved for L1 tests that exercise the loop through many
	// synthetic events.
	//
	// Cmd-layer callers wire this up via WarnOnlyRatchetFromDisk so
	// rescue consumption is persisted to
	// .agents/overnight/warn-only-budget.json across runs. Tests leave
	// it nil and get infinite warn-only.
	WarnOnlyBudget *WarnOnlyRatchet

	// QueuePath optionally points at an operator-pinned roadmap
	// (markdown) whose items Dream works in order before falling through
	// to fitness-driven work selection. Reuses the evolve pinned-queue
	// format; see skills/evolve/references/pinned-queue.md.
	QueuePath string

	// CloseLoopCallbacks optionally wires the in-process flywheel
	// close-loop helpers into REDUCE so Dream can execute the real
	// maintenance path instead of degrading with "callbacks not wired".
	// Leaving this zero-valued preserves the previous "skip close-loop"
	// behavior for callers that do not need the mutation.
	CloseLoopCallbacks lifecycle.CloseLoopOpts

	// CheckpointMaxBytes caps disk usage across all concurrent checkpoint
	// snapshots in a single run. Default: 512 MB. On exceed, NewCheckpoint
	// returns an error and the iteration degrades (never half-mutates).
	CheckpointMaxBytes int64

	// LockStaleAfter is the threshold past which a .agents/overnight/run.lock
	// file with a dead PID is reclaimed on startup. Default: 12h.
	LockStaleAfter time.Duration

	// LogWriter receives structured progress output. Typically Dream's
	// existing overnight.log file. Nil is allowed — RunLoop substitutes
	// io.Discard.
	LogWriter io.Writer
}

// WithMeasureFailureCap returns a copy of opts with the consecutive
// MEASURE failure cap set atomically — both the cap value and the
// explicit-set flag are written together so normalize() can distinguish
// a caller-provided cap of 0 (halt on first failure) from the unset
// default. Pass -1 to disable the cap entirely (unbounded).
func (opts RunLoopOptions) WithMeasureFailureCap(n int) RunLoopOptions {
	opts.MaxConsecutiveMeasureFailures = n
	opts.explicitMeasureFailureCap = true
	return opts
}

// WarnOnlyRatchet is the caller-supplied budget for the C3 warn-only
// ratchet. The loop reads Remaining to decide whether warn-only protection
// is still in effect, mutates it in-place when a rescue is consumed, and
// invokes OnConsume (if non-nil) so the caller can persist the new value.
//
// The loop never opens the budget file on its own. All I/O lives in the
// cmd layer via WriteBudget/DecrementBudget, keeping RunLoop pure and
// leaving tests free to construct a budget without touching disk.
type WarnOnlyRatchet struct {
	// Initial is the rescue ceiling at the start of the loop. Surfaced
	// into RunLoopResult for the morning report renderer.
	Initial int

	// Remaining is the live rescue counter. The loop decrements this in
	// place; when it hits zero, warn-only protection is off for the rest
	// of the run.
	Remaining int

	// OnConsume, when non-nil, is called each time the loop consumes one
	// rescue. It receives the new Remaining value after the decrement.
	// A returned error is appended to result.Degraded but does not halt
	// the loop — a persistence failure must not wedge Dream.
	OnConsume func(newRemaining int) error
}

// defaultRunTimeout is the documented default wall-clock cap.
const defaultRunTimeout = 2 * time.Hour

// maxRunTimeout is the hard upper bound regardless of flag value.
const maxRunTimeout = 6 * time.Hour

// defaultPlateauEpsilon is the documented default plateau threshold.
const defaultPlateauEpsilon = 0.01

// defaultPlateauWindowK is the documented default plateau window.
const defaultPlateauWindowK = 2

// defaultRegressionFloor is the documented default per-metric regression floor.
const defaultRegressionFloor = 0.05

// defaultCheckpointMaxBytes is the documented default checkpoint-storage cap.
const defaultCheckpointMaxBytes = int64(512 * 1024 * 1024) // 512 MB

// defaultLockStaleAfter is the documented default stale-lock reclaim threshold.
const defaultLockStaleAfter = 12 * time.Hour

// defaultMaxConsecutiveMeasureFailures is the documented default for the
// C4 MEASURE consecutive-failure cap. Three failures is large enough to
// absorb a single transient MEASURE flake followed by two retries, and
// small enough that a systemic MEASURE breakage halts within one minute
// of wall-clock time rather than silently accumulating degraded
// iterations for the full run budget. Derived from the 2026-02-22
// evolve-overnight 115-cycle runaway retrospective.
const defaultMaxConsecutiveMeasureFailures = 3

// normalize returns a copy of opts with zero/out-of-range values replaced
// by documented defaults. It never returns an error; every correction is
// a silent substitution recorded in the RunLoopResult's Degraded list.
func (opts RunLoopOptions) normalize() (RunLoopOptions, []string) {
	var degraded []string
	if opts.RunTimeout <= 0 {
		opts.RunTimeout = defaultRunTimeout
	}
	if opts.RunTimeout > maxRunTimeout {
		degraded = append(degraded, "RunTimeout clamped to 6h hard max")
		opts.RunTimeout = maxRunTimeout
	}
	if opts.PlateauEpsilon <= 0 {
		opts.PlateauEpsilon = defaultPlateauEpsilon
	}
	if opts.PlateauWindowK < defaultPlateauWindowK {
		// K=1 is ill-defined; a single noisy sample would halt the loop.
		if opts.PlateauWindowK > 0 {
			degraded = append(degraded, "PlateauWindowK raised to 2 (minimum)")
		}
		opts.PlateauWindowK = defaultPlateauWindowK
	}
	if opts.RegressionFloor <= 0 {
		opts.RegressionFloor = defaultRegressionFloor
	}
	if opts.CheckpointMaxBytes <= 0 {
		opts.CheckpointMaxBytes = defaultCheckpointMaxBytes
	}
	if opts.LockStaleAfter <= 0 {
		opts.LockStaleAfter = defaultLockStaleAfter
	}
	// Micro-epic 5 (C4): apply the default cap only when the caller did
	// not go through WithMeasureFailureCap. A caller-provided 0 is
	// preserved (halt on first failure); a caller-provided -1 is
	// preserved (unbounded); any untouched field gets the documented
	// default.
	if !opts.explicitMeasureFailureCap {
		opts.MaxConsecutiveMeasureFailures = defaultMaxConsecutiveMeasureFailures
	}
	return opts, degraded
}
