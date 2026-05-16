// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// ConvergenceCriteria is the structural stop predicate for the
// evolve loop. The default AgentOps 3.0 predicate is:
// CI green streak >= 3, HIGH+MEDIUM unconsumed findings <= 1, and a
// captured fitness baseline.
type ConvergenceCriteria struct {
	MinGreenCIStreak        int
	MaxUnconsumedHighMedium int
	RequireFitnessBaseline  bool
}

// DefaultConvergenceCriteria returns the canonical evolve stop
// predicate recorded by the BC3 Loop epic.
func DefaultConvergenceCriteria() ConvergenceCriteria {
	return ConvergenceCriteria{
		MinGreenCIStreak:        3,
		MaxUnconsumedHighMedium: 1,
		RequireFitnessBaseline:  true,
	}
}

// ConvergenceInput is the evidence slice the predicate evaluates.
// RecentCIRuns MUST be ordered most-recent first, matching
// CIStatusPort.Recent. UnconsumedHighMedium is the current count of
// unconsumed HIGH+MEDIUM findings. FitnessBaselineCaptured reports
// whether the loop has a baseline artifact for comparison.
type ConvergenceInput struct {
	RecentCIRuns            []CIRun
	UnconsumedHighMedium    int
	FitnessBaselineCaptured bool
}

// ConvergenceResult is the evaluated stop decision plus the observed
// values that produced it. Reasons is populated only for unmet
// criteria and is safe for callers to mutate.
type ConvergenceResult struct {
	Converged               bool
	CIGreenStreak           int
	UnconsumedHighMedium    int
	FitnessBaselineCaptured bool
	Reasons                 []string
}

// ConvergenceCheckPort is the BC3 Loop predicate for stopping an
// autonomous improvement run. Callers pass already-gathered evidence;
// the port does not fetch CI, scan ledgers, or read fitness files
// itself. That keeps the stop decision deterministic and reusable
// across CLI commands, skills, and future daemon workers.
//
// Contract:
//
//   - Check MUST count only the leading most-recent streak of runs
//     whose Status is "completed" and Conclusion is "success".
//   - Queued, in-progress, failed, cancelled, skipped, or unknown
//     runs MUST break the green streak.
//   - Default criteria are green streak >=3, HIGH+MEDIUM <=1, and
//     baseline captured.
//   - Result.Reasons MUST name every unmet criterion.
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC3 row). This port is
// the typed replacement for ad hoc convergence shell predicates in
// evolve scripts.
type ConvergenceCheckPort interface {
	Check(ctx context.Context, input ConvergenceInput) (ConvergenceResult, error)
}
