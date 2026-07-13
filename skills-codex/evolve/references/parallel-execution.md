# Parallel Goal Execution

## Architecture

When `--parallel` is enabled, `/evolve` uses `/swarm` to execute multiple independent
goal improvements concurrently instead of fixing one goal per cycle.

```
/evolve --parallel (Fitness Loop)
  │
  ├─ Step 2: Measure ALL goals
  │
  ├─ Step 3: Select top N independent failing goals (max_parallel, default 3)
  │  └─ select_parallel_goals: heuristic independence via check-script overlap
  │
  ├─ Step 4: Parallel execution via /swarm
  │  ├─ TaskCreate for each selected goal
  │  ├─ Artifact isolation: .agents/evolve/parallel-rpi/{goal.id}/
  │  ├─ Git isolation: /swarm --worktrees (each worker in /tmp/evolve-{goal.id})
  │  └─ /swarm spawns N fresh-context workers, each runs full /rpi cycle:
  │     └─ research → plan → pre-mortem → crank → vibe → post-mortem
  │
  ├─ Step 5: Single regression gate (re-measure ALL goals after wave)
  │  ├─ If ANY goal regressed → revert ENTIRE parallel wave
  │  └─ If clean → log cycle with goal_ids array
  │
  └─ Step 6-7: Log, loop (same as sequential)
```

## The Fractal Pattern

Swarm is the universal coordination primitive at every level:

```
LEVEL 0: /evolve --parallel
  └─ /swarm (parallel goal improvements)     ← NEW: swarm at evolve level
     └─ LEVEL 1: /rpi (per-goal lifecycle)
        └─ research → plan → crank → vibe → post-mortem
           └─ LEVEL 2: /crank (epic execution)
              └─ /swarm (parallel issue implementation)  ← existing: swarm at crank level
                 └─ LEVEL 3: workers (atomic tasks)
```

Each level creates fresh context for the next (Ralph Wiggum pattern).
The pattern is always: **one leader + N fresh-context workers + validation + cleanup**.

## Goal Independence Detection

`select_parallel_goals` uses a heuristic check:

1. Start with highest-weight failing goal
2. For each remaining eligible goal (weight-sorted):
   - Compare check commands for shared scripts/paths
   - If independent: add to selection (up to max_parallel)
   - If overlapping: skip (handled in next cycle)

**This is a heuristic, not a guarantee.** Goals don't declare which files their
improvements will modify — only which scripts verify them. Two goals with different
check scripts may still modify overlapping files.

**The regression gate (Step 5) is the real safety net.** If parallel goals conflict,
the regression check detects it and reverts the entire wave. This makes false
negatives in independence detection safe (they just cost one wasted cycle).

## Artifact Isolation

Each parallel /rpi worker needs isolated artifact directories to prevent collision:

| Directory | Purpose | Isolation |
|-----------|---------|-----------|
| `.agents/evolve/parallel-rpi/{goal.id}/` | /rpi phase summaries, next-work | Per-goal subdirectory |
| `.agents/evolve/parallel-results/{goal.id}.md` | Worker result summary | Per-goal file |
| `/tmp/evolve-{goal.id}` | Git worktree | Per-goal worktree via /swarm |

Without isolation, N concurrent /rpi cycles would collide on `.agents/rpi/`
(phase summaries, next-work.jsonl) and git index locks.

## Git Isolation

Parallel workers MUST use worktree isolation (via `/swarm --worktrees`):

- Each worker operates in `/tmp/evolve-{goal.id}` worktree
- No git lock conflicts (each worktree has its own index)
- Lead merges worktrees after all complete, before regression gate
- On regression: revert all merged commits using `cycle_start_sha`

## Regression Handling

**Sequential mode:** Revert commits from one goal's /rpi cycle.

**Parallel mode:** Revert ALL commits from the entire parallel wave.
The `cycle_start_sha` (captured before the wave) anchors the revert point.
All N goal improvements are rolled back together — even goals that individually
succeeded. This is by design: if goals interfere, we can't know which one
caused the regression without testing each in isolation.

## Cycle History Schema

Sequential cycles use `target` (string). Parallel cycles use `goal_ids` (array) with `parallel: true`:

```jsonl
{"cycle": 1, "target": "test-pass-rate", "result": "improved", "sha": "abc1234", ...}
{"cycle": 2, "goal_ids": ["doc-coverage", "lint-clean"], "result": "improved", "sha": "def5678", "parallel": true, ...}
```

Legacy entries may use `goal_id` instead of `target` and `commit_sha` instead of `sha`. Tools should handle both.

## Compounding

Each parallel /rpi worker runs its own /post-mortem, which feeds the knowledge
flywheel independently. Learnings from all N parallel cycles compound into the
flywheel, feeding the next /evolve cycle.

## When to Use

| Scenario | Mode |
|----------|------|
| 1-2 failing goals | Sequential (default) — parallelism overhead not worth it |
| 3+ independent failing goals | `--parallel` — significant speedup |
| Goals with overlapping files | Sequential — parallel would cause conflicts |
| First run on new repo | Sequential — learn the codebase before parallelizing |

## Constraints

- Max 5 parallel goals per wave (`--max-parallel` cap)
- Default 3 parallel goals (balance between speedup and resource usage)
- Each /rpi worker needs a full context window — budget accordingly
- Worktree isolation required (no shared-worktree parallel /rpi)
