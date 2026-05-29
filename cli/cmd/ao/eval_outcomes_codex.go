package main

import "github.com/boshu2/agentops/cli/internal/evalsubstrate"

// gradeLocally is the Codex/NTM offline grading path (ag-hdqu0.7). Codex has no
// Managed Agents loop and no Workflow tool, so it calls the SAME
// `ao eval outcomes compile` to get a holdout-safe Rubric, grades the candidate
// locally — Inspect AI over the dev split, or the bushido llama.cpp Qwen3.6 grader
// over tailnet — and writes through the SAME ingestOutcomesScore. This function is
// the output adapter for that local grade: it folds per-criterion scores into the
// weight-normalized outcomesScore that ingest consumes, so the Codex path yields a
// verdict byte-identical in shape to the cloud path with ZERO outbound cloud calls.
//
// perCriterion maps Criterion.ID -> [0,1] score; criteria the grader did not score
// count as 0. The aggregate is the weight-normalized mean over the rubric's own
// criteria (so a rubric and its grade can never disagree on which criteria exist).
func gradeLocally(r evalsubstrate.Rubric, perCriterion map[string]float64, threshold float64) outcomesScore {
	scores := make(map[string]float64, len(r.Criteria))
	var weighted, weightSum float64
	for _, c := range r.Criteria {
		s := perCriterion[c.ID]
		scores[c.ID] = s
		weighted += s * c.Weight
		weightSum += c.Weight
	}
	aggregate := 0.0
	if weightSum > 0 {
		aggregate = weighted / weightSum
	}
	return outcomesScore{
		SourceTaskID:     r.SourceTaskID,
		JudgeContentHash: r.JudgeContentHash,
		Aggregate:        aggregate,
		Threshold:        threshold,
		CriterionScores:  scores,
	}
}
