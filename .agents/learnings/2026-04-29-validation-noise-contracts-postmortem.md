---
id: learning-2026-04-29-validation-noise-contracts-postmortem
type: learning
date: 2026-04-29
category: testing
confidence: high
maturity: provisional
utility: 0.5
---

# Learning: Validation Noise Contracts Need Scoped Proof

## What We Learned

Broad validation gates can become noisy when a tooling-only change triggers a
validator that depends on unrelated local artifacts or live runtime inventory.
The durable fix is to scope validators to the changed proof surface and pair
every accepted behavior with a fixture that fails before the implementation
change.

## Why It Matters

This keeps high-signal gates blocking real drift while preventing another
session's local state from obscuring whether the current branch is correct.

## Source

Post-mortem for `agentops-dtg`, branch `rpi/agentops-dtg-ci-noise-audit`.
