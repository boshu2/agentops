---
id: learning-2026-04-26-dream-generator-sidecar-single-writer
type: learning
date: 2026-04-26
category: architecture
confidence: high
maturity: provisional
utility: 0.7
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: Dream Generator Sidecars Need One Writer

## What We Learned

Dream can safely add read-side parallelism when each generator emits a durable
sidecar and a single REDUCE stage owns the `next-work.jsonl` append.

## Why It Matters

This keeps candidate discovery scalable without moving `/evolve` away from its
serial, fitness-gated execution model.

## Source

Post-mortem of PR #155, especially `cli/internal/overnight/ingest.go`,
`cli/internal/overnight/generator_sidecars.go`, and
`docs/contracts/dream-run-contract.md`.
