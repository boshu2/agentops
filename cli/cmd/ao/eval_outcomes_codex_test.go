package main

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

// TestEvalOutcomesCodexPath_LocalGradeProducesVerdict proves the Codex/NTM
// offline grading path (ag-hdqu0.7): the SAME compile (ProjectRubric) and ingest
// (ingestOutcomesScore) serve a locally-graded run with ZERO cloud dependency,
// producing a verdict identical in shape to the cloud Outcomes path. The local
// grader (gradeLocally) stands in for the Inspect AI dev-split / bushido Qwen3.6
// graders — all offline, all funnelling through the one verdict format.
func TestEvalOutcomesCodexPath_LocalGradeProducesVerdict(t *testing.T) {
	rubric := evalsubstrate.ProjectRubric(
		evalsubstrate.Task{ID: "task-capital-cities", Description: "answer accurately"},
		[]evalsubstrate.Criterion{
			{ID: "accuracy", Description: "names the correct capital", Weight: 0.7},
			{ID: "concision", Description: "one short sentence", Weight: 0.3},
		},
		"sha256:judge1",
	)

	// Local grader output (Inspect AI dev-split / Qwen3.6 stand-in) — offline.
	score := gradeLocally(rubric, map[string]float64{"accuracy": 1.0, "concision": 1.0}, 0.8)
	if score.SourceTaskID != rubric.SourceTaskID {
		t.Errorf("score.SourceTaskID = %q, want %q", score.SourceTaskID, rubric.SourceTaskID)
	}
	if score.JudgeContentHash != rubric.JudgeContentHash {
		t.Errorf("local grade must carry the rubric judge hash, got %q", score.JudgeContentHash)
	}
	if score.Aggregate != 1.0 {
		t.Errorf("weighted aggregate of all-1.0 criteria = %v, want 1.0", score.Aggregate)
	}

	// Same ingest path as the cloud Outcomes run → one verdict format.
	v := ingestOutcomesScore(score)
	if v.Verdict != "PASS" {
		t.Errorf("Verdict = %q, want PASS", v.Verdict)
	}
	if v.SatisfactionScore == nil || *v.SatisfactionScore != 1.0 {
		t.Errorf("SatisfactionScore = %v, want 1.0", v.SatisfactionScore)
	}
	if v.SatisfactionBreakdown["accuracy"] != 1.0 {
		t.Errorf("breakdown[accuracy] = %v, want 1.0", v.SatisfactionBreakdown["accuracy"])
	}
}

// TestGradeLocally_WeightedAggregate: the local grader computes a weight-normalized
// aggregate over exactly the rubric's criteria (unknown criteria score 0).
func TestGradeLocally_WeightedAggregate(t *testing.T) {
	rubric := evalsubstrate.ProjectRubric(
		evalsubstrate.Task{ID: "t"},
		[]evalsubstrate.Criterion{{ID: "a", Weight: 3}, {ID: "b", Weight: 1}},
		"sha256:j",
	)
	score := gradeLocally(rubric, map[string]float64{"a": 1.0, "b": 0.0}, 0.5)
	// (1.0*3 + 0.0*1) / (3+1) = 0.75
	if score.Aggregate != 0.75 {
		t.Errorf("weighted aggregate = %v, want 0.75", score.Aggregate)
	}
	if ingestOutcomesScore(score).Verdict != "PASS" { // 0.75 >= 0.5
		t.Error("0.75 vs threshold 0.5 should be PASS")
	}
}
