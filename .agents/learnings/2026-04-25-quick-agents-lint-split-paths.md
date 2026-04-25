---
id: learning-2026-04-25-agents-lint-split-paths
type: learning
source: retro-quick
source_bead: agentops-gpu
source_phase: validate
date: 2026-04-25
category: validation
confidence: high
maturity: provisional
utility: 0.7
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: Agents Lint Must Detect Split Path Construction

## What We Learned

The `.agents` write-surface lint cannot rely only on literal
`.agents/<subdir>` strings. Real production code also constructs surfaces with
split joins such as `filepath.Join(cwd, ".agents", "wiki", "sources")`, which
made legitimate surfaces invisible to the old contract check.

## Why It Matters

Source-location lint evidence is more useful when it covers both literal and
split path construction. It turns a clean-looking contract into a stronger
operator control-plane check and prevents on-disk doctor diagnostics from
finding surfaces the lint never saw.

## Source

Validation of `agentops-gpu`, which added `ao agents doctor` and expanded
`scripts/check-agents-write-surfaces.sh` to detect split Go joins.
