# Worktree-Per-Worker Isolation Pattern

## Problem

Parallel workers in a shared worktree collide when touching the same files, producing build breaks and merge conflicts. Even workers on different files can conflict on generated artifacts (go.sum, lock files).

## When to Use

- **Refactoring epics** — workers modify different functions in different files
- **Multi-epic dispatch** — workers from different epics touch different packages
- **Wave has 3+ workers** — higher collision probability

## When NOT to Use

- **Workers share files** — serialize instead (worktrees can't solve same-file conflicts)
- **Single worker per wave** — no collision risk
- **Pure doc changes** — no build artifacts to conflict on

## Implementation

### 1. Create worktrees before spawning

```bash
for epic_id in $WAVE_EPICS; do
  git worktree add "/tmp/swarm-${epic_id}" -b "swarm/${epic_id}"
done
```

### 2. Inject path into worker prompt

Include in every worker's TaskCreate description:
```
WORKING DIRECTORY: /tmp/swarm-<epic-id>
All file operations MUST use paths rooted at this directory.
```

### 3. Validate in worktree

Run tests inside each worktree before merging:
```bash
cd /tmp/swarm-${epic_id} && go test ./... && go build ./...
```

### 4. Merge and cleanup

```bash
git merge --no-ff "swarm/${epic_id}" -m "chore: merge swarm/${epic_id}"
git worktree remove "/tmp/swarm-${epic_id}"
git branch -d "swarm/${epic_id}"
```

## CC Estimation Heuristic

Each extract-method refactoring saves approximately 3-5 CC points. For CC 30+, plan 6-10 extractions to reach CC <15. See `skills/plan/references/complexity-estimation.md`.

## Evidence

- **ag-atu**: 3 parallel workers refactoring 3 Go files (rpi_parallel.go, dedup.go, markdown.go). Zero merge conflicts. All CC targets met.
- **Swarm SKILL.md**: 4 parallel agents in shared worktree produced 1 build break and 1 algorithm duplication.

## Related

- `skills/swarm/SKILL.md` — Worktree Isolation section (execution detail)
- `skills/swarm/references/local-mode.md` — Step 2b worktree setup
- `skills/crank/references/wave-patterns.md` — FIRE loop and wave planning
