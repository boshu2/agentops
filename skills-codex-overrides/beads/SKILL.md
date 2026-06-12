---
name: beads
description: 'Track br issues and dependencies for Codex agents. Triggers: "beads", "br", "track issue", "create task", "find work", "ready issues".'
---

# $beads — Issue Tracking (Codex Tailoring)

This override captures the Codex-native execution model for beads-based issue tracking.

## Key Distinction

Codex agents use `br` CLI directly for issue management. There is no task-queue abstraction — agents read issues via `br ready`, claim via `br update --claim`, and close via `br close`. The orchestrator assigns work to sub-agents by including the issue ID in the spawn prompt.

## Codex-Native Flow

### Finding Work

```bash
br ready                    # unblocked issues
br list --status=open       # all open
br show <id>                # details + dependencies
```

### Creating Issues

```bash
br create --title="<title>" --description="<desc>" --type=task --priority=2
br dep add <child> <parent>  # child depends on parent
```

### Working Issues

1. `br update <id> --claim` — claim before starting
2. Implement the work
3. `br close <id>` — mark complete after verification

### Multi-Agent Coordination

When spawning workers via `spawn_agent(...)`, include the issue ID in the prompt:

```
spawn_agent(prompt="Implement issue <id>: <title>. Details: <description>. Files: <file-list>.")
```

Workers close their own issues after verification. The orchestrator validates via `br list --status=open` after `wait_agent(...)` returns.

## Constraints

1. Always use `br` CLI — never track issues in markdown files or inline state.
2. One issue per worker. If a worker needs to split work, it creates child issues with `br create` + `br dep add`.
3. Workers must `br close` their issue only after the acceptance criteria pass.
