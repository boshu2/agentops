---
description: Execute an epic to completion using swarm for parallel wave execution
---

# /crank

Runs an entire epic through waves of parallel execution until ALL children are CLOSED.

## Usage

```
/crank <epic-id>
/crank ao-epic-123
```

## Architecture

Crank is the autonomous orchestrator that uses swarm for each wave:

```
Crank (orchestrator)           Swarm (executor)
    |                              |
    +-> br ready (wave issues)     |
    |                              |
    +-> TaskCreate from beads  --->+-> Spawn agents (fresh context)
    |                              |
    +-> /swarm                 --->+-> Execute in parallel
    |                              |
    +-> Verify + br update     <---+-> Results
    |                              |
    +-> Loop until epic DONE       |
```

**Separation of concerns:**
- **Crank** = Beads-aware orchestration, epic lifecycle, knowledge flywheel
- **Swarm** = Fresh-context parallel execution (Ralph Wiggum pattern)

## How It Works

The FIRE Loop:

1. **FIND**: `BEADS_DIR="$(ao beads dir)" br ready` - get unblocked beads issues
2. **IGNITE**: Create TaskList tasks, invoke `/swarm`
3. **REAP**: Swarm collects results, crank syncs to beads
4. **ESCALATE**: Fix blockers, retry failures
5. Loop until all children are CLOSED

## Output

```
/crank ao-epic-123

Epic: "Add user dashboard"
Total: 8 issues

[Wave 1] br ready → [ao-1, ao-2, ao-3]
         TaskCreate for each
         /swarm → 3 agents spawned
         ao-1 DONE, ao-2 DONE, ao-3 BLOCKED

[Wave 2] br ready → [ao-4, ao-5, ao-3]
         TaskCreate for each
         /swarm → 3 agents spawned
         ...

[Final Vibe] Running /vibe on recent changes...
             All checks passed.

<promise>DONE</promise>
Epic: ao-epic-123
Issues completed: 8
Waves: 4/50
```

## Limits

- **MAX_EPIC_WAVES = 50** - Prevents infinite loops
- Swarm handles parallelism per wave (no max agent limit in swarm)

## Failure Handling

Crank handles failures automatically:
- Retry failed issues in next wave
- Skip blocked issues (revisit when unblocked)
- Escalate persistent failures after 3 retries

## When to Use

| Scenario | Skill |
|----------|-------|
| Execute entire epic autonomously | `/crank` |
| Just parallel execution (no beads) | `/swarm` directly |
| Single issue | `/implement` |

## Next

- `/validate` - Runs automatically at end
- `/post-mortem` - Extract learnings after epic completes
