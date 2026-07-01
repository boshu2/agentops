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
- br (beads_rust) CLI installed; invoke as `BEADS_DIR="$(ao beads dir)" br`

## Available Commands

| Command | Purpose |
|---------|---------|
| `/plan <goal>` | Decompose goal into beads issues |
| `/research <topic>` | Same as L2 |
| `/implement [id]` | Execute specific issue, then close it |
| `/post-mortem [topic]` | Same as L2 |

## Beads Commands

All commands take `BEADS_DIR="$(ao beads dir)"` so linked worktrees use the canonical private ledger:

```bash
BEADS_DIR="$(ao beads dir)" br ready --json             # Show unblocked issues
BEADS_DIR="$(ao beads dir)" br list --status open --json # All open issues
BEADS_DIR="$(ao beads dir)" br show <id> --json         # View issue details
BEADS_DIR="$(ao beads dir)" br update <id> --claim --json
BEADS_DIR="$(ao beads dir)" br close <id> --reason "Done"
BEADS_DIR="$(ao beads dir)" br sync --flush-only        # Export the git-JSONL ledger (never touches git itself)
```

## Key Concepts

- **Issues**: Atomic units of work
- **Dependencies**: Issues can block each other
- **Session close**: `BEADS_DIR="$(ao beads dir)" br sync --flush-only` after issue updates, then `git -C "$(ao beads dir)" push` to share the private ledger

## What's NOT at This Level

- No parallel execution
- No `/crank` (autonomous execution)

## Next Level

Once comfortable with issue tracking, progress to [L4-parallelization](../L4-parallelization/) to execute waves.
