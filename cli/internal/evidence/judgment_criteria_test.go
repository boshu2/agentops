package evidence

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestJudgmentsRejectPartialRequiredCriterionCoverage(t *testing.T) {
	f := newJudgmentFixture(t, "codex")
	intent := []byte("c1: the value is correct\nc2: changes stay in scope\n")
	write(t, f.options.Intent, intent)
	f.receipt.AcceptanceDigest = Hash(intent)
	f.options.RequiredCriteria = []string{"c1", "c2"}
	f.publish(t, nil) // A valid, correctly bound PASS contains only c1.
	result, err := VerifyJudgments(f.options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Satisfied || len(result.Legs) != 1 || result.Legs[0].Verdict != "PASS" || !strings.Contains(strings.Join(result.Legs[0].Problems, ";"), "missing_required_criterion: c2") {
		t.Fatalf("partial PASS must remain visible and unproven: %+v", result)
	}
}

func TestJudgmentsRequireExactSelectedCriterionIDs(t *testing.T) {
	cases := []struct {
		name             string
		required, actual []string
		verdict          string
		wantSatisfied    bool
		wantProblem      string
	}{
		{"complete PASS", []string{"c1", "c2"}, []string{"c1", "c2"}, "PASS", true, ""},
		{"order independent", []string{"c1", "c2"}, []string{"c2", "c1"}, "PASS", true, ""},
		{"partial PASS", []string{"c1", "c2"}, []string{"c1"}, "PASS", false, "missing_required_criterion: c2"},
		{"duplicate actual", []string{"c1", "c2"}, []string{"c1", "c2", "c1"}, "PASS", false, "duplicate_criterion: c1"},
		{"unknown actual", []string{"c1", "c2"}, []string{"c1", "c2", "other"}, "PASS", false, "unexpected_criterion: other"},
		{"unknown expected", []string{"c1", "other"}, []string{"c1", "c2"}, "PASS", false, "missing_required_criterion: other"},
		{"complete FAIL", []string{"c1", "c2"}, []string{"c1", "c2"}, "FAIL", false, "verdict_not_pass"},
		{"partial FAIL", []string{"c1", "c2"}, []string{"c1"}, "FAIL", false, "missing_required_criterion: c2"},
		{"legacy unselected policy", nil, []string{"c1"}, "PASS", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newJudgmentFixture(t, "codex")
			f.options.RequiredCriteria = tc.required
			f.publish(t, func(v map[string]any) {
				v["verdict"] = tc.verdict
				criteria := []any{}
				for _, id := range tc.actual {
					criteria = append(criteria, map[string]any{"id": id, "result": tc.verdict, "evidence_refs": []string{"test:receipt"}})
				}
				v["criteria"] = criteria
			})
			before, err := os.ReadFile(f.options.Verdicts[0])
			if err != nil {
				t.Fatal(err)
			}
			result, err := VerifyJudgments(f.options)
			if err != nil {
				t.Fatal(err)
			}
			if result.Satisfied != tc.wantSatisfied || !slices.Equal(result.RequiredCriteria, tc.required) || len(result.Legs) != 1 || result.Legs[0].Verdict != tc.verdict {
				t.Fatalf("coverage or original verdict changed: %+v", result)
			}
			if tc.wantProblem != "" && !strings.Contains(strings.Join(result.Legs[0].Problems, ";"), tc.wantProblem) {
				t.Fatalf("wanted %s, got %+v", tc.wantProblem, result)
			}
			if tc.name == "complete FAIL" && !slices.Equal(result.Legs[0].Problems, []string{"verdict_not_pass"}) {
				t.Fatalf("complete FAIL gained unrelated problems: %+v", result)
			}
			after, err := os.ReadFile(f.options.Verdicts[0])
			if err != nil || string(before) != string(after) {
				t.Fatal("original verdict mutated")
			}
		})
	}
}

func TestJudgmentsInvalidCriterionPolicyBeforeSubjectReads(t *testing.T) {
	for _, required := range [][]string{{}, {""}, {" "}, {"c1", "c1"}, {" c1"}} {
		f := newJudgmentFixture(t, "codex")
		f.options.RequiredCriteria = required
		f.options.Manifest = "/missing-subject-must-not-be-read"
		if result, err := VerifyJudgments(f.options); err == nil || !strings.Contains(err.Error(), "criterion IDs") || result != nil {
			t.Fatalf("invalid policy %q did not fail before reads: %+v, %v", required, result, err)
		}
	}
}
