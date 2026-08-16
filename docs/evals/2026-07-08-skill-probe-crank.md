# Skill behavioral probe — crank, 2026-07-08

> **LEGACY EVIDENCE STATUS.** The committed fixture set predates capture-time
> fixture hashes and producer/config manifests. The current fail-closed harness
> cannot provenance-verify or reproduce this run. This report preserves the
> stored response classification and the run's historical account, but the
> ledger marks it `LEGACY-UNVERIFIED` and it does not count as probe coverage.
>
> **HONESTY.** The discriminator asks only whether the stored response separates
> write-scope collisions when planning parallel waves. It does not establish
> crank quality, the producer identity, a model-level behavior, or an execution
> outcome. Small N (2) is directional, not statistical (ADR-0011).

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
- The historical run record says dispatch used `codex exec` with `gpt-5.5`.
  No capture manifest binds that producer/config label to the fixture bytes.
- Transcripts remain under `evals/skill-probes/crank/fixtures/` for inspection
  and discriminator regression checks, not reproducible generation replay.

## Stored result — LEGACY-UNVERIFIED (historically INERT)

| Arm | present / usable | rate | what the agent did |
|-----|------------------|------|--------------------|
| control (no crank) | 2 / 2 | 1.0 | e.g. `Wave 1: bead-A, bead-B, bead-D` / `Wave 2: bead-C` — B and C separated |
| treatment (crank prelude injected) | 2 / 2 | 1.0 | B and C separated |

**Historical classification: `INERT`** because the stored treatment rate 1.0
is not greater than the stored control rate 1.0. This is not a current
manifest-backed verdict.

## What the stored classification means

Both archived control responses and both archived treatment responses placed B
and C in different waves. That response set contains no discriminator
separation. It does not show that `gpt-5.5`, frontier models generally, or a
verified producer would behave the same way, because producer/config identity
was not capture-bound.

A new claim about producer strength, task difficulty, or crank's marginal
behavior requires a capture-manifest-backed run. The historical null can help
shape that probe, but cannot decide whether to keep or cull the skill.

## Coverage status

This dated report satisfied the earlier `age-e508.1` record-keeping acceptance.
Under the current evidence contract it remains historical context only. The
`LEGACY-UNVERIFIED` ledger row is intentionally excluded from measured coverage
until a capture-manifest-backed run records a current verdict.
