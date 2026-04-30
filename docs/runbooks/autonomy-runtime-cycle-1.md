# Autonomy Runtime Cycle-1 Runbook

> **Status:** Ported from olympus 2026-04-29. Commands/paths adapted for agentops. Where olympus terms have no agentops equivalent, TODO callouts mark the gap.

**Date:** 2026-02-12 (originally authored in olympus)
**Scope:** Safe activation of cycle-1 actor runtime foundations (spec + runtime scaffold + temporal actor-context threading).

> **TODO: olympus actor-runtime / temporal workflow equivalent in agentops?**
> The cycle-1 work in olympus targeted a Temporal-based quest workflow with `ActorPool` threading. agentops's autonomy surface is `ao rpi` / `ao evolve` (supervisor mode + RPI phased runs). The structure of this runbook is preserved so the activation/rollback shape can be reused; concrete spec paths and test names will need to be re-pointed at the agentops equivalents (likely `cli/internal/rpi/...` and `ao rpi serve` / `ao daemon run` surfaces) once the analog is implemented.

## Activation

1. Pull latest `main` and sync beads (`git fetch --prune origin && git switch main && git reset --hard origin/main`; then `bd sync` if used in this clone).
2. Run baseline quality gates:
   - `cd cli && make build` (produces `cli/bin/ao`)
   - `cd cli && go test ./internal/rpi/...`
   - `cd cli && go test ./internal/daemon/...`

   > **TODO: olympus `internal/runtime/` / `internal/temporal/` equivalent in agentops?**
   > In olympus the runtime + temporal packages are the test surface. In agentops the closest equivalents live under `cli/internal/rpi/` and `cli/internal/daemon/`. Confirm exact package paths before executing.
3. Verify required specs and index references exist:
   - `docs/contracts/agentops-daemon.md` (daemon contract — closest analog to olympus's `docs/specs/autonomy-runtime.md`)
   - `docs/documentation-index.md` references the daemon contract and any new autonomy spec.

   > **TODO: olympus `docs/specs/autonomy-runtime.md` equivalent in agentops?**
   > agentops doesn't ship a `docs/specs/` tree; contracts live under `docs/contracts/` and are listed in `docs/documentation-index.md`. Replace the precise filename when an autonomy-runtime spec lands.
4. Execute the target RPI run in dry/safe mode first and confirm no regressions in run artifacts and the bead ledger:
   - `ao rpi phased --dry-run --goal <goal>` (or equivalent: `ao evolve --dream-only` for knowledge-only cycles)
   - `ao rpi status` to inspect produced artifacts.

## Feature Flags

Cycle-1 runtime fields are additive and optional (kept here as the original olympus shape; map to agentops opt-in flags as the analog ships):

- `QuestWorkflowInput.ActorPool`
- `HeroWaveInput.ActorPool`
- `DemigodActivityInput.ActorPool`

> **TODO: olympus `ActorPool` / `HeroWaveInput` / `DemigodActivityInput` equivalent in agentops?**
> agentops's RPI lifecycle does not have a Temporal workflow input surface. The analogous "additive opt-in" knob is likely an `ao rpi`/`ao evolve` flag (e.g. `--supervisor`, `--ralph`, or a future `--actor-pool`). Replace these field names once the agentops equivalent is defined.

Activation rule:

- Start with the legacy path (no opt-in flag set).
- Introduce the new opt-in flag only in controlled test runs.

## Rollback Trigger

Rollback immediately when any of the following occurs:

1. RPI run determinism / replay errors appear (deterministic-mode smoke regressions, run-ledger inconsistencies).

   > **TODO: olympus Temporal workflow determinism/replay equivalent in agentops?**
   > Temporal-style replay determinism does not have a 1:1 analog in agentops. The closest signal is RPI phased-run reproducibility plus daemon job ledger integrity (`ao daemon jobs list`, `ao daemon events tail`).
2. Quality gate behavior deviates from the expected non-bypassable flow (`scripts/pre-push-gate.sh`, `scripts/ci-local-release.sh`, `ao goals validate`).
3. RPI run artifacts or bead evidence become incomplete for ratchet-relevant steps (`ao ratchet check`, `ao ratchet status`).

Rollback steps:

1. Stop using the new opt-in flag / pool input.
2. Re-run with the legacy single-actor path.
3. Capture failing artifacts and create follow-up bead(s) with references (`bd create --title ... --notes ...`).

## Evidence Verification

Verify lifecycle and orchestration evidence via:

1. RPI / bead events include the relevant lifecycle markers and payload fields.

   > **TODO: olympus quest-event types (`hero_hunt`, `hero_embark`, `hero_ratchet`) equivalent in agentops?**
   > agentops emits daemon events (`ao daemon events tail`) and RPI phased lifecycle markers; there is no direct rename map for the olympus HERO event types. Re-derive the watchlist from `cli/internal/daemon/events` once the cycle-1 analog ships.
2. RPI run ledger / bead store contains attempt records for affected beads (`bd show <id>`, `ao rpi status`).
3. Daemon / RPI tests pass:
   - `cd cli && go test ./internal/rpi/... -run TestPhasedRunActorThreading` (rename when the agentops test exists)

   > **TODO: olympus `TestQuestWorkflowActorPoolThreading` equivalent in agentops?**
4. Autonomy smoke remains green:
   - `cd cli && go test ./cmd/ao/... -run TestAutopilotSmoke` (or the agentops smoke equivalent in `cli/cmd/ao/`).

   > **TODO: olympus `TestAutopilotSmoke` equivalent in agentops?**

## Operator Notes

- This runbook does not enable daemon/fleet orchestration beyond what the cycle-1 opt-in flag exposes.
- This runbook does not relax validation boundaries (`/vibe`, `/council`, `ao goals validate`, gate scripts all still apply).
- This runbook is cycle-1 only; fleet/autopilot runtime expansion is a follow-on cycle.

## Adaptation map (olympus → agentops)

| Olympus | agentops |
|---|---|
| `ol <cmd>` | `ao <cmd>` |
| `zeus step` / `zeus run --quest <id>` | `ao rpi phased` / `ao rpi loop` (per-cycle vs. supervised loop) |
| `apollo claim <bead>` | `bd update --claim <id>` (bd-flavored claim) |
| `athena validate <bead>` | `/vibe`, `/council`, or `ao goals validate` (per package mapping) |
| `hephaestus extract` | `ao forge` (transcript / batch / markdown subcommands) |
| `docs/specs/*` | `docs/contracts/*` + `docs/documentation-index.md` |
| `internal/runtime/`, `internal/temporal/` | `cli/internal/rpi/`, `cli/internal/daemon/` (closest analogs; verify per package) |
