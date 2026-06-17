package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// MoatClaimVerdict is the publication-tier verdict for the gold/corpus axis when
// aggregating moat-eligible ScenarioDeltaScorecards. See
// docs/evals/applied-ood-claim-rule.md.
type MoatClaimVerdict string

const (
	// MoatVerdictPositive means eligible scorecards collectively support a moat
	// positive claim under the locked publication rule.
	MoatVerdictPositive MoatClaimVerdict = "moat_positive"
	// MoatVerdictHonestNull means eligible scorecards had headroom but the corpus
	// did not improve work — a valid null result.
	MoatVerdictHonestNull MoatClaimVerdict = "honest_null"
	// MoatVerdictInconclusive means the inputs cannot support positive or null.
	MoatVerdictInconclusive MoatClaimVerdict = "inconclusive"
)

// MoatClaimResult is the persisted output of the moat claim aggregation surface
// (age-sb0). It renders a moat positive/null/inconclusive verdict over one or
// more scenario A/B scorecards.
type MoatClaimResult struct {
	SchemaVersion        int              `json:"schema_version"`
	GeneratedAt          time.Time        `json:"generated_at"`
	Verdict              MoatClaimVerdict `json:"verdict"`
	Reason               string           `json:"reason"`
	ScenarioCount        int              `json:"scenario_count"`
	MeanAggregateDelta   float64          `json:"mean_aggregate_delta"`
	ScorecardScenarioIDs []string         `json:"scorecard_scenario_ids"`
	ExcludedCeiling      []string         `json:"excluded_ceiling_violation,omitempty"`
	ExcludedGateFail     []string         `json:"excluded_gate_fail,omitempty"`
}

// ErrMoatIneligibleScorecard is returned when aggregation is asked to include a
// scorecard with moat_eligible=false. The claim surface fail-closes rather than
// silently mixing plumbing into a moat verdict (age-6ys/age-sb0).
type ErrMoatIneligibleScorecard struct {
	ScenarioID   string
	VerdictClass string
}

func (e ErrMoatIneligibleScorecard) Error() string {
	class := e.VerdictClass
	if class == "" {
		class = "unknown"
	}
	return fmt.Sprintf(
		"moat claim aggregation refused: scorecard %q has moat_eligible=false (verdict_class=%q, NOT-moat-evidence/plumbing); cannot aggregate into a moat verdict (age-6ys/age-sb0)",
		e.ScenarioID, class,
	)
}

// LoadScenarioDeltaScorecard reads a ScenarioDeltaScorecard JSON artifact.
func LoadScenarioDeltaScorecard(path string) (ScenarioDeltaScorecard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ScenarioDeltaScorecard{}, fmt.Errorf("read scenario delta scorecard %s: %w", path, err)
	}
	var card ScenarioDeltaScorecard
	if err := json.Unmarshal(data, &card); err != nil {
		return ScenarioDeltaScorecard{}, fmt.Errorf("decode scenario delta scorecard %s: %w", path, err)
	}
	if strings.TrimSpace(card.ScenarioID) == "" {
		return ScenarioDeltaScorecard{}, fmt.Errorf("scenario delta scorecard %s has no scenario_id", path)
	}
	return card, nil
}

// AggregateMoatClaim renders a moat positive/null/inconclusive verdict over the
// provided scorecards. It fail-closes on any moat_eligible=false input — plumbing
// scorecards must never be aggregated into a moat claim (age-sb0).
func AggregateMoatClaim(cards []ScenarioDeltaScorecard) (MoatClaimResult, error) {
	if len(cards) == 0 {
		return MoatClaimResult{}, fmt.Errorf("moat claim aggregation requires at least one scorecard")
	}
	for _, card := range cards {
		if !card.MoatEligible {
			return MoatClaimResult{}, ErrMoatIneligibleScorecard{
				ScenarioID:   card.ScenarioID,
				VerdictClass: card.VerdictClass,
			}
		}
	}

	now := time.Now().UTC()
	result := MoatClaimResult{
		SchemaVersion: 1,
		GeneratedAt:   now,
		ScenarioCount: len(cards),
	}
	for _, card := range cards {
		result.ScorecardScenarioIDs = append(result.ScorecardScenarioIDs, card.ScenarioID)
	}

	var admissible []ScenarioDeltaScorecard
	var positiveEligible []ScenarioDeltaScorecard
	for _, card := range cards {
		if card.CeilingViolation {
			result.ExcludedCeiling = append(result.ExcludedCeiling, card.ScenarioID)
			continue
		}
		admissible = append(admissible, card)
		if card.Gate.Pass && card.AggregateDelta > 0 {
			positiveEligible = append(positiveEligible, card)
		} else if !card.Gate.Pass {
			result.ExcludedGateFail = append(result.ExcludedGateFail, card.ScenarioID)
		}
	}

	if len(admissible) == 0 {
		result.Verdict = MoatVerdictInconclusive
		result.Reason = "no admissible scorecards after excluding ceiling violations"
		return result, nil
	}

	var deltaSum float64
	allHadHeadroom := true
	for _, card := range admissible {
		deltaSum += card.AggregateDelta
		if card.Without.Score >= card.SatisfactionThreshold {
			allHadHeadroom = false
		}
	}
	result.MeanAggregateDelta = roundDelta(deltaSum / float64(len(admissible)))

	switch {
	case len(positiveEligible) == len(admissible):
		result.Verdict = MoatVerdictPositive
		result.Reason = fmt.Sprintf(
			"all %d admissible scorecards have positive delta and passed gate; mean aggregate_delta=%.4f",
			len(admissible), result.MeanAggregateDelta,
		)
	case allHadHeadroom && len(positiveEligible) == 0:
		result.Verdict = MoatVerdictHonestNull
		result.Reason = fmt.Sprintf(
			"admissible scorecards had headroom but corpus did not beat control; mean aggregate_delta=%.4f",
			result.MeanAggregateDelta,
		)
	default:
		result.Verdict = MoatVerdictInconclusive
		result.Reason = "mixed headroom or delta signals across admissible scorecards — cannot claim positive or honest null"
	}
	return result, nil
}

// WriteMoatClaimResult persists a MoatClaimResult JSON artifact.
func WriteMoatClaimResult(path string, result MoatClaimResult) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is required")
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal moat claim result: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write moat claim result: %w", err)
	}
	return nil
}
