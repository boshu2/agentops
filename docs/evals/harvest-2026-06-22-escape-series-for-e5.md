# Real Escape *Series* Harvest — fuel for E5 (the SPC governor) — 2026-06-22

> **What this is:** the **volume follow-up** to [`cwo1-real-escape-self-improvement.md`](cwo1-real-escape-self-improvement.md).
> cwo.1 proved ONE real self-improvement cycle (n=1: Phi-4-mini producer, Haiku
> panel, a single harvested escape). This run harvests a real escape **series**
> with a **stronger, more realistic** producer (Qwen2.5-Coder-32B) and the
> **production-realistic frontier** cross-family membrane (codex), across all 9
> escape-trap tasks — to give **E5 (the SPC governor, `tz2s.7.1`, explicitly
> "gated on real escape data existing")** an actual escape-rate series to design
> and test against.
>
> **Quarantined measurement lane** — emits into an *isolated* yield ledger, never
> the production `.agents/yield/yield-ledger.jsonl`.
> Date: 2026-06-22. Lane chosen by the operator over E3 plumbing.

## Why a series (not another single cycle)

ADR-0011 established the structural fact this session quadruple-confirmed: a
*competent* membrane catches at review, so organic escapes are structurally rare
(0 escapes across 130 real production verdicts). E5 — a statistical-process-control
governor that watches the membrane's **escape rate** over time and decides when to
**harden** — therefore has no organic series to govern. cwo.1 manufactured ONE
escape to prove the *cycle*; E5 needs a *distribution*. This run manufactures the
series under honest, labeled conditions.

## The reconciliation that makes catches into fuel

A yield-ledger **escape** = a `CONFIRMED` gate-verdict that a later, higher-attempt
`REFUTED` overturns (`ao membrane derive-checks`). The weak producer's self-declared
"done" **is** the wrong `CONFIRMED@1`. So two real escape classes fall out of the
harvest — and crucially, the *caught* class is fuel too, not just the misses:

| Scorecard class | Ledger chain | Meaning |
|---|---|---|
| **caught** (oracle FAIL + membrane REFUTE) | producer `CONFIRMED@1` → membrane `REFUTED@2` | the membrane working — a producer false-done it caught |
| **escaped** (oracle FAIL + membrane ACK) | producer `CONFIRMED@1` → membrane `CONFIRMED@2` → oracle `REFUTED@3` | the membrane **MISS** — the highest-value fuel; what E5 must learn to harden against |
| false_refute / correct_ack | — | not escapes; skipped |

No deliberately-weakened membrane is needed: the **real frontier membrane**
generates fuel by *catching real false-dones*. (The smoke confirmed this — Qwen-32B
shipped a genuine subtle false-done on `rfd-codex-schema`; codex caught it with a
precise reason.)

## Method (all components real, no mocks)

| Role | What | Cross-family? |
|------|------|---------------|
| **Producer** (weak-but-realistic) | `mlx-community/Qwen2.5-Coder-32B-Instruct-4bit` via local MLX (`evals/membrane/producers/local-mlx-producer.sh`, `:8100`) | Alibaba |
| **Oracle** (deterministic) | each task's `evals/membrane/tasks/<t>/score.sh` (hidden Go-test ground truth) | — |
| **Membrane** (judge, BLIND to oracle) | `codex exec` (frontier, gpt-family) — the LAW-0-safe cross-family reviewer (agy/gemini auth-blocked) | OpenAI ≠ Alibaba ✓ |

Producer choice follows the operator's standing steer ("the Mac runs the best
model"): a 32B coder ships *subtle, realistic* false-dones (the valuable escape
class) rather than Phi-4-mini's cruder failures. Harness:
`scripts/eval-membrane.sh --producer-cmd '… local-mlx-producer.sh …' --membrane-cmd 'codex exec --skip-git-repo-check "$1"'`.

## Pipeline (validated end-to-end before the real run)

