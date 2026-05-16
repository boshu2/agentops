// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// InMemoryConvergenceCheck is a deterministic ConvergenceCheckPort
// implementation. It does not retain state; "in-memory" means the
// caller supplies the already-materialized evidence slice directly.
type InMemoryConvergenceCheck struct {
	criteria ConvergenceCriteria
}

// NewInMemoryConvergenceCheck returns a convergence checker. Passing
// no criteria uses DefaultConvergenceCriteria; passing one criteria
// value uses it as-is.
func NewInMemoryConvergenceCheck(criteria ...ConvergenceCriteria) *InMemoryConvergenceCheck {
	c := DefaultConvergenceCriteria()
	if len(criteria) > 0 {
		c = criteria[0]
	}
	return &InMemoryConvergenceCheck{criteria: c}
}

// Check evaluates the configured convergence criteria.
func (c *InMemoryConvergenceCheck) Check(ctx context.Context, input ConvergenceInput) (ConvergenceResult, error) {
	if err := ctx.Err(); err != nil {
		return ConvergenceResult{}, err
	}

	greenStreak := leadingGreenCIStreak(input.RecentCIRuns)
	result := ConvergenceResult{
		CIGreenStreak:           greenStreak,
		UnconsumedHighMedium:    input.UnconsumedHighMedium,
		FitnessBaselineCaptured: input.FitnessBaselineCaptured,
	}

	if greenStreak < c.criteria.MinGreenCIStreak {
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

func leadingGreenCIStreak(runs []CIRun) int {
	streak := 0
	for _, run := range runs {
		if run.Status != CIRunStatusCompleted || run.Conclusion != CIRunConclusionSuccess {
			return streak
		}
		streak++
	}
	return streak
}

// Compile-time assertion: InMemoryConvergenceCheck satisfies the port.
var _ ConvergenceCheckPort = (*InMemoryConvergenceCheck)(nil)
