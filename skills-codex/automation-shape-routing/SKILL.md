---
name: automation-shape-routing
description: 'Front door for agent automation: choose'
---
# Automation Shape Routing

Choose the smallest execution shape that preserves the required evidence and
control. This skill routes; it does not build or start a substrate.

## Route in order

1. **One deliverable?** Do it inline. Use a small in-session fresh-context
   fanout only when independent perspectives are the product. Do not create a
   reusable artifact for a one-off task.
2. **Reusable sequential procedure?** Use `skill-builder`.
3. **Must-never-regress constraint?** Route through `operationalize` to a gate.
4. **Fixed typed DAG, headless, no attach/steer?** Use `workflow-builder` only
   where that runtime is explicitly selected and available.
5. **Persistent, attachable roles over AgentOps beads?** Use `agent-native`.
   NTM is the pane adapter; workers execute whole loop skills, while Agent Mail
   coordinates only multiple live actors.
6. **Durable city of quests with GC-native supervision/store?** Route to
   `using-gc` only when the operator explicitly selects Gas City. GC is not an
   automatic fallback or an `ao` runtime enum.

## Deciding axes

| Axis | Lightweight choice | Escalated choice |
|---|---|---|
| lifetime | current turn | persistent/attachable worker |
| topology | one writer or bounded fanout | durable role graph |
| control | no mid-run steering | observe/nudge/replace |
| output | one artifact | reusable skill/workflow/gate |
| store | repo bead chain | operator-selected GC quest store |
| contention | one writer | partition, then Agent Mail reservation |

Parallelism buys independence, not guaranteed speed. Refuse persistent
orchestration for one-shot work, colliding write scopes, or a sequential chain
that has no exploitable concurrency.

## Handoff

Return exactly one of:

- `inline` or `bounded-fanout`
- `skill-builder`
- `workflow-builder`
- `agent-native` with a named reason persistent panes help
- `using-gc` with explicit operator choice
- `operationalize:gate`

Name the deciding axis and invoke the owner. Do not copy the delegated workflow
into this router.
