---
utility: 0.7966
last_reward: 0.80
reward_count: 253
last_reward_at: 2026-03-31T09:33:17-04:00
confidence: 0.8314
last_decay_at: 2026-04-11T23:02:47-04:00
helpful_count: 252
maturity: established
maturity_changed_at: 2026-03-03T10:14:50-05:00
maturity_reason: utility 0.73 >= 0.55, reward_count 6 >= 5, helpful > harmful (5 > 0)
---

# Warn-Then-Fail Ratchet

## Pattern

Introduce new quality gates as warn-only checks first, then flip them to
blocking only after the metric has enough baseline data and the team has fixed
known false positives.

## Why It Works

Fresh gates often encode the right principle but the wrong threshold. Warn-only
mode lets the project collect evidence, tune the metric, and avoid breaking CI
for noise. Once the baseline is stable, the same check becomes a fail gate so the
quality bar ratchets upward instead of drifting.

## How To Apply

1. Define the metric and the exact command that measures it.
2. Record the current baseline and known false positives.
3. Run warn-only until the corpus or sample size is representative.
4. File tracked beads for every real defect the warning finds.
5. Flip to fail-only after the threshold is stable and documented.

## Retrieval Cues

Warn then fail ratchet, warn-only gate, quality ratchet, baseline metric,
retrieval quality metric, pre-push ratchet, threshold hardening, CI gate.
