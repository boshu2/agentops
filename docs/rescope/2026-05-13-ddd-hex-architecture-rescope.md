# Rescope — DDD / Hex Architecture Arc

**Date:** 2026-05-13
**Trigger:** operator instruction during /evolve cycle 154 — "rescope all the work from the update principles and ddd/hex architecture work we just pushed"
**Scope:** cycles ~80-154 (the BC ports + production adapters + CLI-wiring work)

## What we built

| Layer | Count | Status |
|---|---|---|
| BC ports (interfaces + DTOs) | 14 | Complete — BC1 Corpus, BC2 Validation, BC3 Loop, BC4 Factory, BC5 Runtime |
| In-memory test doubles + tests | 14 | Complete |
| Production adapters | 14 | Complete |
| CLI surfaces exposing adapters | 10 (71%) | Partial — 4 adapters not yet CLI-exposed |
| Compile-time `var _ Port = (*production…)(nil)` assertions | 14 | Complete |

Adapters: CorpusReader, CorpusWriter, FindingCompiler, Citation, GateRunner, ClaimEvidenceBinder, CIStatus, ClaimEvidence, LoopReader, LoopWriter, Operator, EventBus, FactoryAdmission, Harness.

CLI surfaces: `ao corpus inject/capture`, `ao citation verify`, `ao ci latest/recent`, `ao gate run`, `ao loop history/append`, `ao operator record/list`, `ao harness status`, `ao claim bind/list`.

## What is missing — the load-bearing observation

**Grep `cli/internal/ports` imports outside `cli/cmd/ao/`: zero.**

Every consumer of the port surface lives in `cli/cmd/ao/` — i.e., the CLI commands we just wired in cycles 144-154. The actual business logic in:

- `cli/internal/lifecycle/defrag.go` (reads `cycle-history.jsonl` directly)
- `cli/internal/quality/metrics_health.go` (reads `cycle-history.jsonl` directly)
- `cli/cmd/ao/context_assemble.go` (reads `cycle-history.jsonl` directly)
- `cli/internal/daemon/*` (operator events, gate results, ledger bindings — all direct I/O)
- skill scripts under `skills/**/*.sh` (direct `jsonl` parsing)

…still does direct filesystem I/O against the same artifacts the ports were extracted to encapsulate.

**The ports exist; the architecture does not.** The hex shape is half-built: outer ring (ports + adapters + CLI) is in, but the inner ring (business code that should route through ports) hasn't been migrated yet. We solved the easy half.

## Why this happened (honest retrospective)

The CLI-wiring template (cycle-147) became a compounding metronome — 7 consecutive cycles, ~285 LOC each, ~10 min each, 8 tests each. That cadence is *gratifying* and produces commits, but each new adapter just exposes one more latent capability rather than activating an existing one. The cycle-122 "wire-up arc" learning predicted this exact failure mode: "production adapters are latent until cross-package consumers exist."

We then went from extracting ports → wrapping them with production adapters → wrapping the adapters with CLI commands. We never went the other direction: rewiring real consumers (defrag, metrics_health, context_assemble, daemon, skills) to route through the ports.

## In, out, deferred

### IN — finish-line work (the half we haven't done)

