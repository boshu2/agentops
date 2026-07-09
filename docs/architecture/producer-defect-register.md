# Producer-Defect Register

> **What this is.** The measured half of the catch→producer loop ([ADR-0014](../adr/ADR-0014-catch-to-producer-loop-judgment-catches-need-a-producer-route.md)). When `/post-mortem` Step BP.7 routes a **recurring** membrane catch (`HitCount ≥ 2`) to a **producer-side fix** — a skill standard, a `CLAUDE.md`/`AGENTS-*.md` footgun, a `/plan` rule, or a `/discovery` pre-mortem — it records one row here. The point is not the list; it is the **recurrence measure**: after a producer fix binds, that class should be caught *less*. A class that keeps recurring after its fix means the binding was too weak — escalate it (prose → checklist → gate) or fix the wrong root.
>
> **Why a register and not just checks.** `ao membrane triage` (2026-07-08) reports `Axis2Compilable=0.00`: none of the 13 recurring classes can become a mechanical gate — they are judgment-class. Their only real destination is the producer. This register is where that routing is tracked and its effect is checked. It is deliberately falsifiable: if recurrence does not drop, the loop is not working and this file shows it.

## How to use (from Step BP.7)

1. `ao membrane triage` → note each class with `HitCount ≥ 2`.
2. Classify: **compilable** → open a gate bead (Loop B, rare). **Judgment-class** → bind a producer fix (Loop C) and add a row below.
3. Bind at the highest surface the defect's *nature* allows — for a judgment defect that is the skill/footgun/plan-rule, which is the correct binding, not a fallback.
4. Fill `Recurrence before` from the catch count at fix time. Leave `Recurrence after` to be filled by the *next* post-mortem that runs triage — that is the measurement.

## Register

| Date | Defect class (what the membrane keeps catching) | Nature | Producer fix + binding surface | Recurrence before → after | Status |
|---|---|---|---|---|---|
| 2026-07-08 | **Tracker-shape fail-open** — code/skills assume `br`'s JSON shape; on `bd` a field is absent (e.g. `show` omits `dependents[]`, and omits an empty `description`), so an audit/consumer silently sees zero and passes. Caught on `age-f07z` (multi-family panel; a single reviewer missed it). | judgment | Rule: *tracker-agnostic code must enumerate children via `ao beads exec children` (never `show \| jq .dependents[]`), and treat bd-absent fields as null.* Bound in the tracker/standards skill surface + the corrected `post-mortem` audit. | 1 → _(pending next triage)_ | bound |
| 2026-07-08 | **Wrong-cwd `go` repro** — the pawl auto-repro / a test invocation runs `go test ./cli/...` from the repo root (no `go.mod` there; module is `cli/`) → false failure regardless of correctness. | compilable | `_repro_go_workdir` cds to the enclosing `go.mod` (`age-n8dt`) + root-fallback retry (`age-7hgb`). This one *was* mechanizable (Loop B). | ~2 → _(pending)_ | bound (gate) |
| 2026-07-08 | **Stale-binary false-fail** — sub-checks resolve a stale `cli/bin/ao`, false-failing `provenance.chain`; and manual land steps mix code into the `#trivial` bind / leak a stale ledger edge. | compilable / process | `ScriptRunner` injects `AO_BIN=self` (`age-jmfl`); the `ao land` verb makes the land a single trusted-path command with auto-bind (`age-m21d`). | recurring this session → _(pending)_ | bound |
| 2026-07-08 | **Inverted epic-dependency** — `ao beads exec dep add <child> <epic>` records *child blocked-by epic* (wrong direction), so the child can't close ("Skipped: blocked by <epic>"). Hit by the orchestrator this session. | judgment | Footgun rule in the beads/tracker skill: the correct edge is epic-blocked-by-child (or a parent-child edge); `br dep remove <child> <epic>` to undo. | 1 → _(pending)_ | bound |

## Backlog: the 122 unclassified catches

`ao membrane triage` reports **122 unclassified catches** under the 22 named classes. Classifying them is the fuel for this register — each recurring class that surfaces is a candidate producer fix. Left unclassified they are invisible. Classifying a batch each post-mortem (not all at once) is the sustainable cadence.
