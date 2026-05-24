# Commit Strategies

Crank supports two commit strategies for how changes are committed after task completion.

## wave-batch (default)

The team lead commits once after all workers in a wave have completed and passed validation.

**Pros:**
- No merge conflicts (single committer)
- Clean git history (one commit per wave)
- Proven pattern across 7+ epics

**Cons:**
- Coarse bisectability (entire wave in one commit)
- Harder to attribute changes to specific issues

**Commit message format:** `feat(<epic-id>): wave N - <summary of changes>`

## per-task (opt-in via `--per-task-commits`)

Workers commit after their individual task passes validation.

**Pros:**
- Fine-grained git bisect (one commit per issue)
- Per-issue traceability in git history
- Better attribution

**Cons:**
- Merge conflict risk when multiple workers modify overlapping files
- Requires parallel-wave guard for safety

**Commit message format:** `feat(<issue-id>): <issue-title>`

## Parallel-Wave Guard (mandatory for per-task)

When a wave has 2+ workers modifying overlapping files, per-task commits are automatically disabled for that wave:

1. Before wave start, check file boundaries from plan/task metadata
2. **If file boundaries are absent** for any worker in a multi-worker wave → fall back to wave-batch (safe default). Only allow per-task when ALL workers have explicit boundary declarations.
3. If any file appears in 2+ workers' boundaries → fall back to wave-batch
3. Log: "Per-task commits disabled for wave N (overlapping file boundaries: <files>). Using wave-batch."
4. Record fallback in wave checkpoint JSON: `"commit_strategy": "wave-batch-fallback"`

Single-worker waves are always safe for per-task commits (no conflict possible).

## State Tracking

When `--per-task-commits` is active:
- `crank_state.per_task_commits = true`
- Each wave checkpoint includes: `"commit_strategy": "per-task" | "wave-batch" | "wave-batch-fallback"`
