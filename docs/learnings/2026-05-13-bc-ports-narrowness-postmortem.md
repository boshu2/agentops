# BC Ports — phase-2 narrowness post-mortem

**Date:** 2026-05-13
**Trigger:** three consecutive phase-2 migration attempts (cycles 155-157) all revealed schema mismatches between the typed ports and their actual consumers
**Status:** durable learning (epic-level finding, not a per-cycle one)

## What we tried

After the rescope doc (`docs/rescope/2026-05-13-ddd-hex-architecture-rescope.md`) declared phase-1 (port + adapter extraction) complete and phase-2 (consumer migration) pending, we picked three migration targets:

1. `cli/internal/lifecycle/defrag.go::SweepOscillatingGoals` → `LoopReaderPort`
2. `cli/internal/quality/metrics_health.go::LoadCycleHistory` → `LoopReaderPort`
3. `cli/cmd/ao/context_assemble.go::gatherHistory` → `LoopReaderPort`

Each was supposed to be a 1-2 cycle drop-in: grep the direct I/O, swap to the typed port, keep regression tests green.

None of the three was a drop-in.

## What we found

| Target | Finding | Outcome |
|---|---|---|
| `defrag.go` | Reads a `Target` field that `CycleEntry` does not project. Worse: zero entries in `cycle-history.jsonl` actually have a `target` field. The production sweep has been a permanent no-op against a per-goal schema that never materialized. | Deferred (`soc-1q1x`) pending schema decision |
| `metrics_health.go` | `quality.LoadCycleHistory` was dead code with zero production callers — defined twice, tested directly, called by nothing in production code. | Deleted in cycle 156 (~46 lines) |
| `context_assemble.go` | `gatherHistory` consumes 13 fields (timestamp, cycle, target, goal_ids, result/status, sha, canonical_sha, log_sha, goals_passing, goals_total, summary, error, plus aliases). `CycleEntry` projects only 5 (Number, Mode, Result, Commit, Milestone). Migration would drop almost everything the formatter needs. | Deferred (`soc-0pku`) — same blocker as defrag.go |

## The single load-bearing observation

**The 14 BC ports are over-narrow for the consumers that already exist.**

Each port projects a deliberately minimal model — the cycle-108 `LoopReaderPort` doc comment even states "the port surface stays narrow." That was a virtuous-sounding design choice in isolation, but every real consumer of cycle-history.jsonl uses a *flexible* read model with field aliasing (`primary: "result", aliases: ["status"]`) and consumes fields that don't appear in the port projection at all (timestamp, target, goal_ids, sha, summary).

The narrow surface was a feature on paper and a bug in practice. We extracted ports against a *theoretical* schema (the minimum we thought sufficient) instead of against the *empirical* schema (what callers actually need).

## Why this happened (three causes)

1. **Ports were extracted before consumers were studied.** The cycle 100-122 wire-up arc extracted 14 ports by reasoning forward from "what does BC3 mean abstractly?" instead of backward from "what do `defrag`, `metrics_health`, `context_assemble`, `dream-compounder`, `post-mortem`, and the daemon actually read?"

2. **The on-disk format is hand-edited and field-flexible.** Operators append cycle-history entries with whatever fields make sense for the cycle (e.g., `milestone`, `validator_after`, `net_change`, `work_ref`). A narrow `CycleEntry` struct can't model that. The format is naturally `map[string]any`, not `struct`.

3. **Test doubles confirmed the wrong shape.** In-memory test doubles (`InMemoryLoopReader`) faithfully implement the narrow port, so the port's tests pass with flying colors. The tests verify the contract, not its fitness for purpose. We had 100% test coverage of the wrong abstraction.

## What this changes about the rescope

The phase-2 plan was "migrate 3 consumers in 1-2 cycles each." Reality is:

- 1 target was dead code (delete, not migrate). DONE.
- 2 targets need schema work before migration is even possible.

Phase 2 is **not** 6 hours of mechanical work. It's a design decision: do we widen `CycleEntry` to be a structured superset of what callers need, or do we accept that some consumers must stay outside the port?

### Option A — widen `CycleEntry`

Pros: Migrations unblock; ports become useful for real consumers.
Cons: Port surface expands by ~10 fields; some are optional and won't be set by all writers; the "narrow" design philosophy is abandoned. The `var _ LoopReaderPort = (*productionLoopReader)(nil)` assertions still hold, but the port no longer protects against schema drift — it just enforces field presence.

### Option B — add `ReadRaw(ctx) ([]map[string]any, error)` to LoopReaderPort

Pros: Keeps the typed surface for cycle-counter-style queries (Latest, Range, IdleStreak); admits the reality that some consumers need flexible access.
Cons: A `map[string]any` accessor on a typed port is a code smell — the port is half-typed at that point.

### Option C — accept that cycle-history is fundamentally a log, not a domain entity

Pros: Honest. Cycle history is operator-flexible JSONL, not a stable domain model. Maybe BC3 was the wrong bounded context to extract from this artifact.
Cons: Concedes that the LoopReaderPort was never the right abstraction. Phase-1 of the arc was partly misdirected work.

## Recommended next move

**Pick option A** but only do it on demand: when a real consumer needs a field, add it to `CycleEntry` then. Don't try to model the full superset preemptively (that recreates the original mistake in the other direction).

Concrete: extend `CycleEntry` with `Timestamp time.Time` and `Summary string` as a minimum to unblock `context_assemble.go`'s migration. Then re-attempt `soc-0pku`. Leave `Target` / `goal_ids` for a separate decision tied to whether the oscillation-detection schema ever materializes.

## Lessons (durable)

1. **Extract ports from real callers, not theoretical contexts.** Read every existing consumer's read-model before defining the port's projection. If three different callers read three different field sets, the port either needs to be wider or a different one needs to exist.
2. **Test doubles validate the contract, not its fitness.** A green test suite for an in-memory adapter does not mean the abstraction is right — it means the abstraction is internally consistent.
3. **Narrow ports against flexible logs are an impedance mismatch.** Operator-curated JSONL with hand-added fields is a flexible-schema artifact. Modeling it with a fixed struct discards the flexibility that made operators choose JSONL.
4. **Phase-2 migrations are diagnostic.** Every failed migration attempt teaches you what the port should have looked like. Don't bury the finding by force-fitting the migration; let the failure narrow the design.

## Cycle-by-cycle summary

| Cycle | Action | Outcome |
|---|---|---|
| 155 (rescope) | Declared phase-1 done, phase-2 pending; filed soc-1q1x, soc-8fjo, soc-0pku | Honest assessment of the arc |
| 156 | Attempted soc-8fjo (metrics_health migration) | Target was dead code; deleted ~46 lines; soc-8fjo closed |
| 157 | Attempted soc-0pku (context_assemble migration) | Port too narrow; consumer reads 13 fields, port projects 5; deferred with this learning |

Total phase-2 outcome: -46 LOC of dead code removed + this durable learning. The architecture did not advance; the *understanding* of why it can't advance with the current port surface did.
