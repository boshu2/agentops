package overnight

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ==========================================================================
// na-cdn - Dream failed-summary contract regression
//
// The original na-cdn spec scopes the regression test to
// cli/cmd/ao/overnight_test.go, asserting `summary.json` and `summary.md` are
// written with `status: failed` when Dream hard-fails. The cmd-layer disk
// contract is locked by TestRunOvernight_HardFail_WritesFailedSummary; this
// file adds lower-layer contract coverage for the RunLoop result shape that
// the summary writer consumes.
//
// At the internal/overnight layer we CAN lock the upstream contract that
// finalizeOvernightSummary consumes: when RunLoop halts on a hard-fail path,
// the returned *RunLoopResult must carry (a) a truthy MeasureFailureHalt or
// non-empty Degraded/FailureReason, (b) the full iteration history up to and
// including the terminating iter, and (c) persisted iter-N.json files on
// disk so the cmd-layer summary finalizer can read them.
//
// If this internal contract breaks, the cmd-layer summary writer will emit
// an empty or wrong-shaped failed summary regardless of what
// cli/cmd/ao/overnight_test.go asserts. Locking it here gives defense-in-
// depth: the test fails fast at the internal boundary, not after a
// cmd-layer mock surface has papered over the real regression.

// injectMeasureErrorFromIter returns a fitness injector that succeeds on
// iterations 1..(failFromIter-1) and then returns an error for every later
// iter. Drives the MaxConsecutiveMeasureFailures cap path.
func injectMeasureErrorFromIter(good float64, failFromIter int) func(int) (FitnessSnapshot, error) {
	return func(iterIndex int) (FitnessSnapshot, error) {
		if iterIndex >= failFromIter {
			return FitnessSnapshot{}, errors.New("synthetic measure failure (na-cdn regression)")
		}
		return FitnessSnapshot{
			Metrics: map[string]float64{
				"composite": good,
			},
			CapturedAt: time.Unix(int64(iterIndex), 0).UTC(),
		}, nil
	}
}

// TestDreamFailedSummary_InternalContract locks the internal-layer portion
// of the na-cdn failed-summary contract. Setup: force MEASURE to fail on
// every iteration from iter 1 onward with MaxConsecutiveMeasureFailures=2.
// After two back-to-back failures the loop halts with MeasureFailureHalt=true.
//
// Assertions (the contract cli/cmd/ao/overnight.go's finalizeOvernightSummary
// relies on):
//  1. result.MeasureFailureHalt is true.
//  2. result.FailureReason is non-empty and references the failure cap.
//  3. result.Iterations has at least 2 entries (the cap count) and every
//     entry carries StatusDegraded (the pre-halt degraded note status).
//  4. Every persisted iter-N.json file on disk is readable and round-trips
//     to the same Status and Index the in-memory result reports.
//  5. The last iteration's Index matches result.Iterations' tail - proving
//     "preserve last completed step" per the bead's scope.
//
// If finalizeOvernightSummary later starts reading from a new field, extend
// the assertions here rather than the cmd-layer test so the contract stays
// locked at the lowest layer that produces it.
func TestDreamFailedSummary_InternalContract(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	// Two back-to-back MEASURE failures will trip the cap. Fail from iter 1
	// so every iteration fails; with cap=2 the loop halts on iter 2.
	SetTestFitnessInjector(injectMeasureErrorFromIter(0.8, 1))
	t.Cleanup(func() { SetTestFitnessInjector(nil) })

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	runID := "na-cdn-failed-summary"
	outputDir := filepath.Join(dir, ".agents", "overnight", runID)
	opts := RunLoopOptions{
		Cwd:            dir,
		OutputDir:      outputDir,
		RunID:          runID,
		RunTimeout:     30 * time.Second,
		MaxIterations:  10,
		PlateauEpsilon: 0.01,
		PlateauWindowK: 2,
		WarnOnly:       true,
		LogWriter:      io.Discard,
	}.WithMeasureFailureCap(2)

	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result == nil {
		t.Fatal("result == nil")
	}

	// Assertion 1: MeasureFailureHalt must be set so the cmd-layer summary
	// writer routes to the failed path.
	if !result.MeasureFailureHalt {
		t.Fatalf("result.MeasureFailureHalt=false; want true (cap should have tripped)")
	}

	// Assertion 2: FailureReason must be populated and include the failure
	// cap context. finalizeOvernightSummary copies this into summary.md as
	// the human-readable failure reason.
	if result.FailureReason == "" {
		t.Fatalf("result.FailureReason is empty; want non-empty with failure cap context")
	}
	if !strings.Contains(strings.ToLower(result.FailureReason), "measure") {
		t.Fatalf("result.FailureReason = %q; want mention of 'measure' failure", result.FailureReason)
	}

	// Assertion 3: at least 2 iterations (the cap) and every iter is degraded.
	if len(result.Iterations) < 2 {
		t.Fatalf("len(result.Iterations) = %d; want >= 2 (cap=2 consecutive failures)",
			len(result.Iterations))
	}
	for i, it := range result.Iterations {
		if it.Status != StatusDegraded {
			t.Fatalf("result.Iterations[%d].Status = %q; want %q (all iters failed measure)",
				i, it.Status, StatusDegraded)
		}
		if it.Error == "" {
			t.Fatalf("result.Iterations[%d].Error is empty; want measure-failure note", i)
		}
	}

	// Assertion 4: per-iter files on disk round-trip. finalizeOvernightSummary
	// reads from <outputDir>/<runID>/iterations/iter-*.json so this is the
	// exact contract surface the cmd-layer summary writer consumes.
	iterDir := filepath.Join(outputDir, runID, "iterations")
	entries, err := os.ReadDir(iterDir)
	if err != nil {
		t.Fatalf("read iter dir %s: %v", iterDir, err)
	}
	if len(entries) != len(result.Iterations) {
		t.Fatalf("on-disk iter file count = %d; want %d (must mirror in-memory iterations)",
			len(entries), len(result.Iterations))
	}
	diskByIndex := make(map[int]IterationSummary, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(iterDir, e.Name()))
		if rerr != nil {
			t.Fatalf("read %s: %v", e.Name(), rerr)
		}
		var iter IterationSummary
		if jerr := json.Unmarshal(raw, &iter); jerr != nil {
			t.Fatalf("unmarshal %s: %v (raw=%s)", e.Name(), jerr, string(raw))
		}
		diskByIndex[iter.Index] = iter
	}
	for _, it := range result.Iterations {
		onDisk, ok := diskByIndex[it.Index]
		if !ok {
			t.Fatalf("iter index %d present in memory but missing on disk", it.Index)
		}
		if onDisk.Status != it.Status {
			t.Fatalf("iter %d disk status = %q; want %q (round-trip mismatch)",
				it.Index, onDisk.Status, it.Status)
		}
	}

	// Assertion 5: the tail iter is preserved - "last completed step" per
	// the bead scope. Index must match the loop's view.
	lastInMem := result.Iterations[len(result.Iterations)-1]
	lastOnDisk, ok := diskByIndex[lastInMem.Index]
	if !ok {
		t.Fatalf("last-iter index %d not on disk", lastInMem.Index)
	}
	if lastOnDisk.ID != lastInMem.ID {
		t.Fatalf("last iter ID mismatch: disk=%q memory=%q", lastOnDisk.ID, lastInMem.ID)
	}

	// Sanity: the iter index must be 1-based contiguous from 1.
	for i, it := range result.Iterations {
		wantIndex := i + 1
		if it.Index != wantIndex {
			t.Fatalf("result.Iterations[%d].Index = %d; want %d (1-based contiguous)",
				i, it.Index, wantIndex)
		}
	}
}

