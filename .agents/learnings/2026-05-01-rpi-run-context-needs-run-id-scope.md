---
id: learning-2026-05-01-rpi-run-context-needs-run-id-scope
type: learning
date: 2026-05-01
category: process
confidence: high
maturity: provisional
utility: 0.7
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: RPI Run Context Needs Run-ID Scope

## What We Learned

RPI phase handoff readers must scope structured handoffs and legacy summary
fallbacks to the current run ID before injecting them into later phase prompts.
Otherwise a fresh run can inherit stale goals, summaries, or implementation
context from unrelated prior RPI sessions.

## Why It Matters

Autonomous phased runs depend on clean phase boundaries. Stale handoff injection
can send implementation or validation down the wrong objective while still
looking like a valid RPI prompt.

## Source

Post-mortem for `soc-b8jo` nightly/RPI E2E test sessions and commit `80a21e2e`.
