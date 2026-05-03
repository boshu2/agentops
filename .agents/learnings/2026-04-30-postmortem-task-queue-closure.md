---
id: learning-2026-04-30-postmortem-task-queue-closure
type: learning
date: 2026-04-30
category: process
confidence: high
maturity: provisional
utility: 0.6
---

# Learning: Task Queue Closures Need Task-Mode Proof

## What We Learned

The closure-integrity audit is optimized for epics with child beads. A task-only PR queue drain can have strong merge evidence and still produce a collection warning because there are no child issues to inspect.

## Why It Matters

Post-mortem tooling should accept task-level proof packets or a task mode so queue-drain work stays mechanically replayable without forcing artificial child beads.

## Source

Post-mortem for `soc-j8d6`, where `closure-integrity-audit.sh --scope auto soc-j8d6` found no children while GitHub merge commits proved the work.
