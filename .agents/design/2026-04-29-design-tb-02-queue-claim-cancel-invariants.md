---
id: design-2026-04-29-tb-02-queue-claim-cancel-invariants
type: design
date: 2026-04-29
goal: TB-02 queue claim/cancel invariants
verdict: PASS
---

# Design: TB-02 Queue Claim/Cancel Invariants

## Goal

Prepare the next daemon tracer bullet after TB-01: add queue primitives for
claiming only supported work, ledger-only job cancellation, terminal replay
invariants, and expired-running reclaim. Daemon supervisor execution remains
out of scope until this slice passes.

## Alignment Matrix

| Dimension | Score | Rationale |
|-----------|-------|-----------|
| Gap Alignment | 3/3 | Directly advances Dream autonomy and always-on daemon execution by making the durable queue safe to supervise. |
| Persona Fit | 3/3 | Serves the agent orchestrator and quality-first maintainer personas by preventing unsupported job starvation and terminal-state corruption. |
| Competitive Diff | 2/3 | Strengthens the local operational layer and ledger-first worker story that differentiates AgentOps. |
| Precedent | 3/3 | Existing `Queue`, ledger, projection, transition, and lease tests provide direct implementation precedent. |
| Scope Fit | 3/3 | Surgical single-package slice: `cli/internal/daemon/jobs.go` and `jobs_test.go`; no supervisor or HTTP UX. |

Average: 2.8/3.0

## Verdict

DESIGN VERDICT: PASS

TB-02 is product-aligned and appropriately narrow. Proceed to research, plan,
and pre-mortem for this single slice.
