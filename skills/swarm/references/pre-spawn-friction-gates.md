# Pre-Spawn Friction Gates

> From swarm friction analysis of 14,753 sessions. Git sync conflicts rose from 71 to 82.6 per 1K sessions (Jan → Feb).
> 58% of friction is addressable with process gates, not model improvements.

## Pre-Spawn Checklist (Mandatory)

Before spawning ANY worker (team agent, Codex sub-agent, or worktree agent):

### Gate 1: Base Branch Sync
```bash
git fetch origin && git diff --stat HEAD origin/$(git branch --show-current)
```
If there are upstream changes, rebase/merge BEFORE spawning. Stale base branches cause 82.6/1K merge conflicts.

### Gate 2: File Ownership Manifest
Before spawning, generate an explicit file ownership manifest:

```
Worker A owns: cli/cmd/ao/goals.go, cli/cmd/ao/goals_test.go
Worker B owns: skills/plan/SKILL.md, skills/plan/references/
Worker C owns: hooks/session-start.sh
```

**Rule:** No file may appear in two workers' manifests. If two tasks touch the same file, combine them into one worker.

### Gate 3: Dependency Graph Verification
Verify no worker depends on another worker's output:

```
Worker A: independent (no deps)
Worker B: independent (no deps)
Worker C: depends on Worker A output → MOVE TO WAVE 2
```

Workers with dependencies go in later waves. Never spawn dependent workers in the same wave.

### Gate 4: 15-Minute Misalignment Circuit Breaker
Set a timer. If a worker hasn't produced its first meaningful output (commit, test result, or file change) within 15 minutes:

1. Check if the worker is stuck in research/planning loop
2. If stuck: kill and re-scope with smaller task
3. If making progress: extend timer by 15 minutes

### Gate 5: Wave Size Cap
Maximum 4 workers per wave. Evidence: waves of 5+ have exponentially higher merge conflict rates. If you have 6 tasks, split into Wave 1 (4 workers) + Wave 2 (2 workers).

## Post-Spawn Gates

### After Each Worker Completes
1. Run the external gate command (not worker self-report)
2. Check for file conflicts with other workers' outputs
3. If conflicts: use merge arbiter (see `references/conflict-recovery.md`)

### After Each Wave Completes
1. Merge all worker outputs to base branch
2. Run full test suite on merged result
3. Refresh base SHA before spawning next wave
4. Re-evaluate remaining tasks (some may be unnecessary after Wave N results)

## Anti-Patterns

- ❌ Spawning workers without checking base branch freshness
- ❌ Two workers editing the same file "they'll figure it out"
- ❌ Spawning 8 workers at once for "maximum parallelism"
- ❌ Worker A depends on Worker B's output in the same wave
- ✅ Explicit file manifest per worker, verified for no overlap
- ✅ Wave cap of 4, dependency-ordered waves
- ✅ Base branch synced immediately before spawn
