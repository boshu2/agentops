package eval

import (
	"fmt"
	"sort"
	"testing"
)

func TestBuildScorecardRPICategories(t *testing.T) {
	candidate := scorecardRun("candidate-rpi", StatusPass, map[string]float64{
		"artifact-completeness": 1,
		"phase-order":           1,
		"objective-spine":       1,
		"validation-separation": 1,
		"scenario-satisfaction": 1,
		"runtime-safety":        1,
	})

	scorecard, err := BuildScorecard(candidate, nil, ScorecardOptions{Kind: ScorecardKindRPI})
	if err != nil {
		t.Fatalf("BuildScorecard returned error: %v", err)
	}

	if scorecard.Kind != ScorecardKindRPI {
		t.Fatalf("kind = %s, want %s", scorecard.Kind, ScorecardKindRPI)
	}
	if scorecard.CandidateRunID != "candidate-rpi" {
		t.Fatalf("candidate_run_id = %q, want candidate-rpi", scorecard.CandidateRunID)
	}
	if scorecard.Verdict != VerdictPass {
		t.Fatalf("verdict = %s, want pass", scorecard.Verdict)
	}
	assertScorecardCategories(t, scorecard, []string{
		"artifact completeness",
		"phase order",
		"objective spine",
		"validation separation",
		"scenario satisfaction",
		"runtime safety",
	})
	for _, category := range scorecard.Categories {
		if category.CandidateScore != 1 {
			t.Fatalf("%s candidate_score = %v, want 1", category.Category, category.CandidateScore)
		}
		if category.BaselineScore != nil {
			t.Fatalf("%s baseline_score = %v, want nil", category.Category, *category.BaselineScore)
		}
		if category.Delta != nil {
			t.Fatalf("%s delta = %v, want nil", category.Category, *category.Delta)
		}
		if category.Verdict != VerdictPass {
			t.Fatalf("%s verdict = %s, want pass", category.Category, category.Verdict)
		}
	}
}

func TestBuildScorecardSkillChangeCategories(t *testing.T) {
	candidate := scorecardRun("candidate-skill", StatusPass, map[string]float64{
		"structural": 1,
		"trigger":    1,
		"runtime":    1,
		"scenario":   1,
		"stocktake":  1,
	})

	scorecard, err := BuildScorecard(candidate, nil, ScorecardOptions{Kind: ScorecardKindSkillChange})
	if err != nil {
		t.Fatalf("BuildScorecard returned error: %v", err)
	}

	if scorecard.Kind != ScorecardKindSkillChange {
		t.Fatalf("kind = %s, want %s", scorecard.Kind, ScorecardKindSkillChange)
	}
	if scorecard.Verdict != VerdictPass {
		t.Fatalf("verdict = %s, want pass", scorecard.Verdict)
	}
	assertScorecardCategories(t, scorecard, []string{
		"structural",
		"trigger",
		"runtime",
		"scenario",
		"stocktake",
	})
}

func TestBuildScorecardMissingCategoryFails(t *testing.T) {
	candidate := scorecardRun("candidate-missing", StatusPass, map[string]float64{
		"artifact-completeness": 1,
		"phase-order":           1,
		"objective-spine":       1,
		"validation-separation": 1,
		"scenario-satisfaction": 1,
	})

	scorecard, err := BuildScorecard(candidate, nil, ScorecardOptions{Kind: ScorecardKindRPI})
	if err != nil {
		t.Fatalf("BuildScorecard returned error: %v", err)
	}

	if scorecard.Verdict != VerdictFail {
		t.Fatalf("verdict = %s, want fail", scorecard.Verdict)
	}
	category := requireScorecardCategory(t, scorecard, "runtime safety")
	if category.CandidateScore != 0 {
		t.Fatalf("runtime safety candidate_score = %v, want 0", category.CandidateScore)
	}
	if category.Verdict != VerdictFail {
		t.Fatalf("runtime safety verdict = %s, want fail", category.Verdict)
	}
	if category.Reason != `candidate has no matching case or dimension score for required category "runtime safety"` {
		t.Fatalf("runtime safety reason = %q", category.Reason)
	}
}

