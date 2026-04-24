---
id: learning-2026-04-14-dream-longhaul-postmortem
type: learning
date: 2026-04-14
category: process
confidence: high
maturity: provisional
utility: 0.8
harmful_count: 0
reward_count: 0
helpful_count: 0
---

# Learning: Dream Long-Haul Needs Durable Proof and Cheap Corroboration

## What We Learned

### L1: Cheap packet corroboration is the right thing to spend Dream time on before council
The long-haul controller earned its keep when it used existing Dream evidence
to raise the top morning packet from medium to high confidence and then skipped
the more expensive council lane. That preserved the short-path default while
still improving morning output when the first pass was weak but recoverable.

### L2: Closed beads need durable proof surfaces, not ephemeral seed paths
The code for `na-22xi` shipped cleanly, but post-mortem still found replay
failures because two closed child beads cited `.agents/brainstorm/...` and
`.agents/research/...` seed files that are not present in the repo. If closure
evidence is not durable, the flywheel loses mechanical replay even when the
implementation itself is sound.

## Why It Matters

Dream should spend extra runtime on the cheapest probe that can improve the
morning handoff, and the issue tracker should only claim proof that later
audits can actually replay. Those two constraints keep Dream useful without
turning it into unmeasured latency or unverifiable bookkeeping.

## Source

Post-mortem of epic `na-22xi` on 2026-04-14, covering commits `a01235a3`
(`feat(dream): add adaptive long-haul corroboration`) and `0cfb0c44`
(`fix(gate): fallback retrieval ratchet manifest`).
