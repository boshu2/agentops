# Phases — full contract

> Scaffold stub. Per-phase step contracts are authored by the phase beads of epic ag-5u50.

## Phase 1 — Audit (read-only)
- `audit-scan` — inventory every memory layer (Claude MEMORY.md, ao/flywheel, .agents/{knowledge,learnings}, openclaw, bd): counts, sizes, dup-rate, maturity distribution, recency. (bead .7)
- `classify` — KEEP (authored/established) vs DROP (provisional dumps, N× dup extracts); dedup; dry-run manifest. (bead .8)
- audit report + reversibility plan = **Gate A** (bead .9)

## Phase 2 — Migrate (writes to bd only)
- `import` — keepers → bd with type + provenance; idempotent; import-ledger. (bead .10)
- typed `remember`/`recall` wrapper over bd keys. (bead .11)
- `recall` — decay-ranked, token-budgeted injection. (bead .12)
- unified write path: all harnesses → bd. (bead .13)
- `gen-memory-md` — regenerate the thin read-only cache. (bead .14)

## Phase 3 — GC / Retire (destructive)
- utility scoring + promotion ladder wired to eviction. (bead .15)
- scheduled GC + dedup → cold archive. (bead .16)
- contradiction/supersede detection. (bead .17)
- `retire` — decommission dead daemons + reclaim disk. **Gate B/C.** (bead .21)
- cross-host consistency check (.22); `mem stats` observability (.23)
