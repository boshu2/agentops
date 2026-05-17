package goalsfitness

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/scenarioresults"
)

// aggFromResults builds an Aggregator over an in-memory artifact via the
// newAggregatorFromLoad seam, so satisfaction tests do not depend on the
// runtime artifact path or shared fixture files.
func aggFromResults(results []scenarioresults.ScenarioResult) *Aggregator {
	return newAggregatorFromLoad(scenarioresults.LoadResult{
		Status:   scenarioresults.StatusOK,
		Artifact: &scenarioresults.Artifact{Results: results},
	})
}

// res is a terse ScenarioResult constructor for table tests.
func res(id string, score, threshold float64, verdict string) scenarioresults.ScenarioResult {
	return scenarioresults.ScenarioResult{
		ScenarioID: id,
		Score:      score,
		Threshold:  threshold,
		Verdict:    verdict,
		JudgedAt:   "2026-05-17T09:00:00Z",
	}
}

func TestParseScenarioThreshold_ExactCases(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    float64
		wantErr bool
	}{
		{name: "empty defaults to 0.8", in: "", want: DefaultScenarioThreshold},
		{name: "whitespace defaults to 0.8", in: "  ", want: DefaultScenarioThreshold},
		{name: "explicit value", in: "0.6", want: 0.6},
		{name: "explicit one", in: "1", want: 1},
		{name: "explicit zero", in: "0", want: 0},
		{name: "non-numeric errors", in: "high", wantErr: true},
		{name: "above range errors", in: "1.5", wantErr: true},
		{name: "below range errors", in: "-0.2", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseScenarioThreshold(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseScenarioThreshold(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseScenarioThreshold(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseScenarioThreshold(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestEvaluateSatisfaction_VerdictAndFraction(t *testing.T) {
	cases := []struct {
		name             string
		results          []scenarioresults.ScenarioResult
		link             DirectiveLink
		threshold        float64
		wantVerdict      Verdict
		wantSatisfaction float64
		wantSatisfied    int
		wantLinked       int
		wantEvaluated    int
		wantSkipped      int
		wantMissing      int
		wantWarning      bool
	}{
		{
			name: "below directive threshold reports fail",
			// one of two scenarios satisfied -> fraction 0.5 < 0.8.
			results: []scenarioresults.ScenarioResult{
				res("s-1", 0.95, 0.8, "pass"),
				res("s-2", 0.40, 0.8, "fail"),
			},
			link:             DirectiveLink{DirectiveID: "d-below", ScenarioIDs: []string{"s-1", "s-2"}},
			threshold:        0.8,
			wantVerdict:      VerdictFail,
			wantSatisfaction: 0.5,
			wantSatisfied:    1,
			wantLinked:       2,
			wantEvaluated:    2,
		},
		{
			name: "threshold equality reports pass",
			// 4 of 5 satisfied -> fraction 0.8 == threshold 0.8.
			results: []scenarioresults.ScenarioResult{
				res("s-1", 0.95, 0.8, "pass"),
				res("s-2", 0.91, 0.8, "pass"),
				res("s-3", 0.88, 0.8, "pass"),
				res("s-4", 0.85, 0.8, "pass"),
				res("s-5", 0.10, 0.8, "fail"),
			},
			link: DirectiveLink{
				DirectiveID: "d-equal",
				ScenarioIDs: []string{"s-1", "s-2", "s-3", "s-4", "s-5"},
			},
			threshold:        0.8,
			wantVerdict:      VerdictPass,
			wantSatisfaction: 0.8,
			wantSatisfied:    4,
			wantLinked:       5,
			wantEvaluated:    5,
		},
		{
			name: "per-scenario threshold equality counts as satisfied",
			// scenario score exactly meets its own threshold -> satisfied.
			results: []scenarioresults.ScenarioResult{
				res("s-1", 0.8, 0.8, "pass"),
			},
			link:             DirectiveLink{DirectiveID: "d-scn-eq", ScenarioIDs: []string{"s-1"}},
			threshold:        0.8,
			wantVerdict:      VerdictPass,
			wantSatisfaction: 1.0,
			wantSatisfied:    1,
			wantLinked:       1,
			wantEvaluated:    1,
		},
		{
			name: "mixed pass fail fraction computed correctly",
			// 2 of 3 satisfied -> fraction 0.6667 >= threshold 0.6 -> pass.
			results: []scenarioresults.ScenarioResult{
				res("s-1", 0.90, 0.8, "pass"),
				res("s-2", 0.30, 0.8, "fail"),
				res("s-3", 0.81, 0.8, "pass"),
			},
			link: DirectiveLink{
				DirectiveID: "d-mixed",
				ScenarioIDs: []string{"s-1", "s-2", "s-3"},
			},
			threshold:        0.6,
			wantVerdict:      VerdictPass,
			wantSatisfaction: 2.0 / 3.0,
			wantSatisfied:    2,
			wantLinked:       3,
			wantEvaluated:    3,
		},
		{
			name: "all skipped yields unknown never pass",
			results: []scenarioresults.ScenarioResult{
				res("s-1", 0.0, 0.8, "skip"),
				res("s-2", 0.0, 0.8, "skip"),
			},
			link:             DirectiveLink{DirectiveID: "d-skip", ScenarioIDs: []string{"s-1", "s-2"}},
			threshold:        0.8,
			wantVerdict:      VerdictUnknown,
			wantSatisfaction: 0,
			wantSatisfied:    0,
			wantLinked:       2,
			wantSkipped:      2,
		},
		{
			name: "all missing yields unknown never pass",
			results: []scenarioresults.ScenarioResult{
				res("s-other", 0.95, 0.8, "pass"),
			},
			link:             DirectiveLink{DirectiveID: "d-missing", ScenarioIDs: []string{"s-1", "s-2"}},
			threshold:        0.8,
			wantVerdict:      VerdictUnknown,
			wantSatisfaction: 0,
			wantSatisfied:    0,
			wantLinked:       2,
			wantMissing:      2,
		},
		{
			name:        "zero linked scenarios yields unknown plus warning",
			results:     []scenarioresults.ScenarioResult{res("s-1", 0.95, 0.8, "pass")},
			link:        DirectiveLink{DirectiveID: "d-empty", ScenarioIDs: nil},
			threshold:   0.8,
			wantVerdict: VerdictUnknown,
			wantLinked:  0,
			wantWarning: true,
		},
		{
			name: "satisfied but below threshold still fails",
			// all evaluated, 1 of 2 satisfied -> fraction 0.5; threshold 1.0.
			results: []scenarioresults.ScenarioResult{
				res("s-1", 0.99, 0.8, "pass"),
				res("s-2", 0.70, 0.8, "fail"),
			},
			link:             DirectiveLink{DirectiveID: "d-strict", ScenarioIDs: []string{"s-1", "s-2"}},
			threshold:        1.0,
			wantVerdict:      VerdictFail,
			wantSatisfaction: 0.5,
			wantSatisfied:    1,
			wantLinked:       2,
			wantEvaluated:    2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := aggFromResults(tc.results)
			got := a.EvaluateSatisfaction(tc.link, tc.threshold)

			if got.Verdict != tc.wantVerdict {
				t.Fatalf("Verdict = %q, want %q", got.Verdict, tc.wantVerdict)
			}
			if got.Satisfaction != tc.wantSatisfaction {
				t.Fatalf("Satisfaction = %v, want %v", got.Satisfaction, tc.wantSatisfaction)
			}
			if got.Satisfied != tc.wantSatisfied {
				t.Fatalf("Satisfied = %d, want %d", got.Satisfied, tc.wantSatisfied)
			}
			if got.Linked != tc.wantLinked {
				t.Fatalf("Linked = %d, want %d", got.Linked, tc.wantLinked)
			}
			if got.Evaluated != tc.wantEvaluated {
				t.Fatalf("Evaluated = %d, want %d", got.Evaluated, tc.wantEvaluated)
			}
			if got.Skipped != tc.wantSkipped {
				t.Fatalf("Skipped = %d, want %d", got.Skipped, tc.wantSkipped)
			}
			if got.Missing != tc.wantMissing {
				t.Fatalf("Missing = %d, want %d", got.Missing, tc.wantMissing)
			}
			if (got.Warning != "") != tc.wantWarning {
				t.Fatalf("Warning = %q, wantWarning = %v", got.Warning, tc.wantWarning)
			}
			if got.Threshold != tc.threshold {
				t.Fatalf("Threshold = %v, want %v", got.Threshold, tc.threshold)
			}
			if got.DirectiveID != tc.link.DirectiveID {
				t.Fatalf("DirectiveID = %q, want %q", got.DirectiveID, tc.link.DirectiveID)
			}
		})
	}
}

func TestEvaluateSatisfaction_NoArtifactIsUnknown(t *testing.T) {
	// A skip-state load (no artifact) must never pass, even with linked scenarios.
	a := newAggregatorFromLoad(scenarioresults.LoadResult{
		Status:  scenarioresults.StatusUnknown,
		Warning: "scenario-results artifact not found",
	})
	got := a.EvaluateSatisfaction(
		DirectiveLink{DirectiveID: "d-x", ScenarioIDs: []string{"s-1"}}, 0.8)

	if got.Verdict != VerdictUnknown {
		t.Fatalf("Verdict = %q, want %q", got.Verdict, VerdictUnknown)
	}
	if got.Satisfaction != 0 {
		t.Fatalf("Satisfaction = %v, want 0", got.Satisfaction)
	}
	if got.Warning == "" {
		t.Fatalf("Warning = empty, want loader warning passed through")
	}
}

func TestVerdict_DurableValues(t *testing.T) {
	// The four durable verdict values must remain exactly pass|fail|skip|unknown.
	cases := map[Verdict]string{
		VerdictPass:    "pass",
		VerdictFail:    "fail",
		VerdictSkip:    "skip",
		VerdictUnknown: "unknown",
	}
	for v, want := range cases {
		if string(v) != want {
			t.Fatalf("verdict %v string = %q, want %q", v, string(v), want)
		}
	}
}
