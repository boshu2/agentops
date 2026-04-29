---
id: pre-mortem-2026-04-29-tb-02-queue-claim-cancel-invariants
type: pre-mortem
date: 2026-04-29
source: .agents/plans/2026-04-29-tb-02-queue-claim-cancel-invariants.md
scope_mode: hold
verdict: WARN
---

# Pre-Mortem: TB-02 Queue Claim/Cancel Invariants

## Council Verdict: WARN

The plan is implementable and correctly scoped, but it carries one important
semantic risk: cancel-vs-terminal races are decided by append order. That rule
is acceptable for TB-02 if it is tested directly and surfaced through a typed
cancel outcome instead of hidden as silent success.

| ID | Finding | Severity | Required Plan Treatment |
|----|---------|----------|-------------------------|
| pm-tb02-001 | `CancelJob` must not imply provider cooperative cancellation. | low | Keep cancellation ledger-only; provider cancellation belongs to later executors. |
| pm-tb02-002 | Cancel-vs-terminal precedence is append-order. | moderate | Add first-terminal-wins replay tests in both event orders. |
| pm-tb02-003 | Silent ignored cancels will mislead later CLI UX. | moderate | Return `CancelJobResult.Outcome` with already-terminal variants. |
| pm-tb02-004 | `ClaimNextMatching` can accidentally swallow matcher panics or nil matcher ambiguity. | low | Treat nil matcher as match-all; do not recover matcher panics. |

## Pseudocode Fixes

```go
func (q *Queue) CancelJob(input CancelJobInput, opts QueueMutationOptions) (CancelJobResult, error) {
    job, err := q.currentJob(input.JobID)
    if err != nil {
        return CancelJobResult{}, err
    }
    if isTerminalStatus(job.Status) {
        return CancelJobResult{Job: job, Outcome: cancelOutcomeForTerminal(job.Status)}, nil
    }
    event := NewLedgerEvent(... EventJobCancelled ...)
    if err := q.appendQueueEvent(event, opts); err != nil {
        return CancelJobResult{}, err
    }
    updated, err := q.currentJob(input.JobID)
    if err != nil {
        return CancelJobResult{}, err
    }
    return CancelJobResult{Job: updated, Outcome: CancelJobOutcomeCancelled}, nil
}
```

## Decision Gate

[x] PROCEED - WARN accepted with append-order and outcome-shape constraints.
[ ] ADDRESS - Not required before implementation because the plan already
    includes the required treatment.
[ ] RETHINK - Not required.
