---
type: learning
source: post-mortem-quick
date: 2026-04-29
maturity: provisional
utility: 0.6
---

# Learning: Eval Triage Closeout Needs Explicit Disposition

**Category**: process
**Confidence**: medium

## What We Learned

When an evaluation-suite triage epic ships only the foundational slices, closing the remaining slices should say whether they were implemented, deferred, or intentionally killed. Marking deferred slices as closed is acceptable only when the close reason preserves that distinction and points back to the delivered evidence, otherwise the issue tracker overstates what the evaluation suite can prove.

## Source

Quick capture via `$post-mortem --quick` after closing `ag-v29.3` through `ag-v29.6` as operator-deferred follow-up work and reconciling parent epic `ag-v29`.