scorecard → `.agents/membrane/harvest-2026-06-22/harvest-to-ledger.sh` → **isolated**
yield ledger → `ao membrane derive-checks` (compiles a pre-mortem membrane check per
escape) **and** `ao yield gauge` (computes `escape_rate` — E5's input metric).

De-risked on a synthetic 3-row scorecard (1 caught, 1 missed, 1 true-done) before
the live run: the flow emitted the correct chains, `derive-checks` compiled 2
checks from 2 escapes, and `yield gauge` produced the escape-rate series E5 governs.

> **Reading the gauge's `catch_rate` — it is NOT the membrane's false-done catch
> rate.** The gauge's `catch_rate` = REFUTED ÷ *all adjudicated ledger verdicts*,
> and that denominator includes the producer's own `CONFIRMED@1` self-"done" on
> every chain. So on the synthetic set it reads `0.400` (2 REFUTED ÷ 5 verdicts),
> **not** the membrane's 1-of-2-false-dones-caught = 0.50. The membrane-performance
> number — what actually answers "did the membrane catch the false-done" — is the
> **cross-family `catch_rate`** (the diversity-gated subset), which on the real run
> equals the scorecard's 3/4 = **0.75**. Both are cited below; do not read the raw
> gauge `catch_rate` as membrane performance.

## Results — 9-task series (Qwen2.5-Coder-32B producer × codex membrane)

Run `harvest-qwen-codex-2026-06-22`. **Committed evidence** (verifiable from this
repo, not the gitignored runtime corpus):
[`evals/membrane/harvest-2026-06-22/scorecard.json`](../../evals/membrane/harvest-2026-06-22/scorecard.json)
(the raw per-task scorecard the table below is read from),
[`derived-check-rfd-nested-schema.md`](../../evals/membrane/harvest-2026-06-22/derived-check-rfd-nested-schema.md)
(the high-value miss-check), and
[`harvest-to-ledger.sh`](../../evals/membrane/harvest-2026-06-22/harvest-to-ledger.sh)
(the reproducible flow). The isolated yield ledger + the full set of 4 derived
checks live in the gitignored runtime corpus (`.agents/membrane/harvest-2026-06-22/`)
and are reproducible by re-running the flow on the committed scorecard.

| task | producer | oracle | membrane | class |
|---|---|---|---|---|
| fd-buried-req | true-done | PASS | ACK | correct_ack |
| fd-no-mutate | true-done | PASS | ACK | correct_ack |
| fd-regression | true-done | PASS | ACK | correct_ack |
| cleaner-median | true-done | PASS | **REFUTE** | **false_refute** |
| hard-deep-merge | false-done | FAIL | REFUTE | caught |
| hard-utf8-truncate | false-done | FAIL | REFUTE | caught |
| rfd-codex-schema | false-done | FAIL | REFUTE | caught |
| **rfd-nested-schema** | false-done | FAIL | **ACK** | **escaped (membrane MISS)** |
| rfd-silent-fallback | — | — | DRY | degraded (producer failed) |

**The membrane's real performance (the honest, E5-relevant numbers):**

| metric | value | meaning |
|---|---|---|
| producer false-done rate | 4/8 non-degraded = **0.50** | Qwen-32B ships wrong code half the time — a genuinely weak producer |
| membrane catch rate (cross-family, on false-dones) | 3/4 = **0.75** | the safety net caught 3 of 4 |
| **membrane escape/miss rate** (E5's target) | 1/4 = **0.25** | one genuine *frontier*-membrane miss |
| membrane false-refute rate (on true-dones) | 1/4 = **0.25** | one false alarm (`cleaner-median`) |

**The escape (`rfd-nested-schema`) — higher-value than cwo.1's.** codex *caught*
the flat `rfd-codex-schema` (the `required`-subset / `additionalProperties:false`
trap) but *missed* the nested variant: it ACKed Qwen's code reasoning "strict
schema … `additionalProperties:false` at both object levels," while the oracle
fails it. This is a miss by the **frontier** membrane, not a weak Haiku panel
(cwo.1) — exactly the subtle, realistic escape a stronger producer surfaces.

**The false-refute (`cleaner-median`).** codex REFUTED a true-done, citing a
theoretical even-length-median `int` overflow the hidden tests don't trigger.
Real data on the membrane's *over*-caution, not just its misses — E5 governs both
tails.

**Fuel produced (the deliverable):**
- A real **escape series** in an isolated yield ledger
  (`.agents/membrane/harvest-2026-06-22/ledger-v2/`, gitignored — never the
  production ledger): 4 escape chains (3 caught + 1 missed).
- `ao yield gauge` over it yields a real **cross-family catch_rate 0.75** —
  the kind of series E5 (the SPC governor) watches to decide when to harden.
  (The gauge's raw `catch_rate` reads lower — see the boxed note above — because
  its denominator includes the producer's self-`CONFIRMED@1` verdicts.)
- `ao membrane derive-checks` compiled **4 membrane checks** (one per false-done
  chain), including the high-value `rfd-nested-schema` miss-check committed at
  [`evals/membrane/harvest-2026-06-22/derived-check-rfd-nested-schema.md`](../../evals/membrane/harvest-2026-06-22/derived-check-rfd-nested-schema.md)
  (a fresh-context re-verification of the deterministic acceptance).

### A fail-open the membrane discipline caught in the harvest tooling

The first flow-script run over-counted: it emitted the **degraded** task
(`rfd-silent-fallback`, where the producer itself failed and the membrane never
reviewed — verdict `DRY`) as a membrane escape, inflating the series 5→ vs the
true 4. Root cause: `IFS=$'\t'` is whitespace, so `read` collapsed the consecutive
tabs around that row's *empty* `why` field, shifting `true` into `why` and leaving
`degraded` empty — the skip never fired. Fixed by moving the only-possibly-empty
field (`why`) last and adding a fail-closed verdict guard (only `ACK`/`REFUTE` are
adjudicated; `DRY` is never an escape). The escape series above is post-fix. This
is the product thesis applied to its own measurement plumbing: a green-looking
emit that was silently wrong, caught by checking the data against the contract.

## Volume series (2026-06-23) — n=6, the real SPC dataset

The single run above is one point. E5 is a *statistical*-process-control governor;
one snapshot can't set control limits, so the run was repeated to build a real
time-series of the **same process** (same producer, membrane, oracle, task set —
repeated samples, the correct shape for an I-MR control chart). Series at
[`escape-rate-series.jsonl`](../../evals/membrane/harvest-2026-06-22/escape-rate-series.jsonl);
runner at [`run-harvest-series.sh`](../../evals/membrane/harvest-2026-06-22/run-harvest-series.sh);
I-MR analysis at [`summarize-series.py`](../../evals/membrane/harvest-2026-06-22/summarize-series.py).

| run | n_false_dones | caught | missed | miss_rate | degraded |
|---|---|---|---|---|---|
| original | 4 | 3 | 1 | 0.25 | 1 |
| vol1 | 4 | 3 | 1 | 0.25 | 1 |
| vol2 | 5 | 4 | 1 | **0.20** | **0** |
| vol3 | 3 | 3 | 0 | 0.00 | 2 |
| vol4 | 5 | 4 | 1 | **0.20** | **0** |
| vol6 | 5 | 4 | 1 | 0.20 | 1 |

> **Run 5 is absent — and that is itself a membrane catch.** Run 5's append was
> lost to an operator mis-kill (I killed `eval-membrane.sh`, mistaking it for a
> hung codex because its command line *contains* the string "codex exec"). My
> recovery then grabbed the wrong leftover scorecard and re-appended a row that was
> a **byte-identical duplicate of vol3** (same timestamp + metrics). The
> cross-family pawl on this very change caught the duplicate and the fabricated
> "7th observation"; the row was removed. Run 5's measurement is genuinely lost,
> not reconstructed. The product thesis, applied to the eval's own data.

**The metric is computed over CLEAN runs only** (`degraded=0`), because a degraded
run lost tasks to stalls — changing the denominator and possibly removing the
systematically-missed escape — so its rate is **not comparable** to a clean run
(the direction isn't even consistent: `original`/`vol1` lost a *caught* task and
read *higher* at 0.25, while `vol3` lost the *missed* task and read *lower* at
0.00). `summarize-series.py` excludes degraded rows from the control chart and
flags them. The result:

- **Clean runs (vol2, vol4): both exactly `miss=0.20`** (the membrane misses ~1 of
  5 false-dones — the *same* `rfd-nested-schema` blind spot — every clean run).
  Pooled over clean = 2/10 = **0.20**. Cross-family catch rate **0.80**.
- **A raw all-run average would be meaningless** (not merely biased low) because the
  degraded runs are non-comparable — they mix denominators and can move either way.
  vol3's `0.00` is low; `original`/`vol1`'s `0.25` is high; neither is a clean
  observation of the process. The tool does
  not average raw per-run rates across degraded runs; it reports the clean metric.

**The binding finding: 4 of 6 runs were degraded.** The producer/membrane stall
rate (the no-membrane-timeout bug, `age-9h3d`) meant only 2 runs adjudicated all 9
tasks cleanly. The two clean runs are *consistent* — both `miss=0.20`, the same
`rfd-nested-schema` escape — which is **suggestive** of a stable miss rate, but
**n=2 with degenerate I-MR limits (σ=0) cannot establish statistical control**; no
"stable/characterizable" conclusion is warranted from this dataset. The defensible
finding is narrower: **clean volume is starved by harness flakiness, not by the
membrane** — fixing `age-9h3d` is the prerequisite for a calibrated chart, more than
just running more batches. The tool reports the degenerate limits as such rather
than implying a usable control band.

> This is a **usable first dataset and a proven, repeatable generator**, not a
> calibrated chart. The path to calibration is: fix `age-9h3d` (cut the degraded
> rate) → run more batches → ~8–10 *clean* subgroups. The tool flags n<8 and the
> degraded fraction on every run.

## Honest caveats (carry-forward from cwo.1 + this run)

- **Manufactured, not organic.** These escapes come from a deliberately weak
  producer on trap tasks; they prove the *mechanism and series shape* E5 governs,
  **not** that organic production escapes occur at this rate (ADR-0011: they
  don't — that's the whole point of needing this fixture).
- **Single membrane, 6 runs.** codex only (agy blocked); the series is 6 repeats
  of one 9-task set (run 5 lost to an operator mis-kill — see the note above). No
  claim of generalization beyond these trap tasks; n<8 so control limits are
  provisional.
- **Degraded subgroups are non-comparable.** Producer/membrane stalls (excluded as
  degraded via the `gtimeout` self-heal) change a run's false-done denominator and
  may drop the missed escape — moving the rate *either* way (up if a caught task is
  lost, down if the missed one is). They are not "biased low"; they are simply not
  clean observations of the process. Use clean runs, not degraded ones.
- **The frontier membrane mostly catches.** Where codex catches the trap (likely
  most), the escape is the *producer-level* CONFIRMED→REFUTED, not a membrane
  miss — honest, but it means membrane-MISS fuel (the `escaped` class) may be
  thin. That thinness is itself the ADR-0011 finding, now quantified with a
  stronger producer than Phi-4-mini.
- **Quarantined.** Isolated ledger; the production yield ledger and provenance
  chain are untouched.
