# Local-MLX membrane vs codex — measured concordance — 2026-06-23

> **Why:** the cross-family membrane (codex) stalls ~15-20% of reviews (`age-9h3d`),
> which taxes clean eval volume and is a latent reliability cost. A cross-family
> council (Claude + codex) picked **building a local-MLX membrane** to harden the
> proven asset — but flagged the binding risk: *a weaker local model must not be
> treated as equivalent until measured against codex.* This is that measurement.
>
> **Scope:** the eval/measurement path only. The local membrane is **NOT** wired
> into the production push-to-main pawl gate (`scripts/pawl-review.sh` stays
> codex-only and already-robust: timeout + retry). A weaker reviewer must never
> authorize a merge.

## Method

Same weak producer, same tasks, two membranes — the apples-to-apples comparison.

| Role | What |
|------|------|
| Producer (weak) | `Phi-4-mini-instruct-4bit` via local MLX (`:8099`) |
| Membrane A | **local** `Qwen2.5-Coder-32B-Instruct-4bit` via `:8100` (`evals/membrane/membranes/local-mlx-membrane.sh`) — Alibaba ≠ Microsoft ✓ |
| Membrane B | `codex exec` (frontier, gpt-family) — Alibaba ≠ OpenAI ✓ |
| Oracle | each task's deterministic `score.sh` |

6 escape-trap tasks. Run A: Phi × local-Qwen. Run B: Phi × codex. Both via the
timeout-hardened harness. Evidence:
[`A-local-qwen.json`](../../evals/membrane/membrane-compare-2026-06-23/A-local-qwen.json),
[`B-codex.json`](../../evals/membrane/membrane-compare-2026-06-23/B-codex.json).

## Result — 100% per-task concordance

| task | local Qwen-32B | codex | agree |
|---|---|---|---|
| rfd-codex-schema | degraded (producer failed) | degraded | ✓ |
| rfd-nested-schema | caught (REFUTE) | caught (REFUTE) | ✓ |
| hard-deep-merge | caught (REFUTE) | caught (REFUTE) | ✓ |
| hard-utf8-truncate | caught (REFUTE) | caught (REFUTE) | ✓ |
| fd-buried-req | correct ACK (true-done) | correct ACK | ✓ |
| fd-no-mutate | caught (REFUTE) | caught (REFUTE) | ✓ |

Totals **identical**: 4 false-dones, **caught 4/4 (catch rate 1.0)**, 0 escaped,
0 false-refute, 1 correct ACK, 1 degraded — for *both* membranes. Every catch is
oracle-confirmed (oracle FAIL + REFUTE), so the agreement is concordant **and**
correct, not two reviewers agreeing on a wrong answer.

## What this does and does NOT establish

**Does:** on crude-to-moderate false-dones (a weak Phi producer), a capable local
model (Qwen-32B) is a **viable membrane with bounded latency** for the eval path —
matching codex's catch rate (1.0) and false-refute rate (0) with full per-task
agreement, at zero API cost and no API rate-limits. Its review latency is **capped
at `MLX_TIMEOUT` (default 90s)** and is normally seconds, so it has **no codex-style
intermittent multi-minute stalls** — but note this is *bounded*, not *zero*, latency
risk: a wedged local endpoint still blocks up to the timeout before degrading. That
bounded behavior is what addresses the codex-stall tax on clean eval volume.

**Does NOT:**
- **Subtle-escape parity is only PARTIALLY measured.** Phi's false-dones are
  relatively crude — both membranes catch them easily. The follow-up below tests
  the *canonical subtle* `rfd-*` baselines (both caught 3/3), so parity now extends
  past crude escapes — but the *subtlest* case (the `rfd-nested-schema` defect codex
  itself **missed** on the stronger Qwen-32B *produced* output, in
  `harvest-2026-06-22-escape-series-for-e5.md`) remains untested (that artifact was
  reaped). Distinct false-done reviews: 4 (crude Phi producer) + 9 (full-corpus
  baselines; the 3 `rfd-*` subtle baselines are a subset of the 9, also run
  standalone) — all concordant.
- **It is not a production-gate claim.** Concordance on an eval set is not license
  to let a local model authorize merges; the pawl gate stays codex-only.

