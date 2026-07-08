# Skill behavioral probe — graphify (CALIBRATION), 2026-07-08

> **HONESTY.** A probe measures **BEHAVIOR-CHANGE, not quality-uplift.** This run
> answers only: did loading the graphify "use the graph before grep" guidance
> change which tool the agent actually reached for FIRST? It does not claim
> graphify is good or bad. Small N (2) is **directional, not statistical**
> (ADR-0011 discipline — do not overclaim).

## Purpose: calibrate the instrument against a known-INERT result

This is the harness's **calibration** run. The `age-e508.1` acceptance requires
that, on the 2026-06-30 graphify scenario, the harness **reproduces the INERT
verdict** — i.e. that the ruler reads a known measurement correctly before we
trust it on new skills.

The original measurement (memory
`doc-instruction-to-use-tool-before-grep-is-inert`): on 2026-06-30, after
landing a `/research` "Tier-1b" rule saying *"when graphify is installed, query
structure via explain/path/query BEFORE broad grep,"* a controlled A/B found:

- **Control** (structural question, no guidance): grep-first, never used graphify.
- **Treatment** (SAME question + the Tier-1b instruction handed verbatim,
  graphify installed + a graph present): **also grep-first, 0/2 used the tool.**
- Verdict: **INERT.** Documentary acceptance ≠ behavioral acceptance.

## Method

- Probe: `evals/skill-probes/graphify-tool-preference/`
- Arms differ **only** by `treatment-prelude.md` (the Tier-1b guidance); the
  `question.md` is identical.
- Discriminator (`discriminator.sh`): behavior PRESENT iff the transcript shows a
  graphify structural **action** (`graphify <sub>`, an `mcp__graphify` tool call,
  or a read of `graphify-out/`) **before** any grep/rg. It checks the ACTION, not
  a mention.
- Fixtures are **reconstructed from the documented 06-30 record** (both arms
  grep-first, no graphify call) — this is a regression fixture encoding a known
  outcome to calibrate the classifier, not a verbatim console capture. Run via
  `--replay` (deterministic, zero token cost).

```bash
bash scripts/probe-skill.sh --probe graphify-tool-preference --replay
```

## Result — REPRODUCED

| Arm | present / usable | rate |
|-----|------------------|------|
| control | 0 / 2 | 0.0 |
| treatment | 0 / 2 | 0.0 |

**Verdict: `INERT`** (treatment_rate 0.0 is not > control_rate 0.0). This matches
the documented 2026-06-30 result: the loaded guidance did not change which tool
the agent reached for. Calibration passes — the harness reads the known-INERT
case correctly.

## A discriminator bug the calibration caught (worth recording)

The first discriminator matched the bare token `graphify-out/` — which appeared
in the treatment fixtures' *prose* header describing the environment — and
mis-scored the known-INERT case as `BEHAVIORAL`. That is exactly the failure this
whole bead exists to prevent: **measuring a mention, not an action.** Calibration
against the known result caught it; the discriminator was tightened to count only
real invocations (a command, a tool call, or a file read), and the prose token
was removed from the fixtures. A probe you cannot calibrate is a badge you cannot
trust.
