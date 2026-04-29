---
id: learning-2026-04-27-branch-audit-triage-classes
type: learning
date: 2026-04-27
category: process
confidence: high
maturity: provisional
utility: 0.5
source_epic: branch-audit-2026-04-27
reward_count: 0
helpful_count: 0
harmful_count: 0
---

# Learning: Branch Audits Need Explicit Disposition Classes

## What We Learned

A branch cleanup is easiest to complete safely when every ref is assigned one of five classes: active PR to merge, no-op/superseded PR to close, stale residue to archive-tag and delete, landed source branch to archive after split/merge, or documented preserve ref to retain.

## Why It Matters

The classification prevents two common mistakes: deleting recoverability for unfinished work, and leaving stale remote heads around after their useful commits have already landed.

## Source

Post-mortem of the 2026-04-27 branch audit and merge cleanup.
