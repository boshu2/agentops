---
id: learning-2026-05-01-archive-manifests-are-promotion-dedupe-inputs
type: learning
date: 2026-05-01
category: architecture
confidence: high
maturity: provisional
utility: 0.7
harmful_count: 0
reward_count: 0
helpful_count: 0
---

# Learning: Cleanup Archives Are Promotion Dedupe Inputs

## What We Learned

After corpus cleanup, already-known learning and pattern bodies may live outside
the active `.agents/learnings/` and `.agents/patterns/` directories. Promotion
dedupe must treat cleanup archives such as `.agents/archive/dedup/` and
`.agents/defrag/*/files/.agents/{learnings,patterns}/` as part of the known-body
set, otherwise stale pending files can recreate bodies that cleanup already
classified as seen-again drift.

## Why It Matters

`promoted-index.jsonl` can be missing, empty, or stale. A safe AgentOps lifecycle
path needs a canonical semantic-body hash and a live known-body scan that covers
active artifacts and cleanup archives before close-loop, startup maintenance,
pool promotion, or batch promotion writes new learning/pattern artifacts.

## Source

Post-mortem of `fix(pool): block recreation of archived knowledge bodies`
(`075d3178`, 2026-05-01) and Mt. Olympus dogfood validation after corpus cleanup.
