---
title: Agents Control Plane Hardening Pre-Mortem
date: 2026-04-25
skill: agentops:discovery
verdict: WARN
---

# Pre-Mortem: `.agents` Operator Control Plane Hardening

## Verdict

WARN. The packet is worth doing and the risks are manageable, but the work spans
CLI behavior, shell gates, docs, and local shared-agent state. The mitigation is
strict sequencing by bead dependency and narrow read-only diagnostics.

## Risk 1: `doctor` Becomes A Repair Tool

`ao agents doctor` could drift into mutation or migration behavior, which would
increase risk and blur operator intent.

Mitigation: keep `doctor` diagnostic-only. It may report status, unknown dirs,
next commands, and structured JSON. It must not delete, migrate, or rewrite
`.agents` content.

## Risk 2: Repo-Root Resolution Picks The Wrong Root

Using a generic parent search could accidentally treat an unrelated parent
`.agents` directory as the project root.

Mitigation: resolve the project root through repo markers and existing command
helpers, then test from repo root, `cli/`, and a temp directory. Do not rely only
on the nearest `.agents` directory.

## Risk 3: Lint JSON Becomes Fragile

Adding source-location evidence in shell can create invalid JSON if paths or
subdir names are escaped by hand.

Mitigation: use `jq` or another existing structured path where practical, and
add BATS coverage that validates JSON with `jq`.

## Risk 4: Local Hash-Gate Tuning Weakens CI

Making the local gate less noisy under concurrent agents could accidentally
allow CI drift to pass.

Mitigation: keep fail-open behavior limited to documented local conditions.
Tests should assert CI strictness separately from local override behavior.

## Risk 5: Documentation Gets Ahead Of Code

The operator guide could describe `doctor`, source-location lint, or hash-gate
triage before the implementation is finalized.

Mitigation: keep `agentops-gpu.7` blocked on `.2`, `.3`, and `.5`; update docs
after behavior is implemented and validated.

## Decision

Proceed with the plan. Accept WARN because every risk has a narrow mitigation
and the dependency graph prevents the highest-risk ordering mistakes.
