// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionConvergenceCheck satisfies ports.ConvergenceCheckPort with
// a pure deterministic evaluator for the evolve STOP predicate. It is
// intentionally side-effect free: callers gather CI, findings, and
// baseline evidence through their own ports/adapters, then pass the
// typed evidence here.
type productionConvergenceCheck struct {
	criteria ports.ConvergenceCriteria
}

func newProductionConvergenceCheck(criteria ...ports.ConvergenceCriteria) *productionConvergenceCheck {
	c := ports.DefaultConvergenceCriteria()
	if len(criteria) > 0 {
		c = criteria[0]
	}
	return &productionConvergenceCheck{criteria: c}
}

func (c *productionConvergenceCheck) Check(ctx context.Context, input ports.ConvergenceInput) (ports.ConvergenceResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.ConvergenceResult{}, err
	}

	result := ports.ConvergenceResult{
		CIGreenStreak:           productionLeadingGreenCIStreak(input.RecentCIRuns),
		UnconsumedHighMedium:    input.UnconsumedHighMedium,
		FitnessBaselineCaptured: input.FitnessBaselineCaptured,
	}
	if result.CIGreenStreak < c.criteria.MinGreenCIStreak {
		result.Reasons = append(result.Reasons, "ci-green-streak-below-threshold")
	}
	if input.UnconsumedHighMedium > c.criteria.MaxUnconsumedHighMedium {
		result.Reasons = append(result.Reasons, "unconsumed-high-medium-above-threshold")
	}
	if c.criteria.RequireFitnessBaseline && !input.FitnessBaselineCaptured {
		result.Reasons = append(result.Reasons, "fitness-baseline-missing")
	}
	result.Converged = len(result.Reasons) == 0
	result.Reasons = append([]string(nil), result.Reasons...)
	return result, nil
}

func productionLeadingGreenCIStreak(runs []ports.CIRun) int {
	streak := 0
	for _, run := range runs {
		if run.Status != ports.CIRunStatusCompleted || run.Conclusion != ports.CIRunConclusionSuccess {
			return streak
		}
		streak++
	}
	return streak
}

// Compile-time assertion: productionConvergenceCheck satisfies the port.
var _ ports.ConvergenceCheckPort = (*productionConvergenceCheck)(nil)
