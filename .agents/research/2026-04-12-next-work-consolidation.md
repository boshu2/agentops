---
type: research
date: 2026-04-12
---

# Next-Work Queue Consolidation

53 items across 12 themes. After de-duplication, merging related items, and removing already-done work:

## Already Done (remove from queue)

| Item | Status | Evidence |
|---|---|---|
| Resolve duplicate retag release runs (ag-wyf) | DONE | cc32dfae cancel-in-progress:true |
| Make release audits point at final tagged CI artifacts (ag-x1f) | DONE | 4971f369 head→tail fix |
| Remove UTC-only churn from CLI reference headers (ag-hu5) | DONE | Already removed in prior session |
| Write AgentOps philosophy doc | DONE | 586519fe docs/philosophy.md |
| Fix TestFlagMatrix compile test failure | DONE | Binary rebuild fixed it |
| Teach closure-integrity audit to consume evidence-only packets | DONE | d9f23d79 |
| Replace stale next-work heuristic suppression with proof-backed | DONE | c8a31f15 executionPacketPathIsValid |
| Spike gemma4:e4b on larger chunks (4-8K chars) | DONE | 4.3K chunk validated 4.3s |
| W5: Dream RunLoop pre-hook | DONE | af7e3d3c runPostLoopTier1Forge |
| W6: Tier 2 structural promotion eval harness | DONE | 3a752427 ao forge review |

**10 items already completed.** Consume these from the queue.

## Merged Items (combine into single work items)

### CLOSURE-PROOF Epic (5 items → 1)
Merge: "Require scoped proof metadata", "Narrow packet catch-all", "Normalized file-scope extraction", "Evidence-only maintenance epics", "Stale next-work suppression proof"
→ **Single item: "Harden closure-integrity proof pipeline"**
All about making the audit script handle edge cases. The two big fixes already shipped; remaining items are polish.

### RELEASE Epic (4 remaining → 1)
Merge: "mkdir -p guard", "Stale artifact directory", "Retag annotated→lightweight downgrade", "Wall-clock dates in reference docs"
→ **Single item: "Release script hardening pass"**
All minor hardening of ci-local-release.sh and retag-release.sh.

### RETRIEVAL Epic (7 items → 2)
Group A (measurement): "retrieval-bench --live in CI", "eval harness 20 queries", "warn-then-fail ratchet on retrieval"
→ **Single item: "Retrieval quality measurement + ratchet"**

Group B (injection): "Wire metadata.stability", "Content-hash dedup", "Audit context latency", "Statusline bridge"
→ **Single item: "Inject pipeline quality improvements"**

### FINDINGS-PREVENTION (2 items → 1)
Merge: "Constraint execution mode", "Applicability-based prevention rules"
→ **Single item: "Formalize finding→constraint activation path"**

### WIKI-PIPELINE (2 remaining → 1)
Merge: "W7 Claude/Codex Tier 2 review backend", "ao config models --set-tier"
→ **Single item: "Tier 2 LLM-backed review + config surface"**

## Consolidated Queue (22 items from 53)

### HIGH PRIORITY (ship next)

1. **Retrieval quality measurement + ratchet** [RETRIEVAL]
   Build eval harness with 20 known-good queries. Wire retrieval-bench --live into CI nightly. Add warn-then-fail ratchet on precision metric. This is the #1 blocker identified by every council.

2. **Inject pipeline quality improvements** [RETRIEVAL]
   Wire metadata.stability into inject ranking. Content-hash dedup (replace title-based). Audit lazy-loading. INDEX.md-first query strategy.

3. **Harden closure-integrity proof pipeline** [CLOSURE-PROOF]
   Normalize file-scope extraction from bead prose. Narrow packet catch-all. Require scoped proof on child close.

### MEDIUM PRIORITY (next wave)

4. **Release script hardening pass** [RELEASE]
   mkdir -p guard, annotated tag preservation, artifact dir validation, wall-clock date removal.

5. **Add output_contract to remaining skills** [SKILL-CONTRACTS]
   Standardize output contracts across all 66 skills.

6. **Add additionalProperties check to /plan skill** [SKILL-CONTRACTS]
   Schema strictness gate during plan decomposition.

7. **Formalize finding→constraint activation path** [FINDINGS-PREVENTION]
   Define execution mode and concrete enforcement semantics for compiled constraints.

8. **Wire bd-audit + bd-cluster into crank/swarm** [PLANNING-PROCESS]
   Pre-flight gates: bd-audit blocks on >50% flagged beads, bd-cluster suggests merges.

9. **EV-3.2: Weighted matching** [OTHER]
   Adds weighted scoring to substring matching in the evolve work selector.

10. **Populate MixedModeEffective + vendor fields** [OTHER]
    Complete the mixed-mode council metadata pipeline.

11. **Tier 2 LLM-backed review + config** [WIKI-PIPELINE]
    Claude/Codex review backend for ao forge review. ao config models --set-tier.

12. **Backfill next-work queue to schema v1.3** [NEXT-WORK-QUEUE]
    Migrate existing rows, add drift validation.

13. **Close ag-yeg directive gaps** [TESTING]
    Runtime tests + citation gating for remaining directives.

14. **Multi-wave file assignment enforcement** [TESTING]
    Plans must assign every touched file including tests to a specific wave.

15. **Add source_bead to /retro quick-capture** [PLANNING-PROCESS]
    Template enhancement for provenance tracking.

16. **Pre-mortem pseudocode in issue body** [DOCS]
    Write fixes as code blocks, not prose descriptions.

### LOW PRIORITY (backlog)

17. **Populate .agents/patterns/ bodies** [OTHER]
    Frontmatter-only pattern files need content.

18. **Surface quality signals in /status** [CLI-CONFIG]
19. **Add input validation as pre-mortem check** [PLANNING-PROCESS]
20. **Refactor collectLearnings complexity** [TESTING]
21. **Turn-level content writer + XML strip** [OTHER]
22. **Post-merge Session Intelligence readout** [OTHER]