func TestBuildScorecardBaselineDeltaRegressionImprovement(t *testing.T) {
	t.Run("improvement", func(t *testing.T) {
		candidate := scorecardRun("candidate-improves", StatusPass, map[string]float64{
			"artifact-completeness": 1,
			"phase-order":           1,
			"objective-spine":       1,
			"validation-separation": 1,
			"scenario-satisfaction": 1,
			"runtime-safety":        1,
		})
		baseline := scorecardRun("baseline", StatusPass, map[string]float64{
			"artifact-completeness": 0.8,
			"phase-order":           1,
			"objective-spine":       1,
			"validation-separation": 1,
			"scenario-satisfaction": 1,
			"runtime-safety":        1,
		})

		scorecard, err := BuildScorecard(candidate, baseline, ScorecardOptions{Kind: ScorecardKindRPI})
		if err != nil {
			t.Fatalf("BuildScorecard returned error: %v", err)
		}
		if scorecard.Verdict != VerdictImprovement {
			t.Fatalf("verdict = %s, want improvement", scorecard.Verdict)
		}
		category := requireScorecardCategory(t, scorecard, "artifact completeness")
		assertFloatPtr(t, category.BaselineScore, 0.8, "artifact completeness baseline_score")
		assertFloatPtr(t, category.Delta, 0.2, "artifact completeness delta")
		if category.Verdict != VerdictImprovement {
			t.Fatalf("artifact completeness verdict = %s, want improvement", category.Verdict)
		}
		if category.Reason != "candidate improved artifact completeness by 0.2000" {
			t.Fatalf("artifact completeness reason = %q", category.Reason)
		}
	})

	t.Run("regression", func(t *testing.T) {
		candidate := scorecardRun("candidate-regresses", StatusPass, map[string]float64{
			"artifact-completeness": 1,
			"phase-order":           1,
			"objective-spine":       1,
			"validation-separation": 1,
			"scenario-satisfaction": 1,
			"runtime-safety":        0.7,
		})
		baseline := scorecardRun("baseline", StatusPass, map[string]float64{
			"artifact-completeness": 1,
			"phase-order":           1,
			"objective-spine":       1,
			"validation-separation": 1,
			"scenario-satisfaction": 1,
			"runtime-safety":        0.9,
		})

		scorecard, err := BuildScorecard(candidate, baseline, ScorecardOptions{
			Kind:                  ScorecardKindRPI,
			MaxCategoryRegression: 0.05,
		})
		if err != nil {
			t.Fatalf("BuildScorecard returned error: %v", err)
		}
		if scorecard.Verdict != VerdictRegression {
			t.Fatalf("verdict = %s, want regression", scorecard.Verdict)
		}
		category := requireScorecardCategory(t, scorecard, "runtime safety")
		assertFloatPtr(t, category.BaselineScore, 0.9, "runtime safety baseline_score")
		assertFloatPtr(t, category.Delta, -0.2, "runtime safety delta")
		if category.Verdict != VerdictRegression {
			t.Fatalf("runtime safety verdict = %s, want regression", category.Verdict)
		}
		if category.Reason != "candidate regressed runtime safety by -0.2000" {
			t.Fatalf("runtime safety reason = %q", category.Reason)
		}
	})
}

func TestBuildScorecardFailedCandidateFails(t *testing.T) {
	candidate := scorecardRun("candidate-failed", StatusError, map[string]float64{
		"artifact-completeness": 1,
		"phase-order":           1,
		"objective-spine":       1,
		"validation-separation": 1,
		"scenario-satisfaction": 1,
		"runtime-safety":        1,
	})

	scorecard, err := BuildScorecard(candidate, nil, ScorecardOptions{Kind: ScorecardKindRPI})
	if err != nil {
		t.Fatalf("BuildScorecard returned error: %v", err)
	}
	if scorecard.Verdict != VerdictFail {
		t.Fatalf("verdict = %s, want fail", scorecard.Verdict)
	}
	if scorecard.Reason != "candidate run status is error" {
		t.Fatalf("reason = %q, want candidate run status is error", scorecard.Reason)
	}
}

func scorecardRun(runID string, status Status, caseScores map[string]float64) *RunRecord {
	run := minimalRunRecord(runID, averageScore(caseScores), map[Dimension]float64{
		DimensionCorrectness: averageScore(caseScores),
	})
	run.Status = status
	run.Verdict = VerdictPass
	if status == StatusFail || status == StatusError {
		run.Verdict = VerdictFail
	}
	run.CaseResults = nil
	slugs := make([]string, 0, len(caseScores))
	for slug := range caseScores {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		score := caseScores[slug]
		run.CaseResults = append(run.CaseResults, CaseResult{
			ID:     fmt.Sprintf("scorecard.%s.surface", slug),
			Status: StatusPass,
			Score:  score,
			DimensionScores: map[Dimension]float64{
				DimensionCorrectness: score,
			},
		})
	}
	return run
}

func averageScore(scores map[string]float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	sum := 0.0
	for _, score := range scores {
		sum += score
	}
	return roundScore(sum / float64(len(scores)))
}

func assertScorecardCategories(t *testing.T, scorecard *Scorecard, want []string) {
	t.Helper()
	if len(scorecard.Categories) != len(want) {
		t.Fatalf("category count = %d, want %d: %+v", len(scorecard.Categories), len(want), scorecard.Categories)
	}
	for i, category := range scorecard.Categories {
		if category.Category != want[i] {
			t.Fatalf("categories[%d] = %q, want %q", i, category.Category, want[i])
		}
	}
}

func requireScorecardCategory(t *testing.T, scorecard *Scorecard, name string) ScorecardCategory {
	t.Helper()
	for _, category := range scorecard.Categories {
		if category.Category == name {
			return category
		}
	}
	t.Fatalf("category %q not found in %+v", name, scorecard.Categories)
	return ScorecardCategory{}
}

func assertFloatPtr(t *testing.T, got *float64, want float64, label string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %v", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %v, want %v", label, *got, want)
	}
}
