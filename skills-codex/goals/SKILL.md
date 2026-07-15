---
name: goals
description: Measure declared project fitness goals
---
# Goals — read-only fitness measurement

Inspect the active goals document and run only the caller-selected measurement,
validation, drift, history, export, or meta-goal command.

## Boundary

- Prefer `GOALS.md` when both Markdown and legacy YAML exist.
- Preserve stable directive and gate identities in the report.
- Every measured gate must name its executable check and observed outcome.
- Do not add, remove, prioritize, recommend, apply, prune, migrate, or otherwise
  mutate goals.
- Do not translate a fitness gap into work selection or a next action.

## Read-only commands

```bash
ao goals measure --json
ao goals validate --json
ao goals drift
ao goals history
ao goals export
ao goals meta --json
```

Run the requested command once. Return the command, exit code, goal-level
results, aggregate measurement, missing evidence, and checked/not-checked scope.
Then stop.