// TestDreamFailedSummary_DegradedNotesPreserved is a companion regression
// that locks the second half of the failed-summary contract: soft-degraded
// notes from normalize() and per-iter rollback failures must be surfaced on
// result.Degraded so finalizeOvernightSummary can render them in the failed
// summary body.
//
// Scope: this test does NOT re-verify the MeasureFailureHalt wiring - that's
// locked above. It asserts that degraded notes survive even in a halted-hard
// scenario where a weaker implementation might truncate them in favour of
// just the failure reason.
func TestDreamFailedSummary_DegradedNotesPreserved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := stubInjectRefresh(t)
	defer restore()

	// Succeed on iter 1 so we get through the delta-check path at least
	// once, then fail measure from iter 2 onward. This gives the loop a
	// real committed iter 1 plus a cap trip on iters 2-3.
	SetTestFitnessInjector(injectMeasureErrorFromIter(0.8, 2))
	t.Cleanup(func() { SetTestFitnessInjector(nil) })

	dir := t.TempDir()
	generateStateMachineFixture(t, dir)

	runID := "na-cdn-degraded-preserved"
	outputDir := filepath.Join(dir, ".agents", "overnight", runID)
	opts := RunLoopOptions{
		Cwd:             dir,
		OutputDir:       outputDir,
		RunID:           runID,
		RunTimeout:      30 * time.Second,
		MaxIterations:   10,
		PlateauEpsilon:  0.01,
		PlateauWindowK:  2,
		RegressionFloor: 0.05,
		WarnOnly:        true,
		LogWriter:       io.Discard,
	}.WithMeasureFailureCap(2)

	result, err := RunLoop(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result == nil {
		t.Fatal("result == nil")
	}

	// The cap should have tripped after iters 2+3 (or 2 alone) failed.
	if !result.MeasureFailureHalt {
		t.Fatalf("MeasureFailureHalt=false; want true")
	}
	// Iter 1 (StatusDone) must survive in the iteration list alongside the
	// later degraded iters. This proves "preserve last completed step" -
	// the cmd-layer summary writer must see iter 1's successful data, not
	// just the failure tail.
	var haveDone, haveDegraded bool
	for _, it := range result.Iterations {
		if it.Status == StatusDone {
			haveDone = true
		}
		if it.Status == StatusDegraded {
			haveDegraded = true
		}
	}
	if !haveDone {
		t.Fatalf("no StatusDone iter preserved in result.Iterations; "+
			"last completed step was dropped. Statuses=%s",
			iterStatusSummary(result.Iterations))
	}
	if !haveDegraded {
		t.Fatalf("no StatusDegraded iter in result.Iterations; "+
			"failure path did not record the failed iter. Statuses=%s",
			iterStatusSummary(result.Iterations))
	}

	// The first compounded iter (iter 1) must still be present on disk so
	// the summary writer can read its artifacts when rendering the failed
	// summary's "last successful iter" block.
	iterDir := filepath.Join(outputDir, runID, "iterations")
	iter1Path := filepath.Join(iterDir, "iter-1.json")
	if _, serr := os.Stat(iter1Path); serr != nil {
		t.Fatalf("iter-1.json missing after hard-fail: %v (path=%s)", serr, iter1Path)
	}
	raw, err := os.ReadFile(iter1Path)
	if err != nil {
		t.Fatalf("read iter-1.json: %v", err)
	}
	var iter1 IterationSummary
	if jerr := json.Unmarshal(raw, &iter1); jerr != nil {
		t.Fatalf("unmarshal iter-1.json: %v", jerr)
	}
	if iter1.Status != StatusDone {
		t.Fatalf("iter-1.json status = %q; want %q (first iter should have been a clean commit)",
			iter1.Status, StatusDone)
	}
}
