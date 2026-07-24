//go:build !windows

package goals

import (
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestRunGoals_GoroutineLeak(t *testing.T) {
	// The bug: runGoals spawns a goroutine listening on sigCh but never
	// closes sigCh after signal.Stop, so the goroutine leaks on every call.
	// Before the fix, each Measure() call leaks 1 goroutine.
	// Snapshot current goroutines so we only detect leaks from THIS test,
	// not from parallel tests that may have in-flight subprocess cleanup.
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "leak-test", Check: "true", Weight: 1, Type: GoalTypeHealth},
		},
	}
	for i := 0; i < 5; i++ {
		Measure(gf, 5*time.Second)
	}
}
