---
id: learning-2026-05-01-orchestrator-scope-refinement
type: learning
date: 2026-05-01
category: process
confidence: high
maturity: provisional
utility: 0.8
source_epic: soc-irg1
---

# Learning: When pre-mortem flags an undersized surface, refine scope at orchestration time before dispatching the worker

## What We Learned

Pre-mortem Finding 1 said: "I5 surface enumeration is incomplete; extend to cli/embedded, tests/, skills/, docs/, scripts/." Worker estimate was ~82 files in cli/cmd/ao/ alone. Actual baseline (computed by orchestrator pre-flight): **305 files across 10 surfaces**.

Of those 305, only ~150 are actually executable code that COMPUTES paths; the rest are markdown documentation where `.agents/` references are prose. Two ways to handle this:

1. **Naive:** Hand the worker all 305 files and a "migrate everything" prompt → worker spends hours grinding markdown that shouldn't be migrated, possibly damaging prose docs in the process.
2. **What we did:** Orchestrator refined I5 in-flight to "focused exemplar pass + warn-only ratchet + follow-up bd issue." Scope dropped to ~15-20 file migration. Follow-up issue (`soc-re0w` P3) tracks the ~140-file long tail.

The refinement produced 4 clean commits with full test coverage in one worker pass. The follow-up captures the long-tail without burning tokens on a single mega-task.

## Why It Matters

The pre-mortem layer can SURFACE that the original estimate was wrong, but it can't fix the resulting plan in-place. The orchestrator's job — between pre-mortem WARN and worker dispatch — is to interpret findings and adapt the worker prompt. This is a **planning judgment call**, not a worker decision.

Anti-pattern this prevents: "We told the worker to migrate 305 files because pre-mortem said the surface was bigger." That's not following pre-mortem advice — that's blame-shifting to the worker.

Refinement heuristic:
- If surface estimate is off by >2x AND new estimate is >50 files → orchestrator must decompose before dispatch
- If surface includes mixed types (executable code vs prose docs) → orchestrator must filter to actionable subset
- File a follow-up bd issue at the same priority as the parent OR at +1 priority drop, with the long tail's expected scope estimated explicitly

## Source

Epic soc-irg1 (gstack absorption Tier 1). I5 worker delivery: 4 commits (`34125b94..2bbeb2dd`), 15 files migrated, warn-only ratchet baseline = 151 occurrences. Follow-up bd issue `soc-re0w` filed for the ~140-file long tail.

## Applies When

- Plan estimate is detailed (file counts, surface enumerated)
- Pre-mortem flags the estimate as wrong (with magnitude)
- Orchestrator has time/budget to re-baseline before dispatch (`grep -rln` is cheap)
- Worker would otherwise face a >2-hour mechanical refactor

## Counter-applies

- Surface really is one homogeneous type (no filtering needed)
- Worker decomposition is part of the worker contract (some tasks are "find and grind")
- No clean way to defer the long tail (no follow-up issue model)
