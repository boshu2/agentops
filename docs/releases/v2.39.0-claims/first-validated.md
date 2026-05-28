# AOP-CLAIM-README-FIRST-VALIDATED — evidence (v2.39.0)

**Claim location:** README.md quickstart section that promises a
5-minute install → first-validated-flow experience.

**Claim summary:** A new user can install AgentOps and reach a
validated first artifact within 5 minutes.

## Repo surfaces that demonstrate it

- `scripts/install.sh` — the install path.
- `tests/install/test-install-smoke.sh` — structural syntax check.
- `tests/install/test-five-minute-journey.sh` — 7-checkpoint journey
  test with a 300-second hard floor. Checkpoints: install.sh syntax,
  `ao version`, rpi skill present, quickstart skill present, `ao
  quickstart --help` reachable, skill install dir reachable,
  `.agents/rpi` artifact slot.
- `.github/workflows/install-e2e.yml` — nightly + manual workflow
  that runs the install end-to-end on ubuntu-latest and macos-latest.

## Verification surface

Local: `bash tests/install/test-five-minute-journey.sh` — fails if
total wall-clock exceeds 300 seconds or any checkpoint regresses.
CI: install-e2e.yml runs the same script.

Current measurement (HEAD): 7/7 checkpoints pass in ~1 second.

## Why this is enough

The 5-minute promise is measurable. The journey test pins both the
shape and the time budget. A regression that breaks any step or
exceeds the floor surfaces in CI.

## Companion bead

soc-dec2.1 (PG1).
