---
id: learning-2026-04-25-eval-longhaul-closeout-disposition
type: learning
date: 2026-04-25
category: process
confidence: high
maturity: provisional
utility: 0.5
---

# Learning: Long Evolve Runs Need Disposition Validation

## What We Learned

Product validation can be green while closeout is still blocked by repository disposition or replayability gaps. After a long autonomous eval run, run both the product gates and the closure/worktree audits before selecting the next RPI target.

## Why It Matters

This prevents agents from compounding on a dirty or unreplayable state after the eval suite itself says PASS.

## Source

Post-mortem for `agentops-dv5` after the 2026-04-25 eval-environment longhaul run.
