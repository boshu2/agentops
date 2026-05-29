package evalsubstrate

import (
	"encoding/json"
	"strings"
)

// Rubric is the holdout-safe projection of a locked eval Task's grading criteria,
// used as the payload for an Outcomes-style grading transport (cloud Managed
// Agents, async jobs, or the local Codex/NTM path). It is a DERIVED artifact:
// ~/.agents/evals/SCHEMA.md remains the sole authority and a Rubric is never an
// alternate bar. A Rubric NEVER carries ground-truth answers (GroundTruthRow.Value)
// — that is the load-bearing holdout-isolation invariant at the cloud boundary,
// because Managed Agents are not ZDR. The projection enforces this by construction:
// ProjectRubric does not accept ground-truth rows, so it cannot copy them.
type Rubric struct {
	SchemaVersion    int         `json:"schema_version"`
	SourceTaskID     string      `json:"source_task_id"`
	JudgeContentHash string      `json:"judge_content_hash"`
	Instructions     string      `json:"instructions,omitempty"`
	Criteria         []Criterion `json:"criteria"`
}

// Criterion is a single allowlisted grading dimension projected from the judge
// spec. Only these fields ever cross the boundary — no target, value, or answer.
type Criterion struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

// ProjectRubric builds a holdout-safe Rubric from a locked Task, its grading
// criteria (sourced upstream from the judge spec), and the judge content hash
// used for drift detection (SCHEMA rc2 parity). It copies ONLY the allowlisted
// criterion fields and the task's id/description; it never reads or copies any
// GroundTruthRow value. The judgeContentHash lets a stale rubric self-invalidate
// exactly like a stale local judge.
func ProjectRubric(task Task, criteria []Criterion, judgeContentHash string) Rubric {
	out := make([]Criterion, 0, len(criteria))
	for _, c := range criteria {
		out = append(out, Criterion{
			ID:          c.ID,
			Description: c.Description,
			Weight:      c.Weight,
		})
	}
	return Rubric{
		SchemaVersion:    SchemaVersion,
		SourceTaskID:     task.ID,
		JudgeContentHash: judgeContentHash,
		Instructions:     task.Description,
		Criteria:         out,
	}
}

// ContainsAny is the defense-in-depth re-scan guard (ag-hdqu0 holdout-leak guard
// layer 3). It reports the first forbidden substring (e.g. a holdout
// GroundTruthRow.Value) found in the Rubric's JSON encoding, or ("", false) when
// the payload is clean. Empty forbidden strings never match. Even though
// ProjectRubric cannot copy ground truth by construction, every caller MUST
// re-scan the emitted payload with the run's holdout values before it crosses
// the cloud boundary.
func (r Rubric) ContainsAny(forbidden []string) (string, bool) {
	blob, err := json.Marshal(r)
	if err != nil {
		return "", false
	}
	payload := string(blob)
	for _, f := range forbidden {
		if f == "" {
			continue
		}
		if strings.Contains(payload, f) {
			return f, true
		}
	}
	return "", false
}
