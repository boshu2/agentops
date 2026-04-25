package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScorecardKind identifies the product surface a scorecard evaluates.
type ScorecardKind string

const (
	// ScorecardKindRPI evaluates the RPI lifecycle contract.
	ScorecardKindRPI ScorecardKind = "rpi"
	// ScorecardKindSkillChange evaluates whether a skill change improved or
	// regressed the skill contract.
	ScorecardKindSkillChange ScorecardKind = "skill-change"
)

// ScorecardOptions configures candidate-vs-baseline scorecard generation.
type ScorecardOptions struct {
	Kind                  ScorecardKind
	MaxCategoryRegression float64
}

// Scorecard is a category-level better-or-worse report for an eval run.
type Scorecard struct {
	SchemaVersion          int                 `json:"schema_version"`
	Kind                   ScorecardKind       `json:"kind"`
	CandidateRunID         string              `json:"candidate_run_id"`
	BaselineRunID          string              `json:"baseline_run_id,omitempty"`
	Verdict                Verdict             `json:"verdict"`
	Reason                 string              `json:"reason,omitempty"`
	AggregateScore         float64             `json:"aggregate_score"`
	BaselineAggregateScore *float64            `json:"baseline_aggregate_score,omitempty"`
	AggregateDelta         *float64            `json:"aggregate_delta,omitempty"`
	Categories             []ScorecardCategory `json:"categories"`
}

// ScorecardCategory records one required scorecard category and its baseline
// comparison when a baseline run is provided.
type ScorecardCategory struct {
	Category       string   `json:"category"`
	CandidateScore float64  `json:"candidate_score"`
	BaselineScore  *float64 `json:"baseline_score,omitempty"`
	Delta          *float64 `json:"delta,omitempty"`
	Verdict        Verdict  `json:"verdict"`
	Reason         string   `json:"reason"`
}

type scorecardDefinition struct {
	label     string
	slug      string
	dimension Dimension
}

// BuildScorecard builds a category-level scorecard from a candidate eval run
// and, optionally, a baseline run.
func BuildScorecard(candidate *RunRecord, baseline *RunRecord, opts ScorecardOptions) (*Scorecard, error) {
	if candidate == nil {
		return nil, fmt.Errorf("candidate run is required")
	}
	if err := ValidateRun(candidate); err != nil {
		return nil, err
	}
	if baseline != nil {
		if err := ValidateRun(baseline); err != nil {
			return nil, err
		}
	}
	kind := opts.Kind
	if kind == "" {
		kind = ScorecardKindRPI
	}
	defs, err := scorecardDefinitions(kind)
	if err != nil {
		return nil, err
	}

	card := &Scorecard{
		SchemaVersion:  1,
		Kind:           kind,
		CandidateRunID: candidate.RunID,
		Verdict:        VerdictPass,
		AggregateScore: candidate.AggregateScore,
	}
	if baseline != nil {
		card.BaselineRunID = baseline.RunID
		score := baseline.AggregateScore
		card.BaselineAggregateScore = &score
		delta := roundDelta(candidate.AggregateScore - baseline.AggregateScore)
		card.AggregateDelta = &delta
	}

	hasFailure := false
	hasRegression := false
	hasImprovement := false
	for _, def := range defs {
		category := scorecardCategory(candidate, baseline, def, opts.MaxCategoryRegression)
		switch category.Verdict {
		case VerdictFail:
			hasFailure = true
		case VerdictRegression:
			hasRegression = true
		case VerdictImprovement:
			hasImprovement = true
		}
		card.Categories = append(card.Categories, category)
	}

	switch {
	case candidate.Status == StatusFail || candidate.Status == StatusError:
		card.Verdict = VerdictFail
		card.Reason = fmt.Sprintf("candidate run status is %s", candidate.Status)
	case hasFailure:
		card.Verdict = VerdictFail
		card.Reason = "candidate is missing required scorecard categories"
	case hasRegression:
		card.Verdict = VerdictRegression
		card.Reason = "candidate regressed one or more scorecard categories"
	case hasImprovement:
		card.Verdict = VerdictImprovement
		card.Reason = "candidate improved one or more scorecard categories"
	default:
		card.Verdict = VerdictPass
		card.Reason = "candidate preserved required scorecard categories"
	}
	return card, nil
}

