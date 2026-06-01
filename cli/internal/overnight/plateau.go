package overnight

import (
	"fmt"
)

// PlateauState is a rolling window of fitness deltas used to detect when
// the outer Dream loop has stopped making meaningful progress.
//
// Observe fires only when K consecutive |delta| values all fall below
// epsilon. A single out-of-window sample clears the streak and forces
// the state machine to accumulate K fresh sub-epsilon observations
// before it will fire. This absorbs a single noisy iteration by design
// — see pm-FEAS-08 in the pre-mortem for the rationale (we would
// rather spend one extra iteration than halt on a transient wobble).
//
// PlateauState is not safe for concurrent use. The outer loop runs
// sequentially so a mutex would be pure overhead.
type PlateauState struct {
	// window holds the most recent sub-epsilon |delta| observations.
	// Its length is capped at k; on overflow the oldest entry is
	// discarded. A non-sub-epsilon observation truncates it back to
	// empty.
	window []float64
	// k is the number of consecutive sub-epsilon deltas required to
	// fire. Always >= 2; enforced by NewPlateauState (which returns an
	// error otherwise).
	k int
	// epsilon is the absolute-value threshold below which a delta is
	// considered "no progress".
	epsilon float64
	// halted records whether Observe has already fired on this state.
	// Subsequent Observe calls return false until Reset is invoked.
	halted bool
	// haltedReason is the human-readable reason string returned by
	// Reason once halted is true.
	haltedReason string
}

// NewPlateauState constructs a plateau tracker configured with the
// given window size k and absolute-value threshold epsilon.
//
// Returns an error if k < 2. A window of 1 would mean a single noisy
// sample could halt the loop, which is the exact failure mode the
// plateau state machine is designed to absorb. RunLoopOptions.normalize()
// enforces the same invariant on the flag surface, so only programmer
// error in direct callers should ever trigger the error path.
func NewPlateauState(k int, epsilon float64) (*PlateauState, error) {
	if k < 2 {
		return nil, fmt.Errorf("overnight.NewPlateauState: k must be >= 2, got %d", k)
	}
	return &PlateauState{
		window:  make([]float64, 0, k),
		k:       k,
		epsilon: epsilon,
	}, nil
}

// Observe feeds the latest iteration delta into the state machine and
// returns true iff the plateau condition just fired on this call.
//
// Observe is a one-shot signal: once it has fired, subsequent calls
// keep returning false until Reset is invoked. The caller is expected
// to halt the loop on the first true return and never rely on
// repeated firings.
//
// The absolute value of delta is compared against epsilon so that a
// mildly-negative delta (small regression inside the per-metric
// floors) still counts toward plateau. A large negative delta that
// exceeds epsilon truncates the window back to empty.
func (p *PlateauState) Observe(delta float64) bool {
	if p.halted {
		return false
	}

	abs := delta
	if abs < 0 {
		abs = -abs
	}

	if abs >= p.epsilon {
		// Out of window — truncate the streak.
		p.window = p.window[:0]
		return false
	}

	p.window = append(p.window, abs)
	if len(p.window) > p.k {
		// Slide the window: drop the oldest observation. We only
		// care about the most recent k values.
		p.window = p.window[len(p.window)-p.k:]
	}
	if len(p.window) >= p.k {
		p.halted = true
		p.haltedReason = fmt.Sprintf("plateau: %d consecutive |delta| < %g", p.k, p.epsilon)
		return true
	}
	return false
}

// Reset clears the rolling window and the halted flag, returning the
// state machine to its freshly-constructed state. Intended for tests;
// production callers construct a new PlateauState per run.
func (p *PlateauState) Reset() {
	p.window = p.window[:0]
	p.halted = false
	p.haltedReason = ""
}

// Reason returns a human-readable description of why the state
// machine halted, or the empty string if Observe has never fired on
// this instance. The returned string embeds both the configured K and
// epsilon so log output is self-contained.
func (p *PlateauState) Reason() string {
	return p.haltedReason
}
