---
utility: 0.7952
last_reward: 0.80
reward_count: 132
last_reward_at: 2026-04-11T20:02:42-04:00
confidence: 0.9541
last_decay_at: 2026-04-12T12:33:50-04:00
helpful_count: 131
maturity: established
maturity_changed_at: 2026-03-05T15:24:08-05:00
maturity_reason: utility 0.70 >= 0.55, reward_count 5 >= 5, helpful > harmful (4 > 0)
---

# Pre-Mortem First

## Pattern

Run the pre-mortem before implementation, not after the first patch. A
pre-mortem asks whether the plan is ready to implement, what will probably fail,
and what evidence would prove the work is safe to start.

## Why It Works

Most agent failures are plan-quality failures that only become expensive once
code is already edited. A short pre-mortem catches unclear acceptance criteria,
missing ownership boundaries, weak validation, and hidden dependency conflicts
while they are still cheap to correct.

## How To Apply

1. State the implementation objective in one sentence.
2. Name the files or surfaces allowed to change.
3. List the expected validation commands.
4. Ask what would make the work fail in CI, product fit, security, or handoff.
5. Tighten the plan until the remaining risks are explicit and acceptable.

## Retrieval Cues

Pre-mortem first, validate plan, council judges, implementation readiness,
failure forecast, plan gate, acceptance criteria, validation before coding.
