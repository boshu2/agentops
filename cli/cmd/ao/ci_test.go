// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestCIStatus_LatestEmitsOneRun(t *testing.T) {
	stub := func(_ context.Context, _ ciStatusOptions) ([]ports.CIRun, error) {
		return []ports.CIRun{
			{Sha: "abc", Workflow: "validate.yml", Status: ports.CIRunStatusCompleted, Conclusion: ports.CIRunConclusionSuccess},
		}, nil
	}
	var buf bytes.Buffer
	err := ciStatusRun(context.Background(), ciStatusOptions{
		sha:      "abc",
		writer:   &buf,
		statusFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("len = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], `"Sha":"abc"`) {
		t.Fatalf("missing Sha in output: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"Conclusion":"success"`) {
		t.Fatalf("missing Conclusion: %s", lines[0])
	}
}

func TestCIStatus_LatestEmptyEmitsZeroLines(t *testing.T) {
	stub := func(_ context.Context, _ ciStatusOptions) ([]ports.CIRun, error) {
		return []ports.CIRun{}, nil
	}
	var buf bytes.Buffer
	err := ciStatusRun(context.Background(), ciStatusOptions{
		sha:      "deadbeef",
		writer:   &buf,
		statusFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty result should emit 0 bytes, got %q", buf.String())
	}
}

func TestCIStatus_RecentEmitsMultipleRuns(t *testing.T) {
	stub := func(_ context.Context, _ ciStatusOptions) ([]ports.CIRun, error) {
		return []ports.CIRun{
			{Sha: "a", Workflow: "validate.yml"},
			{Sha: "b", Workflow: "validate.yml"},
			{Sha: "c", Workflow: "release.yml"},
		}, nil
	}
	var buf bytes.Buffer
	err := ciStatusRun(context.Background(), ciStatusOptions{
		limit:    3,
		writer:   &buf,
		statusFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("len = %d, want 3", len(lines))
	}
}

func TestCIStatus_StubErrorPropagates(t *testing.T) {
	stub := func(_ context.Context, _ ciStatusOptions) ([]ports.CIRun, error) {
		return nil, errors.New("gh not authenticated")
	}
	err := ciStatusRun(context.Background(), ciStatusOptions{
		sha:      "abc",
		statusFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ci status:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}
