// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

// ProductionCIStatus satisfies CIStatusPort by shelling out to
// `gh run list --json …` and parsing the JSON array into CIRun
// records. Test-friendly: the gh-invocation is held behind a func
// field so tests substitute a stub that returns canned JSON without
// requiring gh on the test runner.
//
// Default cap on Recent: 50 (port contract suggestion). Caller may
// pass any positive limit; non-positive limit collapses to the cap.
type ProductionCIStatus struct {
	// runGH is the subprocess hook. Production runs `gh` via
	// exec.CommandContext; tests substitute a stub that returns
	// canned bytes. Returning ([]byte, error) keeps the surface
	// minimal — non-zero exit returns the error so the adapter
	// surfaces it as a fmt.Errorf("ci_status: gh: %w", …) failure.
	runGH func(ctx context.Context, args []string) ([]byte, error)
}

func NewProductionCIStatus() *ProductionCIStatus {
	return &ProductionCIStatus{runGH: defaultRunGH}
}

// defaultRunGH is the real gh-invocation. exposed for testing as a
// pluggable field on the struct.
func defaultRunGH(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("gh %v: %w (stderr: %s)", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// ciStatusMaxRecent is the cap when limit <= 0.
const ciStatusMaxRecent = 50

// ghRunRecord matches the fields the adapter reads from
// `gh run list --json headSha,workflowName,status,conclusion`.
type ghRunRecord struct {
	HeadSha      string `json:"headSha"`
	WorkflowName string `json:"workflowName"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
}

// Latest returns the most recent CIRun for the given sha. Missing
// sha → non-nil error; missing run for that sha → zero-value CIRun
// + nil error (per port contract).
func (c *ProductionCIStatus) Latest(ctx context.Context, sha string) (CIRun, error) {
	if err := ctx.Err(); err != nil {
		return CIRun{}, err
	}
	if sha == "" {
		return CIRun{}, errors.New("ProductionCIStatus: sha required")
	}
	runs, err := c.fetch(ctx, []string{
		"run", "list",
		"--commit", sha,
		"--limit", "1",
		"--json", "headSha,workflowName,status,conclusion",
	})
	if err != nil {
		return CIRun{}, err
	}
	if len(runs) == 0 {
		return CIRun{}, nil
	}
	return runs[0], nil
}

// Recent returns up to `limit` most-recent runs.
func (c *ProductionCIStatus) Recent(ctx context.Context, limit int) ([]CIRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pageSize := limit
	if pageSize <= 0 {
		pageSize = ciStatusMaxRecent
	}
	return c.fetch(ctx, []string{
		"run", "list",
		"--limit", fmt.Sprintf("%d", pageSize),
		"--json", "headSha,workflowName,status,conclusion",
	})
}

// fetch invokes runGH and decodes the JSON array into CIRun records.
// FailedJobs is intentionally left empty — the cheap list endpoint
// doesn't include per-job names; a richer adapter would do a follow-up
// `gh run view --json jobs` per failing run.
func (c *ProductionCIStatus) fetch(ctx context.Context, args []string) ([]CIRun, error) {
	raw, err := c.runGH(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("ci_status: gh: %w", err)
	}
	var recs []ghRunRecord
	if err := json.Unmarshal(raw, &recs); err != nil {
		return nil, fmt.Errorf("ci_status: parse: %w", err)
	}
	out := make([]CIRun, 0, len(recs))
	for _, r := range recs {
		out = append(out, CIRun{
			Sha:        r.HeadSha,
			Workflow:   r.WorkflowName,
			Status:     CIRunStatus(r.Status),
			Conclusion: CIRunConclusion(r.Conclusion),
		})
	}
	return out, nil
}

// Compile-time assertion: ProductionCIStatus satisfies the port.
var _ CIStatusPort = (*ProductionCIStatus)(nil)
