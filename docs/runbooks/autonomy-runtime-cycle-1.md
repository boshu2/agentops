# Autonomy Runtime Cycle-1 Runbook

**Date:** 2026-05-20
**Scope:** Safe activation of AgentOps cycle-1 in-session autonomy surfaces: RPI
phased runs and `ao evolve` supervisor loops.

> **3.0 note:** the daemon-backed job-execution lane this runbook originally
> covered (`ao daemon jobs submit`, the `agentopsd` control plane) was **removed**
> in the AgentOps 3.0 rearchitecture — AgentOps is in-session only and ships no
> daemon of its own (see
> [ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md)). The in-session
> loop (`ao rpi`, `ao evolve`) runs end-to-end in a plain session; to run it
> unattended out of session, dispatch it on the **reference orchestration substrate** (a
> controller agent dispatches ready beads to worker panes that run `ao rpi`). This
> runbook now covers the in-session surfaces only.

## Activation

1. Pull latest `main` and sync beads (`git fetch --prune origin && git switch main && git reset --hard origin/main`; then `bd sync` if used in this clone).
2. Run baseline quality gates:
   - `cd cli && make build` (produces `cli/bin/ao`)
   - `cd cli && go test ./internal/rpi/...`
3. Verify required specs and index references exist:
   - `docs/contracts/rpi-run-registry.md`
   - `docs/documentation-index.md` references the RPI contracts.
4. Execute the target RPI run in dry/safe mode first and confirm no regressions in run artifacts and the bead ledger:
   - `ao rpi phased --dry-run "<goal>"` for one explicit run
   - `ao evolve --dry-run --max-cycles 1 "<goal>"` for the supervisor loop surface
   - `ao evolve --dream-only` for knowledge-only cycles
   - `ao rpi status` to inspect produced artifacts.

## Feature Flags

Cycle-1 runtime controls are in-session command flags:

- `ao evolve --supervisor=true` is the default supervised loop posture.
- `ao evolve --max-cycles <n>` bounds autonomous iterations.
- `ao evolve --dream-first` / `--dream-only` limits work to knowledge compounding before or instead of code cycles.
- `ao rpi phased --runtime <name>` and `--runtime-cmd <cmd>` select the worker runtime for phased execution.

Activation rule:

- Start with `--dry-run` and `--max-cycles 1`.
- For unattended out-of-session runs, dispatch the loop on the reference orchestration substrate only after the local RPI and evolve dry runs are clean.
- Keep manual merge/review in the loop until the release-readiness contract says otherwise.

## Rollback Trigger

Rollback immediately when any of the following occurs:

1. RPI run determinism / replay errors appear (deterministic-mode smoke regressions, run-ledger inconsistencies).
2. Quality gate behavior deviates from the expected non-bypassable flow (`scripts/pre-push-gate.sh`, `scripts/ci-local-release.sh`, `ao goals validate`).
3. RPI run artifacts or bead evidence become incomplete for ratchet-relevant steps (`ao ratchet check`, `ao ratchet status`).

Rollback steps:

1. Stop using the new opt-in flag / pool input.
2. Re-run with the legacy single-actor path.
3. Capture failing artifacts and create follow-up bead(s) with references (`bd create --title ... --notes ...`).

## Evidence Verification

Verify lifecycle and orchestration evidence via:

1. RPI / bead events include the relevant lifecycle markers and payload fields.
   - RPI phased state and artifacts live under `.agents/rpi/`.
   - Out-of-session job events (when dispatched on the substrate) are inspectable through the substrate's own event surface, not an AgentOps daemon.
2. RPI run ledger / bead store contains attempt records for affected beads (`bd show <id>`, `ao rpi status`).
3. RPI tests pass:
   - `cd cli && go test ./internal/rpi/...`
4. Autonomy smoke remains green:
   - `cd cli && go test ./cmd/ao/... -run 'Test.*(RPI|Evolve)'`
   - `bash scripts/ci-local-release.sh --fast --jobs 4` before promoting the activation.

## Operator Notes

- This runbook covers the in-session loop only; out-of-session/fleet orchestration is delegated to the reference orchestration substrate, not an AgentOps daemon.
- This runbook does not relax validation boundaries (`/vibe`, `/council`, `ao goals validate`, gate scripts all still apply).
- This runbook is cycle-1 only; fleet/autopilot runtime expansion is a follow-on cycle.

## AgentOps Runtime Surface

| Concern | AgentOps surface |
|---|---|
| CLI | `ao <cmd>` |
| One bounded autonomy run | `ao rpi phased "<goal>"` |
| Supervised loop | `ao evolve --max-cycles <n> "<goal>"` or `ao rpi loop` |
| Work claim | `bd update <id> --claim` |
| Validation | `/vibe`, `/council`, `ao goals validate`, `scripts/ci-local-release.sh` |
| Knowledge extraction | `ao forge` |
| Contracts | `docs/contracts/*` + `docs/documentation-index.md` |
| Runtime packages | `cli/internal/rpi/`, `cli/cmd/ao/` |
