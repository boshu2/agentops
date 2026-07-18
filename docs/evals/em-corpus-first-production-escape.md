# Escape-Corpus — First Real Escape Harvested in Production Development (EM.2.11)

> **Claim under test:** the escape-corpus seeds from a *genuine* escape produced
> during real product development — not a synthetic fixture, not a quarantined
> weak-producer lane (that was [age-1gl / cwo1](./cwo1-real-escape-self-improvement.md)).
>
> **Result: one real escape harvested + classified honestly, gauge-neutral.**
> Date: 2026-06-22. Cross-family council modeled the recording (see "Honesty" below).

## The escape

While building the membrane spine this session, `EM.2.10` (the "cut wire" —
escapes compile to enforced constraints, commit `7b358185d`) was **cross-family
CONFIRMED** by the pawl (round 1, no holes found) and **landed on main**. Its
unit test set `GateVerdictInput` fields directly. Then `EM.3`'s installed-binary
e2e (`scripts/em-loop-donetest.sh`, commit `a48413029`) caught a real bug **in
that CONFIRMED code**: `emitYieldEvent` parsed the new detector fields from the
the retired `yield emit gate-verdict --json` command body but **dropped them** when building the
`GateVerdictInput` — the CLI *producer seam* the unit test bypassed by setting the
struct directly. On the shipped binary, no constraint compiled.

This is a **genuine escape**: a CONFIRMED-then-overturned miss (the `.2.10`
CONFIRMED gate-verdict exists in the yield ledger at attempt 1; the e2e is the
higher-attempt catch). The membrane confirmed `.2.10`, then a stronger check
(the installed-binary e2e) caught what the cross-family pawl + the green unit
suite both missed.

| Field | Value |
|---|---|
| Escape (confirmed) | `EM.2.10` — `7b358185d` — cross-family CONFIRMED, landed |
| Catch (overturn) | `EM.3` installed-binary e2e — `a48413029` |
| Domain | BC2 Validation (the membrane / yield-ledger producer path) |
| What was missed | a `GateVerdictBody` field not threaded into `emitYieldEvent`'s `GateVerdictInput` literal — the CLI producer seam; unit tests set the struct directly and skipped it |
| Classification | **advisory (process-gap)** — see below |
| Fix | `a48413029` thread the fields body→input; extended `TestEmitYieldEvent_GateVerdictCarriesDomainAndReason` (which existed for the *same* EM.2.1 body→input drop class) |

## Why ADVISORY, not a mechanical constraint

The detector is **structural**, not a path-scoped regex: "a struct-literal
omitted a field its type defines" (the `go.md` *Struct Fields* rule — "grep all
`StructName{` literals and verify each sets the new field"). There is no clean
`regex + path-glob` that catches its re-introduction without false positives, so
it does **not** compile to an enforced constraint — it stays advisory: a finding
the next in-domain pre-mortem loads (domain + what-was-missed), not a gate rule.
The mechanical wire (EM.2.10) is for the *re-introducible* subset; most real
escapes — including this one — are process-gaps.

## Honesty (cross-family council, 2026-06-22)

The council **refuted** the naive "record this session's catches as escape-corpus
entries" plan and set these rules, which this record follows:

1. **Most of this session's ~10 catches were NOT escapes.** They were caught at
   *review* (the pawl REFUTED before any CONFIRMED/merge) — the membrane
   *succeeding*, not missing. Recording them as escapes would be false and would
   corrupt `catch_rate` / `escape_rate`.
2. **Exactly one genuine escape** (the emit-seam above) is harvested.
3. **No synthetic gate events.** The `.2.10` CONFIRMED already existed; the catch
   is recorded here as documentary provenance, **not** injected as a fabricated
   REFUTED into the live yield ledger — so the live catch/escape-rate gauges are
   not gamed. The corpus seeds from real escapes via the now-proven wire
   (EM.2.10 → EM.3/EM.TEST → EM.4 → EM.2.9), not by manufacturing entries to hit
   a number.

## What this demonstrates

The membrane caught itself in production: a cross-family CONFIRMED change carried
a real bug that a *stronger, deterministic* check (the installed-binary e2e)
overturned — the exact "stronger reviewer overturns a weaker CONFIRMED" shape the
escape model exists to capture. The first real corpus entry is harvested honestly,
small and true, rather than padded.
