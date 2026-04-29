---
id: design-2026-04-29-eval-suite-triage
type: design
date: 2026-04-29
---

# Design: Eval Suite Triage

## Goal Statement

Analyze and refactor the AgentOps evaluation suite so it clearly separates contract canaries, gate wrappers, behavioral fixtures, baseline comparisons, scorecards, live runtime checks, and private holdouts; then improve, merge, or remove checks based on the signal they provide.

## Alignment Matrix

| Dimension | Score | Rationale |
|-----------|-------|-----------|
| Gap Alignment | 3/3 | Directly addresses judgment validation, loop closure, and the known gap that multi-runtime proof is tiered rather than complete. |
| Persona Fit | 3/3 | Helps maintainers and orchestrators decide whether agent, skill, runtime, and gate changes improved or regressed the system. |
| Competitive Diff | 3/3 | Strengthens AgentOps' moat as an operational layer that turns validation and learning into repeatable evidence. |
| Precedent | 3/3 | Builds on the existing eval-environment longhaul, promoted baselines, scorecards, holdout isolation, and model-upgrade canaries. |
| Scope Fit | 2/3 | The goal is broad, but a six-issue staged plan keeps the first pass focused on taxonomy, governance, and high-signal refactors. |

Average: 2.8/3.0

## Inline Assessment

The goal fits PRODUCT.md's mission: AgentOps is valuable when bookkeeping, validation, and flows compound into better next sessions. The current eval suite is already a key proof surface, but it needs signal taxonomy and culling discipline so "eval passed" means something precise. The plan should not replace deterministic PR gates with live model runs; it should establish a layered proof model and keep live/holdout checks advisory or release-scoped until they are calibrated.

## Final Verdict

DESIGN VERDICT: PASS

Recommendation: proceed with discovery and planning. The plan should explicitly preserve deterministic public canaries, define what each canary proves, and avoid making live runtime/model checks blocking in normal PR CI.
