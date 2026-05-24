# Phase Budgets Reference

Time budgets for each RPI phase, scaled by complexity level. Prevents sessions from stalling in research or planning without producing actionable artifacts.

## Budget Tables

### Fast Complexity

| Phase | Budget | Rationale |
|-------|--------|-----------|
| Research | 3 min | Quick keyword search + 2-3 file reads |
| Plan | 2 min | Single epic, 1-3 issues, no deep decomposition |
| Pre-mortem | 1 min | Inline check, no council spawn |
| Implementation | unlimited | Crank wave limits apply (MAX_EPIC_WAVES=50) |
| Validation | skipped | Fast complexity skips Phase 3 |

### Standard Complexity

| Phase | Budget | Rationale |
|-------|--------|-----------|
| Research | 5 min | Explore agent + knowledge lookup + file analysis |
| Plan | 5 min | Epic decomposition, 3-6 issues with dependencies |
| Pre-mortem | 3 min | Quick council (2 judges) |
| Implementation | unlimited | Crank wave limits apply |
| Validation | 5 min | Quick vibe + post-mortem |

### Full Complexity

| Phase | Budget | Rationale |
|-------|--------|-----------|
| Research | 10 min | Deep exploration, multiple Explore agents, cross-file analysis |
| Plan | 10 min | Complex decomposition, 7+ issues, multi-wave dependency graph |
| Pre-mortem | 5 min | Full council (3+ judges), deep risk analysis |
| Implementation | unlimited | Crank wave limits apply |
| Validation | 10 min | Full vibe + comprehensive post-mortem |

## Why Implementation Is Always Unlimited

Implementation (crank) has its own backpressure mechanisms:
- MAX_EPIC_WAVES = 50 (hard limit)
- Per-wave acceptance checks (Step 5.5)
- Per-task validation contracts
- Retry limits on blocked/partial status

Adding a time budget on top of these would create competing constraints that are hard to reason about. The wave limit is a better control than wall-clock time for implementation work.

## Worked Examples

### Example 1: Research Phase Expires

```
RPI mode: rpi-phased (complexity: standard)
Phase: research (budget: 300s)

[0:00]  /research "add user authentication"
[1:30]  Explore agent returns relevant files
[3:00]  Knowledge lookup finds 2 prior learnings
[4:45]  Reading auth middleware patterns...
[5:00]  BUDGET EXPIRED — research phase time-boxed at 300s

Writing [TIME-BOXED] marker to .agents/rpi/phase-1-summary-*.md
Auto-transitioning to plan phase with partial research artifacts.

Note: Research produced file list + 2 learnings. Plan phase proceeds
with available context. This is NOT a retry — attempt counter stays at 0.
```

### Example 2: Budget Expiry vs Retry Gate

```
Phase: pre-mortem (budget: 180s, attempt: 1/3)

[0:00]  /pre-mortem spawns council
[2:30]  Council returns verdict: FAIL (3 critical risks)
[2:30]  Verdict is FAIL → triggers retry gate (attempt 1/3)

[2:30]  Re-running /plan with findings context...
[4:00]  /pre-mortem attempt 2 spawns council
[5:00]  BUDGET EXPIRED at 300s (cumulative across retries)

Writing [TIME-BOXED] marker. Auto-transitioning to implementation.
Pre-mortem verdict was FAIL but budget expired — proceed with WARN.
Attempt counter: 2/3 (budget expiry does NOT count as attempt 3).
```

### Example 3: Custom Budget Override

```bash
/rpi --budget=research:180,plan:120 "quick API endpoint"

# Result: research gets 3 min, plan gets 2 min
# Other phases use complexity-derived defaults
# Implementation remains unlimited
```

## Interaction with Other Controls

| Control | Scope | Relationship to Budgets |
|---------|-------|------------------------|
| Retry gates (3 attempts) | Per-phase | Orthogonal — budget expiry is not a retry |
| `--fast-path` | All phases | Sets fast budgets regardless of classification |
| `--deep` | All phases | Sets full budgets regardless of classification |
| `--no-budget` | All phases | Disables budgets entirely |
| `--budget=<spec>` | Named phases | Overrides specific phase budgets |
| Crank wave limit | Implementation | Separate backpressure mechanism |
| Council timeout | Within phase | Council has its own per-judge timeout |
