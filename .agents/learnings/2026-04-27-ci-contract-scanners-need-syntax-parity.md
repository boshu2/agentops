---
id: learning-2026-04-27-ci-contract-scanners-need-syntax-parity
type: learning
date: 2026-04-27
category: testing
confidence: 0.1547
maturity: provisional
utility: 0.7300
helpful_count: 1
harmful_count: 0
reward_count: 1
last_reward: 0.80
last_reward_at: 2026-04-27T17:01:45-04:00
last_decay_at: 2026-05-03T00:02:17-04:00
---

# Learning: CI Contract Scanners Need Syntax Parity

## What We Learned

When two CI validators enforce the same contract with separate scanners, a new
accepted syntax must be added to both recognizers and covered by a shared or
paired fixture.

## Why It Matters

This prevents one gate from accepting a contract representation while another
gate still fails on the same shipped code path.

## Source

Post-mortem for PR #167 / bead `ag-0af`, where the shell write-surface gate
recognized `filepath.Join(cwd, ".agents", "<surface>")` references before the
Go smoke scanner did.
