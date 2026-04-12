---
maturity: established
utility: 0.80
---

# Topological Wave Decomposition

## Pattern

Use topological wave decomposition when a plan has many tasks that can run in
parallel only after their dependencies are satisfied. Model each task as a node,
draw dependency edges, then execute the graph in waves where every node in a wave
has no unresolved predecessors.

## Why It Works

This keeps parallel execution honest. It prevents an agent from launching a task
that depends on files, contracts, or evidence another task has not produced yet,
while still allowing independent leaves to move at the same time.

## How To Apply

1. List every task, artifact, and validation gate.
2. Add edges for required inputs, file ownership, generated artifacts, and bead
   blockers.
3. Put all zero-inbound tasks in wave 0.
4. After a wave lands, remove those nodes and recompute the next zero-inbound
   wave.
5. Keep serial merge and validation work as its own final wave.

## Retrieval Cues

Topological wave decomposition, dependency ordering, parallel wave planning,
wave execution, DAG planning, blocker-aware task scheduling.
