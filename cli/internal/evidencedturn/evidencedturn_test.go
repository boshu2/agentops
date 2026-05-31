package evidencedturn

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/turnstate"
)

// wellFormedTransitions builds a verifying, sealed log that folds the bead into
// the "closed" state: ""->in_progress->validated->closed.
func wellFormedTransitions(t *testing.T, beadID string) []turnstate.Transition {
	t.Helper()
	steps := []struct {
		from, to, ts string
	}{
		{turnstate.InitialState, "in_progress", "2026-05-31T00:00:00Z"},
		{"in_progress", "validated", "2026-05-31T01:00:00Z"},
		{"validated", "closed", "2026-05-31T02:00:00Z"},
	}
	var log []turnstate.Transition
	for _, s := range steps {
		var err error
		log, err = turnstate.Append(log, turnstate.Transition{
			ArtifactID: beadID,
			FromState:  s.from,
			ToState:    s.to,
			TS:         s.ts,
		})
		if err != nil {
			t.Fatalf("Append(%s->%s): %v", s.from, s.to, err)
		}
	}
	return log
}

func provEdge(t *testing.T, from, to string) provenancegraph.Edge {
	t.Helper()
	e := provenancegraph.Edge{
		FromID:   from,
		FromType: "commit",
		ToID:     to,
		ToType:   "bead",
		Relation: "commit_implements_decision",
		EvidenceRef: "deadbeef",
		TrustTier: "inferred",
		TS:        "2026-05-31T03:00:00Z",
	}
	sealed, err := provenancegraph.Seal(e, "")
	if err != nil {
		t.Fatalf("Seal edge: %v", err)
	}
	return sealed
}

// wellFormedInput is a complete Evidenced Turn that should evaluate Done.
func wellFormedInput(t *testing.T) Input {
	t.Helper()
	bead := "ag-lmdx.5"
	return Input{
		BeadID:      bead,
		Transitions: wellFormedTransitions(t, bead),
		Scenarios: []Scenario{
			{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: true},
		},
		ProvenanceEdges: []provenancegraph.Edge{provEdge(t, "commit:abc123", bead)},
		OrphanFindings:  nil,
		OrphanChecked:   true,
	}
}

// predicate looks up a predicate row by name; fails the test if absent.
func predicate(t *testing.T, v Verdict, name string) PredicateResult {
	t.Helper()
	for _, p := range v.Predicates {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("verdict missing predicate %q", name)
	return PredicateResult{}
}

func TestEvaluate_CompleteTurnIsDone(t *testing.T) {
	v, err := Evaluate(wellFormedInput(t))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Done != true {
		t.Errorf("Done = %v, want true (gaps: %v)", v.Done, v.Gaps)
	}
	if v.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", v.SchemaVersion, SchemaVersion)
	}
	if v.BeadID != "ag-lmdx.5" {
		t.Errorf("BeadID = %q, want %q", v.BeadID, "ag-lmdx.5")
	}
	if len(v.Gaps) != 0 {
		t.Errorf("Gaps = %v, want empty", v.Gaps)
	}
	if len(v.Predicates) != len(orderedPredicates) {
		t.Errorf("len(Predicates) = %d, want %d", len(v.Predicates), len(orderedPredicates))
	}
	for _, p := range v.Predicates {
		if !p.Passed {
			t.Errorf("predicate %q should pass; reason=%q", p.Name, p.Reason)
		}
	}
}

func TestEvaluate_PredicateOrderIsStable(t *testing.T) {
	v, err := Evaluate(wellFormedInput(t))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for i, want := range orderedPredicates {
		if v.Predicates[i].Name != want {
			t.Errorf("Predicates[%d].Name = %q, want %q", i, v.Predicates[i].Name, want)
		}
	}
}