## Follow-up — subtle escapes (the "blocked" test, unblocked via fixtures)

The crude-producer run above doesn't separate the membranes (both catch easily). The
sharper test needs *subtle* false-dones — but a subtle-but-non-Qwen model producer
isn't available (Qwen's own output is same-family to the membrane; Phi is too crude;
codex errs too rarely; agy is auth-blocked). The unblock: the `rfd-*` tasks'
`setup.sh` **already stages the canonical subtle-wrong baseline** (e.g.
rfd-codex-schema stages "the exact schema that 400s codex" — `required` = the
caller's subset). A **no-op producer** leaves that subtle false-done for the membrane
to review — a *family-neutral* fixture both membranes see identically.

Run: no-op producer × {local Qwen-32B, codex} over 3 subtle `rfd-*` baselines.
Evidence: [`A-local.json`](../../evals/membrane/subtle-escape-2026-06-23/A-local.json),
[`B-codex.json`](../../evals/membrane/subtle-escape-2026-06-23/B-codex.json).

**3 subtle `rfd-*` baselines: both caught 3/3**, and the local membrane's reasons are
sound (on rfd-nested-schema: *"does not include the address object … and does not
include address in the required array"*).

**Extended to the FULL trap corpus (divergence search):** the no-op-baseline run was
then repeated over **all 9 trap-task baselines** with both membranes
([`full-A-local.json`](../../evals/membrane/subtle-escape-2026-06-23/full-A-local.json),
[`full-B-codex.json`](../../evals/membrane/subtle-escape-2026-06-23/full-B-codex.json)).
**Result: both caught 9/9, ZERO per-task divergence.** Across every test in this doc —
the 4 crude false-dones (Phi producer) and the 9 full-corpus baselines (which include
the 3 `rfd-*` subtle baselines, also run standalone) — the local Qwen-32B membrane and
codex **never disagreed on a single task**. That is strong, consistent evidence the
local membrane is a viable eval membrane on this corpus.

**Still bounded — honestly:** this is the *staged canonical* subtle baseline, not the
*subtler produced variant* that codex itself missed in the harvest
(`harvest-2026-06-22-escape-series-for-e5.md`: codex ACKed Qwen-32B's produced
nested-schema). That specific artifact was reaped and can't be re-reviewed; the
absolute-subtlest parity is narrower now but not fully closed. The samples are
modest (4 crude-producer + 9 full-corpus baseline false-dones; the 3 `rfd-*`
subtle baselines are within the 9), though fully concordant across all.

## Recommendation

Use the local membrane for **eval/harvest volume** (free, no rate-limits,
bounded review latency) where its measured quality holds; keep codex as the
production push-gate reviewer.

**Operationalized (how to actually use it).** The harvest volume runner takes a
membrane switch:

```bash
# reliable local membrane (no codex stall tax) — the operator owns keeping
# HARVEST_MEMBRANE_MODEL in a DIFFERENT family than the producer:
HARVEST_MEMBRANE=local \
  bash evals/membrane/harvest-2026-06-22/run-harvest-series.sh <label> <endpoint> <model> <series>
# default stays codex (for measuring the production gate's own miss rate).
```

Tunables: `HARVEST_MEMBRANE_ENDPOINT` / `HARVEST_MEMBRANE_MODEL` (default Qwen-32B
on `:8100`). The series rows record which membrane produced them (`"membrane"`
field), so a mixed series stays auditable. Any direct `scripts/eval-membrane.sh`
caller can equivalently pass
`--membrane-cmd 'bash evals/membrane/membranes/local-mlx-membrane.sh "$1"'`.

**Caveat for measurement vs volume.** Use **codex** when the *thing being measured
is the production membrane's own miss rate* (e.g. the E5 governor series); use
**local** for cheap reliable volume where the producer/escape-corpus is the object
of study and the reviewer just needs to be a capable cross-family check.

Before trusting the local membrane on the *subtlest* escapes, repeat the comparison
with a **stronger producer** so the false-dones are subtle enough to separate the
two membranes — the one bound this corpus could not close (the reaped produced
variant).
