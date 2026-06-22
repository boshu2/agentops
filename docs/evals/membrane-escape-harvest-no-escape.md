# Escape Harvest — Phi-4-mini × Haiku panel: NO_ESCAPE (a valid null) (2026-06-22)

> **Claim under test:** can the weak-producer lane *grow the escape corpus* beyond
> the n=1 [cwo.1 escape](./cwo1-real-escape-self-improvement.md) — harvest more,
> diverse real escapes to fuel E5 (the SPC governor, still design-only)?
>
> **Result: NO_ESCAPE. The cross-family Haiku membrane caught all 9 false-dones.**
> A valid null — the membrane *working*, not a harness failure. Scorecard:
> [`membrane-harvest-2026-06-22-scorecard.json`](./membrane-harvest-2026-06-22-scorecard.json).

## What ran (all real)

| Role | What |
|------|------|
| **Producer** (weak) | `Phi-4-mini-instruct-4bit` (3.8B) via a fresh local MLX server on `:8099` (the cached `:8080` server was a wedged 22-day zombie; codex small models are blocked on the ChatGPT account) |
| **Harvest** | [`evals/membrane/harvest.sh`](../../evals/membrane/harvest.sh) — producer + deterministic oracle over all 9 tasks → `harvest_dir` + `candidates.json` |
| **Membrane** | [`evals/membrane/flywheel-harvest.workflow.js`](../../evals/membrane/flywheel-harvest.workflow.js) — a panel of 3 Claude Haiku reviewers per false-done, BLIND to the oracle (cross-family to the producer) |
| **Control** | an oracle-verified correct `cleaner-median` (4/4 pass) for false-alarm measurement |

An **escape** = oracle-FAIL **AND** membrane-ACK. Of 9 tasks, Phi produced **9
false-dones** (oracle-FAIL on every one) — and the Haiku panel **REFUTED all 9**
(0 ACK, 3/3 REFUTE each). Zero escapes; the derive/re-measure phase never fired.

## Why zero (the finding)

Phi-4-mini wrote **non-compiling** code for **7/9** tasks (missing imports, YAML
emitted instead of JSON, undefined identifiers). A reviewer trivially REFUTEs code
that does not compile. The 2 that compiled (`fd-regression`, `rfd-silent-fallback`)
were also caught 3/3.

**Phi-4-mini is too weak to be a good escape generator.** It fails *loudly*
(non-compiling, obviously broken) rather than *subtly* (compiling, plausible, but
wrong on a non-obvious invariant) — and only subtle failures escape a competent
membrane. The single [cwo.1 escape](./cwo1-real-escape-self-improvement.md)
(`rfd-codex-schema`) required a *compiling-but-subtly-wrong* sample; that run drew
one, this run did not (producer variance on a 3.8B model). An escape needs a
producer in the **sweet spot**: strong enough to compile, weak enough to ship a
subtle bug. Phi is below it; a frontier producer is above it (aces the tasks → no
false-dones, the frontier ceiling).

## Decision (cross-family council, 2026-06-22)

The council was asked: chase a stronger weak producer (Qwen2.5-Coder-32B, cached)
for a nicer lab escape, accept the null and stop, or something else. **Verdict: B —
record the null, land the reusable tooling, stop grinding the quarantined lab lane.**
Rationale:

- cwo.1 already proved the escape→derive→catch **mechanism** at n=1; one more
  *lab* escape from a heavier producer is still **quarantined lab fuel**, marginal
  past its evidentiary value.
- The null is **informative**: it bounds the membrane's competence (this tier
  catches trivial weak-producer failures) and is evidence against Phi-4-mini as an
  escape generator.
- The **higher-value path to E5 fuel is production capture**: real membrane misses
  (CONFIRMED-then-overturned gate-verdicts) accruing over time through the already-
  wired EM spine — *real* escapes, not lab ones. E5 is gated on real escape data
  over **time**, which is a data-accrual question, not a harvest-harder question.
- Qwen2.5-Coder-32B is reserved for a **bounded calibration run** only if E5's
  design specifically needs more synthetic seed cases.

## What this leaves (reusable)

