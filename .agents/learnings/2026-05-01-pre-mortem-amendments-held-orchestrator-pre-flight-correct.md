---
id: learning-2026-05-01-pre-mortem-amendments-held
type: learning
date: 2026-05-01
category: process
confidence: high
maturity: provisional
utility: 0.7
source_epic: soc-irg1
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: Pre-mortem amendments held 4/4 when implemented as bd issue notes

## What We Learned

In epic soc-irg1, a `--quick` pre-mortem returned WARN with 4 fixable findings. All 4 were propagated verbatim into the affected bd issue notes (`bd update <id> --notes "..."`) BEFORE worker dispatch. When the implementing workers ran, all 4 amendments were applied correctly with no rediscovery cost:

- F1 (surface enumeration includes `cli/embedded/` + `make sync-hooks` mandatory) — applied in I5 by the orchestrator post-Wave-1 sync commit + I5 worker re-running sync-hooks.
- F2 (hook activation test, not just `bash -n`) — applied in I3 as `tests/hooks/test-edit-scope-guard-fires.sh` (118 lines, 7 cases, all PASS).
- F3 (malformed-input fail-open in hook) — applied verbatim at `hooks/edit-scope-guard.sh:18-27`.
- F4 (≥4 commits per package family for bisectability) — applied: I5 produced exactly 4 commits, each independently green on `go test ./...`.

Score for the prediction-tracking ledger: **4/4 PREDICTIONS_HELD**.

## Why It Matters

The pre-mortem → bd-issue-notes → worker-reads-notes-on-spawn loop is the operational mechanism by which the council layer drives the implementation layer. Without the explicit bd-notes propagation step (skill `/pre-mortem` STEP 4.6), workers would have to rediscover the findings from the council report — costing both context and accuracy.

The fact that 4/4 held without rediscovery confirms two things:
1. The propagation step works as designed (mechanical reliability).
2. A `--quick` pre-mortem is sufficient for standard-complexity 5-issue epics where the failure surface is well-understood. No `--deep` council needed.

## Source

Epic soc-irg1 (gstack absorption Tier 1). Pre-mortem at `.agents/council/2026-05-01-pre-mortem-gstack-absorption-tier1.md`. Vibe verdict: PASS. All 4 amendments verified in vibe report `.agents/council/2026-05-01-vibe-soc-irg1.md` Specific Concerns table.

## Applies When

- Standard-complexity epic (3-7 issues)
- Pre-mortem returns WARN with concrete pseudocode-form fixes (not abstract concerns)
- Orchestrator can propagate amendments to bd issue bodies BEFORE worker dispatch
- Workers are instructed to `bd show <issue>` and read notes as part of their required reading

## Counter-applies

- Vague WARN findings without concrete fixes — workers can't apply what isn't actionable
- Workers that don't read bd notes (rare; check skill prompts include the read step)
- Pre-mortem produced AFTER worker dispatch (too late; amendments can't propagate)
