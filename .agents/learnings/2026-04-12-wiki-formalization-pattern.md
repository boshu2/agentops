---
date: 2026-04-12
category: architecture
maturity: provisional
utility: 0.8
---

# Learning: INDEX.md is the cheapest high-leverage addition to any .agents/ directory

## What We Learned

Adding INDEX.md (a wikilinked catalog) and LOG.md (append-only operation log) to an existing .agents/ directory turns it into a Karpathy-style wiki with ~2 hours of work. The key insight: .agents/ already had 80% of the wiki pattern (51 subdirs, wikilinks, frontmatter, BM25 search). The missing 20% was just two navigation files and a dedicated wiki/ subdirectory.

The INDEX.md approach also solves token optimization: agents read INDEX.md first (~2K tokens), identify relevant pages, then load only those — instead of spraying everything into context.

## Why It Matters

Any repo using .agents/ can add INDEX.md + LOG.md and immediately get: Obsidian-browsable knowledge, Karpathy-style query (read index → follow links), and a chronological audit trail. Zero Go code required for the basic pattern — just markdown files and a shell script.

## Source

Wiki formalization RPI (2026-04-12). Commit 38a7cc52. Plan at .agents/plans/2026-04-12-agents-wiki-formalization.md.
