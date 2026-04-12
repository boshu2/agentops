---
utility: 0.7951
last_reward: 0.80
reward_count: 152
last_reward_at: 2026-03-31T09:33:17-04:00
confidence: 0.8140
last_decay_at: 2026-04-12T12:33:50-04:00
helpful_count: 151
maturity: established
maturity_changed_at: 2026-03-05T09:08:21-05:00
maturity_reason: utility 0.70 >= 0.55, reward_count 5 >= 5, helpful > harmful (4 > 0)
---

# Council Judges Pattern

## Pattern

Use council judges when a decision benefits from independent perspectives before
implementation or closure. Give each judge the same question and evidence, ask
for a concrete verdict, then consolidate only the actionable disagreements and
shared findings.

## Why It Works

Parallel judges reduce single-agent blind spots. They are most useful for
pre-mortems, product validation, security-sensitive choices, and post-mortems
where the implementation needs an adversarial review before the bead closes.

## How To Apply

1. Write one narrow decision question.
2. Provide the relevant plan, diff, evidence, and constraints.
3. Ask each judge for verdict, blocking concerns, non-blocking concerns, and
   required validation.
4. Merge the results into a small consensus packet.
5. Act only on findings that are specific enough to validate.

## Retrieval Cues

Council judges pattern, multi-model review, consensus council, pre-mortem judge,
validate plan, independent verdicts, adversarial review, judge consolidation.
