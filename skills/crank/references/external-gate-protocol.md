# External Gate Protocol

> Workers must use external gates, not self-assessment. From Ralph Loop pattern and 124 council FAIL analyses.

## The Rule

Workers MUST NOT declare their own work complete. Every wave completion requires an external gate — a runnable command that returns 0 (pass) or non-zero (fail), executed by the orchestrator, not the worker.

## Why This Matters

- Unit tests found zero production bugs across 14,753 sessions analyzed
- L3+ tests (integration, E2E) found all real bugs
- Zero-context smoke tests find 3–5x more issues than self-review
- Self-grading is confirmation bias — the worker who wrote the code is biased toward "looks good"

## Gate Hierarchy

| Gate Level | What It Checks | Who Runs It |
|------------|---------------|-------------|
| L0: Build | Code compiles | CI / orchestrator |
| L1: Unit | Function-level correctness | CI / orchestrator |
| L2: Integration | Component interaction | CI / orchestrator |
| L3: E2E | Full workflow | CI / orchestrator |
| L4: Smoke | Production-like behavior | Fresh-context validator |

**Minimum for wave completion:** L0 + L1 + L2 must pass. L3 recommended.

## Evidence-backed back-pressure

When a gate fails, preserve the result and return it at the wave boundary.
Crank does not increment a local failure allowance or dispatch another
approach. The orchestrator decides whether the evidence calls for `REPAIR`,
`REPLAN`, `HOLD`, or `ANDON`. A safe commit from a prior passing gate remains a
rollback point, not permission to retry.

## Wave Validation Sequence

```
Worker completes issue
  → Orchestrator runs gate command (NOT worker self-report)
  → Gate passes? → Mark issue DONE, proceed to next
  → Gate fails? → Preserve command, exit status, output, and attempted approach
    → Return BLOCKED/PARTIAL evidence to RPI
    → RPI records the next move in one evidence-bound disposition
```

## Anti-Patterns

- ❌ Worker runs tests and reports "all pass" → orchestrator trusts
- ❌ Acceptance criteria: "verify it works" (no runnable command)
- ❌ Wave advances without any gate execution
- ✅ Orchestrator runs `make test` after worker signals completion
- ✅ Each issue has a specific gate command in its acceptance criteria
- ✅ Failed gates produce evidence visible to the orchestrator
