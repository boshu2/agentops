# Autonomy Runtime Cycle-1 Runbook

**Date:** 2026-05-20
**Scope:** Safe activation of AgentOps cycle-1 autonomy surfaces: RPI phased runs,
`ao evolve` supervisor loops, and daemon-backed job execution.

## Activation

1. Pull latest `main` and sync beads (`git fetch --prune origin && git switch main && git reset --hard origin/main`; then `bd sync` if used in this clone).
2. Run baseline quality gates:
   - `cd cli && make build` (produces `cli/bin/ao`)
   - `cd cli && go test ./internal/rpi/...`
   - `cd cli && go test ./internal/daemon/...`
3. Verify required specs and index references exist:
   - `docs/contracts/agentops-daemon.md`
   - `docs/contracts/agentopsd-control-plane.md`
   - `docs/contracts/rpi-run-registry.md`
   - `docs/documentation-index.md` references the daemon and RPI contracts.
4. Execute the target RPI run in dry/safe mode first and confirm no regressions in run artifacts and the bead ledger:
   - `ao rpi phased --dry-run "<goal>"` for one explicit run
   - `ao evolve --dry-run --max-cycles 1 "<goal>"` for the supervisor loop surface
   - `ao evolve --dream-only` for knowledge-only cycles
   - `ao rpi status` to inspect produced artifacts.

## Feature Flags

Cycle-1 runtime controls are command flags and daemon job payloads:

- `ao evolve --supervisor=true` is the default supervised loop posture.
- `ao evolve --max-cycles <n>` bounds autonomous iterations.
- `ao evolve --dream-first` / `--dream-only` limits work to knowledge compounding before or instead of code cycles.
- `ao rpi phased --runtime <name>` and `--runtime-cmd <cmd>` select the worker runtime for phased execution.
- `ao daemon jobs submit --type <type> --payload @payload.json` activates daemon-backed work explicitly.

Activation rule:

- Start with `--dry-run` and `--max-cycles 1`.
- Introduce daemon-backed job submission only after the local RPI and evolve dry runs are clean.
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
   - Daemon jobs and events are inspectable with `ao daemon jobs list` and `ao daemon events show`.
2. RPI run ledger / bead store contains attempt records for affected beads (`bd show <id>`, `ao rpi status`).
3. Daemon / RPI tests pass:
   - `cd cli && go test ./internal/rpi/...`
   - `cd cli && go test ./internal/daemon/...`
4. Autonomy smoke remains green:
   - `cd cli && go test ./cmd/ao/... -run 'Test.*(RPI|Evolve|Daemon|Factory)'`
   - `bash scripts/ci-local-release.sh --fast --jobs 4` before promoting the activation.

## Operator Notes

- This runbook does not enable daemon/fleet orchestration beyond explicit `ao daemon jobs submit` activation.
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
| Runtime packages | `cli/internal/rpi/`, `cli/internal/daemon/`, `cli/cmd/ao/` |
