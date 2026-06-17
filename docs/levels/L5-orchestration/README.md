# L5 — Orchestration

Full autonomous operation with `/crank`.

## What You'll Learn

- Using `/crank` for epic-to-completion
- The ODMCR reconciliation loop
- Swarm vs Crew execution modes
- Integration with the NTM + MCP Agent Mail substrate for parallel workers

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
- **Swarm mode**: Dispatches to parallel worker panes via the NTM tmux swarm, coordinated through MCP Agent Mail
- **Crew mode**: Executes sequentially via `/implement`

## Crank Flow

```
/crank <epic>
    ↓
Observe (BEADS_DIR=$PWD/_beads br show, br ready)
    ↓
Dispatch (NTM swarm pane or /implement)
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
| **Swarm** | Multi-agent contention/durability | Parallel dispatch via NTM tmux swarm + MCP Agent Mail |

## Mastery

At L5, you can hand off entire epics to `/crank` and trust autonomous completion.
