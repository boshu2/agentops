package main

import (
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

func sampleTaskAndCriteria() (evalsubstrate.Task, []evalsubstrate.Criterion) {
	task := evalsubstrate.Task{
		SchemaVersion: evalsubstrate.SchemaVersion,
		ID:            "task-capital-cities",
		Domain:        "geography",
		Description:   "Answer the capital-city question accurately.",
	}
	criteria := []evalsubstrate.Criterion{
		{ID: "accuracy", Description: "Names the correct capital city.", Weight: 0.7},
		{ID: "concision", Description: "Answers in one short sentence.", Weight: 0.3},
	}
	return task, criteria
}

// TestCompileOutcomesRubric_StripsHoldoutTarget: the compiled rubric carries the
// criteria but none of the holdout answer values — the holdout-isolation
// invariant at the cloud boundary (Managed Agents are not ZDR).
func TestCompileOutcomesRubric_StripsHoldoutTarget(t *testing.T) {
	task, criteria := sampleTaskAndCriteria()
	holdout := []string{"Ouagadougou", "Antananarivo"}

	r, err := compileOutcomesRubric(task, criteria, "sha256:abc123", holdout)
	if err != nil {
		t.Fatalf("compile: unexpected error: %v", err)
	}
	if r.SourceTaskID != task.ID {
		t.Errorf("SourceTaskID = %q, want %q", r.SourceTaskID, task.ID)
	}
	if r.JudgeContentHash != "sha256:abc123" {
		t.Errorf("JudgeContentHash = %q, want sha256:abc123", r.JudgeContentHash)
	}
	if len(r.Criteria) != 2 {
		t.Fatalf("len(Criteria) = %d, want 2", len(r.Criteria))
	}
	if hit, found := r.ContainsAny(holdout); found {
		t.Errorf("compiled rubric leaked holdout value %q", hit)
	}
}

// TestCompileOutcomesRubric_RefusesLeak: deny-by-default — if a holdout value
// would appear in the payload (e.g. a criterion description accidentally embeds
// the answer), compile MUST refuse rather than emit a leaking rubric.
func TestCompileOutcomesRubric_RefusesLeak(t *testing.T) {
	task, _ := sampleTaskAndCriteria()
	leaky := []evalsubstrate.Criterion{
		{ID: "accuracy", Description: "The answer is Ouagadougou.", Weight: 1.0},
	}
	_, err := compileOutcomesRubric(task, leaky, "sha256:abc123", []string{"Ouagadougou"})
	if err == nil {
		t.Fatal("expected compile to refuse a leaking rubric, got nil error")
	}
	if !strings.Contains(err.Error(), "Ouagadougou") {
		t.Errorf("error should name the leaked value, got: %v", err)
	}
}
