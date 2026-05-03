---
id: learning-fixture-near-breach
type: learning
date: 2026-04-01
category: testing
confidence: medium
maturity: provisional
utility: 0.36
harmful_count: 2
reward_count: 0
---

# Near-breach Fixture

This fixture sits one regressed verdict away from the harmful>=3 + utility<0.3
threshold-breach that queues a verdict-driven retire candidate to next-work.

After EMA(0.7, 0.3) update with a fresh utility of 0.05:

```text
new_utility = 0.7 * 0.36 + 0.3 * 0.05 = 0.252 + 0.015 = 0.267 (< 0.3)
new_harmful = 2 + 1 = 3 (>= 3)
```
