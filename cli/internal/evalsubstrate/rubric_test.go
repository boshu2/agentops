package evalsubstrate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProjectRubric_StripsHoldoutValues is the keystone holdout-isolation test
// for the Outcomes projection (ag-hdqu0.9 / ag-hdqu0.1). It proves that a Rubric
// built from a locked Task carries grading criteria but ZERO ground-truth answer
// values — the load-bearing invariant at the cloud boundary (Managed Agents are
// not ZDR).
func TestProjectRubric_StripsHoldoutValues(t *testing.T) {
	task := Task{
		SchemaVersion: SchemaVersion,
		ID:            "task-capital-cities",
		Domain:        "geography",
		Description:   "Answer the capital-city question accurately.",
	}
	criteria := []Criterion{
		{ID: "accuracy", Description: "Names the correct capital city.", Weight: 0.7},
		{ID: "concision", Description: "Answers in one short sentence.", Weight: 0.3},
	}
	// The holdout ground-truth answers that MUST NEVER appear in the rubric.
	holdout := []GroundTruthRow{
		{ID: "q1", Value: "Ouagadougou", Split: "holdout"},
		{ID: "q2", Value: "Antananarivo", Split: "holdout"},
	}
	const judgeHash = "sha256:abc123"

	r := ProjectRubric(task, criteria, judgeHash)

	if r.SourceTaskID != task.ID {
		t.Errorf("SourceTaskID = %q, want %q", r.SourceTaskID, task.ID)
	}
	if r.JudgeContentHash != judgeHash {
		t.Errorf("JudgeContentHash = %q, want %q", r.JudgeContentHash, judgeHash)
	}
	if len(r.Criteria) != 2 {
		t.Fatalf("len(Criteria) = %d, want 2", len(r.Criteria))
	}
	if r.Criteria[0].ID != "accuracy" || r.Criteria[0].Weight != 0.7 {
		t.Errorf("Criteria[0] = %+v, want accuracy/0.7", r.Criteria[0])
	}

	// Holdout isolation: the marshaled rubric must contain none of the answers.
	blob, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	payload := string(blob)
	for _, gt := range holdout {
		if strings.Contains(payload, gt.Value) {
			t.Errorf("rubric payload leaked holdout value %q", gt.Value)
		}
	}
}

// TestRubric_ContainsAny is the defense-in-depth re-scan guard (layer 3): even
// though ProjectRubric cannot copy ground truth by construction, callers MUST
// re-scan the emitted payload before it crosses the cloud boundary. The guard
// must (a) pass a clean rubric and (b) catch a planted forbidden value.
func TestRubric_ContainsAny(t *testing.T) {
	clean := ProjectRubric(
		Task{ID: "t1"},
		[]Criterion{{ID: "c1", Description: "be correct", Weight: 1.0}},
		"sha256:deadbeef",
	)
	if hit, found := clean.ContainsAny([]string{"Ouagadougou", "Antananarivo"}); found {
		t.Errorf("clean rubric falsely flagged for %q", hit)
	}
	// Empty forbidden strings must never match.
	if _, found := clean.ContainsAny([]string{""}); found {
		t.Error("empty forbidden string must not match")
	}
	// Plant a forbidden value in the instructions and confirm the guard catches it.
	planted := clean
	planted.Instructions = "The answer is Ouagadougou."
	if hit, found := planted.ContainsAny([]string{"Ouagadougou"}); !found || hit != "Ouagadougou" {
		t.Errorf("guard missed planted value: hit=%q found=%v", hit, found)
	}
}
