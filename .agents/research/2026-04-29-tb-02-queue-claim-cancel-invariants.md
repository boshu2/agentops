---
id: research-2026-04-29-tb-02-queue-claim-cancel-invariants
type: research
date: 2026-04-29
topic: TB-02 queue claim/cancel invariants
---

# Research: TB-02 Queue Claim/Cancel Invariants

**Backend:** inline fallback. Native `Skill()` delegation is not exposed in this
Codex session; `spawn_agent` was not used because the user did not explicitly
request parallel subagents.

**Scope:** `cli/internal/daemon` queue state, ledger replay, terminal-state
precedence, and the existing tracer-bullet plan/pre-mortem artifacts.

## Summary

TB-02 is a tightly scoped queue-state change. `Queue.ClaimNext` currently
claims the first claimable job without a type predicate, `CancelJob` does not
exist, terminal replay already uses first-terminal-wins semantics, and expired
running jobs are projected as retry-waiting/reclaimable. The main planning
constraint is to make cancel-vs-terminal precedence explicit: append order wins,
and a cancel that loses to an existing terminal state must report that outcome.

## Key Files

| File | Purpose |
|------|---------|
| `cli/internal/daemon/jobs.go` | Queue API, claim, heartbeat, completion/failure, replay, lease expiry. |
| `cli/internal/daemon/jobs_test.go` | Existing queue lifecycle tests and natural home for TB-02 tests. |
| `cli/internal/daemon/types.go` | Job statuses, event types, transition matrix, terminal helper. |
| `cli/internal/daemon/types_test.go` | Existing `TestJobStatusTransitionMatrix`. |
| `cli/internal/daemon/store.go` | Append-only ledger write/replay and event ID dedupe. |
| `cli/internal/daemon/projections.go` | Projection replay also short-circuits terminal jobs. |
| `.agents/plans/2026-04-29-agentops-daemon-tracer-bullets.md` | Source tracer-bullet acceptance and validation command. |
| `.agents/council/2026-04-29-validate-tracer-bullets-judge-risk-auditor.md` | Adds append-order cancel-vs-terminal risk. |

## Findings

- `Queue.ClaimNext` reads a snapshot and claims the first `q.isClaimable(job)`
  job, with no predicate for supported job types. This is the exact unsupported
  queue starvation problem TB-02 targets.
  Evidence: `cli/internal/daemon/jobs.go:184`.

- `Queue.ClaimJob` already handles terminal, fresh-running, and unclaimable
  states, then delegates to `claimJobState`.
  Evidence: `cli/internal/daemon/jobs.go:198`, `cli/internal/daemon/jobs.go:327`.

- Lease reclaim support already exists. Expired running jobs are marked
  `retry_waiting` in snapshots and `claimJobState` appends `job.lease_expired`
  before reclaiming.
  Evidence: `cli/internal/daemon/jobs.go:509`, `cli/internal/daemon/jobs.go:334`.

- Terminal replay is first-terminal-wins. `applyQueueEvent` returns immediately
  when the projected job is already completed, failed, or cancelled.
  Evidence: `cli/internal/daemon/jobs.go:520`.

- Projection replay mirrors the same terminal short-circuit, so TB-02 should
  test both queue snapshot semantics and status projection assumptions through
  the existing queue replay path.
  Evidence: `cli/internal/daemon/projections.go:227`.

- `EventJobCancelled` and `JobResultCancelled` already exist, so TB-02 does not
  need new event or status enums.
  Evidence: `cli/internal/daemon/types.go:26`, `cli/internal/daemon/types.go:48`.

- Existing tests cover submit/claim/heartbeat/complete, duplicate terminal
  completion idempotency through the API, lease expiry reclaim, retry cap, and
  append failpoints. TB-02 should extend these instead of adding a new test
  package.
  Evidence: `cli/internal/daemon/jobs_test.go:9`, `cli/internal/daemon/jobs_test.go:82`, `cli/internal/daemon/jobs_test.go:122`, `cli/internal/daemon/jobs_test.go:152`.

- The risk-auditor packet adds a product-facing constraint: cancel-vs-terminal
  races are decided by ledger append order, not `OccurredAt`; cancellation
  should return a typed outcome when it is ignored after a terminal event.
  Evidence: `.agents/council/2026-04-29-validate-tracer-bullets-judge-risk-auditor.md:36`.

## Test Levels

Required: L1 and L2.

Rationale: TB-02 is internal Go logic but touches append-only file I/O and
ledger replay. L1 covers direct queue APIs; L2 covers serialized ledger replay,
lease expiry, and duplicate terminal event behavior.

## Recommendations

1. Add `ClaimNextMatching` and have `ClaimNext` delegate to it with an
   always-true matcher.
2. Add `CancelJobInput` plus a result that exposes
   `cancelled`, `already_terminal_completed`, `already_terminal_failed`, or
   `already_terminal_cancelled`.
3. Keep cancellation ledger-only and append `job.cancelled` only for
   non-terminal queued/running/retry-waiting jobs.
4. Add the exact tracer tests, plus make duplicate terminal replay assert the
   append-order precedence rule explicitly.
