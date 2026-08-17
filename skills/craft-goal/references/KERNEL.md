# Goal graph and ratchet kernel

## Bead graph contract

Record each experiment with its question, acceptance gap, method, expected
observation, falsifier, scope, non-goals, resumable notes, verdict/evidence,
and observed learning. Use `parent-child` for goal membership, `blocks` for
real ordering, `related` for alternatives, and `discovered-from` for
provenance. Live tracker state outranks a static plan.

The tracker is caller-owned. Crafting a prompt starts no tracker, goal, or
experiment and grants no authority to mutate one.

## Ratchet test

An experiment progresses the goal only when it does at least one:

1. proves part of terminal acceptance;
2. falsifies a live hypothesis with discriminating evidence and prunes it;
3. resolves an uncertainty or owner so the next experiment is materially
   different.

Code volume, commits, repeated errors, and rewritten plans are not progress by
themselves. A NOT_PROVEN result counts only when it narrows the next question.
Repetition without new information increments the no-progress counter.

## Mayor transition

At a wave boundary, reconstruct frozen acceptance, remaining graph, verdicts,
uncertainties, and monotonic budgets. Select only experiments tied to an unmet
criterion or blocking uncertainty. Each selected bead gets one RPI and one
author-distinct fresh verdict. Preserve the evidence, classify discoveries,
then checkpoint ratchet versus churn.

A discovery necessary for frozen acceptance may become a later
`discovered-from` child within authority and budget. Useful nonessential work
is linked for later. Acceptance changes, authority changes, and work beyond the
hard envelope enter HOLD.
