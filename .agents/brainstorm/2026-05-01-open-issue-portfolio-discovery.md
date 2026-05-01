---
id: brainstorm-2026-05-01-open-issue-portfolio-discovery
type: brainstorm
date: 2026-05-01
---

# Brainstorm: Open Issue Portfolio Discovery

## Problem Statement

The repo had 142 open bd issues before this discovery started, spread across
15 epics, 34 unparented non-epic issues, and several overlapping daemon,
Dream, OpenClaw, Mt. Olympus, Bushido, eval, and cross-repo tracks. The
problem is not only issue volume. The issue graph also contains duplicate
failure modes, blocked migration lanes, active P0 flywheel cleanup, and broad
P1/P2 ready work that can pull execution in too many directions.

## Approaches Considered

### Approach A: Strict Priority Drain

Execute `bd ready` in priority order, starting with P0 and all ready P1s.
This is simple and uses the tracker as-is.

Pros:

- Fastest path to the active P0.
- Requires no new taxonomy or planning overhead.
- Keeps implementation close to current bd ordering.

Cons:

- Thrashes across repos and subsystems.
- Leaves duplicate and stale issues in place.
- Does not force a choice between overlapping daemon and Dream migration lanes.

Risk: HIGH. It fails the hidden-cost check because it converts portfolio
confusion into execution churn.

### Approach B: Groom First, Then Execute

Stop implementation and run a full backlog hygiene pass first: merge
duplicates, close stale issues, reparent or defer unparented items, and then
resume execution.

Pros:

- Could reduce the 142 issue count quickly.
- Improves `bd ready` signal before work starts.
- Catches stale blockers and duplicate dry-run/flywheel bugs.

Cons:

- Delays active P0 validation.
- Risks spending the whole session curating instead of shipping.
- Requires judgment across several personal repos and external contexts.

Risk: MEDIUM. It is useful, but only after the P0 is stabilized.

### Approach C: Epic Lane Waves

Keep existing implementation epics intact and define portfolio waves around
the current issue graph: incident first, graph normalization second, active
local epics third, one migration lane fourth, cross-repo routing fifth.

Pros:

- Preserves existing bead work and avoids duplicate implementation tasks.
- Reduces concurrency across overlapping daemon/Dream/Mt. Olympus lanes.
- Makes P0, P1, and blocked work explicit without needing a full rewrite.

Cons:

- Adds a small coordinating epic.
- Still requires operator decisions for cross-repo and external-host work.
- Does not immediately reduce the total issue count.

Risk: LOW. The main assumption is that routing work should not reparent or
close existing implementation beads without evidence.

### Approach D: Product Gap Selection

Ignore raw priority and choose work by PRODUCT.md gaps: flywheel/retrieval,
Dream autonomy, multi-runtime proof, and messaging.

Pros:

- Aligns to the product's stated moat.
- Keeps attention on compounding, validation, and worker-context quality.
- Provides a principled way to defer lower-value personal-site/showcase work.

Cons:

- Can underweight operational and security tasks.
- Does not by itself fix tracker clutter.
- Requires translating issue titles into product-gap impact.

Risk: MEDIUM. Useful as a tie-breaker, not as the sole execution policy.

## Selected Approach

Use a hybrid of Approach C and D:

1. Stabilize the P0 and tracker/worktree hygiene first.
2. Normalize duplicates, stale blockers, and unparented P1/P2 items.
3. Drain already-discovered local execution epics before opening new work.
4. Pick exactly one daemon/Dream/Mt. Olympus migration lane at a time.
5. Route cross-repo/security and P3/P4 work into execute, delegate, defer, or
   close decisions.

This keeps issue execution grounded in the existing bd graph while using
PRODUCT.md gaps as a tie-breaker when multiple lanes compete.

## Open Questions

- Should `soc-7wwp` or `soc-qvpb` be the canonical RPI dry-run alias bug, or
  should both be closed in favor of a newer combined bead?
- Should `psite-agu`, `soc-ni8g`, and `soc-q4c` remain separate migration
  epics, or should one be declared the current active lane and the others
  deferred with explicit dates?
- Which cross-repo issues are still actionable from this repo versus needing a
  linked worktree in their owning repo?
- After the P0 flywheel pollution fix is verified, should the next execution
  lane be PR-toil reduction, Bushido CLI, or nightly automation?

## Next Step: /plan

Run:

```text
/plan --auto "Analyze all open bd issues and route them into an incident-first portfolio execution strategy without duplicating existing implementation epics"
```
