# Skill behavioral probe — graphify (CALIBRATION), 2026-07-08

> **LEGACY EVIDENCE STATUS.** These fixtures were reconstructed from a written
> 2026-06-30 account; they are not captured transcripts and have no capture-time
> hashes or producer/config manifest. The current fail-closed harness cannot
> provenance-verify or reproduce the original run. This file preserves the
> historical account and a discriminator regression case, while the ledger
> marks the probe `LEGACY-UNVERIFIED` and excludes it from measured coverage.
>
> **HONESTY.** The stored fixture test asks only whether the classifier detects
> a graph action before grep. It does not establish graphify quality or provide
> fresh behavioral evidence. Small N (2) in the historical account is
> directional, not statistical (ADR-0011).

## Purpose: preserve a historical classification and classifier regression

The reconstructed fixtures encode the documented 2026-06-30 no-action shape so
the discriminator can be regression-tested against it. Classifying those
authored fixtures does not reproduce the original run or verify its producer.

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
  grep-first, no graphify call). They are an authored classifier regression
  case, not verbatim console captures or replayable measurement evidence.

## Stored regression result — LEGACY-UNVERIFIED

| Arm | present / usable | rate |
|-----|------------------|------|
| control | 0 / 2 | 0.0 |
| treatment | 0 / 2 | 0.0 |

**Historical classification: `INERT`** because the reconstructed treatment
rate 0.0 is not greater than the reconstructed control rate 0.0. The fixture
result is consistent with the documented 2026-06-30 account, but it is not a
current manifest-backed verdict or independent evidence of that run.

## A discriminator bug the calibration caught (worth recording)

The first discriminator matched the bare token `graphify-out/` — which appeared
in the treatment fixtures' *prose* header describing the environment — and
mis-scored the authored no-action case as `BEHAVIORAL`. That is exactly the
classifier failure this regression fixture can test: **measuring a mention, not
an action.** The discriminator was tightened to count only real invocations (a
command, a tool call, or a file read), and the prose token was removed from the
fixtures. This validates the narrow classifier regression; it does not promote
the reconstructed fixture into provenance-verified behavioral evidence.
