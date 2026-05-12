// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import "context"

// CIRunStatus is the lifecycle state of a CI run for a single commit.
// Values are deliberately small and stable: `queued` (workflow created
// but not started), `in_progress` (any job still running), `completed`
// (all jobs reached a terminal state). The completed-state conclusion
// lives in CIRunConclusion.
type CIRunStatus string

const (
	CIRunStatusQueued     CIRunStatus = "queued"
	CIRunStatusInProgress CIRunStatus = "in_progress"
	CIRunStatusCompleted  CIRunStatus = "completed"
)

// CIRunConclusion is the terminal outcome when CIRunStatus is
// "completed". Values map onto GitHub Actions' run-conclusion strings
// so adapters can pass them through transparently.
type CIRunConclusion string

const (
	CIRunConclusionSuccess   CIRunConclusion = "success"
	CIRunConclusionFailure   CIRunConclusion = "failure"
	CIRunConclusionCancelled CIRunConclusion = "cancelled"
	CIRunConclusionSkipped   CIRunConclusion = "skipped"
	CIRunConclusionNone      CIRunConclusion = "" // empty = not yet known (queued/in_progress)
)

// CIRun is one CI workflow run for a commit. Sha is the canonical SHA
// the run was triggered against; Workflow names the workflow file
// (e.g. "validate.yml"); Status is the lifecycle state; Conclusion is
// the terminal outcome (empty when Status is queued or in_progress).
// FailedJobs is a slice of failed-job names when Conclusion is
// "failure" (empty otherwise).
type CIRun struct {
	Sha        string
	Workflow   string
	Status     CIRunStatus
	Conclusion CIRunConclusion
	FailedJobs []string
}

// CIStatusPort is the BC2 Validation read-side for CI history. Callers
// — evolve's Step 1.5 healing-first classifier, the supergate's
// "what's the last push CI?" probe, the drift-detection sweep, and
// any future PR-bound CI auditor — depend on this port so they can
// query CI verdict shape without depending on the `gh` CLI directly
// in tests.
//
// Contract:
//
//   - Latest MUST return the most recent CIRun for the given sha. When
//     no run exists, return CIRun{} with Status="" (zero value) and a
//     nil error.
//   - When sha is empty, Latest MUST return a non-nil error.
//   - Recent returns up to `limit` most-recent runs (any sha). Adapters
//     MUST respect limit; limit==0 means "all available" — adapters
//     SHOULD cap that at a reasonable max (e.g. 50).
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC2 row) for the
// canonical Validation context surface. soc-wxh5 epic tracks BC2
// port extraction. Sibling: GateRunnerPort (the execute-side).
type CIStatusPort interface {
	Latest(ctx context.Context, sha string) (CIRun, error)
	Recent(ctx context.Context, limit int) ([]CIRun, error)
}
