package evalsubstrate

import (
	"strings"
	"testing"
)

// TestRubric_FreshAgainst is the stale-rubric rejection guard (ag-hdqu0.4 / SCHEMA
// gate-#2 drift parity): an Outcomes rubric must be refused when its carried
// judge_content_hash no longer matches the current judge, exactly like a stale
// local judge — so a divergent cloud rubric self-invalidates instead of grading
// against an outdated bar.
func TestRubric_FreshAgainst(t *testing.T) {
	base := ProjectRubric(Task{ID: "t1"}, []Criterion{{ID: "c1", Weight: 1}}, "sha256:current")

	if err := base.FreshAgainst("sha256:current"); err != nil {
		t.Errorf("matching hash should be fresh, got error: %v", err)
	}

	err := base.FreshAgainst("sha256:changed")
	if err == nil {
		t.Fatal("mismatched judge hash must be rejected, got nil")
	}
	// The error must name both hashes so the drift is diagnosable.
	if !strings.Contains(err.Error(), "sha256:current") || !strings.Contains(err.Error(), "sha256:changed") {
		t.Errorf("error should name both rubric and current hash, got: %v", err)
	}

	// A rubric with no carried hash cannot be proven fresh → reject.
	noHash := ProjectRubric(Task{ID: "t2"}, nil, "")
	if err := noHash.FreshAgainst("sha256:current"); err == nil {
		t.Error("rubric with empty judge_content_hash must be rejected")
	}

	// An empty current hash means we cannot verify → reject.
	if err := base.FreshAgainst(""); err == nil {
		t.Error("empty current judge hash must be rejected (cannot verify freshness)")
	}
}
