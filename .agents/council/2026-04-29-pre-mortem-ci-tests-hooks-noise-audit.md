---
id: pre-mortem-2026-04-29-ci-tests-hooks-noise-audit
type: pre-mortem
date: 2026-04-29
plan: .agents/plans/2026-04-29-ci-tests-hooks-noise-audit.md
epic: agentops-dtg
mode: quick
---
# Pre-Mortem: CI Tests and Hooks Noise Audit

## Council Verdict: WARN

The plan is ready to execute after the hardening already copied into the plan
and issue notes. The WARN is about scope-control risk, not a blocking design
flaw.

## Verdict Table

| Area | Verdict | Notes |
|------|---------|-------|
| Mechanical verification | WARN | Every issue has runnable gates, but the new inventory could become another stale table unless its consuming validator lands in the same slice. |
| Propagation surface | PASS | Workflow, AGENTS, docs, shell validators, BATS, and hook manifests are enumerated. |
| Scope control | WARN | Hook preflight expansion can accidentally turn existing utility scripts into blockers without a report-first or allowlist phase. |
| Test pyramid | PASS | L0 shell checks, L1 BATS fixtures, L2 parity gates, and final L3 local release validation are specified. |
| Four-surface closure | PASS | Code, tests, docs, and proof commands are tracked in separate issues. |

## Key Concerns

1. **Inventory can become new noise.** A machine-readable inventory is useful only
   if a validator consumes it immediately. The plan now requires `agentops-dtg.2`
   to land the inventory with a consuming check and failing drift fixture.
2. **Hook preflight expansion can over-block.** There are 14 unregistered
   non-JSON hook utility scripts today. The plan now requires report-first
   behavior or an explicit allowlist so non-JSON utilities do not become
   accidental blockers.
3. **Final local release validation is too broad for early waves.** The plan now
   keeps `ci-local-release.sh --fast` as final-wave proof and uses targeted gates
   for earlier issues.

## Required Fix Propagation

Applied to `.agents/plans/2026-04-29-ci-tests-hooks-noise-audit.md`:

- Added an explicit same-slice validator requirement for the inventory issue.
- Added hook preflight report-first/allowlist guidance.
- Added a note that full local release fast validation belongs to the final wave.

Applied to beads:

- `agentops-dtg.2` note: inventory must land with a consuming validator and
  failing drift fixture.
- `agentops-dtg.3` note: hook preflight expansion must use report-first mode or
  an explicit allowlist.
- `agentops-dtg.5` note: `ci-local-release.sh --fast` is final validation, not
  an early-wave blocker.

## Recommendation

Proceed with `agentops-dtg.1` first. It has the clearest bug: a workflow job is
outside the documented CI policy surface.
