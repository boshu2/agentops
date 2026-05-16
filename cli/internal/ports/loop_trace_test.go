// practices: [tdd, ddd-bounded-context]
package ports

import (
	"reflect"
	"testing"
)

// fullTrace returns a CycleTrace with every evidence field populated —
// the shape a non-trivial evolve cycle records.
func fullTrace() *CycleTrace {
	return &CycleTrace{
		GoalHypothesis:     "lifting test-pass-rate raises fitness",
		SelectedGap:        "loop ports lack a trace field",
		Gherkin:            "Feature: trace\n  Scenario: reviewer reconstructs a cycle",
		FirstFailingProof:  "go test ./internal/ports -run TraceCompleteness FAIL",
		RedEvidence:        "TestTraceCompleteness_FullTrace red: undefined CycleTrace",
		GreenEvidence:      "TestTraceCompleteness_FullTrace pass",
		RefactorNote:       "none",
		ValidationEvidence: "go test ./... green",
		RatchetAction:      "ao ratchet record implement",
		GoalReshape:        "goal unchanged; gap closed",
	}
}

func TestTraceCompleteness_FullTraceIsComplete(t *testing.T) {
	exempt, missing := TraceCompleteness(fullTrace())
	if exempt {
		t.Errorf("full trace reported exempt; want not exempt")
	}
	if len(missing) != 0 {
		t.Errorf("full trace missing = %v, want none", missing)
	}
}

func TestTraceCompleteness_NilTraceMissesEverything(t *testing.T) {
	exempt, missing := TraceCompleteness(nil)
	if exempt {
		t.Errorf("nil trace reported exempt; want not exempt")
	}
	if len(missing) != 10 {
		t.Errorf("nil trace missing %d fields, want 10: %v", len(missing), missing)
	}
}

func TestTraceCompleteness_ExemptionSkipsRequiredFields(t *testing.T) {
	exempt, missing := TraceCompleteness(&CycleTrace{
		ExemptionReason: "trivial one-shot typo fix; no Gherkin or failing proof appropriate",
	})
	if !exempt {
		t.Errorf("trace with exemption reason reported not exempt; want exempt")
	}
	if len(missing) != 0 {
		t.Errorf("exempt trace missing = %v, want none", missing)
	}
}

func TestTraceCompleteness_ReportsEachMissingField(t *testing.T) {
	tr := fullTrace()
	tr.RedEvidence = ""
	tr.GoalReshape = ""
	exempt, missing := TraceCompleteness(tr)
	if exempt {
		t.Errorf("partial trace reported exempt; want not exempt")
	}
	want := []string{"red_evidence", "goal_reshape"}
	if !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
}

func TestTraceCompleteness_RefactorNoteIsRequired(t *testing.T) {
	tr := fullTrace()
	tr.RefactorNote = ""
	_, missing := TraceCompleteness(tr)
	found := false
	for _, m := range missing {
		if m == "refactor_note" {
			found = true
		}
	}
	if !found {
		t.Errorf("empty refactor_note not flagged; missing = %v", missing)
	}
}
