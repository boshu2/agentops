// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// soc-y5vh.8: `ao loop converged` exposes the pure BC3 ConvergenceCheckPort.
// The default criteria are green streak >=3, HIGH+MEDIUM <=1, baseline
// captured.

func runConverged(t *testing.T, opts loopConvergedOptions) string {
	t.Helper()
	var buf bytes.Buffer
	opts.writer = &buf
	if err := loopConvergedRun(context.Background(), opts); err != nil {
		t.Fatalf("loopConvergedRun: %v", err)
	}
	return buf.String()
}

func TestLoopConverged_AllCriteriaMet(t *testing.T) {
	out := runConverged(t, loopConvergedOptions{
		greenStreak:          3,
		unconsumedHighMedium: 0,
		fitnessBaseline:      true,
	})
	if !strings.Contains(out, `"converged":true`) {
		t.Fatalf("expected converged:true, got %q", out)
	}
}

func TestLoopConverged_GreenStreakBelowThreshold(t *testing.T) {
	out := runConverged(t, loopConvergedOptions{
		greenStreak:          2,
		unconsumedHighMedium: 0,
		fitnessBaseline:      true,
	})
	if !strings.Contains(out, `"converged":false`) {
		t.Fatalf("expected converged:false, got %q", out)
	}
	if !strings.Contains(out, "ci-green-streak-below-threshold") {
		t.Fatalf("expected ci-green-streak reason, got %q", out)
	}
}

func TestLoopConverged_UnconsumedAboveThreshold(t *testing.T) {
	out := runConverged(t, loopConvergedOptions{
		greenStreak:          5,
		unconsumedHighMedium: 4,
		fitnessBaseline:      true,
	})
	if !strings.Contains(out, "unconsumed-high-medium-above-threshold") {
		t.Fatalf("expected unconsumed-high-medium reason, got %q", out)
	}
}

func TestLoopConverged_MissingBaseline(t *testing.T) {
	out := runConverged(t, loopConvergedOptions{
		greenStreak:          3,
		unconsumedHighMedium: 0,
		fitnessBaseline:      false,
	})
	if !strings.Contains(out, "fitness-baseline-missing") {
		t.Fatalf("expected fitness-baseline-missing reason, got %q", out)
	}
}

func TestLoopConverged_ReportsObservedStreak(t *testing.T) {
	out := runConverged(t, loopConvergedOptions{
		greenStreak:          7,
		unconsumedHighMedium: 0,
		fitnessBaseline:      true,
	})
	if !strings.Contains(out, `"ci_green_streak":7`) {
		t.Fatalf("expected observed streak 7 in output, got %q", out)
	}
}
