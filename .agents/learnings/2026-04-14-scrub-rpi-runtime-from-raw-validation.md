---
id: learning-2026-04-14-scrub-rpi-runtime-from-raw-validation
type: learning
date: 2026-04-14
category: debugging
confidence: high
maturity: provisional
utility: 0.7
---

# Learning: Scrub AGENTOPS_RPI_RUNTIME in Raw Validation

## What We Learned

Raw Go validation can false-red when `AGENTOPS_RPI_RUNTIME` leaks from the host
shell. During the loop baseline, `AGENTOPS_RPI_RUNTIME=bushido` caused
`cli/internal/rpi` defaults tests to fail even though the same package passed
once the variable was scrubbed.

## Why It Matters

Without explicit env scrubbing, operators can misclassify environmental
contamination as a product regression and waste a loop on the wrong problem.

## Source

Post-mortem of the 2026-04-14 evolution loop baseline and follow-up bug
`na-kc2f`.
