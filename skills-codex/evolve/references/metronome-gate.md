# Metronome Gate — Detecting Productive-Looking Repetition

The work-selection ladder treats "tests passed + commit landed" as productive. That signal is necessary but not sufficient: it cannot tell apart a cycle that *applied an existing template again* from one that *closed a new lane of work*. Cycles 144-154 in this repo were 7 consecutive CLI-wiring template applications — every cycle green, every commit clean, every gate happy. Only operator override at cycle 154 stopped the run.

The metronome gate adds a structural detector: when the same `mode` value repeats N times, the next rung of the ladder is forced.

## State

`mode_repeat_streak` is computed by `scripts/evolve-update-session-state.sh` from the trailing run of identical `mode` values in `cycle-history.jsonl`. It is derived, not authored — running the script after every cycle keeps it coherent.

## Thresholds

| `mode_repeat_streak` | Action |
|---|---|
| 0–2 | No gate. The selection ladder runs normally. |
| 3–4 | **Soft block.** If the selected work would produce the same `mode` value, skip to the next rung of the selection ladder. Do not abort — try the rung below. |
| 5+ | **Hard block.** Record the gap as bead/provenance evidence and require operator override (write `.agents/evolve/RESCOPE` with a new directive). The cycle is logged as `unchanged` with `selected_source: "metronome-gate-blocked"`. |

## Why the threshold is 3, not 2

A 2-cycle streak is normal: harvested work often comes in pairs (port + adapter, R + W, parent + child). A 3-cycle streak is the earliest signal that the *template* has become the work, not the *problem*. Stopping at 3 catches the failure mode early without blocking legitimate harvested-pair cycles.

## Naming discipline (operator instruction)

For the gate to work, cycle entries must use *distinguishable* `mode` values. Anti-patterns observed before this gate existed:

- `template-applied-cli-wiring-5th`, `template-applied-cli-wiring-6th`, `template-applied-cli-wiring-7th` — the suffix `-Nth` made these technically unique strings but obviously the same mode. The gate canonicalizes by stripping trailing `-Nth` / `-N` patterns before comparison.
- `wire-up-BC1`, `wire-up-BC2`, `wire-up-BC3` — the BC suffix changes but the work shape is identical. This is a *legitimate harvested-pair sequence* — three separate BCs, three real ports — and the gate correctly does NOT block here because the streak counts identical strings only. Operator distinguishes "BC-pair work" from "template metronome" by intent.

When in doubt, the operator override at threshold 5+ is the safety net.

## Worked example — cycles 144-154

```
cycle 144: wire-up-BC1            (streak: 1)
cycle 145: wire-up-BC1-pair       (streak: 1, different)
cycle 146: wire-up-complete       (streak: 1)
cycle 147: template-applied-cli-wiring-4th  (streak: 1)
cycle 148: template-applied-cli-wiring-5th  (streak: 2 after canonicalization)
cycle 149: template-applied-cli-wiring-6th  (streak: 3 — SOFT BLOCK fires)
cycle 150: template-applied-cli-wiring-7th  (streak: 4 — still blocked, next rung)
cycle 151..154: ...                          (operator override at 154)
```

With the gate active at threshold 3, cycle 149 would have skipped to the next rung of the selection ladder, probably surfacing the latent narrowness problem 5 cycles sooner.

## Interaction with idle_streak

The metronome gate fires on *productive* cycles; the dormancy gate fires on *idle* cycles. They are orthogonal:

- `idle_streak >= 2 AND generator_empty_streak >= 2` → dormancy (see Step 3 hard-gate)
- `mode_repeat_streak >= 3 AND candidate.mode == last_mode` → metronome block (skip rung)
- Both can never fire on the same cycle: idle cycles produce no work, so they cannot extend the metronome.

## Why this lives in /evolve, not /rpi

`/rpi` runs one cycle's worth of work. It has no visibility into prior cycles. The metronome is a *cross-cycle* pattern, only visible from the loop driver. `/evolve` is the only place that has both the ledger and the next-cycle decision.

## See also

- `docs/rescope/2026-05-13-ddd-hex-architecture-rescope.md` — the original operator override that prompted this gate.
- `references/cycle-history.md` — the ledger this gate reads.
- `references/scout-mode.md` — what to do with work that hits the soft-block (read + annotate, no execution).
