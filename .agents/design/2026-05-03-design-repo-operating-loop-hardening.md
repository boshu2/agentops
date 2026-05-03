---
id: design-2026-05-03-repo-operating-loop-hardening
type: design
date: 2026-05-03
goal: repo-operating-loop-hardening
verdict: PASS
---

# Design: Repo Operating Loop Hardening

## Goal Statement

Plan a repo hardening initiative that makes AgentOps validation and release
work boring: non-mutating by default, hook/plugin installs self-consistent,
`.agents` policy explicit, canonical root cleanliness enforced mechanically,
release gates decoupled from stale historical artifacts, and bd/Dolt command
behavior bounded enough for automation.

## Alignment Matrix

| Dimension | Score | Rationale |
| --- | ---: | --- |
| Gap Alignment | 3 | Directly addresses reliability, loop closure, and corpus hygiene gaps called out in `PRODUCT.md`. |
| Persona Fit | 3 | Helps solo developers, orchestrators, and quality-first maintainers by making validation predictable. |
| Competitive Diff | 3 | Strengthens the operational-discipline moat: the corpus is trustworthy because gates do not secretly rewrite it. |
| Precedent | 2 | Prior work exists in release gates, worktree disposition, hash snapshots, and state-path contracts; the pieces need unification. |
| Scope Fit | 2 | The initiative is too broad for one implementation wave, but it is appropriate as a discovery/plan epic with serialized slices. |

Average: 2.6/3.0

## Council Verdict

Quick inline product assessment: PASS with scope warning. The work is on-mission
and improves the bridge-tool reliability needed to preserve the user's corpus.
The risk is not product alignment; the risk is trying to land all hardening at
once instead of sequencing high-leverage guards first.

## Final Verdict

DESIGN VERDICT: PASS

Recommendation: continue discovery. Plan as a full initiative with separate
tracks for validation mutation boundaries, hook packaging, `.agents` policy,
worktree governance, release-artifact validation scope, and bd automation
boundedness.
