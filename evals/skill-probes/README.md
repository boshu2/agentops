# Skill behavioral probes (`evals/skill-probes/`)

> **HONESTY (read first).** A probe measures **BEHAVIOR-CHANGE, not
> quality-uplift.** It answers exactly one question: when a skill is **loaded**
> (treatment) vs **not loaded** (control), does the agent actually **DO** the
> thing differently — a tool call made, an artifact produced, a sequence
> followed? It **never** scores whether the text *mentions* the skill, and it
> **never** claims the skill makes output *better*. `BEHAVIORAL` = loading it
> changed what the agent did; `INERT` = it didn't. Small N (default 2–3) is
> **DIRECTIONAL, not statistical.** Do not overclaim (ADR-0011 discipline).

## Why this exists

Skills are half the product, but tier badges are editorial — the only
enforcement was an enum-membership check. On **2026-06-30** a controlled A/B
measured a doc-instruction skill (graphify's "use the graph before grep" rule)
as behaviorally **INERT**: 0/2 treatment agents obeyed it even handed the
instruction verbatim (memory `doc-instruction-to-use-tool-before-grep-is-inert`).
Documentary acceptance ≠ behavioral acceptance. A catalog whose product-tier
badges are unmeasured is noise wearing a product badge. This harness measures the
behavior separately.

## What a probe is

A directory `evals/skill-probes/<id>/`:

| File | Role |
|------|------|
| `probe.json` | metadata: id, skill, reps, the behavior, the discriminator |
| `question.md` | the scenario question — **IDENTICAL for both arms** |
| `treatment-prelude.md` | the skill guidance injected **only** in the treatment arm (the sole variable) |
| `discriminator.sh` | a **deterministic** behavioral check over one transcript: exit `0`=PRESENT, `1`=ABSENT, `2`=infra. **Checks the ACTION, never a mention.** |
| `fixtures/` | recorded transcripts `control-<n>.txt` / `treatment-<n>.txt` — used by `--replay` for deterministic calibration + a committed, reproducible evidence run |

The **only** difference between arms is the prelude: control prompt = the
question; treatment prompt = the prelude + the same question. This isolates "did
loading the skill change behavior" from everything else.

## Running

```bash
# Deterministic replay over committed fixtures (calibration + CI):
bash scripts/probe-skill.sh --probe graphify-tool-preference --replay

# Live A/B (dispatches codex exec — the sanctioned headless path; NEVER claude -p):
bash scripts/probe-skill.sh --probe crank --live --capture --reps 2 --output out.json
```

Verdict: `BEHAVIORAL` iff `treatment_rate > control_rate`; `INERT` iff not;
`UNMEASURED` iff no usable treatment reps.

## The frontier-aces-it caveat (measured, honest)

A **frontier** producer often already does the right thing, so a skill's marginal
behavioral effect on it is nil → `INERT` even for a genuinely useful skill (see
the `crank` evidence: gpt-5.5 separated the write-scope-colliding beads in both
arms). This is the same lesson as the membrane eval (`membrane-eval-too-easy`):
to surface a skill's behavioral value you need a **weaker producer** (e.g.
`--model gpt-5-mini`, the local llama) or a **harder task**. `INERT` on a frontier
model is a real finding, not a failure — it says "at this altitude, on this model,
the doc-only value is unmeasurable," exactly the honesty the badge needs.

## Spine first, ratchet does the rest

This ships the workflow-spine start set — `crank` measured, `graphify` calibrated
— not all 100+ skills. The advisory gate `skill.probe-coverage`
(`scripts/check-skill-probe-coverage.sh`) NAMES every product-/judgment-tier
skill still lacking a probe result; that ratchet drives coverage over time. The
gate is **advisory-first** (warn, never block) until the spine is covered and the
flip is made deliberately.

## Evidence lands dated

Every run writes a dated evidence file under `docs/evals/` and a row in the
**Behavioral Probe Ledger (MEASURED)** of `skills/SKILL-TIERS.md`.
