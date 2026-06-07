# Pre-Flight Schema Check — Stop Migration Cycles That Can't Succeed

Three phase-2 port migrations attempted in cycles 156-157 of this repo all failed for the same structural reason: the port surface projected ~5 fields, the consumer read ~13 fields. The migration would have silently dropped 8+ fields the consumer relied on. The lesson is captured in `docs/learnings/2026-05-13-bc-ports-narrowness-postmortem.md`. This document encodes that lesson as a pre-cycle check.

## When this gate runs

Before invoking `/rpi` on a cycle whose selected work matches any of:

- title contains `migrate`, `rewire`, `route through`, `port-ify`, `adapt to <Port>`
- title contains `<Module> → <Port>Port` or similar arrow syntax
- bd issue has label `migration` or `bc-port-adoption`
- selected_source is `harvested` and the queue entry has `kind: "migration"`

## Procedure

1. **Identify the port surface.** From the work title, locate the port type definition (typically `cli/internal/ports/<port>.go`). Extract the projected struct fields.

2. **Sample two consumer call sites.** From the work description, find the function or file being migrated. Grep for *its* current consumers — the call sites that depend on the data the migration will route through the port. Pick two representative ones (different files, different formatting/decision shapes).

3. **Field-fit comparison.**
   ```bash
   PORT_FIELDS=$(grep -E '^\s+[A-Z][a-zA-Z0-9_]+\s+' cli/internal/ports/<port>.go | awk '{print $1}' | sort -u)
   CONSUMER_FIELDS=$(grep -oE '\.[A-Z][a-zA-Z0-9_]+\b' <consumer-files> | sed 's/^\.//' | sort -u)
   comm -23 <(echo "$CONSUMER_FIELDS") <(echo "$PORT_FIELDS")
   ```
   The output is the set of fields the consumer reads that the port does NOT project.

4. **Decision.**

   | Missing fields | Action |
   |---|---|
   | 0 | Migration is safe. Proceed with `/rpi`. |
   | 1–2 | Migration with widening. Add the missing fields to the port + writer + tests in the same cycle. Mark mode `port-widen-and-migrate`. |
   | 3+ | **Abort the migration cycle.** File a port-widening bd issue with the field list, log this cycle as `unchanged` with `selected_source: "pre-flight-schema-fail"`, return to Step 3 selection. |

5. **Special case — zero-data target.** If the consumer reads a field (e.g. `Target`) that exists in the type definition but *no actual entries in the production data store have that field set*, the consumer is dead-code-in-disguise. Don't widen — investigate whether the consumer was ever wired. Log as `selected_source: "pre-flight-dead-target"`.

## Why this check is cheap

A migration cycle costs ~15-30 minutes of /rpi (discovery + crank + validation). The pre-flight check is two `grep` calls and one `comm` — under 30 seconds. Catching even one bad migration pays for ~50 pre-flight runs. In this repo, three cycles spent on bad migrations (156, 157, plus the soc-0pku bead) would have been caught by 90 seconds of pre-flight.

## What this does NOT replace

This is a *cycle-selection* gate, not a code-quality gate. Validating the actual migration code still belongs to `/rpi` → `/validate`. The pre-flight only answers "is this migration even shaped right for the current port?"

## Worked example — cycle 157 (soc-0pku)

Selected work: `context_assemble.go → LoopReaderPort`

```
PORT_FIELDS=Number,Mode,Result,Commit,Milestone   # 5
CONSUMER_FIELDS=Timestamp,Cycle,Target,GoalIDs,Result,Status,Sha,CanonicalSha,LogSha,GoalsPassing,GoalsTotal,Summary,Error   # 13
Missing: Timestamp,Target,GoalIDs,Status,Sha,CanonicalSha,LogSha,GoalsPassing,GoalsTotal,Summary,Error   # 11
```

Missing fields = 11. Threshold 3+. **Abort.** File port-widening issue: "Widen CycleEntry: +Timestamp,+Summary at minimum; full set deferred." Cycle is logged as `unchanged`, ladder advances.

This is exactly what cycle 161 ended up doing reactively — adding `StartedAt` and `Title` after the migrations bounced. With pre-flight, the widening would have happened *before* burning three cycles on doomed migrations.

## See also

- `docs/learnings/2026-05-13-bc-ports-narrowness-postmortem.md` — the source observation
- `references/scout-mode.md` — first-class result for cycles that intentionally don't execute
- `references/metronome-gate.md` — sibling cross-cycle gate
