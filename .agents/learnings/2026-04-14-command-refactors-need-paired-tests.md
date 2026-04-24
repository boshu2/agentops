---
id: learning-2026-04-14-command-refactors-need-paired-tests
type: learning
date: 2026-04-14
category: process
confidence: high
maturity: provisional
utility: 0.7
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: Command Refactors Need Paired Test Diffs

## What We Learned

When a slice changes a production command file under `cli/cmd/ao/`, the
command/test-pairing gate should be treated as part of the change contract.
The `codex.go` refactor only cleared the fast gate after the production helper
split was paired with direct lifecycle helper tests in `codex_test.go`.

## Why It Matters

This keeps low-risk refactors low-risk. Planning the test diff up front avoids
late gate failures and reduces amend/push churn on otherwise clean slices.

## Source

Post-mortem of the 2026-04-14 evolution loop, especially `na-i86x`
(`c078d48a`) and its final pre-push repair.
