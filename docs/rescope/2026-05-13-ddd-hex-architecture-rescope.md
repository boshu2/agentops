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
