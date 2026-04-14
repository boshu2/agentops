# Project Memory

## Last Session

## Architecture

- **Harness Reuse**: Long-running RPI improvements should extend existing phase/result/handoff artifacts instead of creating a second run ledger (source: `.agents/learnings/2026-03-28-last-week-commits.md`)
- **Trust-Gated Context**: Startup quality improves when canonical findings and planning rules outrank raw notes and packet families stay experimental until health is proven (source: `.agents/learnings/2026-04-01-session-intelligence-trust-gates.md`)

## Process

- **Proof Before Close**: Child bead closeout should record scoped files or proof artifacts so closure-integrity stays replayable (source: `.agents/learnings/2026-03-28-last-week-commits.md`)
- **Remote CI Counts**: Release completion requires the first green remote validate run, not only the local release gate (source: `.agents/learnings/2026-03-22-v2.29.0-full-release.md`)

## Debugging

- **Tracker Skew First**: When `bd` probes fail with schema errors, check `bd` version and migrations before blaming repo code (source: `.agents/learnings/2026-03-24-codex-hookless-followup.md`)

## Patterns

- **CLI-Owned Lifecycle**: Codex skills should call `ao codex ensure-start` / `ensure-stop` instead of parsing state files themselves (source: `.agents/learnings/2026-03-24-codex-hookless-followup.md`)

## Key Lessons

- **Audit Parser Reality**: Mechanical auditors must parse `File:` prose, anchors, and examples correctly or they create noisy false failures (source: `.agents/learnings/2026-03-28-last-week-commits.md`)
- **Output Mode Orthogonality**: `--json` must preserve normal command side effects and change only serialization (source: `.agents/learnings/2026-04-10-output-modes-must-not-change-command-side-effects.md`)
- **Pair Command Refactors With Tests**: Production command refactors under `cli/cmd/ao/` should ship with direct test diffs so the command/test-pairing gate is designed for, not rediscovered at push time (source: `.agents/learnings/2026-04-14-command-refactors-need-paired-tests.md`)
- **Scrub RPI Runtime Env in Raw Go Checks**: Raw validation of `internal/rpi` is not trustworthy on this machine unless `AGENTOPS_RPI_RUNTIME` is explicitly scrubbed first (source: `.agents/learnings/2026-04-14-scrub-rpi-runtime-from-raw-validation.md`)
