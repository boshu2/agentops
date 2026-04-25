package eval

import (
	"encoding/json"
	"fmt"
	"sort"
)

func CompareRuns(candidate, baseline *RunRecord, opts CompareOptions) (*RunRecord, error) {
	if err := ValidateRun(candidate); err != nil {
		return nil, err
	}
	if err := ValidateRun(baseline); err != nil {
		return nil, err
	}
	compared, err := cloneRun(candidate)
	if err != nil {
		return nil, err
	}
	aggregateDelta := roundDelta(candidate.AggregateScore - baseline.AggregateScore)
	dimensionDeltas := compareDimensionScores(candidate.DimensionScores, baseline.DimensionScores)
	regressions, improvements := classifyDimensionDeltas(dimensionDeltas, opts.MaxDimensionRegression)
	aggregateRegressed := aggregateDelta < -opts.MaxAggregateRegression
	if aggregateRegressed && len(regressions) == 0 {
		regressions = append(regressions, ComparisonItem{
			Dimension: DimensionCorrectness,
			Delta:     aggregateDelta,
			Reason:    fmt.Sprintf("aggregate score regressed by %.4f", aggregateDelta),
		})
	}
	verdict := VerdictPass
	if len(regressions) > 0 {
		verdict = VerdictRegression
	} else if aggregateDelta > 0 || len(improvements) > 0 {
		verdict = VerdictImprovement
	}
	compared.Verdict = verdict
	compared.Baseline = &BaselineRecord{
		Mode:          BaselineModeCompare,
		BaselineRunID: baseline.RunID,
		BaselineRef:   baseline.Git.CandidateRef,
	}
	compared.BaselineComparison = &BaselineComparison{
		Verdict:        verdict,
		BaselineRunID:  baseline.RunID,
		BaselineScore:  baseline.AggregateScore,
		AggregateDelta: aggregateDelta,
		DimensionDelta: dimensionDeltas,
		Regressions:    regressions,
		Improvements:   improvements,
	}
	if opts.OutputPath != "" {
		compared.Artifacts = append(compared.Artifacts, Artifact{
			Path:    opts.OutputPath,
			Purpose: "evaluation comparison run record",
			Kind:    "run_json",
		})
		if err := WriteRun(opts.OutputPath, compared); err != nil {
			return nil, err
		}
	}
	return compared, nil
}

func compareDimensionScores(candidate, baseline map[Dimension]float64) map[Dimension]float64 {
	keys := make(map[Dimension]struct{}, len(candidate)+len(baseline))
	for dim := range candidate {
		keys[dim] = struct{}{}
	}
	for dim := range baseline {
		keys[dim] = struct{}{}
	}
	out := make(map[Dimension]float64, len(keys))
	for dim := range keys {
		out[dim] = roundDelta(candidate[dim] - baseline[dim])
	}
	return out
}

func classifyDimensionDeltas(deltas map[Dimension]float64, maxRegression float64) ([]ComparisonItem, []ComparisonItem) {
	keys := make([]Dimension, 0, len(deltas))
	for dim := range deltas {
		keys = append(keys, dim)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var regressions []ComparisonItem
	var improvements []ComparisonItem
	for _, dim := range keys {
		delta := deltas[dim]
		switch {
		case delta < -maxRegression:
			regressions = append(regressions, ComparisonItem{
				Dimension: dim,
				Delta:     delta,
				Reason:    fmt.Sprintf("%s regressed by %.4f", dim, delta),
			})
		case delta > 0:
			improvements = append(improvements, ComparisonItem{
				Dimension: dim,
				Delta:     delta,
				Reason:    fmt.Sprintf("%s improved by %.4f", dim, delta),
			})
		}
	}
	return regressions, improvements
}

func cloneRun(run *RunRecord) (*RunRecord, error) {
	data, err := json.Marshal(run)
	if err != nil {
		return nil, err
	}
	var cloned RunRecord
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}
