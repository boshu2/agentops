// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestProductionConvergenceCheck_ConvergesWithDefaultCriteria(t *testing.T) {
	checker := newProductionConvergenceCheck()
	result, err := checker.Check(context.Background(), ports.ConvergenceInput{
		RecentCIRuns: []ports.CIRun{
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionFailure},
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
}

func TestProductionConvergenceCheck_ReportsEveryUnmetCriterion(t *testing.T) {
	checker := newProductionConvergenceCheck()
	result, err := checker.Check(context.Background(), ports.ConvergenceInput{
		RecentCIRuns: []ports.CIRun{
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionFailure},
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

func TestProductionConvergenceCheck_NonSuccessBreaksLeadingStreak(t *testing.T) {
	checker := newProductionConvergenceCheck()
	result, err := checker.Check(context.Background(), ports.ConvergenceInput{
		RecentCIRuns: []ports.CIRun{
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSkipped},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
		},
		UnconsumedHighMedium:    0,
		FitnessBaselineCaptured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CIGreenStreak != 1 {
		t.Fatalf("CIGreenStreak = %d, want 1", result.CIGreenStreak)
	}
	if result.Converged {
		t.Fatal("Converged = true, want false")
	}
}

func TestProductionConvergenceCheck_InProgressRunBreaksStreak(t *testing.T) {
	checker := newProductionConvergenceCheck()
	result, err := checker.Check(context.Background(), ports.ConvergenceInput{
		RecentCIRuns: []ports.CIRun{
			{Status: ports.CIRunStatusInProgress, Conclusion: ports.CIRunConclusionNone},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
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
}

func TestProductionConvergenceCheck_UsesCustomCriteria(t *testing.T) {
	checker := newProductionConvergenceCheck(ports.ConvergenceCriteria{
		MinGreenCIStreak:        2,
		MaxUnconsumedHighMedium: 0,
		RequireFitnessBaseline:  false,
	})
	result, err := checker.Check(context.Background(), ports.ConvergenceInput{
		RecentCIRuns: []ports.CIRun{
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
			{Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
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

func TestProductionConvergenceCheck_ReasonsAreDefensiveCopy(t *testing.T) {
	checker := newProductionConvergenceCheck()
	result, err := checker.Check(context.Background(), ports.ConvergenceInput{})
	if err != nil {
		t.Fatal(err)
	}
	result.Reasons[0] = "mutated"

	next, err := checker.Check(context.Background(), ports.ConvergenceInput{})
	if err != nil {
		t.Fatal(err)
	}
	if next.Reasons[0] != "ci-green-streak-below-threshold" {
		t.Fatalf("next Reasons[0] = %q, want ci-green-streak-below-threshold", next.Reasons[0])
	}
}

func TestProductionConvergenceCheck_HonorsContextCancellation(t *testing.T) {
	checker := newProductionConvergenceCheck()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := checker.Check(ctx, ports.ConvergenceInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Check error = %v, want context.Canceled", err)
	}
}
