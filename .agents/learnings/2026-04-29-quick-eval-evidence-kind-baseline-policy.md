---
type: learning
source: retro-quick
source_bead: ag-v29
source_phase: validate
date: 2026-04-29
maturity: provisional
utility: 0.5
reward_count: 0
helpful_count: 0
harmful_count: 0
confidence: 0.0000
---

# Learning: Eval Evidence Kind And Baseline Policy Are Separate Axes

**Category**: testing
**Confidence**: medium

## What We Learned

Do not infer an eval case's evidence kind from `baseline_policy.mode=compare`.
Baseline comparison is governance metadata about candidate-vs-baseline handling,
while evidence kind describes what the case itself proves. Collapsing the two
would make broad baseline adoption hide the real mix of contract canaries, gate
wrappers, behavior fixtures, holdouts, and runtime checks.

## Source

Quick capture via `$validation` during ag-v29 RPI implementation.
