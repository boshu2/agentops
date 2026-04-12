---
utility: 0.7966
last_reward: 0.80
reward_count: 179
last_reward_at: 2026-04-11T20:02:42-04:00
confidence: 0.9633
last_decay_at: 2026-04-12T12:33:50-04:00
helpful_count: 178
maturity: established
maturity_changed_at: 2026-03-05T00:15:38-05:00
maturity_reason: utility 0.70 >= 0.55, reward_count 5 >= 5, helpful > harmful (4 > 0)
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
