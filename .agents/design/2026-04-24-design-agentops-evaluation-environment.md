---
id: design-2026-04-24-agentops-evaluation-environment
type: design
date: 2026-04-24
---

# Design: AgentOps Evaluation Environment

## Goal

Design an evaluation environment for AgentOps that can repeatedly measure whether changes to skills, hooks, and the `ao` CLI make the system better or worse across RPI flows, Claude/Codex runtime behavior, and new model releases.

## Alignment Matrix

| Dimension | Score | Rationale |
|-----------|-------|-----------|
| Gap Alignment | 3/3 | Directly addresses the PRODUCT.md gap that multi-runtime proof is tiered rather than complete, and strengthens the validation/flywheel proof model. |
| Persona Fit | 3/3 | Serves the Quality-First Maintainer, Agent Orchestrator, and Solo Developer by turning agent behavior quality from opinion into repeatable evidence. |
| Competitive Diff | 3/3 | A repo-native eval harness for skills, hooks, CLI behavior, and model migration would make AgentOps' compounding claims mechanically provable rather than narrative. |
| Precedent | 2/3 | Existing headless runtime smoke, Codex parity audits, hooks parity, skill integrity checks, and council/pre-mortem flows provide partial patterns, but not an end-to-end eval baseline. |
| Scope Fit | 2/3 | The objective is broad, but can be staged as baseline capture, scenario runner, scoring contract, dashboards, and gate integration without blocking on a perfect harness. |

Average: 2.6/3.0

## Inline Assessment

The capability is strongly aligned with AgentOps' mission: if the product claims that sessions compound, changes must be scored against stable behavioral baselines. The current product already has structural validation and runtime smoke, but lacks a persistent scenario corpus, model/runtime comparison harness, and trendable quality metrics for skills such as RPI.

## Final Verdict

DESIGN VERDICT: PASS

Recommendation: proceed to research and planning. Treat this as a new validation subsystem rather than a one-off script, with the first milestone focused on repeatable baseline scenarios and scorecards before any CI-blocking gates are introduced.
