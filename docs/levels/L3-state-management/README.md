# L3 — State Management

Add issue tracking with beads for structured work.

## What You'll Learn

- Using `/plan` to decompose work into issues
- Beads commands for issue lifecycle
- Tracking dependencies between tasks
- Session close protocol

## Prerequisites

- Completed L2-persistence
- Comfortable with `.agents/` directory
- br (beads_rust) CLI installed; invoke as `BEADS_DIR=$PWD/_beads br`

## Available Commands

| Command | Purpose |
|---------|---------|
| `/plan <goal>` | Decompose goal into beads issues |
| `/research <topic>` | Same as L2 |
| `/implement [id]` | Execute specific issue, then close it |
| `/retro [topic]` | Same as L2 |

## Beads Commands

All commands take `BEADS_DIR=$PWD/_beads` (the br ledger lives at `_beads/`):

```bash
BEADS_DIR=$PWD/_beads br ready                    # Show unblocked issues
BEADS_DIR=$PWD/_beads br list --status open       # All open issues
BEADS_DIR=$PWD/_beads br show <id>                # View issue details
BEADS_DIR=$PWD/_beads br update <id> --status in_progress
BEADS_DIR=$PWD/_beads br close <id> --reason "Done"
BEADS_DIR=$PWD/_beads br sync                      # Sync the git-JSONL ledger (never touches git itself)
```

## Key Concepts

- **Issues**: Atomic units of work
- **Dependencies**: Issues can block each other
- **Session close**: `br sync` after your issue updates, then `git -C _beads push` to share the ledger

## What's NOT at This Level

- No parallel execution
- No `/crank` (autonomous execution)

## Next Level

Once comfortable with issue tracking, progress to [L4-parallelization](../L4-parallelization/) to execute waves.
