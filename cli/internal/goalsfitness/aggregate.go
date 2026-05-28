// Package goalsfitness aggregates per-scenario satisfaction outcomes into a
// per-directive satisfaction score for `ao goals measure` (F2.1).
//
// It consumes the scenario-results.v1 artifact via the F2.0 scenarioresults
// loader — it never parses council markdown or the artifact JSON directly — and
// projects the latest result per scenario onto a directive's linked scenario
// list (the F1 directive->scenario goals link). A missing artifact yields a
// clean "unknown" outcome: no crash, no false pass.
package goalsfitness

import (
	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

// DirectiveLink is the minimal directive->scenario projection this package
// needs. It is satisfied by goals.ParsedDirective (StableID + Scenarios), kept
// as a local struct so goalsfitness does not import cli/internal/goals.
type DirectiveLink struct {
	// DirectiveID is the stable directive ID (pattern ^d-[a-z0-9][a-z0-9-]*$).
	DirectiveID string
	// ScenarioIDs are the scenario IDs linked to the directive in GOALS.md.
	ScenarioIDs []string
}

// AggregationStatus classifies a per-directive aggregation outcome.
type AggregationStatus string

const (
	// StatusOK means at least one linked scenario produced a usable verdict.
	StatusOK AggregationStatus = "ok"
	// StatusUnknown means no satisfaction evidence is available — a missing
	// artifact, a directive with no linked scenarios, or all linked scenarios
	// absent/skipped. It must never be read as a pass.
	StatusUnknown AggregationStatus = "unknown"
)

// DirectiveAggregation is the per-directive satisfaction rollup.
type DirectiveAggregation struct {
	// DirectiveID is the stable directive ID this rollup covers.
	DirectiveID string
	// Status classifies the rollup; StatusUnknown carries no pass signal.
	Status AggregationStatus
	// Score is the mean scenario score over Evaluated scenarios in [0,1]. It is
	// 0 when Evaluated is 0 and must be ignored unless Status is StatusOK.
	Score float64
	// Linked is the count of scenario IDs declared on the directive.
	Linked int
	// Evaluated is the count of linked scenarios with a pass/fail verdict.
	Evaluated int
	// Skipped is the count of linked scenarios whose latest verdict is skip.
	Skipped int
	// Missing is the count of linked scenarios with no result in the artifact.
	Missing int
	// Contributing lists the scenario IDs (sorted) that fed Score.
	Contributing []string
	// Warning carries a non-fatal loader message (e.g. missing artifact).
	Warning string
}

// Aggregator rolls scenario results up to a per-directive satisfaction score.
type Aggregator struct {
	loaded scenarioresults.LoadResult
	// byScenario maps scenario_id -> latest result; empty when no artifact.
	byScenario map[string]scenarioresults.ScenarioResult
}

// NewAggregator loads the scenario-results artifact from .agents/rpi under
// projectRoot via the F2.0 loader and returns an aggregator over it.
//
// A missing artifact is not an error: the returned aggregator reports
// StatusUnknown for every directive. A malformed artifact in strict mode
// returns the loader's path-specific error.
func NewAggregator(projectRoot string, strict bool) (*Aggregator, error) {
	loaded, err := scenarioresults.Load(projectRoot, strict)
	if err != nil {
		return nil, err
	}
	return newAggregatorFromLoad(loaded), nil
}

// newAggregatorFromLoad builds an Aggregator from an already-resolved
// LoadResult. It is the internal seam used by tests to point at fixture paths
// without depending on the runtime artifact location.
func newAggregatorFromLoad(loaded scenarioresults.LoadResult) *Aggregator {
	byScenario := make(map[string]scenarioresults.ScenarioResult)
	if loaded.Artifact != nil {
		for _, r := range loaded.Artifact.Results {
			byScenario[r.ScenarioID] = latestOf(byScenario, r)
		}
	}
	return &Aggregator{loaded: loaded, byScenario: byScenario}
}

// latestOf returns whichever of the existing result for r.ScenarioID and r
// itself was judged later. The F2.0 loader already dedupes, but this keeps the
// projection correct if it is ever fed raw results.
func latestOf(byID map[string]scenarioresults.ScenarioResult, r scenarioresults.ScenarioResult) scenarioresults.ScenarioResult {
	prev, ok := byID[r.ScenarioID]
	if !ok || r.JudgedAt > prev.JudgedAt {
		return r
	}
	return prev
}

// LoadStatus exposes the underlying artifact load status.
func (a *Aggregator) LoadStatus() scenarioresults.Status {
	return a.loaded.Status
}

// LoadWarning exposes the underlying artifact load warning, if any.
func (a *Aggregator) LoadWarning() string {
	return a.loaded.Warning
}

// Aggregate computes the satisfaction rollup for one directive.
//
// When the artifact is absent (loader StatusUnknown) the rollup is StatusUnknown
// regardless of the directive's linked scenarios — a clean skip, never a pass.
func (a *Aggregator) Aggregate(d DirectiveLink) DirectiveAggregation {
	agg := DirectiveAggregation{
		DirectiveID: d.DirectiveID,
		Status:      StatusUnknown,
		Linked:      len(d.ScenarioIDs),
		Warning:     a.loaded.Warning,
	}
	if a.loaded.IsSkip() {
		return agg
	}
	a.fillCounts(d, &agg)
	if agg.Evaluated > 0 {
		agg.Status = StatusOK
		agg.Score = agg.Score / float64(agg.Evaluated)
	} else {
		agg.Score = 0
	}
	return agg
}

// fillCounts walks the directive's linked scenarios, tallying evaluated/skipped/
// missing counts and accumulating the raw score sum into agg.Score.
func (a *Aggregator) fillCounts(d DirectiveLink, agg *DirectiveAggregation) {
	for _, sid := range d.ScenarioIDs {
		r, ok := a.byScenario[sid]
		if !ok {
			agg.Missing++
			continue
		}
		switch r.Verdict {
		case scenarioresults.VerdictPass, scenarioresults.VerdictFail:
			agg.Evaluated++
			agg.Score += r.Score
			agg.Contributing = append(agg.Contributing, sid)
		default: // VerdictSkip or any non-scoring verdict
			agg.Skipped++
		}
	}
	sortStrings(agg.Contributing)
}

// sortStrings sorts s in place (ascending). Kept local to avoid an import for a
// one-line call and to keep Aggregate's dependency surface minimal.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
