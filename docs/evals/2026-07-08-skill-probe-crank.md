# Skill behavioral probe — crank, 2026-07-08

> **HONESTY.** A probe measures **BEHAVIOR-CHANGE, not quality-uplift.** This run
> answers only: did loading the crank skill change whether the agent respects
> write-scope collisions when planning parallel waves? It does not claim crank
> is good or bad. Small N (2) is **directional, not statistical** (ADR-0011
> discipline — do not overclaim).

## What was measured

Crank's core invariant: *group into a wave only when write scopes do not
collide.* The probe hands the agent 4 beads to execute in parallel waves, where
**bead-B and bead-C both write `cli/internal/foo/shared.go`** (a write-scope
collision) and A/D are disjoint. The **behavioral** discriminator: did the agent
emit a wave plan that keeps B and C in **different** waves (serialized), rather
than co-scheduling them in the same parallel wave? It parses the actual wave
assignment (the ACTION), not whether the plan mentions "write scope."

- Probe: `evals/skill-probes/crank/`
- Arms differ **only** by `treatment-prelude.md` (the crank wave-collision rule);
  `question.md` is identical.
- Live dispatch via `codex exec` (the sanctioned headless path — never
  `claude -p`, LAW 0). Producer model recorded in the fixtures: **gpt-5.5**.
- Transcripts captured to `evals/skill-probes/crank/fixtures/` so the run
  replays deterministically.

```bash
bash scripts/probe-skill.sh --probe crank --live --capture --reps 2
# reproduce from the committed fixtures:
bash scripts/probe-skill.sh --probe crank --replay
```

## Result — INERT (frontier aced both arms)

| Arm | present / usable | rate | what the agent did |
|-----|------------------|------|--------------------|
| control (no crank) | 2 / 2 | 1.0 | e.g. `Wave 1: bead-A, bead-B, bead-D` / `Wave 2: bead-C` — B and C separated |
| treatment (crank loaded) | 2 / 2 | 1.0 | B and C separated |

**Verdict: `INERT`** (treatment_rate 1.0 is not > control_rate 1.0).

## What INERT means here (and what it does NOT)

It does **not** mean crank is worthless. It means: **on a frontier model
(gpt-5.5), at this task altitude, loading crank changed nothing** — the model
already refuses to parallelize two beads that write the same file, with or
without the doctrine. The skill's marginal *behavioral* effect on a strong
producer is nil because the producer already exhibits the behavior.

This is the same lesson the membrane eval already banked
(`membrane-eval-too-easy`, `moat-unproven-at-frontier`): a frontier producer aces
the task and yields no signal. To surface a skill's behavioral value you need a
**weaker producer** (`--model gpt-5-mini`, the local llama) or a **harder task**
where a naive agent actually gets it wrong. That is the honest ratchet, recorded
here rather than papered over.

## Why this is still the required evidence

The `age-e508.1` acceptance is "a probe RUN for crank exists, with a dated
evidence file and the MEASURED column populated" — **not** "crank must be
BEHAVIORAL." Measuring crank and honestly finding INERT-at-frontier is precisely
the product this bead builds: an unmeasured product badge is noise; a measured
one — even when the measurement is "no detectable change on this model" — is
truth. Recorded in `skills/SKILL-TIERS.md` → Behavioral Probe Ledger.