[`harvest.sh`](../../evals/membrane/harvest.sh) +
[`flywheel-harvest.workflow.js`](../../evals/membrane/flywheel-harvest.workflow.js)
codify the producer→oracle→harvest_dir step (which cwo.1 ran ad-hoc) and a
*find-ALL-escapes* membrane workflow (cwo.1's derived from only the single clearest).
A future calibration run is now one command + one workflow invocation, with the
producer a swappable knob (`MLX_MODEL` / `MLX_ENDPOINT`).

## UPDATE — stronger producer (Qwen2.5-Coder-32B): the substrate exists, but the membrane still catches (2026-06-22)

The "stronger weak producer" the cwo.1 doc named as the next strengthening **was
locally runnable after all** — `Qwen2.5-Coder-32B-Instruct-4bit` (the best coder
the Mac holds, ~17 GB) runs fine via MLX (the earlier "memory-infeasible" read was
too conservative — macOS reclaims cache; the Mac had already been running a 35B).
Re-ran the harvest with it. Scorecard:
[`membrane-harvest-qwen-coder-2026-06-22-scorecard.json`](./membrane-harvest-qwen-coder-2026-06-22-scorecard.json).

**Qwen produced the subtle samples Phi could not.** It got 4/9 tasks *correct*
(genuine true-dones) and shipped **2 compiling-but-subtly-wrong** false-dones —
`rfd-codex-schema` and `rfd-nested-schema`, the OpenAI strict-mode bugs (the *same
class* as the lone cwo.1 escape). That is the compiling-but-plausible-yet-wrong
substrate every fuel path needs, and it now demonstrably exists locally.

**But the cross-family Haiku membrane caught BOTH subtle-compiling bugs 3/3.** Unlike
Phi's loud (non-compiling) failures, these were subtle, plausible, compiling schema
code — and the membrane still REFUTED them unanimously. Full tally (panel ×3):

| Task | oracle | compiles | membrane (ACK / REFUTE) | result |
|---|---|---|---|---|
| `rfd-codex-schema` | FAIL | yes | 0 / 3 | **caught** (subtle, compiling) |
| `rfd-nested-schema` | FAIL | yes | 0 / 3 | **caught** (subtle, compiling) |
| `hard-utf8-truncate` | FAIL | no | 0 / 3 | caught |
| `rfd-silent-fallback` | FAIL | no | 0 / 3 | caught |
| `hard-deep-merge` | FAIL | no | **1 / 2** | **1 escape** (noisy, non-compiling) |
| 4 true-dones | PASS | yes | 11 ACK / 1 REFUTE | 1/12 false-alarm |

**Reading (honest):**
- **Escape-rarity is now confirmed against SUBTLE code, not just assumed.** A strong
  producer's compiling, plausible, non-obvious-rule-violating bugs were caught 3/3 by
  a *cheap cross-family* membrane. The membrane catches subtle failures, not only loud
  ones — the strongest evidence yet for the product, and it explains the structural
  starvation: escapes are rare *because the membrane is competent*, even on subtle
  samples from a 32B coder.
- **The one "escape" is weak and noisy:** `hard-deep-merge` is *non-compiling* and was
  missed by only 1 of 3 reviewers. Its derive→re-measure went the WRONG way (caught
  2/3 BEFORE → 0/3 AFTER the derived check) — **not** a self-improvement demonstration;
  inconclusive (n=1, likely the logic-check redirected reviewers off the compile
  error). Recorded as a negative/null, not spun as a win.
- **False-alarm stayed low** (1/12 on genuine true-dones).

**Net:** the user-directed correction (run the best model, not Phi) worked — the
subtle-sample substrate exists locally. The result it produced is that the membrane
*catches* even subtle compiling bugs, so harvested escapes remain rare and the lone
escape was non-compiling noise. The structural finding (a good membrane self-starves
its escape-fueled improvement loop) holds, now tested against a strong producer rather
than inferred from a weak one. No clean new self-improvement cycle was harvested; the
honest yield is stronger *membrane-competence* evidence + a confirmed (not assumed)
escape-rarity.