// WriteScorecard writes a scorecard JSON artifact to disk.
func WriteScorecard(path string, card *Scorecard) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("output path is required")
	}
	if card == nil {
		return fmt.Errorf("scorecard is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create scorecard output directory: %w", err)
	}
	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scorecard: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write scorecard: %w", err)
	}
	return nil
}

func scorecardCategory(candidate *RunRecord, baseline *RunRecord, def scorecardDefinition, maxRegression float64) ScorecardCategory {
	candidateScore, ok := categoryScore(candidate, def)
	if !ok {
		return ScorecardCategory{
			Category:       def.label,
			CandidateScore: 0,
			Verdict:        VerdictFail,
			Reason:         fmt.Sprintf("candidate has no matching case or dimension score for required category %q", def.label),
		}
	}
	category := ScorecardCategory{
		Category:       def.label,
		CandidateScore: candidateScore,
		Verdict:        VerdictPass,
		Reason:         fmt.Sprintf("candidate preserved %s", def.label),
	}
	if baseline == nil {
		if candidateScore < 1 {
			category.Verdict = VerdictFail
			category.Reason = fmt.Sprintf("candidate %s score %.4f is below 1.0000", def.label, candidateScore)
		}
		return category
	}
	baselineScore, baselineOK := categoryScore(baseline, def)
	if !baselineOK {
		category.Reason = fmt.Sprintf("baseline has no matching case or dimension score for category %q", def.label)
		return category
	}
	category.BaselineScore = floatPtr(baselineScore)
	delta := roundDelta(candidateScore - baselineScore)
	category.Delta = floatPtr(delta)
	switch {
	case delta < -maxRegression:
		category.Verdict = VerdictRegression
		category.Reason = fmt.Sprintf("candidate regressed %s by %.4f", def.label, delta)
	case delta > 0:
		category.Verdict = VerdictImprovement
		category.Reason = fmt.Sprintf("candidate improved %s by %.4f", def.label, delta)
	default:
		category.Reason = fmt.Sprintf("candidate preserved %s", def.label)
	}
	return category
}

func scorecardDefinitions(kind ScorecardKind) ([]scorecardDefinition, error) {
	switch kind {
	case ScorecardKindRPI:
		return []scorecardDefinition{
			{label: "artifact completeness", slug: "artifact-completeness", dimension: DimensionArtifactQuality},
			{label: "phase order", slug: "phase-order", dimension: DimensionProcessAdherence},
			{label: "objective spine", slug: "objective-spine", dimension: DimensionProcessAdherence},
			{label: "validation separation", slug: "validation-separation", dimension: DimensionProcessAdherence},
			{label: "scenario satisfaction", slug: "scenario-satisfaction", dimension: DimensionCorrectness},
			{label: "runtime safety", slug: "runtime-safety", dimension: DimensionSafety},
		}, nil
	case ScorecardKindSkillChange:
		return []scorecardDefinition{
			{label: "structural", slug: "structural", dimension: DimensionArtifactQuality},
			{label: "trigger", slug: "trigger", dimension: DimensionProcessAdherence},
			{label: "runtime", slug: "runtime", dimension: DimensionRuntimeCompatibility},
			{label: "scenario", slug: "scenario", dimension: DimensionCorrectness},
			{label: "stocktake", slug: "stocktake", dimension: DimensionLearningClosure},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported scorecard kind %q", kind)
	}
}

func categoryScore(run *RunRecord, def scorecardDefinition) (float64, bool) {
	var scores []float64
	for _, result := range run.CaseResults {
		if scorecardCaseMatches(result.ID, def.slug) {
			scores = append(scores, result.Score)
		}
	}
	if len(scores) > 0 {
		sort.Float64s(scores)
		return roundScore(sumScores(scores) / float64(len(scores))), true
	}
	if score, ok := run.DimensionScores[def.dimension]; ok {
		return score, true
	}
	return 0, false
}

func scorecardCaseMatches(id, slug string) bool {
	for _, segment := range strings.FieldsFunc(strings.ToLower(id), func(r rune) bool {
		return r == '.' || r == '/' || r == ':' || r == '@'
	}) {
		if normalizeScorecardSlug(segment) == slug {
			return true
		}
	}
	return normalizeScorecardSlug(id) == slug
}

func normalizeScorecardSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func sumScores(scores []float64) float64 {
	total := 0.0
	for _, score := range scores {
		total += score
	}
	return total
}

func floatPtr(value float64) *float64 {
	return &value
}
