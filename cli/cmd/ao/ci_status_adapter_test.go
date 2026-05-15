// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// Sibling pattern: cycle 116 claim_evidence_binder_adapter_test.go.
// Substitutes the runGH func field instead of writing a temp gh
// stub binary — cleaner than the subprocess pattern in cycle 115.

func TestProductionCIStatus_LatestParsesJSON(t *testing.T) {
	c := newProductionCIStatus()
	c.runGH = func(_ context.Context, args []string) ([]byte, error) {
		return []byte(`[{"headSha":"abc","workflowName":"validate.yml","status":"completed","conclusion":"success"}]`), nil
	}
	run, err := c.Latest(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if run.Sha != "abc" {
		t.Fatalf("Sha = %q", run.Sha)
	}
	if run.Workflow != "validate.yml" {
		t.Fatalf("Workflow = %q", run.Workflow)
	}
	if run.Status != ports.CIRunStatusCompleted {
		t.Fatalf("Status = %s", run.Status)
	}
	if run.Conclusion != ports.CIRunConclusionSuccess {
		t.Fatalf("Conclusion = %s", run.Conclusion)
	}
}

func TestProductionCIStatus_LatestEmptyArrayReturnsZero(t *testing.T) {
	c := newProductionCIStatus()
	c.runGH = func(_ context.Context, _ []string) ([]byte, error) {
		return []byte(`[]`), nil
	}
	run, err := c.Latest(context.Background(), "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if run.Sha != "" || run.Status != "" {
		t.Fatalf("missing run should be zero value, got %+v", run)
	}
}

func TestProductionCIStatus_LatestPassesShaFlag(t *testing.T) {
	c := newProductionCIStatus()
	var capturedArgs []string
	c.runGH = func(_ context.Context, args []string) ([]byte, error) {
		capturedArgs = args
		return []byte(`[]`), nil
	}
	_, _ = c.Latest(context.Background(), "myssha123")
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "--commit myssha123") {
		t.Fatalf("missing --commit flag: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--limit 1") {
		t.Fatalf("missing --limit 1: %v", capturedArgs)
	}
}

func TestProductionCIStatus_LatestEmptyShaErrors(t *testing.T) {
	c := newProductionCIStatus()
	_, err := c.Latest(context.Background(), "")
	if err == nil {
		t.Fatal("expected error on empty sha, got nil")
	}
}

func TestProductionCIStatus_RecentLimitedToCallerLimit(t *testing.T) {
	c := newProductionCIStatus()
	var capturedArgs []string
	c.runGH = func(_ context.Context, args []string) ([]byte, error) {
		capturedArgs = args
		return []byte(`[]`), nil
	}
	_, _ = c.Recent(context.Background(), 5)
	if !strings.Contains(strings.Join(capturedArgs, " "), "--limit 5") {
		t.Fatalf("--limit 5 not passed: %v", capturedArgs)
	}
}

func TestProductionCIStatus_RecentZeroLimitCappedToMax(t *testing.T) {
	c := newProductionCIStatus()
	var capturedArgs []string
	c.runGH = func(_ context.Context, args []string) ([]byte, error) {
		capturedArgs = args
		return []byte(`[]`), nil
	}
	_, _ = c.Recent(context.Background(), 0)
	if !strings.Contains(strings.Join(capturedArgs, " "), "--limit 50") {
		t.Fatalf("limit=0 should cap to 50, got: %v", capturedArgs)
	}
}

func TestProductionCIStatus_RecentReturnsAllRuns(t *testing.T) {
	c := newProductionCIStatus()
	c.runGH = func(_ context.Context, _ []string) ([]byte, error) {
		return []byte(`[
			{"headSha":"a","workflowName":"validate.yml","status":"completed","conclusion":"success"},
			{"headSha":"b","workflowName":"validate.yml","status":"in_progress","conclusion":""},
			{"headSha":"c","workflowName":"release.yml","status":"completed","conclusion":"failure"}
		]`), nil
	}
	runs, err := c.Recent(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("len = %d, want 3", len(runs))
	}
	if runs[1].Status != ports.CIRunStatusInProgress || runs[1].Conclusion != ports.CIRunConclusionNone {
		t.Fatalf("in_progress run wrong: %+v", runs[1])
	}
	if runs[2].Conclusion != ports.CIRunConclusionFailure {
		t.Fatalf("failure conclusion wrong: %+v", runs[2])
	}
}

func TestProductionCIStatus_GHFailurePropagates(t *testing.T) {
	c := newProductionCIStatus()
	c.runGH = func(_ context.Context, _ []string) ([]byte, error) {
		return nil, errors.New("gh: not authenticated")
	}
	_, err := c.Latest(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "ci_status: gh:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestProductionCIStatus_MalformedJSONErrors(t *testing.T) {
	c := newProductionCIStatus()
	c.runGH = func(_ context.Context, _ []string) ([]byte, error) {
		return []byte(`{this is not array`), nil
	}
	_, err := c.Latest(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "ci_status: parse:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestProductionCIStatus_HonorsContextCancellation(t *testing.T) {
	c := newProductionCIStatus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Latest(ctx, "abc"); err == nil {
		t.Fatal("Latest: expected cancellation, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Latest error = %v, want context.Canceled", err)
	}
	if _, err := c.Recent(ctx, 5); err == nil {
		t.Fatal("Recent: expected cancellation, got nil")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("Recent error = %v, want context.Canceled", err)
	}
}