func TestEvaluate_MissingPredicateFailsWithLegibleReason(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(in *Input)
		wantPredicate string
		wantReason    string // substring the failing reason must contain
		wantState     bool   // expected Passed value for the named predicate
	}{
		{
			name: "uncovered scenario",
			mutate: func(in *Input) {
				in.Scenarios = []Scenario{
					{Slug: "ao-turn-verify", HasPassingTest: false, EvidenceResolved: true},
				}
			},
			wantPredicate: PredicateScenariosCovered,
			wantReason:    "scenario(s) with no passing test: ao-turn-verify",
			wantState:     false,
		},
		{
			name: "evidence does not resolve",
			mutate: func(in *Input) {
				in.Scenarios = []Scenario{
					{Slug: "ao-turn-verify", HasPassingTest: true, EvidenceResolved: false},
				}
			},
			wantPredicate: PredicateEvidenceResolves,
			wantReason:    "Evidence does not resolve to a gate log that exercised it: ao-turn-verify",
			wantState:     false,
		},
		{
			name: "no scenarios at all",
			mutate: func(in *Input) {
				in.Scenarios = nil
			},
			wantPredicate: PredicateScenariosCovered,
			wantReason:    "no Closes-scenario claims",
			wantState:     false,
		},
		{
			name: "no provenance event",
			mutate: func(in *Input) {
				in.ProvenanceEdges = nil
			},
			wantPredicate: PredicateProvenanceEvent,
			wantReason:    `no provenance edge references bead "ag-lmdx.5"`,
			wantState:     false,
		},
		{
			name: "orphan artifact",
			mutate: func(in *Input) {
				in.OrphanFindings = []provenancegraph.OrphanFinding{
					{OrphanArtifactID: "cli/orphan.go", Code: "ORPHAN_ARTIFACT", Message: "no inbound edge"},
				}
			},
			wantPredicate: PredicateNoOrphan,
			wantReason:    "orphan artifact(s) with no inbound provenance edge: cli/orphan.go",
			wantState:     false,
		},
		{
			name: "orphan audit not run",
			mutate: func(in *Input) {
				in.OrphanChecked = false
			},
			wantPredicate: PredicateNoOrphan,
			wantReason:    "orphan audit not run",
			wantState:     false,
		},
		{
			name: "not at terminal state",
			mutate: func(in *Input) {
				// Only the genesis step: folds to "in_progress", not terminal.
				log, err := turnstate.Append(nil, turnstate.Transition{
					ArtifactID: in.BeadID,
					FromState:  turnstate.InitialState,
					ToState:    "in_progress",
					TS:         "2026-05-31T00:00:00Z",
				})
				if err != nil {
					t.Fatalf("Append: %v", err)
				}
				in.Transitions = log
			},
			wantPredicate: PredicateTerminalState,
			wantReason:    `folded state "in_progress" is not at the validated->closed boundary`,
			wantState:     false,
		},
		{
			name: "no transitions for bead",
			mutate: func(in *Input) {
				in.Transitions = nil
			},
			wantPredicate: PredicateTerminalState,
			wantReason:    "has no state_transition log",
			wantState:     false,
		},
		{
			name: "tampered chain",
			mutate: func(in *Input) {
				// Corrupt the tip's hash so FoldVerified fails.
				in.Transitions[len(in.Transitions)-1].Hash = "deadbeef"
			},
			wantPredicate: PredicateChainIntact,
			wantReason:    "does not verify/fold",
			wantState:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := wellFormedInput(t)
			tt.mutate(&in)
			v, err := Evaluate(in)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if v.Done != false {
				t.Errorf("Done = %v, want false", v.Done)
			}
			p := predicate(t, v, tt.wantPredicate)
			if p.Passed != tt.wantState {
				t.Errorf("predicate %q Passed = %v, want %v (reason=%q)", tt.wantPredicate, p.Passed, tt.wantState, p.Reason)
			}
			if !contains(p.Reason, tt.wantReason) {
				t.Errorf("predicate %q reason = %q, want substring %q", tt.wantPredicate, p.Reason, tt.wantReason)
			}
			// The failing predicate's reason must surface in Gaps verbatim.
			wantGap := tt.wantPredicate + ": " + p.Reason
			if !sliceContains(v.Gaps, wantGap) {
				t.Errorf("Gaps = %v, want it to contain %q", v.Gaps, wantGap)
			}
		})
	}
}

func TestEvaluate_EmptyBeadIDErrors(t *testing.T) {
	_, err := Evaluate(Input{BeadID: "   "})
	if err == nil {
		t.Fatal("Evaluate with blank bead_id: want error, got nil")
	}
	if !contains(err.Error(), "bead_id is required") {
		t.Errorf("error = %q, want substring %q", err.Error(), "bead_id is required")
	}
}

func TestEvaluate_MultipleGapsAllReported(t *testing.T) {
	in := wellFormedInput(t)
	in.Scenarios = nil      // fails scenarios_covered + evidence_resolves
	in.ProvenanceEdges = nil // fails provenance_event
	v, err := Evaluate(in)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Done != false {
		t.Errorf("Done = %v, want false", v.Done)
	}
	if len(v.Gaps) != 3 {
		t.Errorf("len(Gaps) = %d, want 3 (gaps=%v)", len(v.Gaps), v.Gaps)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func sliceContains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
