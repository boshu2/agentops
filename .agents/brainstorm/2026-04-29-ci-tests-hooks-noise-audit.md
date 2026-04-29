---
id: brainstorm-2026-04-29-ci-tests-hooks-noise-audit
type: brainstorm
date: 2026-04-29
---
# Brainstorm: CI Tests and Hooks Noise Audit

## Problem Statement

Audit the CI, local validation, and hook surfaces so every blocking failure maps to a clear AgentOps product contract, every advisory signal is explicitly labeled, and duplicated or count-based checks are replaced or reinforced with behavior-focused coverage.

## Approaches Considered

1. **Prune noisy checks first.** Remove or demote checks that frequently warn or duplicate other gates. This is small effort, but high risk because it can hide real release regressions without first identifying each gate's purpose.
2. **Contract-map all gates first.** Inventory CI jobs, local gates, hooks, and tests; classify each by purpose, blocking policy, runtime surface, and failure mode; then file focused remediation work. This is medium effort and gives the cleanest path to reducing noise without weakening coverage.
3. **Golden pipeline rewrite.** Redesign CI and hooks around a new declarative manifest that drives docs, workflow summary, local gates, and tests. This could eliminate drift long-term, but it is large scope and likely too much for a first audit pass.

## Selected Approach

Use the contract-map approach. The execution plan should create a source-of-truth inventory, identify high-noise/advisory surfaces, strengthen weak tests with behavioral assertions, and only then demote, merge, or remove redundant checks.

## Open Questions

- Which current warnings are accepted product signals versus operator-facing noise?
- Should local `--fast` be optimized for changed-file relevance only, or also for low-noise output shape?
- Which hook warnings should remain interactive context, and which belong only in logs or explicit health commands?
- Should CI/local gate metadata be centralized in a machine-readable manifest, or is a smaller parity contract enough for this iteration?

## Next Step: /plan

Run `/plan --auto "audit CI tests and hooks to reduce noise by classifying every gate and hook by product contract, strengthening behavior-focused tests, and filing remediation issues for redundant or noisy checks"`.
