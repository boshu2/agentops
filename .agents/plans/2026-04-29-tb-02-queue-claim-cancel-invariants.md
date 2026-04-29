---
id: plan-2026-04-29-tb-02-queue-claim-cancel-invariants
type: plan
date: 2026-04-29
status: ready-for-implementation
epic_id: agentops-31c
issue_id: agentops-31c.1
source:
  - .agents/plans/2026-04-29-agentops-daemon-tracer-bullets.md
  - .agents/research/2026-04-29-tb-02-queue-claim-cancel-invariants.md
  - .agents/council/2026-04-29-validate-tracer-bullets-judge-risk-auditor.md
---

# Plan: TB-02 Queue Claim And Cancel Invariants

## Context

TB-01 passed. The next safe tracer bullet is TB-02 only: queue-level claim and
cancel primitives. Do not start TB-03 foreground supervisor work until this
validation command passes.

Applied findings: append-order terminal precedence from the risk-auditor packet
is now an explicit implementation constraint.

## Boundaries

In scope:

- `cli/internal/daemon/jobs.go`
- `cli/internal/daemon/jobs_test.go`
- Existing transition test in `cli/internal/daemon/types_test.go`

Out of scope:

- Supervisor loop, executors, HTTP cancel route, CLI job UX, provider
  cooperative cancellation, docs contract edits.

## Baseline Audit

| Check | Evidence |
|-------|----------|
| Claim API exists | `Queue.ClaimNext` at `jobs.go:184`, `Queue.ClaimJob` at `jobs.go:198`. |
| Matching claim missing | No `ClaimNextMatching` symbol exists in `cli/internal/daemon`. |
| Cancel API missing | No `CancelJob` or `CancelJobInput` symbol exists in `cli/internal/daemon`. |
| Cancel event exists | `EventJobCancelled` exists in `types.go:26`; no enum change needed. |
| Terminal replay exists | `applyQueueEvent` returns early for terminal statuses at `jobs.go:520`. |
| Expired running reclaim exists | `jobLeaseExpired` and lease-expired replay paths exist at `jobs.go:592` and `jobs.go:509`. |
| Tests home exists | `jobs_test.go` already covers queue lifecycle, lease expiry, retry cap, and failpoints. |

Baseline command:

```bash
cd cli
go test ./internal/daemon -run 'TestQueueSubmitClaimHeartbeatComplete|TestLeaseExpiryAllowsReclaimWithEpochAndRejectsStaleClaim|TestQueueRetryCapFailsExpiredJob|TestAckFailpointAfterAppendBeforeAckIsRecoverableByIdempotency|TestJobStatusTransitionMatrix'
```

## Implementation

### `cli/internal/daemon/jobs.go`

Add:

```go
type CancelJobOutcome string

const (
    CancelJobOutcomeCancelled                CancelJobOutcome = "cancelled"
    CancelJobOutcomeAlreadyTerminalCompleted CancelJobOutcome = "already_terminal_completed"
    CancelJobOutcomeAlreadyTerminalFailed    CancelJobOutcome = "already_terminal_failed"
    CancelJobOutcomeAlreadyTerminalCancelled CancelJobOutcome = "already_terminal_cancelled"
)

type CancelJobInput struct {
    JobID     string
    RequestID RequestID
    Actor     string
    Reason    string
}

type CancelJobResult struct {
    Job     QueueJobState   `json:"job"`
    Outcome CancelJobOutcome `json:"outcome"`
}
```

Add:

```go
func (q *Queue) ClaimNextMatching(
    actor string,
    match func(QueueJobState) bool,
    opts QueueMutationOptions,
) (QueueClaim, error)
```

Behavior:

- Treat nil `match` as match-all.
- Iterate snapshot order.
- Skip jobs that are not `q.isClaimable(job)`.
- Skip jobs where `match(job)` is false.
- Return first successful `q.claimJobState`.
- Return `ErrNoClaimableJobs` when nothing matches.

Refactor:

```go
func (q *Queue) ClaimNext(actor string, opts QueueMutationOptions) (QueueClaim, error) {
    return q.ClaimNextMatching(actor, nil, opts)
}
```

Add:

```go
func (q *Queue) CancelJob(input CancelJobInput, opts QueueMutationOptions) (CancelJobResult, error)
```

