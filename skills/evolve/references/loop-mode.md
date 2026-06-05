# /evolve Loop Mode

`--mode=loop` flips /evolve from agent-self-regulated to operator-driven.

## Invariants under --mode=loop

1. `ao loop write-stop-marker` exits 1 unconditionally
2. DORMANT/STOP/KILL markers are operator-only (operator writes by hand or via `ao loop operator-stop`)
3. Scope-filter (Step 3) splits too-big work into smaller beads via `bd create --deps discovered-from:<parent>`, never halts
4. Step 7 stop reasons are stripped of CONTEXT_BUDGET_EXHAUSTED; that becomes a non-sticky HANDOFF signal cleared by next cron-fire

## CLI primitives that enforce

- `/evolve --mode=loop` — flag itself
- `ao loop write-stop-marker` — refuses under loop
- `ao loop operator-stop` — explicit operator override (separate code path)
- `ao loop blocked` (Wave 2) — typed blocked event instead of STOP

See: docs/plans/2026-05-21-evolve-loop-epic-design.md §A1 + §A7
