// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestInMemoryConvergenceCheck_CheckConvergesWithDefaultCriteria(t *testing.T) {
	checker := NewInMemoryConvergenceCheck()
	result, err := checker.Check(context.Background(), ConvergenceInput{
		RecentCIRuns: []CIRun{
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionFailure},
		},
		UnconsumedHighMedium:    1,
		FitnessBaselineCaptured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Converged {
		t.Fatalf("Converged = false, want true; reasons=%v", result.Reasons)
	}
	if result.CIGreenStreak != 3 {
		t.Fatalf("CIGreenStreak = %d, want 3", result.CIGreenStreak)
	}
	if len(result.Reasons) != 0 {
		t.Fatalf("Reasons = %v, want empty", result.Reasons)
	}
}

func TestInMemoryConvergenceCheck_CheckReportsEveryUnmetCriterion(t *testing.T) {
	checker := NewInMemoryConvergenceCheck()
	result, err := checker.Check(context.Background(), ConvergenceInput{
		RecentCIRuns: []CIRun{
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionFailure},
		},
		UnconsumedHighMedium:    2,
		FitnessBaselineCaptured: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Converged {
		t.Fatal("Converged = true, want false")
	}
	want := []string{
		"ci-green-streak-below-threshold",
		"unconsumed-high-medium-above-threshold",
		"fitness-baseline-missing",
	}
	if !reflect.DeepEqual(result.Reasons, want) {
		t.Fatalf("Reasons = %v, want %v", result.Reasons, want)
	}
}

func TestInMemoryConvergenceCheck_InProgressRunBreaksGreenStreak(t *testing.T) {
	checker := NewInMemoryConvergenceCheck()
	result, err := checker.Check(context.Background(), ConvergenceInput{
		RecentCIRuns: []CIRun{
			{Status: CIRunStatusInProgress, Conclusion: CIRunConclusionNone},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
		},
		UnconsumedHighMedium:    0,
		FitnessBaselineCaptured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CIGreenStreak != 0 {
		t.Fatalf("CIGreenStreak = %d, want 0", result.CIGreenStreak)
	}
	if result.Converged {
		t.Fatal("Converged = true, want false")
	}
}

func TestInMemoryConvergenceCheck_CountsOnlyLeadingGreenStreak(t *testing.T) {
	checker := NewInMemoryConvergenceCheck()
	result, err := checker.Check(context.Background(), ConvergenceInput{
		RecentCIRuns: []CIRun{
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSkipped},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
		},
		UnconsumedHighMedium:    0,
		FitnessBaselineCaptured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CIGreenStreak != 2 {
		t.Fatalf("CIGreenStreak = %d, want 2", result.CIGreenStreak)
	}
}

func TestInMemoryConvergenceCheck_UsesCustomCriteria(t *testing.T) {
	checker := NewInMemoryConvergenceCheck(ConvergenceCriteria{
		MinGreenCIStreak:        2,
		MaxUnconsumedHighMedium: 0,
		RequireFitnessBaseline:  false,
	})
	result, err := checker.Check(context.Background(), ConvergenceInput{
		RecentCIRuns: []CIRun{
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
			{Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
		},
		UnconsumedHighMedium:    0,
		FitnessBaselineCaptured: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Converged {
		t.Fatalf("Converged = false, want true; reasons=%v", result.Reasons)
	}
}

func TestInMemoryConvergenceCheck_ReasonsAreDefensiveCopy(t *testing.T) {
	checker := NewInMemoryConvergenceCheck()
	result, err := checker.Check(context.Background(), ConvergenceInput{})
	if err != nil {
		t.Fatal(err)
	}
	result.Reasons[0] = "mutated"

	next, err := checker.Check(context.Background(), ConvergenceInput{})
	if err != nil {
		t.Fatal(err)
	}
	if next.Reasons[0] != "ci-green-streak-below-threshold" {
		t.Fatalf("next Reasons[0] = %q, want ci-green-streak-below-threshold", next.Reasons[0])
	}
}

func TestInMemoryConvergenceCheck_HonorsContextCancellation(t *testing.T) {
	checker := NewInMemoryConvergenceCheck()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := checker.Check(ctx, ConvergenceInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Check error = %v, want context.Canceled", err)
	}
}
