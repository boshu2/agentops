---
id: learning-2026-04-30-pr-queue-stack-drain
type: learning
date: 2026-04-30
category: process
confidence: high
maturity: provisional
utility: 0.7
---

# Learning: Stack PR Queues Drain Base-First

## What We Learned

When several open PRs are stacked or contaminated by older queue commits, merge and fetch the foundation PRs first, then rebase the remaining branches onto the updated target. This lets duplicate commits auto-drop and keeps conflict resolution focused on the actual remaining feature.

## Why It Matters

Base-first queue drains reduce repeated rebase work and make force-with-lease updates safer because each branch is repaired against the real target state.

## Source

Post-mortem for `soc-j8d6`, the 2026-04-30 AgentOps PR queue repair.
