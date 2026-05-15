// practices: [hexagonal-architecture, tdd]
package ports

import (
	"context"
	"errors"
	"testing"
)

// Sibling pattern: inmemory_gate_runner_test.go (cycle 99). Same
// shape — L1-style behavior + port-contract assertions.

func TestInMemoryCIStatus_LatestReturnsMostRecentForSha(t *testing.T) {
	runs := []CIRun{
		{Sha: "aaaa1111", Workflow: "validate.yml", Status: CIRunStatusCompleted, Conclusion: CIRunConclusionFailure},
		{Sha: "aaaa1111", Workflow: "validate.yml", Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
		{Sha: "bbbb2222", Workflow: "validate.yml", Status: CIRunStatusInProgress},
	}
	a := NewInMemoryCIStatus(runs)

	// aaaa1111 has two runs; Latest should return the most-recent (success)
	v, err := a.Latest(context.Background(), "aaaa1111")
	if err != nil {
		t.Fatal(err)
	}
	if v.Conclusion != CIRunConclusionSuccess {
		t.Fatalf("Conclusion = %q, want success", v.Conclusion)
	}

	// bbbb2222 is in_progress, no conclusion yet
	v, err = a.Latest(context.Background(), "bbbb2222")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != CIRunStatusInProgress {
		t.Fatalf("Status = %q, want in_progress", v.Status)
	}
	if v.Conclusion != CIRunConclusionNone {
		t.Fatalf("Conclusion = %q, want empty (in_progress)", v.Conclusion)
	}
}

func TestInMemoryCIStatus_LatestUnknownShaReturnsZeroValue(t *testing.T) {
	a := NewInMemoryCIStatus([]CIRun{{Sha: "exists", Status: CIRunStatusCompleted}})
	v, err := a.Latest(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if v.Sha != "" {
		t.Fatalf("Sha = %q, want empty (zero-value)", v.Sha)
	}
	if v.Status != "" {
		t.Fatalf("Status = %q, want empty (zero-value)", v.Status)
	}
}

func TestInMemoryCIStatus_LatestEmptyShaErrors(t *testing.T) {
	a := NewInMemoryCIStatus(nil)
	_, err := a.Latest(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty sha, got nil")
	}
}

func TestInMemoryCIStatus_RecentReturnsMostRecentFirst(t *testing.T) {
	runs := []CIRun{
		{Sha: "older", Status: CIRunStatusCompleted, Conclusion: CIRunConclusionSuccess},
		{Sha: "middle", Status: CIRunStatusCompleted, Conclusion: CIRunConclusionFailure},
		{Sha: "newest", Status: CIRunStatusInProgress},
	}
	a := NewInMemoryCIStatus(runs)

	got, err := a.Recent(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Sha != "newest" {
		t.Fatalf("got[0].Sha = %q, want newest", got[0].Sha)
	}
	if got[2].Sha != "older" {
		t.Fatalf("got[2].Sha = %q, want older", got[2].Sha)
	}
}

func TestInMemoryCIStatus_RecentRespectsLimit(t *testing.T) {
	runs := []CIRun{
		{Sha: "a"}, {Sha: "b"}, {Sha: "c"}, {Sha: "d"}, {Sha: "e"},
	}
	a := NewInMemoryCIStatus(runs)
	got, err := a.Recent(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Sha != "e" || got[1].Sha != "d" {
		t.Fatalf("Recent[0..1] = %q,%q, want e,d", got[0].Sha, got[1].Sha)
	}
}

func TestInMemoryCIStatus_RecentLimitLargerThanRunsReturnsAll(t *testing.T) {
	runs := []CIRun{{Sha: "x"}, {Sha: "y"}}
	a := NewInMemoryCIStatus(runs)
	got, err := a.Recent(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestInMemoryCIStatus_NilRunsIsSafe(t *testing.T) {
	a := NewInMemoryCIStatus(nil)
	v, err := a.Latest(context.Background(), "any")
	if err != nil {
		t.Fatal(err)
	}
	if v.Sha != "" {
		t.Fatalf("Sha = %q, want empty", v.Sha)
	}
	got, err := a.Recent(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Recent len = %d, want 0", len(got))
	}
}

func TestInMemoryCIStatus_HonorsContextCancellation(t *testing.T) {
	a := NewInMemoryCIStatus(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Latest(ctx, "x")
	if err == nil {
		t.Fatal("expected cancellation error from Latest, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Latest error = %v, want context.Canceled", err)
	}
	_, err = a.Recent(ctx, 5)
	if err == nil {
		t.Fatal("expected cancellation error from Recent, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Recent error = %v, want context.Canceled", err)
	}
}

func TestInMemoryCIStatus_FailedJobsRoundTrip(t *testing.T) {
	a := NewInMemoryCIStatus([]CIRun{{
		Sha:        "abc123",
		Status:     CIRunStatusCompleted,
		Conclusion: CIRunConclusionFailure,
		FailedJobs: []string{"bats-tests", "registry-check"},
	}})
	v, err := a.Latest(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.FailedJobs) != 2 {
		t.Fatalf("FailedJobs len = %d, want 2", len(v.FailedJobs))
	}
	if v.FailedJobs[0] != "bats-tests" {
		t.Fatalf("FailedJobs[0] = %q, want bats-tests", v.FailedJobs[0])
	}
}
