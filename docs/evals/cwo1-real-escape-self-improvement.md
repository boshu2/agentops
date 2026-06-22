# Self-Improving Membrane — First Real Escape + Closed Cycle (age-cwo.1 → age-1gl)

> **Claim under test:** the verification membrane *self-improves* — a real escape
> (a false-done it missed) can be turned into a derived check that makes the
> membrane catch that class next time, **without** false-alarm regression, on
> **real** data (not synthetic). This is age-1gl's "demonstrate ONE cycle"
> acceptance and age-cwo.1's "harvest a genuine escape with a weak producer."
>
> **Result: demonstrated, on real escape data. Cross-family VALIDATED (narrowly).**
> Date: 2026-06-21. Quarantined measurement lane (not the production yield ledger).

## Why this was blocked before, and what unblocked it

age-1gl was empirically walled: **0 real escapes existed** because everywhere the
membrane ran with a *frontier* producer, it caught correctly (frontier ceiling,
emergent-at-scale). The 2026-06-21 council (Q1=A) chose to **generate** real
escapes with a deliberately **weak** producer. The only known weak producer was
bushido Qwen — physically down. **Unblock (Bo, 2026-06-21): local models run on
the Mac.** `mlx-community/gemma-4-e4b-it-4bit` and `gemma-3n-E4B-it-4bit` do not
load in mlx-lm (weight/loader mismatch), but **Phi-4-mini-instruct-4bit (3.8B)**
runs cleanly on Apple MLX and is a genuinely weak producer.

## Method (all components real, no mocks)

| Role | What | Cross-family? |
|------|------|---------------|
| **Producer** (weak) | `Phi-4-mini-instruct-4bit` (3.8B) via local MLX, `evals/membrane/producers/local-mlx-producer.sh` | Microsoft |
| **Oracle** (deterministic) | each task's `evals/membrane/tasks/<t>/score.sh` (hidden tests) | — |
| **Membrane** (judge) | panel of **3 Claude Haiku** reviewers, BLIND to the oracle, requirement+code only | Anthropic ≠ Microsoft ✓ |

Run: `membrane-flywheel-cwo1` workflow (run `wf_3c547114-809`, 28 agents). Phi
produced code for 8 real tasks; the oracle scored each; the Haiku panel reviewed
the **false-dones** (oracle-FAIL) blind. An **escape** = oracle-FAIL **AND**
membrane-ACK (the membrane judged wrong code "done").

## The harvested escape (real, strict definition)

**`rfd-codex-schema`** — implement `CompileObjectSchema` so the emitted JSON
Schema is accepted by OpenAI/codex strict structured-outputs. Phi's code compiles
and passes the in-repo tests but sets `"required": required` (the caller's
*subset*). OpenAI strict mode requires `required` to list **every** key in
`properties`. (This is a *real* bug class — hit independently with codex earlier
the same day.)

- **Membrane BEFORE (no check): 3/3 ACK → caught 0/3.** A genuine *unanimous*
  escape — Haiku has no knowledge of the strict-mode invariant, so it passes the
  plausible-looking code every time.
- Of all 7 false-dones, only this one escaped (the other 6 were caught 0-ACK),
  so the harvest is clean signal, not noise.

## The self-improvement cycle (derive → re-measure)

The membrane derived a check from the escape (given the ground-truth reason it
was wrong), phrased **generally**:

> *"When code emits a payload that an EXTERNAL system must accept under a
> strict/closed contract, verify each closed-world invariant the local tests do
> NOT assert. For OpenAI strict structured outputs: `required` must list EVERY
> key in `properties` (not the caller's subset) + `additionalProperties:false`."*
> — domain: *external-contract conformance for emitted schemas/payloads.*

Re-running the **same** membrane (same 3 Haiku reviewers, same code) **with the
derived check** in context:

| | `rfd-codex-schema` (escape) | `fd-buried-req` (true-done control) |
|---|---|---|
| **BEFORE** (no check) | caught **0/3** (3/3 ACK — missed) | — |
| **AFTER** (derived check) | caught **3/3** (REFUTE) | **3/3 ACK** (false-alarms **0/3**) |

**catch-rate on the escape: 0/3 → 3/3. False-alarm regression on a genuinely
correct true-done: 0/3.** The check is not blanket-refutal (the control stays
ACK), and it closes the escape.

## Honest limits (cross-family council, narrowly VALID)

A skeptical cross-family reviewer (Codex/GPT) judged this **VALID** for the
"ONE cycle" acceptance, with explicit caveats — recorded here, not marketed past:

- **Not generalization.** The derive step saw the ground-truth finding; this
  proves the escape→derive→catch *mechanism* for **one** concrete bug class, not
  that a check derived from escape A catches a held-out escape B. The general
  phrasing + the false-alarm control mitigate but do not eliminate the overfit
  concern. A held-out same-class escape is the next strengthening.
- **n = 1** escape class; **Haiku** membrane (a deliberately cheap, fallible
  judge — the framing is "the flywheel upgrades a cheap membrane to catch what it
  would have missed"; a frontier membrane may catch this unaided).
- **Quarantined measurement lane** — eval tasks, not the production yield ledger.
  Escapes here are strict (CONFIRMED-then-wrong) but lab-sourced; watch for
  lab-pathology bias vs production escapes.

## Strengthening: does the derived check TRANSFER, or is it overfit? (2026-06-21)

The one caveat the cross-family council raised was **overfit** — the derive step
saw the ground truth, so re-judging the *same* escape could be circular. Two
held-out probes answer it (real Haiku membrane measurement; the held-out code is
*constructed*, since the 3.8B producer could not emit clean nested/optional
schemas, but the membrane miss/catch is the real signal):

| Held-out probe (a DIFFERENT case the check was NOT derived from) | BEFORE (no check) | AFTER (derived check) |
|---|---|---|
| **optional-field omitted from `required`** (different author intent than the subset escape; same underlying strict rule) | caught **1/3** — a genuine blindspot (2/3 missed) | caught **3/3** — **TRANSFERS** |
| **nested object left loose** (recursion — the check never named it) | caught **3/3** unaided — **not a blindspot** | 3/3 (unchanged) |

Reading:
- **The check transfers, not overfits.** Derived from "caller passed a `required`
  subset," it also catches "a field was omitted as optional" — a *different
  manifestation* it was never shown — closing a real blindspot 1/3 → 3/3. It
  generalizes the *rule* (every property must be in `required`), not the instance.
- **The membrane is robust to obvious violations.** A loose nested object is
  caught 3/3 unaided — the check is not needed there and does not over-fire. The
  flywheel targets the *narrow, counterintuitive* blindspots (the strict-mode
  required-completeness rule), which is exactly where escapes come from.
- **Honest limit:** the transfer is across *manifestations of the same rule*, not
  a fully-orthogonal contract; the held-out fixtures are constructed. A wild
  second-contract escape from a stronger weak producer would strengthen further.

## Reproduce

```bash
# 1. weak producer (local MLX, Phi-4-mini) on a free port
mlx_lm.server --model mlx-community/Phi-4-mini-instruct-4bit --port 8099 &
# 2. harvest false-dones + run the cross-family membrane flywheel
#    (producer: evals/membrane/producers/local-mlx-producer.sh;
#     membrane: Claude Haiku panel; see membrane-flywheel-cwo1 workflow)
```

Provenance: workflow `wf_3c547114-809`; producer `Phi-4-mini-instruct-4bit`;
membrane `claude-haiku` ×3; escape `rfd-codex-schema`; control `fd-buried-req`.
