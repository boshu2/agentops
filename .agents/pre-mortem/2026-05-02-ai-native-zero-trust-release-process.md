---
type: pre-mortem
date: 2026-05-02
plan: .agents/plans/2026-05-02-ai-native-zero-trust-release-process.md
epic: soc-owed
verdict: WARN
mode: quick
---

# Pre-Mortem: AI-Native Zero-Trust Release Process

## Verdict

WARN. The plan is valid, but two risks must be handled during implementation:

1. Digital twin can become theater if it only runs `ao version` in a temp
   directory. It must exercise install/upgrade/operator workflows.
2. A stricter release workflow can accidentally block public publishing on
   private host availability. HIL waivers must remain explicit and audited.

## Checks

| Pattern | Verdict | Notes |
| --- | --- | --- |
| Mechanical verification | PASS | Issues include BATS, shell syntax, parity, and artifact audit tests. |
| Self-assessment | PASS | Official readiness becomes evidence-file-backed. |
| Context rot | PASS | Plan and execution packet are file-backed. |
| Propagation blindness | PASS | Workflow, scripts, docs, schemas, and release artifacts are all included. |
| Dead infrastructure | WARN | Real HIL targets are not yet selected. Preserve waiver semantics. |
| Rollback/rescue | WARN | Retag/release flows need a clear recovery path if pre-publish evidence fails. |
| Four-surface closure | PASS | Code, docs, tests, and artifacts are all represented. |

## Required Hardening

- `soc-owed.3` must define digital-twin pass criteria before wiring it into
  official readiness.
- `soc-owed.5` must prove GoReleaser cannot publish on doc-only success.
- `soc-owed.6` must reject weak HIL evidence such as version-only smoke in
  official mode unless a waiver is explicitly recorded.

Proceed with implementation after target inventory decisions are made.
