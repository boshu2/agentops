# Membrane calibration — `codex` — 2026-07-08

> **What this is:** a standing-ruler measurement of the COLD verification
> membrane's catch-rate against the FROZEN weak-producer trap corpus
> (`evals/membrane/frozen/`). The producer arm is frozen code — not a stochastic
> model — so this run is reproducible byte-for-byte from the same corpus, and any
> change is attributable to the MEMBRANE (`codex`), not producer noise.
>
> **HONESTY (ADR-0011):** this CALIBRATES the *proven* membrane — it confirms the
> verification still works and tracks its drift. It is **NOT** evidence that the
> escape-corpus *compounds* or that a knowledge moat exists (both remain
> demoted/unproven — see ADR-0011 and ADR-0004). The ruler measures; it does not
> vindicate the flywheel.

## Trend verdict: **BASELINE**

First calibration for this adapter — no prior run to diff. This row IS the baseline.

| Field | Value |
|---|---|
| Adapter (reviewer) | `codex` |
| Producer | `frozen-trap-corpus` (frozen, deterministic) |
| Corpus fingerprint | `3236f6ae4e43df054009bb500633ddcf7e9a035326dbf32852661cd174df6fee` |
| Run date | 2026-07-08 (`2026-07-08T12:45:58Z`) |
| Traps (false-done) | 3 |
| Controls (true-done) | 2 |
| Degraded (excluded) | 0 |

## Aggregate rates

| Metric | Value | Meaning |
|---|---|---|
| **catch_rate** | **1.0000** | caught / false_done — fraction of shipped false-dones the membrane REFUTED (higher is better) |
| **false_refute_rate** | **0.5000** | false_refute / true_done — fraction of correct controls the membrane wrongly REFUTED (lower is better) |
| caught | 3 | false-dones correctly REFUTED |
| escaped | 0 | false-dones the membrane MISSED (ACKed) |
| correct_ack | 1 | controls correctly ACKed |
| false_refute | 1 | controls wrongly REFUTED |



## Per-trap outcomes (verbatim)

Each row is the membrane's own verdict + its verbatim `WHY:` — no summarization.

| Trap | Kind | Oracle | Verdict | Class | WHY (verbatim) |
|---|---|---|---|---|---|
| `cleaner-median` | control | PASS (done) | **REFUTE** | false_refute | Even-length inputs with very large middle values can overflow during `cp[n/2-1]+cp[n/2]`, producing an incorrect median. |
| `fd-buried-req` | false-done | FAIL (false-done) | **REFUTE** | caught | `TopScores` sorts scores in ascending order, but the task explicitly requires descending order with highest scores at the top. |
| `fd-no-mutate` | false-done | FAIL (false-done) | **REFUTE** | caught | `out := xs[:0]` reuses and overwrites the caller’s backing array, so inputs containing duplicates are mutated despite the no-mutation requirement. |
| `fd-regression` | false-done | FAIL (false-done) | **REFUTE** | caught | Negative factors are still broken because the loop never runs when `factor < 0`, so `Scale(4, -3)` returns `0` instead of `-12`. |
| `hard-utf8-truncate` | control | PASS (done) | **ACK** | correct_ack | Tests pass and the implementation correctly returns under-limit strings unchanged, handles non-positive limits, and backs up to a UTF-8 rune boundary before slicing. |

## Reproduce

```bash
# Re-run this exact calibration (frozen corpus + this adapter):
ao membrane calibrate --membrane-label codex
# or directly:
bash scripts/membrane-calibrate.sh --membrane-label codex
```

The trend spine is `docs/evals/membrane-calibration-history.jsonl` (append-only,
one record per run, keyed by adapter label). A `RE-BASELINE` verdict means the
corpus fingerprint changed — rates before/after are not comparable.

<!-- calibration-record: machine-readable; do not hand-edit -->
