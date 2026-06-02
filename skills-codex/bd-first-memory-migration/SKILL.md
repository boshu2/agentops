---
name: bd-first-memory-migration
description: 'Consolidate fragmented agent-memory layers into one bd-canonical store, then GC/retire the rest. Triggers: "memory migration", "consolidate agent memory", "beads-first memory".'
---

# bd-first-memory-migration (Codex twin)

> Codex-runtime twin of `skills/bd-first-memory-migration`. Make `bd` the single
> source of truth for agent memory: salvage keepers, derive caches, GC/retire the
> rest. Three phases, gated between destructive steps.

## Phases

1. **Audit** (read-only) — use the source skill's audit scan, classify, and
   audit report helpers to inventory every layer and produce the Gate A
   keep/drop manifest + reversibility plan. ⛔ Gate A before any write.
2. **Migrate** (writes to bd only) — use the source skill's import, typed memory,
   decay-ranked recall, unified remember, and generated MEMORY.md helpers.
3. **GC / Retire** (destructive) — utility scoring, scheduled GC/dedup, retire
   dead stores, reclaim disk. ⛔ Gate B (rollback test) + ⛔ Gate C (human go/no-go).

## Guardrails

- Idempotent, reversible, backup-aware; every destructive step has `--dry-run`.
- **Pace bd/Dolt writes** — bulk salvage unpaced can OOM a memory-capped server.
- bd-canonical (not br); never hard-delete authored content — archive instead.

See the canonical source skill (`skills/bd-first-memory-migration/`) for the
full SKILL.md, DATA-MODEL, ROLLBACK contract, and worked examples.