Behavior:

- Load current job by ID.
- If status is terminal, return the job and an already-terminal outcome without
  appending a new event.
- If status is queued, running, retry-waiting, or degraded, append
  `EventJobCancelled`.
- Include `reason` in payload only when non-empty.
- Include `result_status: "cancelled"` in the payload for projection parity.
- Return the post-append job with `OutcomeCancelled`.
- Do not require a live claim token. TB-02 cancellation is operator/ledger
  state, not provider cooperative cancel.

Keep append-order precedence explicit: when a completed event appears before a
cancelled event in the ledger, final status remains completed. When cancelled
appears first, final status remains cancelled.

### `cli/internal/daemon/jobs_test.go`

Add these tests:

- `TestQueue_ClaimNextMatchingSkipsUnsupported`
- `TestQueue_CancelJob`
- `TestQueue_DuplicateTerminalEventsDoNotMutateFinalState`
- `TestQueue_RestartReclaimsExpiredRunningJob`

Extend/fold existing tests only when the exact requested test name remains
present.

Test details:

- `TestQueue_ClaimNextMatchingSkipsUnsupported`: submit an unsupported-for-this
  worker job first, then a supported job second. Matcher accepts only
  `JobTypeOpenClawSnapshot` or another chosen supported type. Assert second job
  is claimed and first remains queued.
- `TestQueue_CancelJob`: cover queued cancellation and running cancellation.
  Assert exactly one `job.cancelled` event per cancel and `OutcomeCancelled`.
- `TestQueue_DuplicateTerminalEventsDoNotMutateFinalState`: construct queue
  events in append order and assert first terminal wins for completed then
  cancelled, and cancelled then completed. Assert later terminal events do not
  change `LastEventID` or final status.
- `TestQueue_RestartReclaimsExpiredRunningJob`: submit, claim, advance `Now`
  past lease expiry, construct a new queue over the same store, and assert the
  restarted queue can claim the job with incremented lease epoch.

### `cli/internal/daemon/types_test.go`

No required edit if existing `TestJobStatusTransitionMatrix` still covers the
matrix in the requested validation regex. Only extend if the implementation
needs a new allowed transition.

## File Dependency Matrix

| Task | File | Access | Notes |
|------|------|--------|-------|
| TB-02 | `cli/internal/daemon/jobs.go` | write | Add matching claim and cancel primitives. |
| TB-02 | `cli/internal/daemon/jobs_test.go` | write | Add exact acceptance tests. |
| TB-02 | `cli/internal/daemon/types.go` | read | Existing enums and transition helpers. |
| TB-02 | `cli/internal/daemon/types_test.go` | read/write-if-needed | Keep `TestJobStatusTransitionMatrix` passing. |
| TB-02 | `cli/internal/daemon/store.go` | read | Ledger append/replay semantics. |
| TB-02 | `cli/internal/daemon/projections.go` | read | Projection terminal short-circuit parity. |

No same-wave write conflicts. This plan contains one implementation task.

## Validation

Required:

```bash
cd cli
go test ./internal/daemon -run 'TestQueue_ClaimNextMatchingSkipsUnsupported|TestQueue_CancelJob|TestQueue_DuplicateTerminalEventsDoNotMutateFinalState|TestQueue_RestartReclaimsExpiredRunningJob|TestJobStatusTransitionMatrix'
```

Recommended after the required gate:

```bash
cd cli
go test ./internal/daemon
```

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001 Mechanical enforcement | PASS | Exact Go test names and package command are listed. |
| PR-002 External validation | N/A | Internal queue logic only; no external service. |
| PR-003 Feedback loops | PASS | Cancel outcome exposes ignored-terminal cases for later CLI UX. |
| PR-004 Separation | PASS | Queue primitive only; supervisor/provider cancel deferred. |
| PR-005 Process gates | PASS | TB-03 blocked until required TB-02 gate passes. |
| PR-006 Cross-layer consistency | PASS | Queue and projection both preserve first-terminal-wins semantics. |
| PR-007 Phased rollout | PASS | Single tracer bullet with narrow file ownership. |

## Execution Order

Wave 1:

1. `agentops-31c.1` - TB-02 Queue claim and cancel invariants

Complexity: fast.
