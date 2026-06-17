# age-6ty pre-push pawl evidence — escape_rate gauge

**Bead:** age-6ty — Escape-tracking for membrane catch_rate (v2 of the catch_rate gauge age-t3f).
**Mode:** fresh-context (default). Author context: `age-6ty-claude-main`. Refuter context: a fresh-context subagent (distinct invocation, no shared accumulated context).

## What landed

`escape_rate` surfaced alongside `catch_rate` in `ao yield gauge`:

- `Gauges.Escapes` + `Gauges.EscapeRate` = `len(DetectEscapes(l, runID)) / confirmed` — the fraction of the membrane's CONFIRMEDs a later attempt proved wrong. The independent quality axis catch_rate can't give: a lenient/rubber-stamp membrane confirms freely, so a HIGH escape_rate exposes untrustworthy CONFIRMEDs regardless of catch_rate.
- `EscapeRate` is nil (no signal) with a note when there are no CONFIRMED verdicts — never a fabricated 0.
- Human report row + JSON field, next to catch_rate.
- The pre-registered actuation hypotheses (§3) are left **verbatim** — escape_rate actuation is deferred with the rest (ag-qpg99), not slipped into a frozen pre-registration.

## Honest read on real data

On the real dogfood ledger (`run-2026-06-14-dynamo-dogfood`): `catch_rate 0.667` (6 refuted / 9 adjudicated), `escape_rate 0.000` (0 escapes / 3 confirmed) — the membrane's three confirms all held, so there is genuinely nothing to escape. The gauge reports the real state; it does not manufacture signal.

## Adversarial review (CONFIRMED)

A fresh-context reviewer red-teamed the diff and **CONFIRMED**, after probing the key risk — can escapes exceed confirmed (ratio > 1)?

- **Ratio ≤ 1 proven safe.** `DetectEscapes` returns at most one escape per bead, and a bead only escapes if it has ≥1 CONFIRMED verdict — so every +1 to escapes implies ≥1 to `confirmed`. The reviewer built and ran the probe sequence (confirmed@1 → refuted@2 → confirmed@3 = per-bead 1 escape / per-verdict 2 confirmed → 0.5) and the worst case (5 beads each confirmed-then-refuted → exactly 1.0, never above).
- **Divide guard correct** (nil + note on zero confirmed), verified by test and by reading the branch.
- **Purely additive** — no existing gauge (catch_rate, Q, A, L, E, C) value perturbed; `ActuationHypotheses()` untouched at the verbatim 5 §3 entries.
- **Tests real** — production-`Writer` fixtures, exact-value asserts (`Confirmed==2, Escapes==1, *EscapeRate==0.5`), proving the per-bead-vs-per-verdict denominator semantics.

Non-blocking nit noted (human row prints "1 escapes" — grammar; matches the existing catch_rate row's "%d refuted" style, left for consistency).

## Deterministic evidence

```
go build ./...                                                    # clean
go vet ./cmd/ao/ ./internal/yieldledger/                         # clean
go test ./internal/yieldledger/ ./cmd/ao/ -run 'Escape|Gauge|Yield|ActuationHypotheses'  # ok
scripts/validate-go-fast.sh                                       # race-fast PASS
ao gate check --fast --scope head                                # 26/26 pass, 0 fail
```
