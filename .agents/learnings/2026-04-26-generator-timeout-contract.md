---
id: learning-2026-04-26-generator-timeout-contract
type: learning
date: 2026-04-26
category: testing
confidence: high
maturity: provisional
utility: 0.7
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: Generator Timeouts Need Author Contracts

## What We Learned

Timeout wrappers can keep Dream moving when a generator stalls, but future
in-process generators still need an authoring contract that requires
context-aware IO and a stall test.

## Why It Matters

Without that contract, a future generator could ignore cancellation and leave
work running until process exit even though the coordinator records a soft-fail
sidecar.

## Source

Post-mortem of PR #155, especially
`TestRunFindingGenerator_StalledGeneratorWritesSoftFailSidecar` and the
context-aware `mine.Run` changes.
