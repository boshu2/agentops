---
id: learning-2026-04-27-stacked-pr-patience-pattern
type: learning
date: 2026-04-27
category: process
confidence: high
maturity: provisional
utility: 0.6
source_epic: ag-3lx
reward_count: 0
helpful_count: 0
harmful_count: 0
---

# Learning: Wait For Stacked-PR Auto-Rebase Instead Of Pre-emptively Resolving

## What We Learned

When a stack of dependent PRs targets `feat/A -> feat/B -> main`, do NOT pre-emptively resolve conflicts on the upper PR while the lower PR is still open. GitHub auto-rebases the upper PR's base when the lower one merges, and the conflict surface is computed against the actual new main — not the speculative one. Pre-emptive resolution diverges and creates re-work.

In this epic, PR #171 showed `mergeStateStatus: DIRTY / mergeable: CONFLICTING` while #170 was still open. After #170 merged, #171 auto-rebased to base=main and became `MERGEABLE` without operator intervention.

## Why It Matters

Saves an entire conflict-resolution pass per stacked PR. For a 3-deep stack with 17 conflict files, that's potentially 30+ minutes of work avoided.

## When To Apply

- Stacked PRs where each PR's base is the head of the PR below it
- The lower PR is healthy and likely to merge
- You're tempted to fix the "CONFLICTING" status now to get ahead

## When NOT To Apply

- The lower PR is blocked indefinitely or rejected
- The conflict resolution requires planning input you have only at this moment
- Operator is leaving and the next person needs a clean handoff

## Source

Session resumption on epic ag-3lx, 2026-04-27. Initial state showed PR #171 conflicting; deferred resolution; #170 merged via sibling session; #171 became mergeable without manual rebase.
