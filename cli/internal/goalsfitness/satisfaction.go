package goalsfitness

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultScenarioThreshold is the directive-level scenario-satisfaction
// threshold applied when a directive declares no "**Scenario threshold:**"
// attribute in GOALS.md. It matches the PHASE 7 design pin of 0.8.
const DefaultScenarioThreshold = 0.8

// Verdict is a durable per-directive scenario-satisfaction verdict.
//
// The four values are the only ones ever stored. "RED" is a UI rendering of
// VerdictFail, not a data value — it must never be persisted.
type Verdict string

const (
	// VerdictPass means the satisfied fraction met or exceeded the threshold.
	VerdictPass Verdict = "pass"
	// VerdictFail means the satisfied fraction fell below the threshold. The UI
	// may label this "RED"; the stored value stays "fail".
	VerdictFail Verdict = "fail"
	// VerdictSkip means every linked scenario was skipped (none missing). It is
	// not a pass: the directive produced no satisfaction evidence.
	VerdictSkip Verdict = "skip"
	// VerdictUnknown means there is no usable evidence — no artifact, no linked
	// scenarios, or all linked scenarios missing/mixed-absent. Never a pass.
	VerdictUnknown Verdict = "unknown"
)

// SatisfactionResult is the per-directive scenario-satisfaction verdict for
// `ao goals measure`. It is additive to the existing Steer/gate checks.
type SatisfactionResult struct {
	// DirectiveID is the stable directive ID this verdict covers.
	DirectiveID string
	// Verdict is the durable verdict (pass|fail|skip|unknown).
	Verdict Verdict
	// Satisfaction is scenario_satisfaction: the fraction of linked scenarios
	// whose own score met its own per-scenario threshold, in [0,1]. It is 0
	// when no scenario was evaluated and must be read only with Verdict in
	// {pass, fail}.
	Satisfaction float64
	// Threshold is the directive's scenario-satisfaction threshold actually
	// applied (the parsed attribute, or DefaultScenarioThreshold).
	Threshold float64
	// Linked is the count of scenario IDs declared on the directive.
	Linked int
	// Satisfied is the count of linked scenarios whose score met its threshold.
	Satisfied int
	// Evaluated is the count of linked scenarios with a pass/fail verdict.
	Evaluated int
	// Skipped is the count of linked scenarios whose latest verdict is skip.
	Skipped int
	// Missing is the count of linked scenarios with no result in the artifact.
	Missing int
	// Warning carries a lint-style non-fatal message (e.g. zero linked
	// scenarios, or a loader warning passed through from Aggregate).
	Warning string
}

// ParseScenarioThreshold converts a directive's "**Scenario threshold:**"
// attribute value into a float. An empty value yields DefaultScenarioThreshold.
// A non-numeric or out-of-[0,1] value is an error so a malformed GOALS.md does
// not silently degrade a directive's gate.
func ParseScenarioThreshold(attr string) (float64, error) {
	trimmed := strings.TrimSpace(attr)
	if trimmed == "" {
		return DefaultScenarioThreshold, nil
	}
	f, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid scenario threshold %q: not a number", attr)
	}
	if f < 0 || f > 1 {
		return 0, fmt.Errorf("invalid scenario threshold %q: must be in [0,1]", attr)
	}
	return f, nil
}

// EvaluateSatisfaction computes the per-directive scenario_satisfaction
// fraction and its durable verdict, given a directive->scenario link and the
// directive's scenario-satisfaction threshold.
//
// scenario_satisfaction is the fraction of the directive's linked scenarios
// whose own score met its own per-scenario satisfaction threshold (the
// threshold carried by the scenario result artifact). The verdict is:
//
//   - unknown — zero linked scenarios, no artifact, or every linked scenario
//     missing/mixed-absent with no evaluable evidence;
//   - skip — every linked scenario present but skipped;
//   - fail — fraction strictly below the directive threshold;
//   - pass — fraction at or above the directive threshold (equality passes).
//
// It composes with the F2.1 Aggregate: EvaluateSatisfaction calls
// a.Aggregate(d) for the load/skip/count plumbing, then layers the per-scenario
// threshold comparison on top via the package-internal byScenario projection.
func (a *Aggregator) EvaluateSatisfaction(d DirectiveLink, directiveThreshold float64) SatisfactionResult {
	agg := a.Aggregate(d)
	res := SatisfactionResult{
		DirectiveID: d.DirectiveID,
		Threshold:   directiveThreshold,
		Linked:      agg.Linked,
		Evaluated:   agg.Evaluated,
		Skipped:     agg.Skipped,
		Missing:     agg.Missing,
		Warning:     agg.Warning,
		Verdict:     VerdictUnknown,
	}
	if agg.Linked == 0 {
		res.Warning = appendWarning(res.Warning,
			fmt.Sprintf("directive %s links zero scenarios; scenario gate is unknown", d.DirectiveID))
		return res
	}
	res.Satisfied = a.countSatisfied(d)
	res.classify(agg)
	return res
}

// countSatisfied counts the directive's linked scenarios whose own score met
// its own per-scenario threshold. Missing or skipped scenarios never count as
// satisfied. Threshold equality counts as satisfied.
func (a *Aggregator) countSatisfied(d DirectiveLink) int {
	satisfied := 0
	for _, sid := range d.ScenarioIDs {
		r, ok := a.byScenario[sid]
		if !ok {
			continue
		}
		if r.Verdict == "skip" {
			continue
		}
		if r.Score >= r.Threshold {
			satisfied++
		}
	}
	return satisfied
}

// classify sets Satisfaction and Verdict on res from the F2.1 aggregation and
// the already-counted Satisfied tally. It assumes res.Linked > 0.
//
// Per the PHASE 7 pin, when no linked scenario produced a pass/fail verdict the
// directive is unknown — never pass — whether the linked scenarios were all
// skipped, all missing, or a mix of the two.
func (r *SatisfactionResult) classify(agg DirectiveAggregation) {
	if agg.Status == StatusUnknown && agg.Evaluated == 0 {
		// No artifact, or every linked scenario missing/skipped: no evidence.
		r.Verdict = VerdictUnknown
		return
	}
	r.Satisfaction = float64(r.Satisfied) / float64(r.Linked)
	if r.Satisfaction >= r.Threshold {
		r.Verdict = VerdictPass
	} else {
		r.Verdict = VerdictFail
	}
}

// appendWarning joins a new warning onto an existing one with "; ".
func appendWarning(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}