1. **Rewire `cli/internal/lifecycle/defrag.go` to use `LoopReaderPort`.** Single highest-value migration: defrag is core knowledge-compounding code and currently reads `cycle-history.jsonl` raw.
2. **Rewire `cli/internal/quality/metrics_health.go` to use `LoopReaderPort`.** Currently has its own `loadCycleHistory` helper (the one that collided with the CLI's `loadCycleHistory` in cycle 144 — drift evidence).
3. **Rewire `cli/cmd/ao/context_assemble.go` to use `LoopReaderPort` + `CorpusReaderPort`.** Knowledge injection should route through the typed Corpus surface, not direct file walk.
4. **Decide: are the remaining 4 adapter CLI surfaces required?** Only if their value is "a real consumer needs them"; otherwise they're decorative.

### OUT — stop doing

1. **No more "wrap adapter N with CLI command" cycles.** The template is proven, the marginal value is zero until real consumers exist. The 4 unexposed adapters (FactoryAdmission, ClaimEvidence, EventBus, FindingCompiler) stay unexposed until a *consumer needs them*, not because we want a 100% number.
2. **No more port extractions until the existing 14 are load-bearing.** We don't need a 15th port until at least 5 of the existing ports have real (non-CLI) consumers.
3. **No more `bc-ports-inventory.md` updates.** It's already reconciled.

### DEFERRED — keep tracked, don't work now

- HypothesisLedgerPort + ConvergenceCheckPort (`soc-y5vh.3`) — only useful once /rpi or /evolve actually compose with them. Not yet.
- PG3/PG4 auto-promotion logic (`soc-2klg.3`, `soc-2klg.4`) — substantial work; only pay it down when the claim ledger has live consumers.
- Codex skills parity for the new `ao` subcommands — `ao harness status` surfaced 77 OutOfSync codex skills (cycle 149); that's a separate epic, not part of this arc.

## Closure criteria for the DDD/Hex arc

This rescope declares the arc **PHASE-1 COMPLETE, PHASE-2 PENDING**:

- ✅ Phase 1 (done): hex *shape* extracted — 14 ports, 14 production adapters, 10 CLI surfaces, all tested, all reconciled in `docs/contracts/bc-ports-inventory.md`.
- ⏳ Phase 2 (next): hex *adoption* — at least 3 non-CLI consumers (defrag, metrics_health, context_assemble) routed through ports. Until then, the ports are latent infrastructure, not architecture.

Phase 1 took ~70 cycles. Phase 2's first three migrations should each fit in 1-2 cycles if scoped tightly (`grep` the direct-IO call sites → swap to `loopReaderPort.ReadHistory(...)` → keep regression tests).

## Open epic disposition

| Epic | Status | Action |
|---|---|---|
| `soc-2c1p` BC1 Corpus ports | Phase-1 complete | Close after Phase-2 first wave (Corpus consumers migrated) |
| `soc-wxh5` BC2 Validation ports | Phase-1 complete | Close after Phase-2 first wave |
| `soc-y5vh` BC3 Loop ports | Phase-1 complete | Close after defrag.go + metrics_health.go + context_assemble.go migrate |
| `soc-2klg` BC4 Factory ports | Phase-1 complete | Phase-2 deferred (no daemon migration yet) |
| `soc-zd7c` BC5 Runtime ports | Phase-1 complete | Phase-2 deferred (harness has no internal consumers) |

## Concrete next-cycle picks (when /evolve resumes the loop)

1. **`hex-phase2-loop-defrag`** — Rewire `cli/internal/lifecycle/defrag.go` to depend on `LoopReaderPort` instead of reading `cycle-history.jsonl` directly. First proof that the ports are load-bearing.
2. **`hex-phase2-loop-metrics-health`** — Same for `cli/internal/quality/metrics_health.go`. Removes the duplicate `loadCycleHistory` helper that collided in cycle 144.
3. **`hex-phase2-loop-context-assemble`** — Same for `cli/cmd/ao/context_assemble.go`. Closes the BC3 loop reader migration.

After those land, the architecture statement "BC3 is hexagonal" is finally *true*.

## Lessons from the arc

1. **Hex extraction is a two-phase commitment.** Extracting the port + writing the adapter is the warm-up. Migrating consumers is the actual architectural change. Stopping after phase 1 leaves you with the cost of indirection and none of the benefit.
2. **Compounding metronomes are seductive failure modes.** The CLI-wiring template was beautifully repeatable. But repeatability is not the same as value-delivery. After 3-4 applications, the marginal return per cycle dropped to near-zero and we kept going on momentum.
3. **Operator override is a feature, not friction.** This rescope only happened because the operator interrupted the cron loop. The /evolve work-selection ladder doesn't currently distinguish "exposing latent infrastructure" from "rewiring real consumers" — both look productive to the regression gate.

## What "rescope" produced

This document. Plus the three concrete phase-2 picks above, ready to be filed as bd beads when the loop resumes.

---

**Bottom line:** stop adding adapters and CLI commands. Start rewiring real consumers (defrag, metrics_health, context_assemble) to use the ports we already built. Three migrations close the BC3 loop and prove the hex architecture is load-bearing rather than decorative.

## Addendum (cycle 156) — first migration target was dead code

**Update from the first phase-2 attempt.** When opening `cli/internal/quality/metrics_health.go::LoadCycleHistory` to migrate it to `LoopReaderPort`, two findings surfaced:

1. **Schema mismatch on defrag.go.** `defrag.go::SweepOscillatingGoals` reads a `Target` field that `CycleEntry` does NOT project. Worse, **zero entries in the actual `cycle-history.jsonl` have a `target` field** — the production sweep is a permanent no-op against a per-goal schema that never materialized. Migrating defrag.go to LoopReaderPort would not be a clean swap; it'd require either (a) adding `Target` to `CycleEntry` and the ledger writers, or (b) routing defrag at a different artifact (e.g. goals.jsonl).

2. **`metrics_health.LoadCycleHistory` is dead code.** `loadCycleHistory` is defined in `cli/cmd/ao/metrics_health.go:375` as a one-line shim around `quality.LoadCycleHistory`, but the shim has **zero production callers**. The only references are: the shim's own definition, the underlying `quality` function's definition, and two tests that probe the dead path directly. The cycle 144 `loadCycleHistory` collision was actually a collision against dead code — the cleanup was already overdue.

**Action taken in cycle 156:** deleted `quality.LoadCycleHistory` (~23 lines), the shim in `cmd/ao/metrics_health.go` (1 line), and the two dead tests in `cmd/ao/metrics_health_test.go` (~22 lines). Net: ~46 lines of dead code removed, full test suite still green.

**Rescope thesis re-validated:** the architecture exists, but the production code that *should* consume it never did. We kept building infrastructure for consumers that turned out to be either schema-mismatched (defrag) or dead code (metrics_health). The cleaner phase-2 path is **delete dead code AND migrate `context_assemble.go`**, not "migrate all three" as originally listed. Closing `soc-8fjo` (metrics_health migration) — no longer applicable, the target is gone.

Updated phase-2 picks (cycle 156 forward):
- ~~soc-8fjo~~ — RESOLVED via deletion in cycle 156.
- `soc-0pku` — context_assemble.go → LoopReaderPort + CorpusReaderPort. Still the right migration.
- `soc-1q1x` — defrag.go → LoopReaderPort. **Deferred**, pending Target-field schema decision on `CycleEntry`. Filed as a discussion-then-migration follow-up, not an immediate cycle pick.


## Addendum (cycle 157) — third migration attempt forced the schema decision

Third phase-2 attempt (`soc-0pku`: `context_assemble.go::gatherHistory` → `LoopReaderPort`) revealed the same structural problem as the defrag attempt: **the port is too narrow for the real consumer.**

- `CycleEntry` projects 5 fields: Number, Mode, Result, Commit, Milestone.
- `gatherHistory` consumes 13 fields, including aliases: timestamp, cycle, target, goal_ids, result/status, sha, canonical_sha, log_sha, goals_passing, goals_total, summary, error.

Migrating would silently drop 8+ fields the formatter relies on. Not a drop-in.

**This is not a per-cycle problem; it is an architecture-level finding.** Three phase-2 migrations attempted, three different rejection modes:
- soc-8fjo: target was dead code → deleted
- soc-1q1x: target has a `Target` field the port doesn't project → blocked
- soc-0pku: target reads 13 fields, port projects 5 → blocked

**Captured as a durable learning:** `docs/learnings/2026-05-13-bc-ports-narrowness-postmortem.md`.

**Schema decision filed:** `soc-ckc4` — choose whether to (A) widen CycleEntry on-demand, (B) add a ReadRaw escape hatch, or (C) accept that cycle-history is permanently outside the typed port. Recommendation in the bead: option A, start by adding `Timestamp` and `Summary` to unblock soc-0pku, then iterate.

**Phase-2 status after cycle 157:** -46 LOC dead code removed, two migrations blocked pending soc-ckc4, one durable learning banked. The architecture did not advance, but the *understanding of why it can't advance with the current port surface* did. That is the real cycle-157 product.

**Next /evolve cycle pick:** wait for soc-ckc4 decision before attempting any more phase-2 migrations. In the interim, the loop should redirect to entirely different work (testing, docs, drift, validation) rather than force-fitting another mismatch. The CLI-wiring template-application reflex remains banned.


## Closure (cycle 167) — 13-cycle arc retrospective

This rescope opened with cycle 154 (operator override) and closes with cycle 167 (13 productive cycles later). Net session product:

| Phase | Cycles | Lanes worked | Net |
|---|---|---|---|
| Rescope + 2 failed phase-2 attempts | 155-157 | Honest assessment + dead code + narrowness | -46 LOC, 2 durable learnings |
| Pivot — dead-code sweep | 158-160 | Staticcheck U1000 mining | -276 LOC, 38 findings cleared |
| Phase-2 unblocker | 161-162 | CycleEntry widening (R/W symmetric) | +88 LOC (port + tests) |
| Real consumer | 163 | `ao loop verify` audit | +249 LOC, caught 1 real drift |
| Audit cleanup | 164 | Ledger dedup | -1 line (untracked) |
| Doc drift | 165 | Test-arch-debt reconciled | -4 stale refs |
| Bounded sweep | 166 | soc-k083 backlog (5 more) | -21 LOC |
| Closure | 167 | This addendum | (this section) |

**Total: 13 cycles, ~-25 net LOC across all sweeps, 43 staticcheck findings cleared (75 → 32), 2 ports widened (5 → 7 fields), 1 new audit feature (`ao loop verify`), 4 bd issues closed, 4 bd issues filed, 2 durable learnings, 1 rescope doc with 3 addenda.**

### What this session validated

1. **Operator overrides break compounding metronomes.** Cycle 154 was 7 consecutive template-application cycles. The rescope at cycle 155 stopped that pattern dead. Cycles 156-167 each picked a *different* kind of work, no cycle exceeded ~12 minutes, and every cycle delivered durable artifact.

2. **Phase-2 attempts are diagnostic.** Three consecutive failed migrations (cycles 156, 157, and the closed soc-0pku) collectively narrowed the port-widening design space. The "narrow + grow" policy emerged from failed migrations, not from preemptive design.

3. **Dead-code sweeps have a natural shape.** Cluster-pattern recognition (cycle 159) is 5-10x faster than per-symbol review. Both modes are legitimate; knowing when to switch from one to the other is the skill. Bounded pre-commitment (cycle 166: "5 findings") prevents the metronome from restarting.

4. **Real consumers surface real bugs immediately.** `ao loop verify` (cycle 163) caught a duplicate on its very first run — the kind of finding that comes from building tools you actually use against real data, not test fixtures.

### What stays open

- `soc-1q1x` (defrag → LoopReaderPort) — blocked on `Target`-field schema decision; cycle-history.jsonl has no `target` field anywhere, so this is a "redesign defrag, not migrate it" problem.
- `soc-k083` (32 remaining staticcheck findings) — bounded under operator review, not /evolve cycles.
- Phase-2 BC1/BC2/BC4/BC5 — no real consumers yet outside CLI passthroughs. The on-demand widening policy applies if and when one materializes.

### Closure criteria met

The rescope's original phase-1/phase-2 framing:
- ✅ Phase 1 (hex shape): 14 ports, 14 adapters, 10 CLI surfaces
- ⚠️ Phase 2 (hex adoption): 1 real consumer (`ao loop verify`), still mostly latent
- ✅ Decision pending: schema width policy (now "narrow + grow")
- ✅ Decision pending: when to stop sweeping (now "bounded per-cycle")

The arc is **closed for the autonomous /evolve loop**. Further phase-2 work requires operator scope (which port consumer is worth widening for) and is no longer a /evolve cycle pick. The CLI-wiring template-application reflex remains banned.
