# L5 — Orchestration

Full autonomous operation with `/crank`.

## What You'll Learn

- Using `/crank` for epic-to-completion
- The ODMCR reconciliation loop
- Mayor vs Crew execution modes
- Integration with NTM for parallel workers

## Prerequisites

- Completed L4-parallelization
- Comfortable with wave execution
- Understanding of beads issue tracking

## Available Commands

| Command | Purpose |
|---------|---------|
| `/crank` | Autonomous epic-to-completion |
| `/implement-wave` | Same as L4 |
| `/plan <goal>` | Same as L3 |
| `/research <topic>` | Same as L2 |
| `/implement [id]` | Same as L3 |
| `/retro [topic]` | Same as L2 |

## Key Concepts

- **Crank**: Autonomous epic execution - runs until ALL children are CLOSED
- **ODMCR loop**: Observe → Dispatch → Monitor → Collect → Retry
- **Mayor mode**: Dispatches to parallel polecats via NTM
- **Crew mode**: Executes sequentially via `/implement`

## Crank Flow

```
/crank <epic>
    ↓
Observe (bd show, bd ready)
    ↓
Dispatch (gt sling or /implement)
    ↓
Monitor (convoy status)
    ↓
Collect (close completed)
    ↓
Retry (handle failures)
    ↓
Loop until epic CLOSED
```

## Execution Modes

| Mode | When | How |
|------|------|-----|
| **Crew** | Default, single-agent | Sequential `/implement` calls |
| **Mayor** | In ~/gt or mayor/ directory | Parallel dispatch via `gt sling` |

## Mastery

At L5, you can hand off entire epics to `/crank` and trust autonomous completion.
