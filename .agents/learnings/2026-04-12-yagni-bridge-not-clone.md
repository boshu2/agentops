---
date: 2026-04-12
category: architecture
maturity: provisional
utility: 0.9
confidence: 0.0000
reward_count: 0
helpful_count: 0
harmful_count: 0
---

# Learning: Bridge existing systems, don't clone them in a different language

## What We Learned

When a working system exists (JS dream/worker.js with 9K wiki pages, 8-job concurrency, queue management), building a Go clone of the same pipeline (chunker, ollama client, summarizer, session writer) produces a second disconnected system instead of accelerating the original. The result: two wiki surfaces (.agents/ao/sessions/ and vault/wiki/sources/) that don't talk to each other, double-processing of the same sessions, and 4K+ LOC that duplicates existing functionality.

The correct move: bridge to the existing system (enqueue to its queue, index its output, or mount its storage) with ~100 LOC of glue code instead of ~3000 LOC of reimplementation.

## Why It Matters

This is the #1 failure mode in multi-agent/multi-runtime systems: each agent builds its own version of a capability instead of composing with what exists. The cost is not just LOC — it's ongoing maintenance of two systems that drift apart.

## Detection Question

Before building a new pipeline: "Does a working version of this already exist somewhere in the ecosystem (different language, different host, different runtime)?" If yes, bridge before cloning.

## Source

Tier 1 forge session 2026-04-12. Built Go clone of JS worker. JS worker had 9K pages; Go version produced 3 broken pages before debugging.
