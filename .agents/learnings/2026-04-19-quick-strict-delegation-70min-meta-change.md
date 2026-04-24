---
type: learning
source: retro-quick
source_phase: validate
date: 2026-04-19
maturity: provisional
utility: 0.5
confidence: 0.0000
reward_count: 0
helpful_count: 0
harmful_count: 0
---

# Learning: Strict Sub-Skill Delegation Pays for Its Cost

**Category**: process
**Confidence**: medium

## What We Learned

Strict sub-skill delegation (`/rpi` → `Skill(discovery)` → `Skill(crank)` → `Skill(validation)`, each as separate tool invocations with their own artifacts) took ~70 min wall-clock for a 6-issue meta-change to orchestrator skills. That's viable overhead for the contract-enforcement payoff: three independent judges (3 parallel Explore agents in discovery audit, 1 inline vibe judge in validation) collectively found all real issues in the work. Self-review would have shipped the Codex `--no-lifecycle` → `--no-scaffold` drift and the CHANGELOG "Fixed" overstatement.

The lesson: delegation cost is real (~20 min discovery + ~15 min validation = ~35 min of orchestration overhead on 6 small edits) but the defect escape rate from self-assessment is higher than from independent review. Time-per-finding beat self-review's time-per-miss.

## Source

Quick capture via `/retro --quick` — `/validation` phase of the 2026-04-19 RPI DAG hardening session. See `.agents/council/2026-04-19-post-mortem-rpi-dag-hardening.md` for the full post-mortem.
