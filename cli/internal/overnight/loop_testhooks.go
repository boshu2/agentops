package overnight

// Test-only hooks that production code must NEVER set. These are package-
// private and live in a dedicated file so a reviewer grepping RunLoopOptions
// never sees a field they'd have to mentally filter as "production? test?".
//
// CONCURRENCY CONTRACT: these globals are NOT safe for parallel tests.
// Any test that calls SetFaultInjectionAfterIter or SetTestFitnessInjector
// MUST NOT call t.Parallel() and MUST call t.Cleanup to restore the zero
// value before the next test runs. Tests that forget the cleanup will
// bleed state into sibling tests. (Pre-mortem judge-3 B4 catch.)

import (
	"sync"
	"sync/atomic"
)

// faultInjectionAfterIter, when non-zero, causes RunLoop to panic AFTER
// persisting iter-<N>.json but BEFORE updating prevSnapshot. This lets
// TestRunLoop_CrashAtIter2_ResumeRehydrates exercise the resume path
// deterministically. Stored as atomic.Int32 for race-detector cleanliness
// when multiple tests run serially in the same package.
var faultInjectionAfterIter atomic.Int32

// SetFaultInjectionAfterIter is the test-only setter. Calling it from
// non-test code is a bug; nothing in the production build path should
// reference this symbol. MUST be paired with t.Cleanup to restore 0.
func SetFaultInjectionAfterIter(n int) {
	faultInjectionAfterIter.Store(int32(n))
}

// getFaultInjectionAfterIter is the package-internal reader used by RunLoop.
func getFaultInjectionAfterIter() int {
	return int(faultInjectionAfterIter.Load())
}

// testFitnessInjector, when non-nil, causes RunLoop to bypass RunMeasure
// and invoke the injector directly with the 1-based iteration index. The
// returned FitnessSnapshot is used verbatim for DELTA+HALT checks; a
// returned error is treated as a MEASURE failure (feeding the C4
// consecutive-failure cap added in Micro-epic 5). Wrapped in a mutex
// because test dispatch/callback ordering is not lock-free on all archs
// even with atomic.Value.
var (
	testFitnessInjectorMu sync.RWMutex
	testFitnessInjector   func(iterIndex int) (FitnessSnapshot, error)
)

// SetTestFitnessInjector installs a deterministic fitness-producer for
// the RunLoop tests. Call with nil inside t.Cleanup to restore the
// legacy RunMeasure path. Micro-epic 6 (C5) relies on this hook for its
// three deterministic plateau/regression L2 tests — see
// loop_fitness_injection_test.go.
//
// MUST NOT be used from production code. The hook is package-private to
// force callers to live in the overnight package (test files).
func SetTestFitnessInjector(f func(iterIndex int) (FitnessSnapshot, error)) {
	testFitnessInjectorMu.Lock()
	defer testFitnessInjectorMu.Unlock()
	testFitnessInjector = f
}

// getTestFitnessInjector is the package-internal reader used by RunLoop.
// Returns nil when no injector is installed — the loop's hot path then
// falls through to the legacy RunMeasure call unchanged.
func getTestFitnessInjector() func(iterIndex int) (FitnessSnapshot, error) {
	testFitnessInjectorMu.RLock()
	defer testFitnessInjectorMu.RUnlock()
	return testFitnessInjector
}

var (
	testIngestFaultInjectorMu sync.RWMutex
	testIngestFaultInjector   func(iterIndex int) error

	testPostCommitFaultInjectorMu sync.RWMutex
	testPostCommitFaultInjector   func(iterIndex int, cwd string) error
)

// SetTestIngestFaultInjector installs a deterministic INGEST failure hook for
// RunLoop tests. It is intentionally narrower than RunIngest itself: tests use
// it to exercise RunLoop's StatusFailed bookkeeping without corrupting the
// fixture directory or depending on filesystem timing.
func SetTestIngestFaultInjector(f func(iterIndex int) error) {
	testIngestFaultInjectorMu.Lock()
	defer testIngestFaultInjectorMu.Unlock()
	testIngestFaultInjector = f
}

func getTestIngestFaultInjector() func(iterIndex int) error {
	testIngestFaultInjectorMu.RLock()
	defer testIngestFaultInjectorMu.RUnlock()
	return testIngestFaultInjector
}

// SetTestPostCommitFaultInjector installs a deterministic hook that runs after
// Commit succeeds and before the post-commit metadata verification pass. Tests
// use it to exercise StatusHaltedOnRegressionPostCommit without changing the
// Checkpoint implementation or relying on nondeterministic disk faults.
func SetTestPostCommitFaultInjector(f func(iterIndex int, cwd string) error) {
	testPostCommitFaultInjectorMu.Lock()
	defer testPostCommitFaultInjectorMu.Unlock()
	testPostCommitFaultInjector = f
}

func getTestPostCommitFaultInjector() func(iterIndex int, cwd string) error {
	testPostCommitFaultInjectorMu.RLock()
	defer testPostCommitFaultInjectorMu.RUnlock()
	return testPostCommitFaultInjector
}
