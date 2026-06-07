# idea-wizard — methodology (full per-phase mechanics)

The funnel in detail. Each phase ends with a checkpoint; do not advance until it passes.
The whole funnel runs in **plan space** — no files edited, no implementation code — until
Phase 8 completes and you deliberately exit to implementation.

## Phase 1 — Frame

State the project and the improvement **axis** in one sentence. The axis is the lens that
makes generation productive instead of scattershot. Common axes:

- **Performance** — latency, throughput, memory, cold-start.
- **Developer experience (DX)** — build/test speed, ergonomics, onboarding.
- **Reliability** — error handling, idempotency, recovery, observability.
- **User experience (UX)** — flows, defaults, error messages, docs.
- **Cost** — infra spend, token spend, redundant work.
- **Maintainability** — coupling, dead code, test coverage, complexity hotspots.

If the request is "make it better" with no axis, either pick the most valuable axis and say
so, or run the funnel once per axis. **Checkpoint:** the axis is concrete and named.

## Phase 2 — Generate (~30, wide)

List ~30 candidate improvements, numbered, with **zero evaluation**. Techniques to reach the
long tail instead of 30 rewordings of the obvious:

- Walk the system end to end (entry → core → edges) and name a candidate at each layer.
- Invert: "what would make this *worse*?" then negate.
- Borrow: "how does a best-in-class peer do this?"
- Cross axes: even with a primary axis, let a few candidates come from adjacent axes.
- Stretch: include 3–4 deliberately ambitious / "probably too big" candidates.

**Checkpoint:** ≥ 25 distinct candidates; a visible long tail (not all variants of one idea).

## Phase 3 — Think

For every candidate, write exactly three short lines:

- **Upside** — what improves, and roughly how much.
- **Cost** — effort / blast radius / who's affected.
- **Risk** — what could go wrong, what's uncertain.

This is the evidence the winnow runs on. Keep it terse — one line each, not an essay.
**Checkpoint:** every candidate has upside / cost / risk.

## Phase 4 — Winnow (to the best 5)

Cut hard. State the cut criteria explicitly before cutting, e.g. weight by
`upside ÷ cost`, discard anything whose risk is unbounded or unknowable, prefer reversible
changes. For **every** cut idea record a one-clause reason (it stays in the ledger as a
graveyard — useful when a later pass resurrects one).

**Checkpoint:** ~5 survivors; each cut has a recorded reason.

## Phase 5 — Expand (~15)

Decompose each of the 5 survivors into ~3 concrete sub-ideas (~15 total). A survivor like
"speed up cold start" becomes "lazy-load plugins", "cache the parsed config", "defer the
network health check". The sub-ideas are what actually become issues — they're sized to file
and verify. **Checkpoint:** each survivor is broken into actionable sub-ideas.

## Phase 6 — Overlap-check against tracked work

Enumerate existing tracked work and reconcile **before** filing anything:

```bash
br list            # everything tracked for this project
br ready           # what's already unblocked / in flight
br show <id>       # inspect a candidate match for a real overlap
```

For each of the ~15 sub-ideas, decide and **record**:

- **NEW** — nothing covers it → file in Phase 7.
- **MERGE** — an open issue covers part of it → add the missing context to that issue
  instead of filing a twin.
- **DROP** — already fully covered → discard with a note.

**Checkpoint:** every sub-idea has a NEW/MERGE/DROP decision.

## Phase 7 — Operationalize (self-documenting issues + deps + tests)

File each NEW sub-idea as a tracked issue. The exact prompt blocks and the issue-body
template are in [prompts.md](prompts.md). The rules:

- **Self-documenting body.** A cold reader must be able to act on it: problem, why it
  matters, and the acceptance check ("done when …").
- **Dependencies wired.** If improvement B needs A first, `br dep add B A`.
- **A test task per winner.** File a child "Test: verify X" issue and make it depend on the
  improvement. The test task names *how* you'll prove the change is real (a measurement, a
  new assertion, a manual check).

**Checkpoint:** every filed issue is self-documenting and has ≥ 1 dependent test task.

## Phase 8 — Refine in plan space (several passes)

Re-read the filed set as a whole, **before any implementation**, and iterate:

1. **Split** issues that turned out too big to land in one move.
2. **Merge** near-duplicates that survived overlap-check.
3. **Reorder** dependencies so `br ready` surfaces the correct first move.
4. **Re-check** each issue is still self-documenting after the edits.

Run passes until a full pass changes nothing — that's the stable backlog. Only then exit
plan space and begin implementation (which is out of this skill's scope).

## The graveyard is an asset

Keep the cut ideas and the DROP/MERGE notes in the ledger. On a later refinement pass, a
graveyard idea sometimes becomes viable (a dependency landed, the cost dropped). Resurrecting
from a recorded reason is cheap; regenerating from scratch is not.
