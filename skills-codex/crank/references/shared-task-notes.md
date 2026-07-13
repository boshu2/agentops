# SHARED_TASK_NOTES Bridge Pattern

> Persist context across iterations when workers spawn fresh (Ralph Wiggum pattern).

## Problem

Each crank wave spawns fresh workers with no memory of previous waves. Critical context gets lost:
- "Wave 1 tried approach X and it failed because of Y"
- "The auth module has a quirk where Z must happen before W"
- "Wave 2 discovered that file F needs special handling"

Workers rediscover the same issues or make the same mistakes.

## Solution: SHARED_TASK_NOTES.md

A persistent file that the crank orchestrator maintains between waves. Workers READ it at start and the orchestrator APPENDS to it after each wave.

### File Location
```
.agents/crank/SHARED_TASK_NOTES.md
```

### Format
```markdown
# Shared Task Notes — Epic <epic-id>

## Wave 1 (2026-03-21T10:00:00)
- Auth middleware requires `ctx.WithTimeout` wrapping (discovered by worker #2)
- `config.yaml` has a race condition when read concurrently — use sync.Once
- Test fixtures in `testdata/` must be copied, not symlinked (CI rejects symlinks)

## Wave 2 (2026-03-21T10:15:00)
- Rate limiter uses token bucket, not sliding window — don't assume sliding
- The `internal/store` package has unexported helpers that can be reused
```

### Orchestrator Responsibilities

**Before each wave:**
```bash
# Read shared notes and include in every worker prompt
if [ -f .agents/crank/SHARED_TASK_NOTES.md ]; then
    SHARED_NOTES=$(cat .agents/crank/SHARED_TASK_NOTES.md)
    # Include in TaskCreate description:
    # "Context from prior waves:\n${SHARED_NOTES}"
fi
```

**After each wave:**
```bash
# Append new discoveries from wave results
cat >> .agents/crank/SHARED_TASK_NOTES.md <<EOF

## Wave ${wave} ($(date -Iseconds))
$(extract_discoveries_from_wave_results)
EOF
```

### What to Capture

| Category | Example | Source |
|----------|---------|--------|
| Failed approaches | "Approach X failed because Y" | Worker error output |
| Codebase quirks | "Module Z requires special handling" | Worker discoveries |
| Convention discoveries | "Tests must follow pattern P" | Worker observations |
| Dependency notes | "Task A must complete before B" | Orchestrator analysis |
| Fix patterns | "When you see error E, apply fix F" | Worker solutions |

### What NOT to Capture

- Full error logs (too verbose, pollutes context)
- Implementation details (workers should read code directly)
- Task status (tracked by beads/TaskList)
- Anything already in the issue description

### Size Management

Cap at ~50 lines. When exceeding:
1. Summarize older waves into a "## Prior Waves Summary" section
2. Keep last 3 waves in full detail
3. Preserve any entries marked with `[CRITICAL]` regardless of age

### Integration with Worker Prompts

Include shared notes in the worker's TaskCreate description, after the issue body:

```
TaskCreate(
  subject="ag-1234: Add auth middleware",
  description="<issue body>\n\n---\nContext from prior waves (read before starting):\n<shared notes content>"
)
```

Workers should read shared notes before starting implementation and add their own discoveries to their task output for the orchestrator to harvest.

## Anti-Patterns

| Anti-Pattern | Why It Fails | Fix |
|-------------|-------------|-----|
| Workers write directly to SHARED_TASK_NOTES | Parallel writes corrupt file | Only orchestrator writes; workers report in task output |
| Including full error logs | Context pollution, token waste | Summarize: "Error E in file F, caused by C" |
| Not capping size | Old notes dominate context window | Summarize waves older than 3 |
| Skipping for small epics | Even 2-wave epics benefit | Always maintain; overhead is minimal |
