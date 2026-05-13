---
title: BC ports wire-up arc — 12 production adapters in 11 consecutive cycles
date: 2026-05-13
tags: [hexagonal-architecture, ddd-bounded-context, ports, wire-up, evolve-loop]
source: .agents/evolve/cycle-history.jsonl cycles 83+108-118
maturity: lessons-learned
---

# BC ports wire-up arc

11 consecutive `/evolve` cycles (108-118), preceded by cycle 83's
first-mover `productionCitationAdapter`, delivered 12 production
adapters covering all 5 DDD bounded contexts (BC1-BC5). The arc is
the canonical compound-by-repetition demonstration in the cycle
history. Cycles 119-121 reconciled doc-narrative drift after the
arc completed.

## What the arc proved

- **A repeatable file shape ships fast.** Every adapter followed the
  same triplet: `<port>_adapter.go` in `cli/cmd/ao/`, paired
  `<port>_adapter_test.go`, compile-time
  `var _ XPort = (*productionX)(nil)` assertion. Average cycle:
  ~250 LOC + ~10 tests in ~10 minutes wall-clock.
- **8 distinct adapter shapes** all worked under the same triplet:
  CLI wrapper (Citation), JSONL append/read (Loop R/W, Operator,
  ClaimEvidenceBinder w/ upgrade-only rule), tree-walk + hash join
  (Harness), tree-walk + ranker (CorpusReader), idempotent file
  write + frontmatter (CorpusWriter), pure-Go transform
  (FindingCompiler), subprocess bash wrapper (GateRunner), external
  CLI + pluggable runner (CIStatus), sync in-memory pubsub
  (EventBus).
- **Reader+writer pair tests work.** Both BC1 (CorpusReader+Writer
  cycles 112-113) and BC3 (LoopReader+Writer cycles 108-109)
  shipped round-trip tests in the writer's suite — proves the
  on-disk format is shared end-to-end.
- **Subprocess testing has two valid shapes.** Cycle 115's
  GateRunner used a fake-bash-script-in-temp-repo harness;
  cycle 117's CIStatus refined to a pluggable `runGH` func field
  that lets tests substitute canned bytes without writing a script.
  Both are valid; the func-field shape is faster and platform-
  independent (use it when the adapter's interesting logic is the
  parser, not the subprocess invocation).

## Anti-patterns observed during the arc

- **Naming collisions caught by `go vet`.** Cycle 115 introduced a
  local `fileExists` helper that collided with `doctor.go`'s
  existing one. `go vet` caught it; fix was to reuse doctor's
  helper rather than rename. Lesson: before writing a small util
  in an adapter, `grep -n "func <name>" cli/cmd/ao/` first.
- **Shadowing builtins.** Cycle 117 wrote `cap := limit` which
  shadowed `cap()`. `go vet` didn't fail (it's legal) but it's a
  readability trap. Renamed to `pageSize`.
- **Assuming the wrong adapter shape.** Cycle 116
  (ClaimEvidenceBinder) initially looked like a bd-CLI wrapper
  (cycle 83 Citation precedent), but re-reading the port contract
  showed it's a JSONL adapter (the binding ledger lives on disk,
  not in bd). Lesson: always read the port doc-comment first; the
  adapter shape is constrained by the port's persistence model.

## What the arc did NOT do

- **No call-site migration.** Production callers in `cli/cmd/ao`
  still invoke the leaf helpers (e.g. `beads.go:227` still calls
  `verifyCitationInPlace` directly, not via the
  `productionCitationAdapter`). The adapters are reachable but
  latent. The per-BC follow-up bds (`soc-pm5t` for BC1.1, etc.)
  track the migration work.
- **No cross-process EventBus.** `productionEventBus` is sync
  in-process; the adapter doc explicitly names the swap path
  (`event_bus_nats.go` or similar) for when the factory goes
  distributed. Operator transport-choice is deferred.

## Why I stopped at 12 instead of migrating callers

A migration cycle inside `cli/cmd/ao` adds a layer
(`verifyCitationInPlace` → `productionCitationAdapter` →
`verifyFileCitation`) without semantic gain because both endpoints
are in the same package — the port surface only matters at a
package boundary. The genuine call-site value comes when:
- a non-`cli/cmd/ao` package consumes one of these adapters, or
- a test substitutes the InMemoryX adapter to exercise a code path
  without real I/O.

Future cycle should pursue the former (create or migrate a
cross-package caller) rather than manufacture intra-package
routing.

## Cycle-history pointers

- Scaffold arc: cycles 78-106 (12 port interfaces + InMemoryX
  adapters, captured in `docs/contracts/bc-ports-inventory.md`)
- Wire-up arc: cycle 83 + cycles 108-118 (12 production adapters)
- Doc reconciliation arc: cycles 119-121 (3 doc files updated to
  reflect actual state)
- Total: 26 cycles across the BC ports surface
